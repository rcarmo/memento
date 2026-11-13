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

// A read past the last group's exact 32 packed bytes or 32 activations faults.
// This supplements numerical tests; do not run on unsupported CPU instructions.
func TestQ4SIMDGuardPages(t *testing.T) {
	size := os.Getpagesize()
	alloc := func() []byte {
		b, err := unix.Mmap(-1, 0, size*2, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
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
		return b[:size]
	}
	qpage, xpage := alloc(), alloc()
	q := qpage[len(qpage)-32:]
	for i := range q {
		q[i] = byte(i * 7)
	}
	x := unsafe.Slice((*float32)(unsafe.Pointer(&xpage[len(xpage)-128])), 32)
	for i := range x {
		x[i] = float32(i-16) / 8
	}
	// Whole-row AVX2 must not read beyond the final packed block or activations.
	if _, err := simd.New("avx2"); err == nil {
		rpage, apage := alloc(), alloc()
		raw := rpage[len(rpage)-144:]
		for i := range raw {
			raw[i] = byte(i * 7)
		}
		raw[0], raw[1], raw[2], raw[3] = 0, 0x24, 0, 0x20
		activations := unsafe.Slice((*float32)(unsafe.Pointer(&apage[len(apage)-1024])), 256)
		for i := range activations {
			activations[i] = float32(i%19-9) / 8
		}
		var want float64
		decoded := referenceRow(raw, Q4K, len(activations))
		for i, a := range activations {
			want += decoded[i] * float64(a)
		}
		got, ok := fusedQ4Dot(raw, activations, simd.AVX2)
		if !ok || math.Abs(float64(got)-want) > 2e-5*math.Max(1, math.Abs(want)) {
			t.Fatal(got, want)
		}
	}
	for _, backend := range []string{"sse2", "avx2"} {
		if _, err := simd.New(backend); err != nil {
			continue
		}
		for _, high := range []bool{false, true} {
			var want float32
			for i := 0; i < 32; i++ {
				v := q[i] & 15
				if high {
					v = q[i] >> 4
				}
				want += (.25*float32(v) - .125) * x[i]
			}
			var got float32
			if backend == "sse2" {
				got = q4GroupSSE2(&q[0], &x[0], .25, .125, high)
			} else {
				got = q4GroupAVX2(&q[0], &x[0], .25, .125, high)
			}
			if got != want {
				t.Fatal(backend, high, got, want)
			}
		}
	}
}
