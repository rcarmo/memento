//go:build amd64

package embeddinggemma

import "github.com/rcarmo/memento/internal/simd"

//go:noescape
func matmul8AVX2(weights, packed, output *float32, cols, tokens int)
func packedProjectionSupported(backend simd.Backend) bool { return backend == simd.AVX2 }
func packedRows(weights, packed, output []float32, cols, tokens int) {
	// Validate before crossing the unsafe assembly boundary; dimensions are
	// bounded to the architecture and request tile, so products cannot overflow.
	if cols < 1 || cols > 2048 || tokens < 8 || tokens > projectionTile || tokens%8 != 0 || len(weights) < 8*cols || len(packed) < cols*tokens || len(output) < 8*tokens {
		panic("embeddinggemma: invalid packed kernel panel")
	}
	matmul8AVX2(&weights[0], &packed[0], &output[0], cols, tokens)
}
