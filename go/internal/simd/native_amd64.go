//go:build amd64

package simd

func dotSSE2(a, b []float32) float32
func axpySSE2(alpha float32, x, y []float32)
func dotAVX2(a, b []float32) float32
func axpyAVX2(alpha float32, x, y []float32)
func dotRowsSSE2(input, matrix, output []float32)
func axpyRowsSSE2(coefficients, matrix, output []float32)
func dotRowsAVX2(input, matrix, output []float32)
func axpyRowsAVX2(coefficients, matrix, output []float32)
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
func dotRowsNative(backend Backend, input, matrix, output []float32) {
	if backend == AVX2 {
		dotRowsAVX2(input, matrix, output)
		return
	}
	dotRowsSSE2(input, matrix, output)
}
func axpyRowsNative(backend Backend, coefficients, matrix, output []float32) {
	if backend == AVX2 {
		axpyRowsAVX2(coefficients, matrix, output)
		return
	}
	axpyRowsSSE2(coefficients, matrix, output)
}
