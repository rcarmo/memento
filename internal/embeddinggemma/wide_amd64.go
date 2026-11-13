//go:build amd64

package embeddinggemma

//go:noescape
func wideRowsAVX2(weights, packed, output *float32, cols, tokens int)

//go:noescape
func wideScatterAVX2(weights, packed, output *float32, cols, tokens, rows, valid int)
func packWide(x, p []float32, cols, n, nt int) {
	if cols < 8 || cols > 2048 || cols%8 != 0 || n < 1 || n > projectionTile || nt < n || nt > projectionTile || nt%16 != 0 || len(x) < n*cols || len(p) < cols*nt {
		panic("embeddinggemma: invalid wide packing")
	}
	t := 0
	for ; t+8 <= n; t += 8 {
		pack8AVX2(&x[t*cols], &p[(t/16)*cols*16+t%16], cols, 16)
	}
	for ; t < n; t++ {
		for c := 0; c < cols; c++ {
			p[(t/16)*cols*16+c*16+t%16] = x[t*cols+c]
		}
	}
}
func wideRows(w, p, out []float32, cols, nt int) {
	if cols < 1 || cols > 2048 || nt < 16 || nt > projectionTile || nt%16 != 0 || len(w) < 6*cols || len(p) < cols*nt || len(out) < 6*nt {
		panic("embeddinggemma: invalid wide rows")
	}
	wideRowsAVX2(&w[0], &p[0], &out[0], cols, nt)
}
func wideScatter(w, p, out []float32, cols, nt, rows, n int) {
	if cols < 1 || cols > 2048 || nt < 16 || nt > projectionTile || nt%16 != 0 || rows < 6 || rows > 12288 || n < 1 || n > nt || len(w) < 6*cols || len(p) < cols*nt || len(out) < (n-1)*rows+6 {
		panic("embeddinggemma: invalid wide scatter")
	}
	wideScatterAVX2(&w[0], &p[0], &out[0], cols, nt, rows, n)
}
