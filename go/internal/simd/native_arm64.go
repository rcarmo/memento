//go:build arm64

package simd

func dotNEON(a, b []float32) float32
func axpyNEON(alpha float32, x, y []float32)
func dotNative(_ Backend, a, b []float32) float32         { return dotNEON(a, b) }
func axpyNative(_ Backend, alpha float32, x, y []float32) { axpyNEON(alpha, x, y) }
func dotRowsNative(_ Backend, input, matrix, output []float32) {
	for row := range output {
		output[row] = dotNEON(input, matrix[row*len(input):(row+1)*len(input)])
	}
}
func axpyRowsNative(_ Backend, coefficients, matrix, output []float32) {
	for row, coefficient := range coefficients {
		axpyNEON(coefficient, matrix[row*len(output):(row+1)*len(output)], output)
	}
}
