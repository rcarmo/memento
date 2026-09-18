package graphdebug

import (
	"context"
	"testing"
)

type refreshWorker struct {
	accepted       bool
	state          RefreshWorkerState
	root, revision string
	paths          []string
	full           bool
}

func (w *refreshWorker) Enqueue(root, revision string, paths []string, full bool) bool {
	w.root, w.revision, w.paths, w.full = root, revision, paths, full
	return w.accepted
}
func (w *refreshWorker) State() RefreshWorkerState { return w.state }
func TestRefreshCoordinator(t *testing.T) {
	root, path := nodeDB(t)
	worker := &refreshWorker{accepted: true, state: RefreshWorkerState{Alive: true, Running: true, Completed: 2}}
	c := RefreshCoordinator{Service: NewSnapshotService(root, path, emptyControlDB(t)), Worker: worker, RepositoryRoot: root, RefreshMaxPaths: 10, DirectNodeLimit: 10, EdgeLimit: 10}
	state, err := c.Enqueue(context.Background(), "selected", []string{"6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e", "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e"}, false)
	if err != nil || !state.Available || state.QueuedPaths != 2 || len(worker.paths) != 2 || worker.full {
		t.Fatal(state, worker, err)
	}
	state, err = c.Enqueue(context.Background(), "full", nil, true)
	if err != nil || !worker.full || state.QueuedPaths != 2 {
		t.Fatal(state, err)
	}
	if new(RefreshCoordinator).State().Available {
		t.Fatal("nil state")
	}
}
func TestRefreshGuards(t *testing.T) {
	root, path := nodeDB(t)
	service := NewSnapshotService(root, path, emptyControlDB(t))
	for _, tc := range []struct {
		scope   string
		ids     []string
		confirm bool
		max     int
	}{{"full", nil, false, 10}, {"selected", nil, false, 10}, {"visible", []string{"a", "b"}, false, 1}, {"selected", []string{"missing"}, false, 10}, {"bad", nil, false, 10}} {
		c := RefreshCoordinator{Service: service, Worker: &refreshWorker{accepted: true}, RefreshMaxPaths: tc.max, DirectNodeLimit: 10, EdgeLimit: 10}
		if _, err := c.Enqueue(context.Background(), tc.scope, tc.ids, tc.confirm); err == nil {
			t.Fatal(tc)
		}
	}
	c := RefreshCoordinator{Service: service, RefreshMaxPaths: 10, DirectNodeLimit: 10, EdgeLimit: 10}
	if _, err := c.Enqueue(context.Background(), "full", nil, true); err == nil {
		t.Fatal("nil worker")
	}
	c.Worker = &refreshWorker{}
	if _, err := c.Enqueue(context.Background(), "full", nil, true); err == nil {
		t.Fatal("rejected")
	}
}
