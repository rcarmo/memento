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

func TestBlockedPanelsGuardPages(t *testing.T) {
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
		for _, n := range []int{8, 9, 15, 16, 31, 32, 63, 64} {
			nt := (n + 7) &^ 7
			x, p, w, out := alloc(n*cols), alloc(nt*cols), alloc(8*cols), alloc(8*nt)
			for i := range x {
				x[i] = float32(i%31-15) / 31
			}
			for i := range w {
				w[i] = float32(i%23-11) / 23
			}
			clear(p)
			packBlocked(x, p, cols, n, nt)
			for col := 0; col < cols; col++ {
				for tok := 0; tok < nt; tok++ {
					want := float32(0)
					if tok < n {
						want = x[tok*cols+col]
					}
					if p[(tok/8)*cols*8+col*8+tok%8] != want {
						t.Fatal("packing", cols, n, col, tok)
					}
				}
			}
			blockedRows(w, p, out, cols, nt)
			const rows = 17
			scatter := alloc((n-1)*rows + 8)
			for i := range scatter {
				scatter[i] = 12345
			}
			blockedScatter(w, p, scatter, cols, nt, rows, n)
			for row := 0; row < 8; row++ {
				for tok := 0; tok < nt; tok++ {
					var want float64
					if tok < n {
						for col := 0; col < cols; col++ {
							want += float64(w[row*cols+col]) * float64(x[tok*cols+col])
						}
					}
					if math.Abs(float64(out[row*nt+tok])-want) > 2e-5*math.Max(1, math.Abs(want)) {
						t.Fatal("row", cols, n, row, tok)
					}
					if tok < n && out[row*nt+tok] != scatter[tok*rows+row] {
						t.Fatal("scatter")
					}
				}
			}
			for tok := 0; tok < n-1; tok++ {
				for row := 8; row < rows; row++ {
					if scatter[tok*rows+row] != 12345 {
						t.Fatal("gap")
					}
				}
			}
		}
	}
}

func TestBlockedOddWidths(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	for _, cols := range []int{1, 7} {
		for _, nt := range []int{8, 64} {
			w, p, out := make([]float32, 8*cols), make([]float32, cols*nt), make([]float32, 8*nt)
			for i := range w {
				w[i] = float32(i+1) / 17
			}
			for i := range p {
				p[i] = float32(i%13-6) / 13
			}
			blockedRows(w, p, out, cols, nt)
			for valid := 1; valid <= 7; valid++ {
				const rows = 17
				dst := make([]float32, (valid-1)*rows+8)
				blockedScatter(w, p, dst, cols, nt, rows, valid)
				for r := 0; r < 8; r++ {
					for tok := 0; tok < valid; tok++ {
						var want float64
						for c := 0; c < cols; c++ {
							want += float64(w[r*cols+c]) * float64(p[(tok/8)*cols*8+c*8+tok%8])
						}
						if math.Abs(float64(dst[tok*rows+r])-want) > 2e-5*math.Max(1, math.Abs(want)) {
							t.Fatal(cols, nt, valid, r, tok)
						}
					}
				}
			}
		}
	}
}
func TestBlockedInvalidBounds(t *testing.T) {
	for _, f := range []func(){func() { packBlocked(nil, nil, 0, 8, 8) }, func() { packBlocked(make([]float32, 63), make([]float32, 64), 8, 8, 8) }, func() { blockedRows(make([]float32, 7), make([]float32, 8), make([]float32, 64), 1, 8) }, func() { blockedRows(make([]float32, 8), make([]float32, 8), make([]float32, 63), 1, 8) }, func() { blockedScatter(make([]float32, 8), make([]float32, 8), make([]float32, 62), 1, 8, 8, 8) }, func() { blockedScatter(make([]float32, 8), make([]float32, 8), make([]float32, 64), 1, 8, 8, 9) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid blocked bounds accepted")
				}
			}()
			f()
		}()
	}
}
