package graphdebug

import (
	"context"
	"sort"
	"sync"

	"github.com/rcarmo/memento/go/access"
)

type RefreshWorkerState struct {
	Alive, Running, Pending             bool
	LastError, PauseReason, CurrentPath *string
	Completed                           int
}
type RefreshWorker interface {
	Enqueue(repositoryRoot, revision string, paths []string, full bool) bool
	State() RefreshWorkerState
}
type RefreshState struct {
	Available          bool    `json:"available"`
	Running            bool    `json:"running"`
	Pending            bool    `json:"pending"`
	LastError          *string `json:"last_error"`
	LastScope          *string `json:"last_scope"`
	QueuedPaths        int     `json:"queued_paths"`
	RepositoryRevision *string `json:"repository_revision"`
	PauseReason        *string `json:"pause_reason"`
	CurrentPath        *string `json:"current_path"`
	Completed          int     `json:"completed"`
	Alive              bool    `json:"alive"`
}
type RefreshSnapshots interface {
	Overview(context.Context, *access.EffectivePolicy, OverviewOptions) (Overview, error)
	PathsForIDs(context.Context, []string) ([]string, error)
}

type RefreshCoordinator struct {
	Service                                     RefreshSnapshots
	Worker                                      RefreshWorker
	RepositoryRoot                              string
	RefreshMaxPaths, DirectNodeLimit, EdgeLimit int
	lastScope                                   *string
	queuedPaths                                 int
	repositoryRevision                          *string
	mu                                          sync.RWMutex
}

func (c *RefreshCoordinator) State() RefreshState {
	c.mu.RLock()
	lastScope, queued, revision := copyText(c.lastScope), c.queuedPaths, copyText(c.repositoryRevision)
	c.mu.RUnlock()
	if c.Worker == nil {
		return RefreshState{LastScope: lastScope, QueuedPaths: queued, RepositoryRevision: revision}
	}
	state := c.Worker.State()
	return RefreshState{Available: state.Alive, Alive: state.Alive, Running: state.Running, Pending: state.Pending, LastError: copyText(state.LastError), LastScope: lastScope, QueuedPaths: queued, RepositoryRevision: revision, PauseReason: copyText(state.PauseReason), CurrentPath: copyText(state.CurrentPath), Completed: state.Completed}
}
func copyText(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func (c *RefreshCoordinator) Enqueue(ctx context.Context, scope string, conceptIDs []string, confirmFull bool) (RefreshState, error) {
	if c.Worker == nil {
		return RefreshState{}, &SnapshotError{"semantic embedding refresh is unavailable"}
	}
	overview, err := c.Service.Overview(ctx, nil, OverviewOptions{DirectNodeLimit: c.DirectNodeLimit, EdgeLimit: c.EdgeLimit, RefreshMaxPaths: c.RefreshMaxPaths})
	if err != nil {
		return RefreshState{}, err
	}
	revision := overview.Revisions.Repository
	var paths []string
	full := false
	queued := 0
	switch scope {
	case "full":
		if !confirmFull {
			return RefreshState{}, &SnapshotError{"full embedding refresh requires confirmation"}
		}
		full = true
		queued = overview.Metrics.MemoryCount
	case "selected", "visible":
		if len(conceptIDs) == 0 {
			return RefreshState{}, &SnapshotError{scope + " embedding refresh requires concept ids"}
		}
		uniqueMap := map[string]bool{}
		for _, id := range conceptIDs {
			uniqueMap[id] = true
		}
		unique := make([]string, 0, len(uniqueMap))
		for id := range uniqueMap {
			unique = append(unique, id)
		}
		sort.Strings(unique)
		if len(unique) > c.RefreshMaxPaths {
			return RefreshState{}, &SnapshotError{"embedding refresh exceeds configured path limit"}
		}
		paths, err = c.Service.PathsForIDs(ctx, unique)
		if err != nil {
			return RefreshState{}, err
		}
		if len(paths) != len(unique) {
			return RefreshState{}, &SnapshotError{"embedding refresh includes unknown memories"}
		}
		queued = len(paths)
	default:
		return RefreshState{}, &SnapshotError{"unsupported embedding refresh scope"}
	}
	if !c.Worker.Enqueue(c.RepositoryRoot, revision, paths, full) {
		return RefreshState{}, &SnapshotError{"embedding refresh worker is closed or stopped"}
	}
	scopeCopy, revisionCopy := scope, revision
	c.mu.Lock()
	c.lastScope = &scopeCopy
	c.queuedPaths = queued
	c.repositoryRevision = &revisionCopy
	c.mu.Unlock()
	return c.State(), nil
}
