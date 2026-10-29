package embeddinggemma

import (
	"context"
	"fmt"
	"github.com/rcarmo/memento/internal/simd"
)

// A request-owned tile amortises row decoding without expanding a weight
// matrix. The caller holds owner.mu.RLock; scratch and sums are disjoint from
// the token-major input/output. Never process more than projectionTile tokens.
const projectionTile = 64

func (m *Matrix) projectTokensLocked(ctx context.Context, dst, x []float32, n int, scratch, sums []float32, engine simd.Engine) error {
	if n < 1 || n > projectionTile || len(dst) != n*m.rows || len(x) != n*m.columns || len(scratch) < m.columns || len(sums) < n {
		return fmt.Errorf("embeddinggemma: invalid projection tile")
	}
	if n == 1 {
		return m.projectLocked(ctx, dst, x, scratch, engine)
	}
	decoded := scratch[:m.columns]
	for row := 0; row < m.rows; row++ {
		if row&15 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if m.tensor.Type == Q4_K && n < 32 && projectFewQ4(m.rawRow(row), x, sums[:n], n, m.columns, engine.Backend()) {
			for t := 0; t < n; t++ {
				dst[t*m.rows+row] = sums[t]
			}
			continue
		}
		if err := decodeTileRow(decoded, m.rawRow(row), m.tensor.Type, engine.Backend()); err != nil {
			return err
		}
		if err := engine.DotRows(decoded, x, sums[:n]); err != nil {
			return err
		}
		for t := 0; t < n; t++ {
			dst[t*m.rows+row] = sums[t]
		}
	}
	return ctx.Err()
}
