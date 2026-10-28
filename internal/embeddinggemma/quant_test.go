package embeddinggemma

import (
	"context"
	"encoding/binary"
	"github.com/rcarmo/memento/internal/simd"
	"math"
	"math/rand/v2"
	"testing"
)

// Reference reads each coefficient by its logical bit plane, with float64
// arithmetic; it does not call production decoding/scales/half conversion.
func referenceHalf(u uint16) float64 {
	sign := 1.0
	if u&0x8000 != 0 {
		sign = -1
	}
	e := int(u >> 10 & 31)
	m := float64(u & 1023)
	if e == 0 {
		return sign * math.Ldexp(m, -24)
	}
	if e == 31 {
		if m == 0 {
			return sign * math.Inf(1)
		}
		return math.NaN()
	}
	return sign * math.Ldexp(1+m/1024, e-15)
}
func referenceRow(raw []byte, typ Type, n int) []float64 {
	out := make([]float64, n)
	block, size, _ := typ.layout()
	for i := range out {
		r := raw[(i/block)*size:]
		j := i % block
		switch typ {
		case F32:
			out[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(r[j*4:])))
		case F16:
			out[i] = referenceHalf(binary.LittleEndian.Uint16(r[j*2:]))
		case BF16:
			out[i] = float64(math.Float32frombits(uint32(binary.LittleEndian.Uint16(r[j*2:])) << 16))
		case Q8_0:
			out[i] = referenceHalf(binary.LittleEndian.Uint16(r)) * float64(int8(r[j+2]))
		case Q4_K, Q5_K:
			g := j / 32
			idx := j % 32
			var scale, minval uint8
			if g < 4 {
				scale = r[4+g] & 63
				minval = r[8+g] & 63
			} else {
				scale = (r[8+g] & 15) + ((r[g] >> 6) << 4)
				minval = (r[8+g] >> 4) + ((r[4+g] >> 6) << 4)
			}
			off := 16
			if typ == Q5_K {
				off = 48
			}
			v := (r[off+(g/2)*32+idx] >> uint((g%2)*4)) & 15
			if typ == Q5_K {
				v |= ((r[16+idx] >> g) & 1) << 4
			}
			out[i] = referenceHalf(binary.LittleEndian.Uint16(r))*float64(scale)*float64(v) - referenceHalf(binary.LittleEndian.Uint16(r[2:]))*float64(minval)
		case Q6_K:
			h := j / 128
			local := j % 128
			g := local / 32
			idx := local % 32
			loIndex := h*64 + idx + (g%2)*32
			low := (r[loIndex] >> uint((g/2)*4)) & 15
			high := (r[128+h*32+idx] >> uint(g*2)) & 3
			sc := int8(r[192+h*8+g*2+idx/16])
			out[i] = referenceHalf(binary.LittleEndian.Uint16(r[208:])) * float64(sc) * float64(int(low|high<<4)-32)
		}
	}
	return out
}
func encodedRows(typ Type, n, rows int) []byte {
	_, size, _ := typ.layout()
	block, _, _ := typ.layout()
	b := make([]byte, n/block*size*rows)
	rng := rand.New(rand.NewPCG(4, 17))
	for i := range b {
		b[i] = byte(rng.Uint32())
	}
	for i := 0; i < len(b); i += size {
		switch typ {
		case F32:
			binary.LittleEndian.PutUint32(b[i:], math.Float32bits(float32(int8(b[i]))/64))
		case F16:
			binary.LittleEndian.PutUint16(b[i:], 0x3400)
		case BF16:
			binary.LittleEndian.PutUint16(b[i:], 0x3e80)
		case Q8_0:
			binary.LittleEndian.PutUint16(b[i:], 0x2000)
		case Q4_K, Q5_K:
			binary.LittleEndian.PutUint16(b[i:], 0x1400)
			binary.LittleEndian.PutUint16(b[i+2:], 0x1000)
		case Q6_K:
			binary.LittleEndian.PutUint16(b[i+208:], 0x1400)
		}
	}
	return b
}
func TestHalfExhaustive(t *testing.T) {
	for u := 0; u < 65536; u++ {
		a := float64(half(uint16(u)))
		b := referenceHalf(uint16(u))
		if math.IsNaN(b) {
			if !math.IsNaN(a) {
				t.Fatal(u, a, b)
			}
		} else if a != b || math.Signbit(a) != math.Signbit(b) {
			t.Fatal(u, a, b)
		}
	}
}
func TestQuantizedRowsAndProjection(t *testing.T) {
	for _, typ := range []Type{F32, F16, BF16, Q8_0, Q4_K, Q5_K, Q6_K} {
		t.Run(typ.String(), func(t *testing.T) {
			const n, rows = 512, 7
			raw := encodedRows(typ, n, rows)
			f := &File{data: raw}
			m := &Matrix{owner: f, tensor: Tensor{Type: typ, Bytes: uint64(len(raw))}, columns: n, rows: rows}
			x := make([]float32, n)
			for i := range x {
				x[i] = float32(math.Sin(float64(i) * .07))
			}
			row := make([]float32, n)
			dst := make([]float32, rows)
			ref := make([]float64, rows)
			for r := 0; r < rows; r++ {
				want := referenceRow(raw[r*len(raw)/rows:(r+1)*len(raw)/rows], typ, n)
				if err := m.RowTo(row, r); err != nil {
					t.Fatal(err)
				}
				for i, v := range row {
					if math.Abs(float64(v)-want[i]) > 1e-6*math.Max(1, math.Abs(want[i])) {
						t.Fatalf("row%d idx%d got%g want%g", r, i, v, want[i])
					}
					ref[r] += want[i] * float64(x[i])
				}
			}
			for _, backend := range []string{"scalar", "sse2", "avx2", "neon"} {
				e, err := simd.New(backend)
				if err != nil {
					continue
				}
				if err = m.ProjectTo(context.Background(), dst, x, row, e); err != nil {
					t.Fatal(err)
				}
				for r, v := range dst {
					if math.Abs(float64(v)-ref[r]) > 2e-5*math.Max(1, math.Abs(ref[r])) {
						t.Fatalf("%s row%d got%g want%g", backend, r, v, ref[r])
					}
				}
				if n := testing.AllocsPerRun(25, func() {
					if err := m.ProjectTo(context.Background(), dst, x, row, e); err != nil {
						panic(err)
					}
				}); n != 0 {
					t.Fatalf("%s allocated%g", backend, n)
				}
			}
			if err := m.RowTo(row, -1); err == nil {
				t.Fatal("negative row accepted")
			}
			if err := m.RowTo(row[:3], 0); err == nil {
				t.Fatal("short destination")
			}
			if err := m.ProjectTo(context.Background(), dst, x, x, scalarEngine()); err == nil {
				t.Fatal("alias accepted")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := m.ProjectTo(ctx, dst, x, row, scalarEngine()); err != context.Canceled {
				t.Fatal(err)
			}
			if err := m.ProjectTo(nil, dst, x, row, scalarEngine()); err == nil {
				t.Fatal("nil context")
			}
			f.data = nil
			if err := m.RowTo(row, 0); err == nil {
				t.Fatal("closed row")
			}
			if err := m.ProjectTo(context.Background(), dst, x, row, scalarEngine()); err == nil {
				t.Fatal("closed project")
			}
		})
	}
}
func TestDecodeErrors(t *testing.T) {
	for _, typ := range []Type{F32, F16, BF16, Q8_0, Q4_K, Q5_K, Q6_K, 99} {
		if err := decodeRow(make([]float32, 257), nil, typ); err == nil {
			t.Fatal(typ)
		}
	}
}
func BenchmarkQuantProjection(b *testing.B) {
	for _, typ := range []Type{Q4_K, Q5_K, Q6_K, Q8_0} {
		for _, backend := range []string{"scalar", "sse2", "avx2", "neon"} {
			e, err := simd.New(backend)
			if err != nil {
				continue
			}
			b.Run(typ.String()+"/"+backend, func(b *testing.B) {
				const n, rows = 512, 512
				raw := encodedRows(typ, n, rows)
				m := &Matrix{owner: &File{data: raw}, tensor: Tensor{Type: typ, Bytes: uint64(len(raw))}, columns: n, rows: rows}
				x := make([]float32, n)
				for i := range x {
					x[i] = float32(math.Sin(float64(i)))
				}
				scratch := make([]float32, n)
				dst := make([]float32, rows)
				ctx := context.Background()
				b.ReportAllocs()
				b.SetBytes(int64(len(raw)))
				b.ResetTimer()
				for b.Loop() {
					if err := m.ProjectTo(ctx, dst, x, scratch, e); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func scalarEngine() simd.Engine { e, _ := simd.New("scalar"); return e }
