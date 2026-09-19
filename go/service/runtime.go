package service

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/graphdebug"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

// Runtime owns resources shared by the configured service. Close drains
// shielded requests before closing background components, SQLite, and finally
// the process-wide writer lease.
type Runtime struct {
	Paths                 RuntimePaths
	Jobs                  *Jobs
	SemanticWorker        *derived.SemanticWorker
	GraphRefresh          *graphdebug.RefreshCoordinator
	DB                    *sql.DB
	Lease                 *repository.WriterLease
	HTTPHooks             umcp.HTTPHooks
	Closers               []func() error
	AuditPrincipals       func(context.Context) ([]AuditPrincipal, error)
	ProtectedReadPrefixes []string

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
