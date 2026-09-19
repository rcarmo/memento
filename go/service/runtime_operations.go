package service

import (
	"context"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
)

type runtimeStatusOps struct {
	revision  func(repository.GitRepositoryPaths) (string, error)
	state     func(context.Context, *derived.Index) (derived.IndexState, error)
	scan      func(string, repository.BundleFilter) (repository.RepositoryBundle, error)
	proposals func(context.Context, *control.Proposals) ([]control.ProposalRecord, error)
	embedding func(context.Context, *derived.Index) (string, error)
}

func defaultRuntimeStatusOps() runtimeStatusOps {
	return runtimeStatusOps{repository.GetMainRevision, func(ctx context.Context, index *derived.Index) (derived.IndexState, error) { return index.State(ctx) }, repository.ScanBundle, func(ctx context.Context, p *control.Proposals) ([]control.ProposalRecord, error) {
		return p.List(ctx, control.ProposalQuery{})
	}, func(ctx context.Context, index *derived.Index) (string, error) { return index.EmbeddingRevision(ctx) }}
}

// StatusSnapshot returns the process-level operational status used by the CLI.
func (r *Runtime) StatusSnapshot(ctx context.Context, schemaVersion int) (map[string]any, error) {
	return r.statusSnapshot(ctx, schemaVersion, defaultRuntimeStatusOps())
}
func (r *Runtime) statusSnapshot(ctx context.Context, schemaVersion int, ops runtimeStatusOps) (map[string]any, error) {
	revision, err := ops.revision(r.Paths.Repository)
	if err != nil {
		return nil, err
	}
	index := &derived.Index{Path: r.Paths.DerivedDB}
	state, err := ops.state(ctx, index)
	if err != nil {
		return nil, err
	}
	bundle, err := ops.scan(r.Paths.Repository.CurrentDir, repository.BundleFilter{})
	if err != nil {
		return nil, err
	}
	proposalStore := control.Proposals{DB: r.DB}
	proposals, err := ops.proposals(ctx, &proposalStore)
	if err != nil {
		return nil, err
	}
	backlog := 0
	for _, item := range proposals {
		if item.Status == control.Submitted || item.Status == control.Approved {
			backlog++
		}
	}
	capabilities := RuntimeCapabilities{}
	if r.Jobs != nil && r.Jobs.Controls != nil {
		capabilities = r.Jobs.Controls.RuntimeCapabilities
	}
	var embedding any = nil
	embeddingRevision, embeddingErr := ops.embedding(ctx, index)
	if capabilities.SemanticEnabled && embeddingErr == nil && embeddingRevision != "" && embeddingRevision != "disabled" {
		embedding = embeddingRevision
	}
	var worker any = nil
	if r.SemanticWorker != nil {
		s := r.SemanticWorker.State()
		worker = map[string]any{"alive": s.Alive, "running": s.Running, "pending": s.Pending, "pause_reason": s.PauseReason, "current_path": s.CurrentPath, "completed": s.Completed, "last_error": s.LastError}
	}
	warnings := []any{}
	ready := capabilities.SemanticEnabled && capabilities.SemanticLoaded && embeddingRevision == revision
	var model any = nil
	var dimensions any = nil
	if capabilities.SemanticEnabled {
		model = capabilities.SemanticModelID
		dimensions = capabilities.SemanticDimensions
	}
	var needleRuntime any = nil
	if capabilities.NeedleLoaded {
		needleRuntime = "go-scalar"
	}
	r.mu.Lock()
	closed := r.closed
	r.mu.Unlock()
	return map[string]any{"service_version": "0.5.9", "schema_version": schemaVersion, "repo_revision": revision, "index_revision": state.IndexRevision, "index_stale": state.IndexRevision != state.RepoRevision, "visible_concepts": len(bundle.Entries), "semantic_search": map[string]any{"enabled": capabilities.SemanticEnabled, "ready": ready, "model_id": model, "dimensions": dimensions, "embedding_revision": embedding, "sqlite_vector_enabled": false, "warnings": warnings, "worker": worker}, "needle_router": map[string]any{"enabled": capabilities.NeedleEnabled, "loaded": capabilities.NeedleLoaded, "runtime": needleRuntime, "model_path": capabilities.NeedleModelPath}, "proposal_backlog": backlog, "control_db": r.Paths.ControlDB, "derived_db": r.Paths.DerivedDB, "repo_root": r.Paths.Root, "closed": closed}, nil
}
