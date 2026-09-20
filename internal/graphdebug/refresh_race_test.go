package graphdebug

import (
	"context"
	"sync"
	"testing"
)

type lockedRefreshWorker struct {
	mu    sync.Mutex
	state RefreshWorkerState
}

func (w *lockedRefreshWorker) Enqueue(string, string, []string, bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return true
}
func (w *lockedRefreshWorker) State() RefreshWorkerState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}
func TestRefreshConcurrentState(t *testing.T) {
	snapshot := refreshSnapshots{paths: map[string]string{"a": "/a"}, overview: Overview{Metrics: Metrics{MemoryCount: 1}, Revisions: Revisions{Repository: "main"}}}
	c := RefreshCoordinator{Service: snapshot, Worker: &lockedRefreshWorker{state: RefreshWorkerState{Alive: true}}, RefreshMaxPaths: 10}
	var wait sync.WaitGroup
	for i := 0; i < 20; i++ {
		wait.Add(2)
		go func() {
			defer wait.Done()
			if _, err := c.Enqueue(context.Background(), "selected", []string{"a"}, false); err != nil {
				t.Error(err)
			}
		}()
		go func() { defer wait.Done(); _ = c.State() }()
	}
	wait.Wait()
}
