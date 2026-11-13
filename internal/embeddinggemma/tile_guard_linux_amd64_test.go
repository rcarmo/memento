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

func TestQ4TileGuardPages(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	size := os.Getpagesize()
	alloc := func() []byte {
		b, err := unix.Mmap(-1, 0, 2*size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
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
	page := alloc()
	raw := page[len(page)-144:]
	copy(raw, encodedRows(Q4K, 256, 1))
	xp := alloc()
	x := unsafe.Slice((*float32)(unsafe.Pointer(&xp[len(xp)-4096])), 1024)
	for i := range x {
		x[i] = float32(i%29-14) / 9
	}
	dp := alloc()
	dst := unsafe.Slice((*float32)(unsafe.Pointer(&dp[len(dp)-1024])), 256)
	if err := decodeTileRow(dst, raw, Q4K, simd.AVX2); err != nil {
		t.Fatal(err)
	}
	ref := referenceRow(raw, Q4K, 256)
	for i, v := range ref {
		if math.Abs(float64(dst[i])-v) > 1e-6*math.Max(1, math.Abs(v)) {
			t.Fatal(i)
		}
	}
	sp := alloc()
	sums := unsafe.Slice((*float32)(unsafe.Pointer(&sp[len(sp)-16])), 4)
	if !projectFewQ4(raw, x, sums, 4, 256, simd.AVX2) {
		t.Fatal("not used")
	}
	for tok := 0; tok < 4; tok++ {
		var want float64
		for i, v := range ref {
			want += v * float64(x[tok*256+i])
		}
		if math.Abs(float64(sums[tok])-want) > 2e-5*math.Max(1, math.Abs(want)) {
			t.Fatal(tok, sums[tok], want)
		}
	}
}
