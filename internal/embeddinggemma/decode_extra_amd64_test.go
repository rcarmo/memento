//go:build amd64

package embeddinggemma

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/rcarmo/memento/internal/simd"
)

func extraDecodeColumns(typ Type) []int {
	if typ == Q5K {
		return []int{256, 512, 2048}
	}
	return []int{32, 512, 2048}
}

// Both an independent logical-plane FP64 reference and the scalar FP32
// operation contract are checked. Half*63*31 and half*int8 products fit in
// FP32's significand, so finite referenceRow values need only final rounding.
func checkExtraDecode(t *testing.T, raw []byte, typ Type, dst []float32, backend simd.Backend) {
	t.Helper()
	scalar := make([]float32, len(dst))
	if err := decodeRow(scalar, raw, typ); err != nil {
		t.Fatal(err)
	}
	ref := referenceRow(raw, typ, len(dst))
	if err := decodeTileRow(dst, raw, typ, backend); err != nil {
		t.Fatal(err)
	}
	for i, v := range dst {
		want := scalar[i]
		if math.IsNaN(float64(want)) {
			if !math.IsNaN(float64(v)) {
				t.Fatalf("coefficient %d: %g, want NaN", i, v)
			}
		} else if v != want || math.Signbit(float64(v)) != math.Signbit(float64(want)) {
			t.Fatalf("coefficient %d: SIMD %g, scalar %g", i, v, want)
		}
		r := float32(ref[i])
		if math.IsNaN(ref[i]) {
			if !math.IsNaN(float64(v)) {
				t.Fatalf("reference coefficient %d: expected NaN", i)
			}
		} else if v != r {
			t.Fatalf("reference coefficient %d: SIMD %g, independent %g", i, v, r)
		}
	}
}

func TestExtraDecodeRows(t *testing.T) {
	halves := []uint16{0, 0x8000, 1, 0x83ff, 0x0401, 0x1400, 0x3555, 0xb955, 0x7bff, 0xfbff, 0x7c00, 0xfc00, 0x7e01, 0x7c01}
	for _, name := range []string{"scalar", "sse2", "avx2"} {
		e, err := simd.New(name)
		if err != nil {
			continue
		}
		for _, typ := range []Type{Q5K, Q8_0} {
			for _, cols := range extraDecodeColumns(typ) {
				t.Run(fmt.Sprintf("%s/%s/%d", name, typ, cols), func(t *testing.T) {
					block, size, _ := typ.layout()
					storage := make([]float32, cols+2)
					storage[0], storage[cols+1] = 12345, -12345
					// Three independent rows, with different halves in every block.
					for row := 0; row < 3; row++ {
						raw := encodedRows(typ, cols, 1)
						for pass := range halves {
							for b := 0; b < cols/block; b++ {
								r := raw[b*size:]
								binary.LittleEndian.PutUint16(r, halves[(pass+b+row)%len(halves)])
								if typ == Q5K {
									binary.LittleEndian.PutUint16(r[2:], halves[(pass+3*b+row+5)%len(halves)])
									for i := 0; i < 12; i++ {
										r[4+i] = byte(pass*59 + i*47 + b*31 + row*97)
									}
									// Every high-plane position and group varies independently.
									for i := 0; i < 32; i++ {
										r[16+i] = byte((i*73 + b*19 + row*31 + pass*7) & 255)
									}
								} else {
									// Signed extrema and zero at every lane over successive passes.
									for i := 0; i < 32; i++ {
										r[2+i] = byte(i*37 + pass*17 + b*11 + row*29)
									}
								}
							}
							checkExtraDecode(t, raw, typ, storage[1:cols+1], e.Backend())
							if storage[0] != 12345 || storage[cols+1] != -12345 {
								t.Fatal("destination canary overwritten")
							}
						}
					}
				})
			}
		}
	}
}

func TestExtraDecodeHighPlanes(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	for _, cols := range extraDecodeColumns(Q5K) {
		raw := encodedRows(Q5K, cols, 1)
		for pattern := 0; pattern < 256; pattern++ {
			for b := 0; b < cols/256; b++ {
				for i := 0; i < 32; i++ {
					raw[b*176+16+i] = byte(pattern) ^ byte(i*13+b*7)
				}
			}
			checkExtraDecode(t, raw, Q5K, make([]float32, cols), simd.AVX2)
		}
	}
}

func TestExtraDecodeValidationAndAllocations(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	for _, typ := range []Type{Q5K, Q8_0} {
		block, size, _ := typ.layout()
		raw, dst := encodedRows(typ, block, 1), make([]float32, block)
		for _, bad := range []struct {
			dst []float32
			raw []byte
		}{
			{dst[:block-1], raw}, {dst, raw[:size-1]}, {dst, append(append([]byte{}, raw...), 0)}, {nil, raw}, {dst, nil},
		} {
			if err := decodeTileRow(bad.dst, bad.raw, typ, simd.AVX2); err == nil {
				t.Fatal("invalid row accepted", typ)
			}
		}
		if err := decodeTileRow(nil, nil, typ, simd.AVX2); err != nil {
			t.Fatal(err)
		}
		if n := testing.AllocsPerRun(100, func() {
			if err := decodeTileRow(dst, raw, typ, simd.AVX2); err != nil {
				panic(err)
			}
		}); n != 0 {
			t.Fatalf("%s allocated %g", typ, n)
		}
	}
	// Leaf zero-block handling must not dereference either pointer.
	q5DecodeAVX2(nil, nil, 0, nil)
	q8DecodeAVX2(nil, nil, 0, nil)
}

func BenchmarkExtraDecode(b *testing.B) {
	for _, typ := range []Type{Q5K, Q8_0} {
		for _, cols := range extraDecodeColumns(typ) {
			for _, name := range []string{"scalar", "avx2"} {
				e, err := simd.New(name)
				if err != nil {
					continue
				}
				b.Run(fmt.Sprintf("%s/%d/%s", typ, cols, name), func(b *testing.B) {
					raw, dst := encodedRows(typ, cols, 1), make([]float32, cols)
					b.ReportAllocs()
					b.SetBytes(int64(len(raw)))
					b.ResetTimer()
					for b.Loop() {
						if err := decodeTileRow(dst, raw, typ, e.Backend()); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}
