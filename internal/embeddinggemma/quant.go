package embeddinggemma

// GGUF Q4_K/Q5_K/Q6_K layouts adapted from rcarmo/go-pherence
// loader/gguf/quant_matrix.go at 2c283159 (MIT, Copyright 2026 Rui Carmo).
// Unlike a full dequantized model, one bounded row scratch is caller-owned.
import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/rcarmo/memento/internal/simd"
	"math"
	"unsafe"
)

type Matrix struct {
	owner         *File
	tensor        Tensor
	columns, rows int
}

func (m *Matrix) Shape() (rows, columns int) { return m.rows, m.columns }
func (m *Matrix) Type() Type                 { return m.tensor.Type }

// RowTo decodes one immutable mapped row into a caller-owned buffer.
func (m *Matrix) RowTo(dst []float32, row int) error {
	if len(dst) != m.columns || row < 0 || row >= m.rows {
		return errors.New("embeddinggemma: invalid row or destination")
	}
	m.owner.mu.RLock()
	defer m.owner.mu.RUnlock()
	if m.owner.data == nil {
		return errors.New("embeddinggemma: model closed")
	}
	return decodeRow(dst, m.rawRow(row), m.tensor.Type)
}
func (m *Matrix) rawRow(row int) []byte {
	n := m.tensor.Bytes / uint64(m.rows)
	start := m.owner.dataOffset + m.tensor.Offset + uint64(row)*n
	return m.owner.data[start : start+n]
}

// ProjectTo evaluates W*x without materializing W in float32. Inputs, output
// and scratch must not overlap. The caller owns scratch and concurrent calls
// require separate workspaces. Cancellation is checked between output rows.
func (m *Matrix) ProjectTo(ctx context.Context, dst, x, scratch []float32, engine simd.Engine) error {
	if len(dst) != m.rows || len(x) != m.columns || len(scratch) < m.columns {
		return fmt.Errorf("embeddinggemma: projection shape mismatch (%d,%d)", m.rows, m.columns)
	}
	scratch = scratch[:m.columns]
	if overlaps(dst, x) || overlaps(dst, scratch) || overlaps(x, scratch) {
		return errors.New("embeddinggemma: overlapping projection buffers")
	}
	if ctx == nil {
		return errors.New("embeddinggemma: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.owner.mu.RLock()
	defer m.owner.mu.RUnlock()
	if m.owner.data == nil {
		return errors.New("embeddinggemma: model closed")
	}
	for row := 0; row < m.rows; row++ {
		if row&15 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if m.tensor.Type == Q4_K {
			if value, ok := fusedQ4Dot(m.rawRow(row), x, engine.Backend()); ok {
				dst[row] = value
				continue
			}
		}
		if err := decodeRow(scratch, m.rawRow(row), m.tensor.Type); err != nil {
			return err
		}
		value, err := engine.Dot(scratch, x)
		if err != nil {
			return err
		}
		dst[row] = value
	}
	return ctx.Err()
}
func overlaps(a, b []float32) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	as := uintptr(unsafe.Pointer(&a[0]))
	bs := uintptr(unsafe.Pointer(&b[0]))
	return as < bs+uintptr(len(b))*4 && bs < as+uintptr(len(a))*4
}
func half(u uint16) float32 {
	sign := uint32(u&0x8000) << 16
	e := (u >> 10) & 31
	mant := uint32(u & 1023)
	if e == 31 {
		return math.Float32frombits(sign | 0x7f800000 | mant<<13)
	}
	if e == 0 {
		if mant == 0 {
			return math.Float32frombits(sign)
		}
		ex := int32(-14)
		for mant&1024 == 0 {
			mant <<= 1
			ex--
		}
		return math.Float32frombits(sign | uint32(ex+127)<<23 | (mant&1023)<<13)
	}
	return math.Float32frombits(sign | uint32(int(e)-15+127)<<23 | mant<<13)
}
func scaleK(sc []byte, g int) (float32, float32) {
	if g < 4 {
		return float32(sc[g] & 63), float32(sc[g+4] & 63)
	}
	j := g - 4
	return float32((sc[g+4] & 15) | ((sc[j] >> 6) << 4)), float32((sc[g+4] >> 4) | ((sc[j+4] >> 6) << 4))
}
func decodeRow(dst []float32, raw []byte, t Type) error {
	block, bytes, err := t.layout()
	if err != nil {
		return err
	}
	if len(dst)%block != 0 || len(raw) != len(dst)/block*bytes {
		return errors.New("embeddinggemma: encoded row size mismatch")
	}
	switch t {
	case F32:
		for i := range dst {
			dst[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
		}
	case F16:
		for i := range dst {
			dst[i] = half(binary.LittleEndian.Uint16(raw[i*2:]))
		}
	case BF16:
		for i := range dst {
			dst[i] = math.Float32frombits(uint32(binary.LittleEndian.Uint16(raw[i*2:])) << 16)
		}
	case Q8_0:
		for b := 0; b < len(dst)/32; b++ {
			r := raw[b*34:]
			d := half(binary.LittleEndian.Uint16(r))
			for i := 0; i < 32; i++ {
				dst[b*32+i] = d * float32(int8(r[i+2]))
			}
		}
	case Q4_K, Q5_K:
		for b := 0; b < len(dst)/256; b++ {
			r := raw[b*bytes:]
			d, dm := half(binary.LittleEndian.Uint16(r)), half(binary.LittleEndian.Uint16(r[2:]))
			sc := r[4:16]
			qs := r[16:]
			var high []byte
			if t == Q5_K {
				high = qs[:32]
				qs = qs[32:]
			}
			for g := 0; g < 8; g++ {
				s, m := scaleK(sc, g)
				s *= d
				m *= dm
				for i := 0; i < 32; i++ {
					q := qs[(g/2)*32+i]
					v := q & 15
					if g&1 != 0 {
						v = q >> 4
					}
					if t == Q5_K && high[i]&(1<<g) != 0 {
						v += 16
					}
					dst[b*256+g*32+i] = s*float32(v) - m
				}
			}
		}
	case Q6_K:
		for b := 0; b < len(dst)/256; b++ {
			r := raw[b*210:]
			ql, qh, sc := r[:128], r[128:192], r[192:208]
			d := half(binary.LittleEndian.Uint16(r[208:]))
			for h := 0; h < 2; h++ {
				for i := 0; i < 32; i++ {
					is := i / 16
					lo := ql[h*64:]
					hi := qh[h*32+i]
					a := int((lo[i]&15)|((hi&3)<<4)) - 32
					b1 := int((lo[i+32]&15)|(((hi>>2)&3)<<4)) - 32
					c := int((lo[i]>>4)|(((hi>>4)&3)<<4)) - 32
					e := int((lo[i+32]>>4)|(((hi>>6)&3)<<4)) - 32
					base := b*256 + h*128
					dst[base+i] = d * float32(int8(sc[h*8+is])) * float32(a)
					dst[base+32+i] = d * float32(int8(sc[h*8+is+2])) * float32(b1)
					dst[base+64+i] = d * float32(int8(sc[h*8+is+4])) * float32(c)
					dst[base+96+i] = d * float32(int8(sc[h*8+is+6])) * float32(e)
				}
			}
		}
	}
	return nil
}
