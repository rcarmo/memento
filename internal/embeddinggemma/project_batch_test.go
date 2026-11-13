package embeddinggemma

import (
	"context"
	"math"
	"testing"

	"github.com/rcarmo/memento/internal/simd"
)

func TestProjectionTiles(t *testing.T) {
	for _, typ := range []Type{F32, F16, BF16, Q4K, Q5K, Q6K, Q8_0} {
		const cols = 512
		const rows = 7
		raw := encodedRows(typ, cols, rows)
		f := &File{data: raw}
		m := &Matrix{owner: f, tensor: Tensor{Type: typ, Bytes: uint64(len(raw))}, columns: cols, rows: rows}
		for _, backend := range []string{"scalar", "sse2", "avx2", "neon"} {
			engine, err := simd.New(backend)
			if err != nil {
				continue
			}
			for _, n := range []int{1, 2, 3, 4, 5, 15, 16, 31, 32, 63, 64} {
				x := make([]float32, n*cols)
				for i := range x {
					x[i] = float32(i%37-18) / 13
				}
				dst := make([]float32, n*rows)
				scratch := make([]float32, 8*cols)
				sums := make([]float32, 8*projectionTile)
				packed := make([]float32, projectionTile*cols)
				if err := m.projectPackedLocked(context.Background(), dst, x, n, scratch, packed, sums, engine); err != nil {
					t.Fatal(typ, backend, n, err)
				}
				for token := 0; token < n; token++ {
					for row := 0; row < rows; row++ {
						values := referenceRow(m.rawRow(row), typ, cols)
						var want float64
						for j, v := range values {
							want += v * float64(x[token*cols+j])
						}
						if math.Abs(float64(dst[token*rows+row])-want) > 2e-5*math.Max(1, math.Abs(want)) {
							t.Fatal(typ, backend, n, token, row, dst[token*rows+row], want)
						}
					}
				}
				if allocs := testing.AllocsPerRun(2, func() {
					if err := m.projectPackedLocked(context.Background(), dst, x, n, scratch, packed, sums, engine); err != nil {
						panic(err)
					}
				}); allocs != 0 {
					t.Fatal(allocs)
				}
			}
		}
		engine, _ := simd.New("scalar")
		if err := m.projectPackedLocked(context.Background(), nil, nil, 0, nil, nil, nil, engine); err == nil {
			t.Fatal("bounds")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := m.projectPackedLocked(ctx, make([]float32, 2*rows), make([]float32, 2*cols), 2, make([]float32, cols), nil, make([]float32, 2), engine); err != context.Canceled {
			t.Fatal(err)
		}
	}
}
