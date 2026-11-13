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

func TestPackedKernelGuardPages(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	page := os.Getpagesize()
	floats := func(n int) []float32 {
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
	for _, cols := range []int{1, 7, 256, 512, 2048} {
		for _, tokens := range []int{8, 16, 64} {
			w, p, out := floats(8*cols), floats(cols*tokens), floats(8*tokens)
			for i := range w {
				w[i] = float32(i%23-11) / 23
			}
			for i := range p {
				p[i] = float32(i%31-15) / 31
			}
			packedRows(w, p, out, cols, tokens)
			for row := 0; row < 8; row++ {
				for tok := 0; tok < tokens; tok++ {
					var want float64
					for col := 0; col < cols; col++ {
						want += float64(w[row*cols+col]) * float64(p[col*tokens+tok])
					}
					if math.Abs(float64(out[row*tokens+tok])-want) > 2e-5*math.Max(1, math.Abs(want)) {
						t.Fatal(cols, tokens, row, tok, out[row*tokens+tok], want)
					}
				}
			}
		}
	}
}
