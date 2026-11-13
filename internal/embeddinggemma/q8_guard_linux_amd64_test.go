//go:build linux && amd64

package embeddinggemma

import (
	"encoding/binary"
	"os"
	"testing"
	"unsafe"

	"github.com/rcarmo/memento/internal/simd"
	"golang.org/x/sys/unix"
)

// The row kernels must not widen their last quantized or activation load past
// the exact 34-byte Q8_0 block and 32 float32 activation values.
func TestQ8SIMDGuardPages(t *testing.T) {
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
	rawPage, xPage := alloc(), alloc()
	raw := rawPage[len(rawPage)-34:]
	binary.LittleEndian.PutUint16(raw, 0x3c00)
	for i := 0; i < 32; i++ {
		raw[i+2] = byte(int8(i - 16))
	}
	x := unsafe.Slice((*float32)(unsafe.Pointer(&xPage[len(xPage)-128])), 32)
	for i := range x {
		x[i] = float32(i-16) / 8
	}
	want := q8ReferenceDot(raw, x)
	for _, name := range []string{"sse2", "avx2"} {
		engine, err := simd.New(name)
		if err != nil {
			continue
		}
		got, ok := fusedQ8Dot(raw, x, engine.Backend())
		if !ok {
			t.Fatalf("%s did not fuse", name)
		}
		requireQ8Close(t, got, want)
	}
}
