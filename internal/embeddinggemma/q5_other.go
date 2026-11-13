//go:build !amd64

package embeddinggemma

import "github.com/rcarmo/memento/internal/simd"

func fusedQ5Dot(_ []byte, _ []float32, _ simd.Backend) (float32, bool) { return 0, false }
