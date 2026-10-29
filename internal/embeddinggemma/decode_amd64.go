//go:build amd64

package embeddinggemma

import "github.com/rcarmo/memento/internal/simd"

//go:noescape
func q4DecodeAVX2(raw *byte, dst *float32, blocks int, halfTable *float32)
func decodeTileRow(dst []float32, raw []byte, typ Type, backend simd.Backend) error {
	if typ == Q4_K && backend == simd.AVX2 {
		q4DecodeAVX2(&raw[0], &dst[0], len(dst)/256, &quantHalfTable[0])
		return nil
	}
	return decodeRow(dst, raw, typ)
}

//go:noescape
func q4FourAVX2(raw *byte, x *float32, blocks, stride int, halfTable *float32, sums *float32)
func projectFewQ4(raw []byte, x, sums []float32, n, cols int, backend simd.Backend) bool {
	if backend != simd.AVX2 {
		return false
	}
	t := 0
	for ; t+4 <= n; t += 4 {
		q4FourAVX2(&raw[0], &x[t*cols], cols/256, cols, &quantHalfTable[0], &sums[t])
	}
	for ; t < n; t++ {
		sums[t], _ = fusedQ4Dot(raw, x[t*cols:(t+1)*cols], backend)
	}
	return true
}
