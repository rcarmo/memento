package service

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/rcarmo/memento/go/repository"
)

// Runtime owns resources shared by the configured service. Close drains
// shielded requests before closing background components, SQLite, and finally
// the process-wide writer lease.
type Runtime struct {
	Jobs    *Jobs
	DB      *sql.DB
	Lease   *repository.WriterLease
	Closers []func() error

	mu     sync.Mutex
	closed bool
}

func (r *Runtime) Close(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	r.mu.Unlock()

	var result error
	if r.Jobs != nil {
		result = errors.Join(result, r.Jobs.Workers.Drain(ctx))
	}
	for index := len(r.Closers) - 1; index >= 0; index-- {
		if r.Closers[index] != nil {
			result = errors.Join(result, r.Closers[index]())
		}
	}
	if r.DB != nil {
		result = errors.Join(result, r.DB.Close())
	}
	if r.Lease != nil {
		result = errors.Join(result, r.Lease.Release())
	}
	return result
}
