//go:build unix

package needle

import (
	"os"
	"syscall"
)

type mmapFile interface {
	Fd() uintptr
	Stat() (os.FileInfo, error)
	Close() error
}

func mapReadOnly(path string) ([]byte, error) {
	return mapReadOnlyWith(path, func(path string) (mmapFile, error) { return os.Open(path) })
}
func mapReadOnlyWith(path string, open func(string) (mmapFile, error)) ([]byte, error) {
	file, err := open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return mapFileReadOnly(file, stat.Size())
}
func mapFileReadOnly(file mmapFile, size int64) ([]byte, error) {
	if size <= 0 {
		return nil, os.ErrInvalid
	}
	return syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
}
func unmapReadOnly(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return syscall.Munmap(data)
}
