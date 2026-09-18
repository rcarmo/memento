package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcarmo/memento/go/umcp"
)

// Workers mirrors the MCP server's two-slot admission and shielded _memory_call
// jobs. A timeout/cancel ends the wait, not the admitted work. Drain permanently
// closes admission and waits for all jobs; the daemon must call it on shutdown.
// Handlers must own/close per-job SQLite connections and use original retry keys.
type Workers struct {
	mu          sync.Mutex
	closing     bool
	executeBusy bool
	active      map[chan struct{}]struct{}
	timeout     time.Duration // zero uses the source 30-second deadline
}
type workerResult struct {
	value any
	err   error
}

func (w *Workers) Call(ctx context.Context, method string, work func(context.Context) (any, error)) (any, error) {
	w.mu.Lock()
	if w.closing || len(w.active) >= 2 || (w.executeBusy && method != "memory_operation_get") {
		w.mu.Unlock()
		return workerFailure("busy", "service busy; reconcile before retrying"), nil
	}
	ownsAdmission := method != "memory_operation_get"
	if ownsAdmission {
		w.executeBusy = true
	}
	if w.active == nil {
		w.active = map[chan struct{}]struct{}{}
	}
	done := make(chan struct{})
	w.active[done] = struct{}{}
	timeout := w.timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	w.mu.Unlock()
	results := make(chan workerResult, 1)
	// Asyncio shields the thread even when the request task is cancelled. Keep
	// identity/context values but detach Go deadline/cancellation from the work.
	go func() {
		result := workerResult{}
		defer func() {
			if recovered := recover(); recovered != nil {
				result.err = &umcp.ExecutionError{Type: "RuntimeError", Message: fmt.Sprintf("worker panic: %v", recovered)}
			}
			w.mu.Lock()
			delete(w.active, done)
			if ownsAdmission {
				w.executeBusy = false
			}
			close(done)
			w.mu.Unlock()
			results <- result
		}()
		result.value, result.err = work(context.WithoutCancel(ctx))
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-results:
		return result.value, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return workerFailure("indeterminate", "outcome indeterminate; reconcile original idempotency key before retrying"), nil
	}
}

func workerFailure(class, message string) map[string]any {
	return map[string]any{"status": "error", "error_class": class, "message": message, "warnings": []any{}, "index_revision": nil, "repo_revision": nil, "index_stale": false, "operation_id": nil}
}

// SetExecuteBusy is the admission interlock for the separately ported execute
// dispatcher. It does not run plans or grant a third slot for reconciliation.
func (w *Workers) SetExecuteBusy(busy bool) { w.mu.Lock(); w.executeBusy = busy; w.mu.Unlock() }
func (w *Workers) Drain(ctx context.Context) error {
	w.mu.Lock()
	w.closing = true
	pending := make([]chan struct{}, 0, len(w.active))
	for done := range w.active {
		pending = append(pending, done)
	}
	w.mu.Unlock()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
