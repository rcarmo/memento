//go:build amd64

package embeddinggemma

//go:noescape
func blockedRowsAVX2(weights, packed, output *float32, cols, tokens int)

//go:noescape
func blockedScatterAVX2(weights, packed, output *float32, cols, tokens, rows, valid int)
func packBlocked(input, packed []float32, cols, n, nt int) {
	if cols < 8 || cols > 2048 || cols%8 != 0 || n < 1 || n > projectionTile || nt < n || nt > projectionTile || nt%8 != 0 || len(input) < n*cols || len(packed) < nt*cols {
		panic("embeddinggemma: invalid blocked packing")
	}
	t := 0
	for ; t+8 <= n; t += 8 {
		pack8AVX2(&input[t*cols], &packed[t*cols], cols, 8)
	}
	for ; t < n; t++ {
		for col := 0; col < cols; col++ {
			packed[(t/8)*cols*8+col*8+t%8] = input[t*cols+col]
		}
	}
}
func blockedRows(weights, packed, output []float32, cols, tokens int) {
	if cols < 1 || cols > 2048 || tokens < 8 || tokens > projectionTile || tokens%8 != 0 || len(weights) < 8*cols || len(packed) < cols*tokens || len(output) < 8*tokens {
		panic("embeddinggemma: invalid blocked panel")
	}
	blockedRowsAVX2(&weights[0], &packed[0], &output[0], cols, tokens)
}
func blockedScatter(weights, packed, output []float32, cols, tokens, rows, valid int) {
	if cols < 1 || cols > 2048 || tokens < 8 || tokens > projectionTile || tokens%8 != 0 || rows < 8 || rows > 12288 || valid < 1 || valid > tokens || len(weights) < 8*cols || len(packed) < cols*tokens || len(output) < (valid-1)*rows+8 {
		panic("embeddinggemma: invalid blocked scatter")
	}
	blockedScatterAVX2(&weights[0], &packed[0], &output[0], cols, tokens, rows, valid)
}
