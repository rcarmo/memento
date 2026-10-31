//go:build amd64

package embeddinggemma

import (
	"context"
	"github.com/rcarmo/memento/internal/simd"
	"math"
	"testing"
)

func TestScatterDecoderIntegration(t *testing.T) {
	e, err := simd.New("avx2")
	if err != nil {
		t.Skip(err)
	}
	for _, typ := range []Type{Q5_K, Q8_0} {
		for _, cols := range []int{512, 2048} {
			const rows = 17
			const n = 16
			raw := encodedRows(typ, cols, rows)
			f := &File{data: raw}
			m := &Matrix{owner: f, tensor: Tensor{Type: typ, Bytes: uint64(len(raw))}, columns: cols, rows: rows}
			x := make([]float32, n*cols)
			for i := range x {
				x[i] = float32(i%29-14) / 29
			}
			dst := make([]float32, n*rows)
			decoded, packed, sums := make([]float32, 8*cols), make([]float32, projectionTile*cols), make([]float32, 8*projectionTile)
			if err := m.projectPackedLocked(context.Background(), dst, x, n, decoded, packed, sums, e); err != nil {
				t.Fatal(err)
			}
			for row := 0; row < rows; row++ {
				ref := referenceRow(m.rawRow(row), typ, cols)
				for tok := 0; tok < n; tok++ {
					var want float64
					for col, v := range ref {
						want += v * float64(x[tok*cols+col])
					}
					if math.Abs(float64(dst[tok*rows+row])-want) > 2e-5*math.Max(1, math.Abs(want)) {
						t.Fatal(typ, cols, row, tok)
					}
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := m.projectPackedLocked(ctx, dst, x, n, decoded, packed, sums, e); err != context.Canceled {
				t.Fatal(err)
			}
		}
	}
}
func TestScatterInvalidPanel(t *testing.T) {
	for _, c := range []struct{ cols, tokens, rows, w, p, out int }{{0, 8, 8, 8, 8, 64}, {1, 0, 8, 8, 8, 64}, {1, 1, 8, 8, 8, 64}, {1, 8, 7, 8, 8, 64}, {1, 8, 12289, 8, 8, 64}, {1, 8, 8, 7, 8, 64}, {1, 8, 8, 8, 7, 64}, {1, 8, 17, 8, 8, 126}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid scatter panel accepted", c)
				}
			}()
			packedRowsScatter(make([]float32, c.w), make([]float32, c.p), make([]float32, c.out), c.cols, c.tokens, c.rows)
		}()
	}
}
func TestQ4DecodeValidation(t *testing.T) {
	e, err := simd.New("avx2")
	if err != nil {
		t.Skip(err)
	}
	if err := decodeTileRow(nil, nil, Q4_K, e.Backend()); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ dst, raw int }{{255, 144}, {256, 143}, {256, 145}} {
		if err := decodeTileRow(make([]float32, c.dst), make([]byte, c.raw), Q4_K, e.Backend()); err == nil {
			t.Fatal(c)
		}
	}
}
