//go:build linux && amd64

package embeddinggemma

import (
	"os"
	"testing"
	"unsafe"

	"github.com/rcarmo/memento/internal/simd"
	"golang.org/x/sys/unix"
)

// The packed values, high-bit plane and activations all terminate at a guard
// page. This catches wide loads that cross the Q5_K group boundaries.
func TestQ5SIMDGuardPages(t *testing.T) {
	size := os.Getpagesize()
	alloc := func() []byte {
		b, err := unix.Mmap(-1, 0, size*2, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
		if err != nil {
			t.Fatal(err)
		}
		if err = unix.Mprotect(b[size:], unix.PROT_NONE); err != nil {
			_ = unix.Munmap(b)
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := unix.Munmap(b); err != nil {
				t.Error(err)
			}
		})
		return b[:size]
	}
	qpage, hpage, xpage := alloc(), alloc(), alloc()
	q := qpage[len(qpage)-32:]
	high := hpage[len(hpage)-32:]
	x := unsafe.Slice((*float32)(unsafe.Pointer(&xpage[len(xpage)-128])), 32)
	for i := range q {
		q[i] = byte(i*7) | byte((i*11)&15)<<4
		x[i] = float32(i-16) / 8
	}
	for _, name := range []string{"sse2", "avx2"} {
		if _, err := simd.New(name); err != nil {
			continue
		}
		for g := 0; g < 8; g++ {
			for i := range high {
				high[i] = 0
				if i&1 != 0 {
					high[i] = 1 << g
				}
			}
			for _, highNibble := range []bool{false, true} {
				var want float32
				for i := 0; i < 32; i++ {
					v := q[i] & 15
					if highNibble {
						v = q[i] >> 4
					}
					if high[i]&(1<<g) != 0 {
						v |= 16
					}
					want += (.25*float32(v) - .125) * x[i]
				}
				var got float32
				if name == "sse2" {
					got = q5GroupSSE2(&q[0], &high[0], &x[0], .25, .125, highNibble, 1<<g)
				} else {
					got = q5GroupAVX2(&q[0], &high[0], &x[0], .25, .125, highNibble, 1<<g)
				}
				if got != want {
					t.Fatal(name, g, highNibble, got, want)
				}
			}
		}
	}
}
