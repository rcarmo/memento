//go:build !amd64

package embeddinggemma

import "github.com/rcarmo/memento/internal/simd"

func decodeTileRow(dst []float32, raw []byte, typ Type, backend simd.Backend) error {
	return decodeRow(dst, raw, typ)
}
func projectFewQ4(raw []byte, x, sums []float32, n, cols int, backend simd.Backend) bool {
	return false
}
