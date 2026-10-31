//go:build amd64

package embeddinggemma

// One call per complete row. Callers validate sizes before dispatch; kernels
// consume exactly blocks*176/34 raw bytes and write blocks*256/32 floats.
// Separate scale multiply, coefficient multiply and subtract preserve decodeRow
// FP32 operations; neither decoder uses FMA or quantizes activations.

//go:noescape
func q5DecodeAVX2(raw *byte, dst *float32, blocks int, halfTable *float32)

//go:noescape
func q8DecodeAVX2(raw *byte, dst *float32, blocks int, halfTable *float32)
