//go:build linux && amd64

package embeddinggemma

import (
	"fmt"
	"os"
	"testing"
	"unsafe"

	"github.com/rcarmo/memento/internal/simd"
	"golang.org/x/sys/unix"
)

// Exact raw/dst slices abut inaccessible pages, at both boundaries in separate
// runs. Multiple blocks and 2048-float destinations cross writable pages.
func TestExtraDecodeGuardPages(t *testing.T) {
	if _, err := simd.New("avx2"); err != nil {
		t.Skip(err)
	}
	alloc := func(t *testing.T, n int, atEnd bool) []byte {
		t.Helper()
		page := os.Getpagesize()
		usable := (n + page - 1) / page * page
		b, err := unix.Mmap(-1, 0, usable+2*page, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := unix.Munmap(b); err != nil {
				t.Error(err)
			}
		})
		for _, guard := range [][]byte{b[:page], b[page+usable:]} {
			if err := unix.Mprotect(guard, unix.PROT_NONE); err != nil {
				t.Fatal(err)
			}
		}
		start := page
		if atEnd {
			start += usable - n
		}
		return b[start : start+n : start+n]
	}
	for _, typ := range []Type{Q5_K, Q8_0} {
		for _, cols := range extraDecodeColumns(typ) {
			for _, atEnd := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/end=%t", typ, cols, atEnd), func(t *testing.T) {
					fixture := encodedRows(typ, cols, 1)
					raw := alloc(t, len(fixture), atEnd)
					copy(raw, fixture)
					mem := alloc(t, cols*4, atEnd)
					dst := unsafe.Slice((*float32)(unsafe.Pointer(&mem[0])), cols)
					checkExtraDecode(t, raw, typ, dst, simd.AVX2)
				})
			}
		}
	}
}
