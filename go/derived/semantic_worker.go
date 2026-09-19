package derived

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type SemanticWorkerState struct {
	Alive, Running, Pending             bool
	LastError, PauseReason, CurrentPath *string
	Completed                           int
}
type SemanticRefreshIndex interface {
	PendingEmbeddingPaths(context.Context, int) ([]string, error)
	RefreshEmbeddingPaths(context.Context, string, []string, SemanticRefreshConfig, SemanticClient) error
}
type SemanticWorker struct {
	Index                     SemanticRefreshIndex
	Client                    SemanticClient
	Config                    SemanticRefreshConfig
	mu                        sync.Mutex
	wake                      chan struct{}
	done                      chan struct{}
	closed, running, full     bool
	root, revision            string
	paths                     []string
	lastError, pause, current *string
	completed                 int
}

func NewSemanticWorker(index SemanticRefreshIndex, client SemanticClient, config SemanticRefreshConfig) *SemanticWorker {
	w := &SemanticWorker{Index: index, Client: client, Config: config, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go w.loop()
	return w
}
func (w *SemanticWorker) Enqueue(root, revision string, paths []string, full bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return false
	}
	w.root, w.revision = root, revision
	if full {
		w.full = true
	}
	seen := map[string]bool{}
	for _, path := range w.paths {
		seen[path] = true
	}
	for _, path := range paths {
		if !seen[path] {
			w.paths = append(w.paths, path)
			seen[path] = true
		}
	}
	select {
	case w.wake <- struct{}{}:
	default:
	}
	return true
}
func (w *SemanticWorker) State() SemanticWorkerState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return SemanticWorkerState{Alive: !w.closed, Running: w.running, Pending: w.full || len(w.paths) > 0, LastError: copyWorkerText(w.lastError), PauseReason: copyWorkerText(w.pause), CurrentPath: copyWorkerText(w.current), Completed: w.completed}
}
func copyWorkerText(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func (w *SemanticWorker) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		<-w.done
		return
	}
	w.closed = true
	w.paths = nil
	w.full = false
	select {
	case w.wake <- struct{}{}:
	default:
	}
	w.mu.Unlock()
	<-w.done
}
func (w *SemanticWorker) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		state := w.State()
		if !state.Running && !state.Pending {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (w *SemanticWorker) loop() {
	defer close(w.done)
	for {
		w.mu.Lock()
		if w.closed {
			w.running = false
			w.current = nil
			w.mu.Unlock()
			return
		}
		revision := w.revision
		var path string
		if len(w.paths) > 0 {
			path = w.paths[0]
		} else if w.full {
			path = "*"
		}
		w.mu.Unlock()
		if path == "" {
			<-w.wake
			continue
		}
		if path == "*" {
			pending, err := w.Index.PendingEmbeddingPaths(context.Background(), 1)
			if err != nil {
				w.fail(err, "database-busy")
				time.Sleep(time.Millisecond)
				continue
			}
			if len(pending) == 0 {
				w.mu.Lock()
				w.full = false
				w.mu.Unlock()
				continue
			}
			path = pending[0]
		}
		w.mu.Lock()
		w.running = true
		w.current = &path
		w.pause = nil
		w.mu.Unlock()
		err := w.Index.RefreshEmbeddingPaths(context.Background(), revision, []string{path}, w.Config, w.Client)
		w.mu.Lock()
		w.running = false
		w.current = nil
		if err != nil {
			message := err.Error()
			if len([]rune(message)) > 500 {
				message = string([]rune(message)[:500])
			}
			w.lastError = &message
			reason := "error"
			if transientSemanticError(err) {
				reason = "database-busy"
			}
			w.pause = &reason
			if !transientSemanticError(err) && len(w.paths) > 0 && w.paths[0] == path {
				w.paths = w.paths[1:]
			}
		} else {
			w.lastError = nil
			w.pause = nil
			w.completed++
			if len(w.paths) > 0 && w.paths[0] == path {
				w.paths = w.paths[1:]
			}
		}
		w.mu.Unlock()
		if err != nil {
			time.Sleep(time.Millisecond)
		}
	}
}
func (w *SemanticWorker) fail(err error, reason string) {
	w.mu.Lock()
	message := err.Error()
	w.lastError = &message
	w.pause = &reason
	w.mu.Unlock()
}
func transientSemanticError(err error) bool {
	var unavailable *UnavailableError
	if errors.As(err, &unavailable) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "database is locked") || strings.Contains(text, "database is busy") || strings.Contains(text, "database table is locked")
}
