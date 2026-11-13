//go:build amd64

package embeddinggemma

//go:noescape
func pack8AVX2(input, packed *float32, cols, tokens int)
func packTokens(input, packed []float32, cols, n, nt int) {
	if cols < 8 || cols > 2048 || cols%8 != 0 || n < 1 || n > projectionTile || nt < n || nt > projectionTile || nt%8 != 0 || len(input) < n*cols || len(packed) < cols*nt {
		panic("embeddinggemma: invalid packing tile")
	}
	t := 0
	for ; t+8 <= n; t += 8 {
		pack8AVX2(&input[t*cols], &packed[t], cols, nt)
	}
	for ; t < n; t++ {
		for col := 0; col < cols; col++ {
			packed[col*nt+t] = input[t*cols+col]
		}
	}
}
