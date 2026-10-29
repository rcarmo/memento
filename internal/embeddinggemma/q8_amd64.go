//go:build amd64

package embeddinggemma

import "github.com/rcarmo/memento/internal/simd"

// q8DotSSE2 and q8DotAVX2 consume complete Q8_0 rows. Each block contains a
// binary16 scale followed by 32 signed quantized values. The activation side
// remains float32 throughout.
//
//go:noescape
func q8DotSSE2(raw *byte, x, halfTable *float32, blocks int) float32

//go:noescape
func q8DotAVX2(raw *byte, x, halfTable *float32, blocks int) float32

func fusedQ8Dot(raw []byte, x []float32, backend simd.Backend) (float32, bool) {
	if backend != simd.SSE2 && backend != simd.AVX2 {
		return 0, false
	}
	if len(x)%32 != 0 || len(raw)%34 != 0 || len(x)/32 != len(raw)/34 {
		return 0, false
	}
	if len(x) == 0 {
		return 0, true
	}
	if backend == simd.AVX2 {
		return q8DotAVX2(&raw[0], &x[0], &quantHalfTable[0], len(x)/32), true
	}
	return q8DotSSE2(&raw[0], &x[0], &quantHalfTable[0], len(x)/32), true
}
