//go:build amd64

package simd

func dotSSE2(a, b []float32) float32
func axpySSE2(alpha float32, x, y []float32)
func dotAVX2(a, b []float32) float32
func axpyAVX2(alpha float32, x, y []float32)
func dotNative(backend Backend, a, b []float32) float32 {
	if backend == AVX2 {
		return dotAVX2(a, b)
	}
	return dotSSE2(a, b)
}
func axpyNative(backend Backend, alpha float32, x, y []float32) {
	if backend == AVX2 {
		axpyAVX2(alpha, x, y)
		return
	}
	axpySSE2(alpha, x, y)
}
