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

func TestWideGuardPages(t *testing.T) {
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
		for _, n := range []int{8, 9, 15, 16, 17, 31, 32, 63, 64} {
			nt := (n + 15) &^ 15
			x, p, w, out := alloc(n*cols), alloc(nt*cols), alloc(6*cols), alloc(6*nt)
			for i := range x {
				x[i] = float32(i%31-15) / 31
			}
			for i := range w {
				w[i] = float32(i%23-11) / 23
			}
			clear(p)
			packWide(x, p, cols, n, nt)
			for col := 0; col < cols; col++ {
				for tok := 0; tok < nt; tok++ {
					want := float32(0)
					if tok < n {
						want = x[tok*cols+col]
					}
					if p[(tok/16)*cols*16+col*16+tok%16] != want {
						t.Fatal("packing", cols, n, col, tok)
					}
				}
			}
			wideRows(w, p, out, cols, nt)
			const rows = 19
			dst := alloc((n-1)*rows + 6)
			for i := range dst {
				dst[i] = 12345
			}
			wideScatter(w, p, dst, cols, nt, rows, n)
			for r := 0; r < 6; r++ {
				for tok := 0; tok < nt; tok++ {
					var want float64
					if tok < n {
						for col := 0; col < cols; col++ {
							want += float64(w[r*cols+col]) * float64(x[tok*cols+col])
						}
					}
					if math.IsNaN(float64(out[r*nt+tok])) || math.Abs(float64(out[r*nt+tok])-want) > 2e-5*math.Max(1, math.Abs(want)) {
						t.Fatal("rows", cols, n, r, tok)
					}
					if tok < n && out[r*nt+tok] != dst[tok*rows+r] {
						t.Fatal("scatter", cols, n, r, tok)
					}
				}
			}
			for tok := 0; tok < n-1; tok++ {
				for r := 6; r < rows; r++ {
					if dst[tok*rows+r] != 12345 {
						t.Fatal("gap")
					}
				}
			}
		}
	}
}
func TestWideOddBounds(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	for _, cols := range []int{1, 7} {
		const nt = 64
		w, p, out := make([]float32, 6*cols), make([]float32, nt*cols), make([]float32, 6*nt)
		for i := range w {
			w[i] = float32(i+1) / 7
		}
		for i := range p {
			p[i] = float32(i%11-5) / 11
		}
		wideRows(w, p, out, cols, nt)
		for valid := 1; valid <= 7; valid++ {
			dst := make([]float32, (valid-1)*19+6)
			wideScatter(w, p, dst, cols, nt, 19, valid)
			for r := 0; r < 6; r++ {
				for tok := 0; tok < valid; tok++ {
					if dst[tok*19+r] != out[r*nt+tok] {
						t.Fatal(cols, valid, r, tok)
					}
				}
			}
		}
	}
	for _, f := range []func(){func() { packWide(nil, nil, 0, 8, 16) }, func() { wideRows(nil, nil, nil, 1, 16) }, func() { wideScatter(make([]float32, 6), make([]float32, 16), make([]float32, 5), 1, 16, 6, 1) }, func() { wideScatter(make([]float32, 6), make([]float32, 16), make([]float32, 6), 1, 16, 6, 17) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid bounds")
				}
			}()
			f()
		}()
	}
}

func TestWideSmallTailGuardOracle(t *testing.T) {
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
	for _, cols := range []int{1, 7} {
		const nt = 64
		w, p, out := alloc(6*cols), alloc(nt*cols), alloc(6*nt)
		for i := range w {
			w[i] = float32(i+1) / 7
		}
		for i := range p {
			p[i] = float32(i%11-5) / 11
		}
		wideRows(w, p, out, cols, nt)
		for valid := 1; valid <= 7; valid++ {
			dst := alloc((valid-1)*19 + 6)
			wideScatter(w, p, dst, cols, nt, 19, valid)
			for r := 0; r < 6; r++ {
				for tok := 0; tok < valid; tok++ {
					var want float64
					for col := 0; col < cols; col++ {
						want += float64(w[r*cols+col]) * float64(p[(tok/16)*cols*16+col*16+tok%16])
					}
					got := float64(dst[tok*19+r])
					if math.IsNaN(got) || math.Abs(got-want) > 2e-5*math.Max(1, math.Abs(want)) {
						t.Fatal(cols, valid, r, tok)
					}
					if out[r*nt+tok] != dst[tok*19+r] {
						t.Fatal("rows/scatter mismatch")
					}
				}
			}
		}
	}
}
