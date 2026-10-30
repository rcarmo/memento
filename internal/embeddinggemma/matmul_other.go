//go:build !amd64

package embeddinggemma

import "github.com/rcarmo/memento/internal/simd"

func packedProjectionSupported(backend simd.Backend) bool { return false }
func packedRows(weights, packed, output []float32, cols, tokens int) {
	panic("embeddinggemma: unavailable packed projection")
}
