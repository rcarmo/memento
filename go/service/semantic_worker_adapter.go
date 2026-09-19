package service

import (
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/graphdebug"
)

type SemanticRefreshAdapter struct{ Worker *derived.SemanticWorker }

func (a SemanticRefreshAdapter) Enqueue(root, revision string, paths []string, full bool) bool {
	if a.Worker == nil {
		return false
	}
	return a.Worker.Enqueue(root, revision, paths, full)
}
func (a SemanticRefreshAdapter) State() graphdebug.RefreshWorkerState {
	if a.Worker == nil {
		return graphdebug.RefreshWorkerState{}
	}
	state := a.Worker.State()
	return graphdebug.RefreshWorkerState{Alive: state.Alive, Running: state.Running, Pending: state.Pending, LastError: state.LastError, PauseReason: state.PauseReason, CurrentPath: state.CurrentPath, Completed: state.Completed}
}
