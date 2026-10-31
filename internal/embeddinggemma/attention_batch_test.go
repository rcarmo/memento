package embeddinggemma

import (
	"context"
	"math"
	"testing"

	"github.com/rcarmo/memento/internal/simd"
)

func TestContiguousAttention(t *testing.T) {
	for _, backend := range []string{"scalar", "sse2", "avx2", "neon"} {
		engine, err := simd.New(backend)
		if err != nil {
			continue
		}
		for _, n := range []int{1, 7, 31, 32, 69, 127, 128, 129, 255, 256, 257, 513} {
			for _, kv := range []int{1, 2} {
				const head = 8
				q := make([]float32, n*4*head)
				k, v := make([]float32, n*kv*head), make([]float32, n*kv*head)
				for i := range q {
					q[i] = float32(i%17-8) / 17
				}
				for i := range k {
					k[i] = float32(i%13-6) / 13
					v[i] = float32(i%19-9) / 19
				}
				scratch := make([]float32, len(k))
				kc, vc := append([]float32(nil), k...), append([]float32(nil), v...)
				transposeKV(kc, scratch, n, head, kv)
				transposeKV(vc, scratch, n, head, kv)
				for h := 0; h < kv; h++ {
					for token := 0; token < n; token++ {
						for d := 0; d < head; d++ {
							if kc[(h*n+token)*head+d] != k[(token*kv+h)*head+d] || vc[(h*n+token)*head+d] != v[(token*kv+h)*head+d] {
								t.Fatal("KV transpose", backend, n, kv)
							}
						}
					}
				}
				for _, sliding := range []bool{false, true} {
					want, got := make([]float32, len(q)), make([]float32, len(q))
					scores := make([]float32, n)
					if err := attention(context.Background(), want, q, k, v, n, head, kv, sliding, scores, engine); err != nil {
						t.Fatal(err)
					}
					if err := attentionContiguous(context.Background(), got, q, kc, vc, n, head, kv, sliding, scores, engine); err != nil {
						t.Fatal(err)
					}
					for i := range got {
						if math.IsNaN(float64(got[i])) || math.IsInf(float64(got[i]), 0) || math.Abs(float64(got[i]-want[i])) > 2e-6 {
							t.Fatal(backend, n, kv, sliding, i, got[i], want[i])
						}
					}
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if err := attentionContiguous(ctx, make([]float32, len(q)), q, kc, vc, n, head, kv, true, make([]float32, n), engine); err != context.Canceled {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestContiguousAttentionWindowOracle(t *testing.T) {
	const n = 1027
	const head = 2
	const kv = 2
	engine, _ := simd.New("scalar")
	q := make([]float32, n*4*head)
	k, v := make([]float32, n*kv*head), make([]float32, n*kv*head)
	for i := range q {
		q[i] = float32(i%17-8) / 17
	}
	for i := range k {
		k[i] = float32(i%13-6) / 13
		v[i] = float32(i%19-9) / 19
	}
	kc, vc := append([]float32(nil), k...), append([]float32(nil), v...)
	scratch := make([]float32, len(k))
	transposeKV(kc, scratch, n, head, kv)
	transposeKV(vc, scratch, n, head, kv)
	dst := make([]float32, len(q))
	scores := make([]float32, n)
	for _, sliding := range []bool{false, true} {
		if err := attentionContiguous(context.Background(), dst, q, kc, vc, n, head, kv, sliding, scores, engine); err != nil {
			t.Fatal(err)
		}
		for _, p := range []int{0, 511, 512, 513, 1026} {
			for h := 0; h < 4; h++ {
				lo, hi := 0, n
				if sliding {
					lo = max(0, p-512)
					hi = min(n, p+513)
				}
				var denom float64
				want := make([]float64, head)
				for j := lo; j < hi; j++ {
					var dot float64
					for d := 0; d < head; d++ {
						dot += float64(q[p*4*head+h*head+d]) * float64(k[(j*kv+h/2)*head+d])
					}
					s := math.Exp(dot)
					denom += s
					for d := 0; d < head; d++ {
						want[d] += s * float64(v[(j*kv+h/2)*head+d])
					}
				}
				for d := 0; d < head; d++ {
					if math.IsNaN(float64(dst[p*4*head+h*head+d])) || math.IsInf(float64(dst[p*4*head+h*head+d]), 0) || math.Abs(float64(dst[p*4*head+h*head+d])-want[d]/denom) > 2e-6 {
						t.Fatal(sliding, p, h, d)
					}
				}
			}
		}
	}
}

func TestContiguousProductionHeads(t *testing.T) {
	engine, _ := simd.New("scalar")
	const n = 129
	for _, cfg := range []struct{ head, kv int }{{256, 2}, {512, 1}} {
		head, kv := cfg.head, cfg.kv
		q := make([]float32, n*4*head)
		k, v := make([]float32, n*kv*head), make([]float32, n*kv*head)
		for i := range q {
			q[i] = float32(i%17-8) / float32(head)
		}
		for i := range k {
			k[i] = float32(i%13-6) / 13
			v[i] = float32(i%19-9) / 19
		}
		originalK, originalV := append([]float32(nil), k...), append([]float32(nil), v...)
		dst := make([]float32, len(q))
		transposeKV(k, dst[:len(k)], n, head, kv)
		transposeKV(v, dst[:len(v)], n, head, kv)
		if err := attentionContiguous(context.Background(), dst, q, k, v, n, head, kv, false, make([]float32, n), engine); err != nil {
			t.Fatal(err)
		}
		for _, pos := range []int{0, 64, 128} {
			for h := 0; h < 4; h++ {
				kh := h / (4 / kv)
				var denom float64
				want := make([]float64, head)
				for j := 0; j < n; j++ {
					var dot float64
					for d := 0; d < head; d++ {
						dot += float64(q[(pos*4+h)*head+d]) * float64(originalK[(j*kv+kh)*head+d])
					}
					s := math.Exp(dot)
					denom += s
					for d := 0; d < head; d++ {
						want[d] += s * float64(originalV[(j*kv+kh)*head+d])
					}
				}
				for d := 0; d < head; d++ {
					got := float64(dst[(pos*4+h)*head+d])
					if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want[d]/denom) > 2e-6 {
						t.Fatal(cfg, pos, h, d)
					}
				}
			}
		}
	}
}

// Deterministic cancellation at the second row-group check, after one score
// group has executed. No scheduler/timer race is needed to test the boundary.
type attentionCancelContext struct {
	context.Context
	checks int
}

func (c *attentionCancelContext) Err() error {
	c.checks++
	if c.checks >= 3 {
		return context.Canceled
	}
	return nil
}
func TestContiguousGroupCancellation(t *testing.T) {
	engine, _ := simd.New("scalar")
	const n = 513
	const head = 8
	ctx := &attentionCancelContext{Context: context.Background()}
	err := attentionContiguous(ctx, make([]float32, n*4*head), make([]float32, n*4*head), make([]float32, n*head), make([]float32, n*head), n, head, 1, false, make([]float32, n), engine)
	if err != context.Canceled || ctx.checks != 3 {
		t.Fatal(err, ctx.checks)
	}
}
