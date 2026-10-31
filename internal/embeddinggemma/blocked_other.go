//go:build !amd64

package embeddinggemma

func packBlocked(input, packed []float32, cols, n, nt int) { panic("unsupported blocked pack") }
func blockedRows(weights, packed, output []float32, cols, tokens int) {
	panic("unsupported blocked matmul")
}
func blockedScatter(weights, packed, output []float32, cols, tokens, rows, valid int) {
	panic("unsupported blocked scatter")
}
