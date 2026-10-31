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

// Eight-row decoded panel and token-packed activation panel are bounded by the
// existing tile size. Buffers belong to the locked request workspace.
func (m *Matrix) projectPackedLocked(ctx context.Context, dst, x []float32, n int, decoded, packed, sums []float32, engine simd.Engine) error {
	if n < 1 || n > projectionTile || len(dst) != n*m.rows || len(x) != n*m.columns || len(decoded) < m.columns || len(sums) < n {
		return fmt.Errorf("embeddinggemma: invalid packed projection tile")
	}
	if !packedProjectionSupported(engine.Backend()) || n < 8 {
		return m.projectTokensLocked(ctx, dst, x, n, decoded, sums, engine)
	}
	nt := (n + 7) &^ 7
	if len(decoded) < 8*m.columns || len(packed) < nt*m.columns || len(sums) < 8*nt {
		return fmt.Errorf("embeddinggemma: short packed scratch")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	packed = packed[:m.columns*nt]
	clear(packed)
	packTokens(x, packed, m.columns, n, nt)
	for row := 0; row < m.rows; row += 8 {
		if err := ctx.Err(); err != nil {
			return err
		}
		count := min(8, m.rows-row)
		for r := 0; r < count; r++ {
			if err := decodeTileRow(decoded[r*m.columns:(r+1)*m.columns], m.rawRow(row+r), m.tensor.Type, engine.Backend()); err != nil {
				return err
			}
		}
		if count < 8 {
			clear(decoded[count*m.columns : 8*m.columns])
		}
		if count == 8 && n == nt {
			packedRowsScatter(decoded[:8*m.columns], packed, dst[row:], m.columns, nt, m.rows)
			continue
		}
		packedRows(decoded[:8*m.columns], packed, sums[:8*nt], m.columns, nt)
		for r := 0; r < count; r++ {
			for t := 0; t < n; t++ {
				dst[t*m.rows+row+r] = sums[r*nt+t]
			}
		}
	}
	return ctx.Err()
}
