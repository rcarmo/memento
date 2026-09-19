//go:build arm64

package simd

func dotNEON(a, b []float32) float32
func axpyNEON(alpha float32, x, y []float32)
func dotNative(_ Backend, a, b []float32) float32         { return dotNEON(a, b) }
func axpyNative(_ Backend, alpha float32, x, y []float32) { axpyNEON(alpha, x, y) }
