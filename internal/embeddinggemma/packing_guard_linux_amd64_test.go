//go:build linux && amd64

package embeddinggemma

import (
	"github.com/rcarmo/memento/internal/simd"
	"golang.org/x/sys/unix"
	"math"
	"os"
	"testing"
	"unsafe"
)

func TestPackingScatterGuardPages(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	page := os.Getpagesize()
	alloc := func(n int) []float32 {
		size := ((n*4 + page - 1) / page) * page
		b, err := unix.Mmap(-1, 0, size+page, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
		if err != nil {
			t.Fatal(err)
		}
		if err = unix.Mprotect(b[size:], unix.PROT_NONE); err != nil {
			unix.Munmap(b)
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := unix.Munmap(b); err != nil {
				t.Error(err)
			}
		})
		return unsafe.Slice((*float32)(unsafe.Pointer(&b[size-n*4])), n)
	}
	for _, cols := range []int{8, 256, 512, 2048} {
		for _, n := range []int{8, 9, 15, 16, 63, 64} {
			nt := (n + 7) &^ 7
			in, p := alloc(n*cols), alloc(nt*cols)
			for i := range in {
				in[i] = float32(i%31-15) / 31
			}
			clear(p)
			packTokens(in, p, cols, n, nt)
			for col := 0; col < cols; col++ {
				for token := 0; token < nt; token++ {
					want := float32(0)
					if token < n {
						want = in[token*cols+col]
					}
					if p[col*nt+token] != want {
						t.Fatal(cols, n, col, token)
					}
				}
			}

			const rows = 17
			w, out := alloc(8*cols), alloc((n-1)*rows+8)
			for i := range w {
				w[i] = float32(i%23-11) / 23
			}
			for i := range out {
				out[i] = 12345
			}
			packedRowsScatter(w, p, out, cols, nt, rows, n)
			for token := 0; token < n; token++ {
				for r := 0; r < 8; r++ {
					var want float64
					for col := 0; col < cols; col++ {
						want += float64(w[r*cols+col]) * float64(in[token*cols+col])
					}
					if math.Abs(float64(out[token*rows+r])-want) > 2e-5*math.Max(1, math.Abs(want)) {
						t.Fatal(cols, n, token, r)
					}
				}
				if token < n-1 {
					for r := 8; r < rows; r++ {
						if out[token*rows+r] != 12345 {
							t.Fatal("scatter overwrote gap")
						}
					}
				}
			}
		}
	}
}
func TestInvalidPacking(t *testing.T) {
	for _, c := range []struct{ cols, n, nt, in, out int }{{0, 8, 8, 1, 1}, {7, 8, 8, 56, 56}, {8, 8, 7, 64, 64}, {8, 8, 8, 63, 64}, {8, 8, 8, 64, 63}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid packing accepted")
				}
			}()
			packTokens(make([]float32, c.in), make([]float32, c.out), c.cols, c.n, c.nt)
		}()
	}
}
