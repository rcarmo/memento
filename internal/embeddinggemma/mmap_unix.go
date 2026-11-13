//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package embeddinggemma

import (
	"golang.org/x/sys/unix"
	"os"
)

func mapReadOnly(f *os.File, n int) ([]byte, error) {
	return unix.Mmap(int(f.Fd()), 0, n, unix.PROT_READ, unix.MAP_PRIVATE)
}
func unmap(b []byte) error { return unix.Munmap(b) }
