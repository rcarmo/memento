//go:build !amd64

package embeddinggemma

func packTokens(input, packed []float32, cols, n, nt int) {
	for t := 0; t < n; t++ {
		for col := 0; col < cols; col++ {
			packed[col*nt+t] = input[t*cols+col]
		}
	}
}
