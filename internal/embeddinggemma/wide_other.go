//go:build !amd64

package embeddinggemma

func packWide(x, p []float32, cols, n, nt int)               { panic("unsupported wide packing") }
func wideRows(w, p, out []float32, cols, nt int)             { panic("unsupported wide rows") }
func wideScatter(w, p, out []float32, cols, nt, rows, n int) { panic("unsupported wide scatter") }
