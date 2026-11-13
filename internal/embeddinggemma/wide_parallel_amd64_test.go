//go:build amd64

package embeddinggemma

import (
	"context"
	"github.com/rcarmo/memento/internal/simd"
	"math"
	"testing"
)

func TestWideInteriorSpan(t *testing.T) {
	e, err := simd.New("avx2")
	if err != nil {
		t.Skip(err)
	}
	const rows = 37
	const cols = 256
	const n = 17
	const nt = 32
	raw := encodedRows(Q4K, cols, rows)
	m := &Matrix{owner: &File{data: raw}, tensor: Tensor{Type: Q4K, Bytes: uint64(len(raw))}, rows: rows, columns: cols}
	x, p := make([]float32, n*cols), make([]float32, nt*cols)
	for i := range x {
		x[i] = float32(i%29-14) / 29
	}
	packWide(x, p, cols, n, nt)
	for _, span := range [][2]int{{8, 16}, {16, 32}, {32, 37}} {
		dst := make([]float32, n*rows)
		for i := range dst {
			dst[i] = 12345
		}
		if err := m.projectPanels(context.Background(), dst, n, nt, make([]float32, 8*cols), p, make([]float32, 8*nt), e, span[0], span[1]); err != nil {
			t.Fatal(err)
		}
		for r := 0; r < rows; r++ {
			for tok := 0; tok < n; tok++ {
				if r < span[0] || r >= span[1] {
					if dst[tok*rows+r] != 12345 {
						t.Fatal("span overwrite", span, r, tok)
					}
					continue
				}
				ref := referenceRow(m.rawRow(r), Q4K, cols)
				var want float64
				for col, v := range ref {
					want += v * float64(x[tok*cols+col])
				}
				if math.Abs(float64(dst[tok*rows+r])-want) > 2e-5*math.Max(1, math.Abs(want)) {
					t.Fatal(span, r, tok)
				}
			}
		}
	}
}
