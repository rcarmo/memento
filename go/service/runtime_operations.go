package service

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
)

type AuditPrincipal struct {
	Name                string
	Roles, ReadPrefixes []string
}

func StaticAuditPrincipals(config access.AuthorizationConfig) func(context.Context) ([]AuditPrincipal, error) {
	copy := copyAuthorization(config)
	return func(context.Context) ([]AuditPrincipal, error) {
		names := make([]string, 0, len(copy.Principals))
		for name := range copy.Principals {
			names = append(names, name)
		}
		sort.Strings(names)
		result := make([]AuditPrincipal, 0, len(names))
		for _, name := range names {
			policy := copy.Principals[name]
			result = append(result, AuditPrincipal{name, append([]string{}, policy.Roles...), append([]string{}, policy.ReadPrefixes...)})
		}
		return result, nil
	}
}
func (r *Runtime) AuditRepository(ctx context.Context, path *string) (map[string]any, error) {
	report, err := repository.AuditRepository(r.Paths.Repository.CurrentDir, nil)
	if err != nil {
		return nil, err
	}
	issues := []repository.AuditIssue{}
	for _, issue := range report.Issues {
		if path == nil || issue.BundlePath == *path {
			issues = append(issues, issue)
		}
	}
	principals := []AuditPrincipal{}
	if r.AuditPrincipals != nil {
		principals, err = r.AuditPrincipals(ctx)
		if err != nil {
			return nil, err
		}
	}
	warnings := []map[string]any{}
	for _, principal := range principals {
		if warning := access.BroadReadGrantWarning(principal.Roles, principal.ReadPrefixes, r.ProtectedReadPrefixes); warning != nil {
			warnings = append(warnings, map[string]any{"principal": principal.Name, "warning": *warning})
		}
	}
	return map[string]any{"ok": len(issues) == 0, "issues": issues, "warnings": warnings}, nil
}

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

type runtimeRebuildOps struct {
	revision func(repository.GitRepositoryPaths) (string, error)
	rebuild  func(context.Context, *derived.Index, string, string) error
	temp     func(string, string) (string, error)
	parity   func(context.Context, *derived.Index, *derived.Index, string) (derived.ParityReport, error)
	remove   func(string) error
}

func defaultRuntimeRebuildOps() runtimeRebuildOps {
	return runtimeRebuildOps{repository.GetMainRevision, func(ctx context.Context, index *derived.Index, root, revision string) error {
		return index.Rebuild(ctx, root, revision)
	}, os.MkdirTemp, func(ctx context.Context, index, clean *derived.Index, revision string) (derived.ParityReport, error) {
		return index.ParityCheck(ctx, clean, revision)
	}, os.RemoveAll}
}

// RebuildIndex rebuilds the live index and verifies it against a clean rebuild.
func (r *Runtime) RebuildIndex(ctx context.Context) (map[string]any, error) {
	return r.rebuildIndex(ctx, defaultRuntimeRebuildOps())
}
func (r *Runtime) rebuildIndex(ctx context.Context, ops runtimeRebuildOps) (map[string]any, error) {
	revision, err := ops.revision(r.Paths.Repository)
	if err != nil {
		return nil, err
	}
	index := &derived.Index{Path: r.Paths.DerivedDB}
	if err = ops.rebuild(ctx, index, r.Paths.Repository.CurrentDir, revision); err != nil {
		return nil, err
	}
	temp, err := ops.temp(r.Paths.Root, "parity-")
	if err != nil {
		return nil, err
	}
	defer ops.remove(temp)
	clean := &derived.Index{Path: filepath.Join(temp, "derived.sqlite")}
	if err = ops.rebuild(ctx, clean, r.Paths.Repository.CurrentDir, revision); err != nil {
		return nil, err
	}
	report, err := ops.parity(ctx, index, clean, revision)
	if err != nil {
		return nil, err
	}
	if r.SemanticWorker != nil {
		r.SemanticWorker.Enqueue(r.Paths.Repository.CurrentDir, revision, nil, true)
	}
	return map[string]any{"repo_revision": revision, "index_revision": report.CurrentRevision, "parity_matches": report.Matches, "parity_details": report.Details}, nil
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
		needleRuntime = capabilities.NeedleRuntime
	}
	r.mu.Lock()
	closed := r.closed
	r.mu.Unlock()
	serviceVersion := r.ServiceVersion
	if serviceVersion == "" {
		serviceVersion = "0.5.9"
	}
	return map[string]any{"service_version": serviceVersion, "schema_version": schemaVersion, "repo_revision": revision, "index_revision": state.IndexRevision, "index_stale": state.IndexRevision != state.RepoRevision, "visible_concepts": len(bundle.Entries), "semantic_search": map[string]any{"enabled": capabilities.SemanticEnabled, "ready": ready, "model_id": model, "dimensions": dimensions, "embedding_revision": embedding, "sqlite_vector_enabled": false, "warnings": warnings, "worker": worker}, "needle_router": map[string]any{"enabled": capabilities.NeedleEnabled, "loaded": capabilities.NeedleLoaded, "runtime": needleRuntime, "model_path": capabilities.NeedleModelPath}, "proposal_backlog": backlog, "control_db": r.Paths.ControlDB, "derived_db": r.Paths.DerivedDB, "repo_root": r.Paths.Root, "closed": closed}, nil
}
