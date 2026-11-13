//go:build amd64

package embeddinggemma

import (
	"encoding/binary"

	"github.com/rcarmo/memento/internal/simd"
)

// q5GroupSSE2 and q5GroupAVX2 dequantize one packed Q5K group while
// accumulating its dot product. highMask selects the group's bit plane in the
// 32-byte Q5K high-bit area.
//
//go:noescape
func q5GroupSSE2(q, high *byte, x *float32, scale, minimum float32, highNibble bool, highMask uint32) float32

//go:noescape
func q5GroupAVX2(q, high *byte, x *float32, scale, minimum float32, highNibble bool, highMask uint32) float32

func fusedQ5Dot(raw []byte, x []float32, backend simd.Backend) (float32, bool) {
	if backend != simd.SSE2 && backend != simd.AVX2 {
		return 0, false
	}
	var sum float32
	for b := 0; b < len(x)/256; b++ {
		r := raw[b*176:]
		d, dm := half(binary.LittleEndian.Uint16(r)), half(binary.LittleEndian.Uint16(r[2:]))
		high, qs := r[16:48], r[48:176]
		for g := 0; g < 8; g++ {
			s, m := scaleK(r[4:16], g)
			q := &qs[(g/2)*32]
			v := &x[b*256+g*32]
			mask := uint32(1) << g
			if backend == simd.AVX2 {
				sum += q5GroupAVX2(q, &high[0], v, d*s, dm*m, g&1 != 0, mask)
			} else {
				sum += q5GroupSSE2(q, &high[0], v, d*s, dm*m, g&1 != 0, mask)
			}
		}
	}
	return sum, true
}
