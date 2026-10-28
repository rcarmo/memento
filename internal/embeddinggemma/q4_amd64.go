//go:build amd64

package embeddinggemma

import (
	"encoding/binary"
	"github.com/rcarmo/memento/internal/simd"
)

//go:noescape
func q4GroupSSE2(q *byte, x *float32, scale, minimum float32, high bool) float32

//go:noescape
func q4GroupAVX2(q *byte, x *float32, scale, minimum float32, high bool) float32
func fusedQ4Dot(raw []byte, x []float32, backend simd.Backend) (float32, bool) {
	if backend != simd.SSE2 && backend != simd.AVX2 {
		return 0, false
	}
	var sum float32
	for b := 0; b < len(x)/256; b++ {
		r := raw[b*144:]
		d, dm := half(binary.LittleEndian.Uint16(r)), half(binary.LittleEndian.Uint16(r[2:]))
		for g := 0; g < 8; g++ {
			s, m := scaleK(r[4:16], g)
			q := &r[16+(g/2)*32]
			v := &x[b*256+g*32]
			if backend == simd.AVX2 {
				sum += q4GroupAVX2(q, v, d*s, dm*m, g&1 != 0)
			} else {
				sum += q4GroupSSE2(q, v, d*s, dm*m, g&1 != 0)
			}
		}
	}
	return sum, true
}
