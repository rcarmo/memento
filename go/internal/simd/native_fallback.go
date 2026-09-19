//go:build !amd64 && !arm64

package simd

func dotNative(_ Backend, a, b []float32) float32         { return dotScalar(a, b) }
func axpyNative(_ Backend, alpha float32, x, y []float32) { axpyScalar(alpha, x, y) }
