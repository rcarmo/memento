//go:build amd64

package embeddinggemma

import (
	"context"
	"github.com/rcarmo/memento/internal/simd"
	"math"
	"testing"
)

func TestPackedMultiplePanels(t *testing.T) {
	e, err := simd.New("avx2")
	if err != nil {
		t.Skip(err)
	}
	const cols = 256
	for _, rows := range []int{8, 9, 15, 16, 17} {
		for _, n := range []int{8, 9, 15, 16, 32, 64} {
			raw := encodedRows(Q4_K, cols, rows)
			f := &File{data: raw}
			m := &Matrix{owner: f, tensor: Tensor{Type: Q4_K, Bytes: uint64(len(raw))}, columns: cols, rows: rows}
			x := make([]float32, n*cols)
			for i := range x {
				x[i] = float32(i%29-14) / 29
			}
			dst := make([]float32, n*rows)
			if err := m.projectPackedLocked(context.Background(), dst, x, n, make([]float32, 8*cols), make([]float32, projectionTile*cols), make([]float32, 8*projectionTile), e); err != nil {
				t.Fatal(err)
			}
			for r := 0; r < rows; r++ {
				v := referenceRow(m.rawRow(r), Q4_K, cols)
				for token := 0; token < n; token++ {
					var want float64
					for j, a := range v {
						want += a * float64(x[token*cols+j])
					}
					if math.Abs(float64(dst[token*rows+r])-want) > 2e-5*math.Max(1, math.Abs(want)) {
						t.Fatal(rows, n, r, token)
					}
				}
			}
		}
	}
}
func TestPackedInvalidPanel(t *testing.T) {
	for _, c := range []struct{ cols, tokens, w, p, o int }{{0, 8, 8, 8, 64}, {1, 0, 8, 8, 64}, {1, 1, 8, 8, 64}, {1, 8, 7, 8, 64}, {1, 8, 8, 7, 64}, {1, 8, 8, 8, 63}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid panel accepted", c)
				}
			}()
			packedRows(make([]float32, c.w), make([]float32, c.p), make([]float32, c.o), c.cols, c.tokens)
		}()
	}
}
