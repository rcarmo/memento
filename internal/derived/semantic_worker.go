package derived

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type SemanticWorkerPolicy struct {
	Enabled                              bool
	StartupDelay, InteractiveIdle, Delay time.Duration
	CPUBusyLimit                         float64
}
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
	Policy                    SemanticWorkerPolicy
	IdleSeconds               func() time.Duration
	CPUUsage                  func() *float64
	Now                       func() time.Time
	mu                        sync.Mutex
	wake                      chan struct{}
	done                      chan struct{}
	closed, running, full     bool
	root, revision            string
	paths                     []string
	lastError, pause, current *string
	completed                 int
	started, lastCompleted    time.Time
	cancel                    context.CancelFunc
	generation                uint64
	pathGeneration            map[string]uint64
	fullGeneration            uint64
	ctx                       context.Context
	stop                      context.CancelFunc
}

func NewSemanticWorker(index SemanticRefreshIndex, client SemanticClient, config SemanticRefreshConfig) *SemanticWorker {
	return NewProgressiveSemanticWorker(index, client, config, SemanticWorkerPolicy{}, nil, nil, time.Now)
}
func NewProgressiveSemanticWorker(index SemanticRefreshIndex, client SemanticClient, config SemanticRefreshConfig, policy SemanticWorkerPolicy, idle func() time.Duration, cpu func() *float64, now func() time.Time) *SemanticWorker {
	if now == nil {
		now = time.Now
	}
	w := &SemanticWorker{Index: index, Client: client, Config: config, Policy: policy, IdleSeconds: idle, CPUUsage: cpu, Now: now, wake: make(chan struct{}, 1), done: make(chan struct{})}
	w.ctx, w.stop = context.WithCancel(context.Background())
	w.started = now()
	if policy.Enabled {
		reason := "startup"
		w.pause = &reason
	}
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
	w.generation++
	if w.pathGeneration == nil {
		w.pathGeneration = map[string]uint64{}
	}
	if full {
		w.full = true
		w.fullGeneration = w.generation
	}
	seen := map[string]bool{}
	for _, path := range w.paths {
		seen[path] = true
	}
	for _, path := range paths {
		w.pathGeneration[path] = w.generation
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
	w.stop()
	if w.cancel != nil {
		w.cancel()
	}
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
		generation := w.generation
		fullGeneration := w.fullGeneration
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
		fromFull := path == "*"
		if fromFull {
			pending, err := w.Index.PendingEmbeddingPaths(w.ctx, 1)
			if err != nil {
				w.fail(err, "database-busy")
				w.waitWake(250 * time.Millisecond)
				continue
			}
			if len(pending) == 0 {
				w.mu.Lock()
				if w.fullGeneration == fullGeneration {
					w.full = false
				}
				w.mu.Unlock()
				continue
			}
			path = pending[0]
		}
		w.mu.Lock()
		if w.closed {
			w.mu.Unlock()
			return
		}
		if w.generation != generation {
			w.mu.Unlock()
			continue
		}
		if reason, wait := w.pauseForWork(); reason != "" {
			w.pause = &reason
			w.mu.Unlock()
			w.waitWake(wait)
			continue
		}
		w.running = true
		w.current = &path
		w.pause = nil
		ctx, cancel := context.WithCancel(context.Background())
		w.cancel = cancel
		config := w.Config
		if w.Policy.Enabled {
			config.BeforeBatch = w.admitChunkBatch
		}
		w.mu.Unlock()
		err := w.Index.RefreshEmbeddingPaths(ctx, revision, []string{path}, config, w.Client)
		cancel()
		w.mu.Lock()
		w.cancel = nil
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
			if !transientSemanticError(err) {
				if fromFull && w.fullGeneration == fullGeneration {
					w.full = false
				}
				if len(w.paths) > 0 && w.paths[0] == path && w.pathGeneration[path] <= generation {
					w.paths = w.paths[1:]
					delete(w.pathGeneration, path)
				}
			}
		} else {
			w.lastError = nil
			w.pause = nil
			w.completed++
			w.lastCompleted = w.Now()
			if len(w.paths) > 0 && w.paths[0] == path && w.pathGeneration[path] <= generation {
				w.paths = w.paths[1:]
				delete(w.pathGeneration, path)
			}
		}
		w.mu.Unlock()
		if err != nil && transientSemanticError(err) {
			w.waitWake(250 * time.Millisecond)
		}
	}
}

// admitChunkBatch rechecks cancellation, interactive idle and CPU pressure
// between batches of one document. Entry pacing belongs only between documents.
func (w *SemanticWorker) admitChunkBatch(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		w.mu.Lock()
		reason, wait := w.pauseForBatch()
		if reason == "" {
			w.pause = nil
		} else {
			w.pause = &reason
		}
		w.mu.Unlock()
		if reason == "" {
			return nil
		}
		w.waitWake(wait)
	}
}

func (w *SemanticWorker) pauseForWork() (string, time.Duration) {
	if !w.Policy.Enabled {
		return "", 0
	}
	now := w.Now()
	if remaining := w.Policy.StartupDelay - now.Sub(w.started); remaining > 0 {
		return "startup", remaining
	}
	if reason, wait := w.pauseForBatch(); reason != "" {
		return reason, wait
	}
	if !w.lastCompleted.IsZero() {
		if remaining := w.Policy.Delay - now.Sub(w.lastCompleted); remaining > 0 {
			return "pacing", remaining
		}
	}
	return "", 0
}

func (w *SemanticWorker) pauseForBatch() (string, time.Duration) {
	if !w.Policy.Enabled {
		return "", 0
	}
	if w.IdleSeconds != nil {
		if remaining := w.Policy.InteractiveIdle - w.IdleSeconds(); remaining > 0 {
			return "interactive", remaining
		}
	}
	if w.CPUUsage != nil {
		cpu := w.CPUUsage()
		if cpu == nil {
			return "cpu-sampling", time.Second
		}
		if *cpu > w.Policy.CPUBusyLimit {
			return "cpu", time.Second
		}
	}
	return "", 0
}
func (w *SemanticWorker) waitWake(wait time.Duration) {
	if wait < 10*time.Millisecond {
		wait = 10 * time.Millisecond
	}
	timer := time.NewTimer(min(wait, time.Second))
	defer timer.Stop()
	select {
	case <-w.wake:
	case <-timer.C:
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
