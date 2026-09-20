package repository

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
)

type WriterLeaseError struct{ Owner string }

func (e *WriterLeaseError) Error() string { return "writer lease already held by " + e.Owner }

// WriterLease owns the advisory flock on an open file description. It does not
// unlink the file: another process may have opened the same inode already.
// The owner line is diagnostic only, never an authentication credential.
type WriterLease struct {
	Path, Owner string
	mu          sync.Mutex
	handle      leaseHandle
}
type leaseHandle interface {
	io.ReadSeekCloser
	Fd() uintptr
	Truncate(int64) error
	WriteString(string) (int, error)
}

func AcquireWriterLease(path, owner string) (*WriterLease, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return nil, err
	}
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	return acquireWriterHandle(handle, path, owner, syscall.Flock)
}
func acquireWriterHandle(handle leaseHandle, path, owner string, flock func(int, int) error) (*WriterLease, error) {
	if err := flock(int(handle.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		defer handle.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return nil, err
		}
		if _, err = handle.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(handle)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(raw) {
			return nil, fmt.Errorf("invalid UTF-8 in writer lease")
		}
		current := strings.TrimFunc(string(raw), conceptSpace)
		if current == "" {
			current = "unknown writer"
		}
		return nil, &WriterLeaseError{Owner: current}
	}
	// Every setup failure closes the acquired description so a failed start
	// cannot strand the process's lock. Ownership is transferred only on success.
	if _, err := handle.Seek(0, io.SeekStart); err != nil {
		_ = handle.Close()
		return nil, err
	}
	if err := handle.Truncate(0); err != nil {
		_ = handle.Close()
		return nil, err
	}
	if _, err := handle.WriteString(owner + "\n"); err != nil {
		_ = handle.Close()
		return nil, err
	}
	return &WriterLease{Path: path, Owner: owner, handle: handle}, nil
}
func (l *WriterLease) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return releaseWriterHandle(l.handle, syscall.Flock)
}
func releaseWriterHandle(handle leaseHandle, flock func(int, int) error) error {
	if err := flock(int(handle.Fd()), syscall.LOCK_UN); err != nil {
		return err
	}
	return handle.Close()
}
