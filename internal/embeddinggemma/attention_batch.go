package embeddinggemma

import (
	"context"
	"github.com/rcarmo/memento/internal/simd"
	"math"
)

// Transpose token-major KV heads in existing request scratch, not new buffers.
func transposeKV(values, scratch []float32, n, head, kv int) {
	for h := 0; h < kv; h++ {
		for t := 0; t < n; t++ {
			copy(scratch[(h*n+t)*head:][:head], values[(t*kv+h)*head:][:head])
		}
	}
	copy(values, scratch)
}
func attentionContiguous(ctx context.Context, dst, q, k, v []float32, n, head, kv int, sliding bool, scores []float32, engine simd.Engine) error {
	for t := 0; t < n; t++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lo, hi := 0, n
		if sliding {
			lo = max(0, t-512)
			hi = min(n, t+513)
		}
		for h := 0; h < 4; h++ {
			qh := q[t*4*head+h*head:][:head]
			kh := h / (4 / kv)
			keys, values := k[kh*n*head:(kh+1)*n*head], v[kh*n*head:(kh+1)*n*head]
			for j := lo; j < hi; j += 256 {
				end := min(hi, j+256)
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := engine.DotRows(qh, keys[j*head:end*head], scores[j:end]); err != nil {
					return err
				}
			}
			mx := float32(-math.MaxFloat32)
			for j := lo; j < hi; j++ {
				if scores[j] > mx {
					mx = scores[j]
				}
			}
			var sum float32
			for j := lo; j < hi; j++ {
				scores[j] = float32(math.Exp(float64(scores[j] - mx)))
				sum += scores[j]
			}
			for j := lo; j < hi; j++ {
				scores[j] /= sum
			}
			out := dst[t*4*head+h*head:][:head]
			clear(out)
			for j := lo; j < hi; j += 256 {
				end := min(hi, j+256)
				if err := ctx.Err(); err != nil {
					return err
				}
				if err := engine.AXPYRows(scores[j:end], values[j*head:end*head], out); err != nil {
					return err
				}
			}
		}
	}
	return ctx.Err()
}
