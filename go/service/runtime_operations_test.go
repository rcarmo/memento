package service

import (
	"context"
	"errors"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
	"path/filepath"
	"testing"
)

func TestRuntimeStatusSnapshot(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.SchemaVersion = 2
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	runtime, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	record := control.ProposalRequest{ProposalID: "p", AuthorPrincipal: "a", BaseRevision: "r", Intent: "i", Patch: map[string]any{}}
	if _, err = (control.Proposals{DB: runtime.DB}).Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.StatusSnapshot(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot["schema_version"] != 2 || snapshot["visible_concepts"] != 0 || snapshot["proposal_backlog"] != 1 || snapshot["closed"] != false {
		t.Fatal(snapshot)
	}
	semantic := snapshot["semantic_search"].(map[string]any)
	needle := snapshot["needle_router"].(map[string]any)
	if semantic["enabled"] != false || semantic["worker"] != nil || needle["loaded"] != false {
		t.Fatal(snapshot)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestRuntimeStatusSnapshotEnabled(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	runtime, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	runtime.Jobs.Controls.RuntimeCapabilities = RuntimeCapabilities{SemanticEnabled: true, SemanticLoaded: true, SemanticModelID: "m", SemanticDimensions: 2, NeedleEnabled: true, NeedleLoaded: true, NeedleModelPath: "/n"}
	runtime.SemanticWorker = derived.NewSemanticWorker(nil, nil, derived.SemanticRefreshConfig{})
	snapshot, err := runtime.StatusSnapshot(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	semantic := snapshot["semantic_search"].(map[string]any)
	needle := snapshot["needle_router"].(map[string]any)
	if semantic["model_id"] != "m" || semantic["dimensions"] != 2 || semantic["worker"] == nil || needle["runtime"] != "go-scalar" {
		t.Fatal(snapshot)
	}
	runtime.SemanticWorker.Close()
	runtime.SemanticWorker = nil
	_ = runtime.Close(ctx)
}
func TestRuntimeStatusSnapshotInjectedFailures(t *testing.T) {
	boom := errors.New("boom")
	base := runtimeStatusOps{revision: func(repository.GitRepositoryPaths) (string, error) { return "r", nil }, state: func(context.Context, *derived.Index) (derived.IndexState, error) { return derived.IndexState{}, nil }, scan: func(string, repository.BundleFilter) (repository.RepositoryBundle, error) {
		return repository.RepositoryBundle{}, nil
	}, proposals: func(context.Context, *control.Proposals) ([]control.ProposalRecord, error) { return nil, nil }, embedding: func(context.Context, *derived.Index) (string, error) { return "", nil }}
	runtime := &Runtime{Jobs: &Jobs{Controls: &ProposalControls{RuntimeCapabilities: RuntimeCapabilities{SemanticEnabled: true}}}}
	embeddingOps := base
	embeddingOps.embedding = func(context.Context, *derived.Index) (string, error) { return "r", nil }
	snapshot, err := runtime.statusSnapshot(context.Background(), 2, embeddingOps)
	if err != nil || snapshot["semantic_search"].(map[string]any)["embedding_revision"] != "r" {
		t.Fatal(snapshot, err)
	}
	for _, stage := range []string{"revision", "state", "scan", "proposals"} {
		ops := base
		switch stage {
		case "revision":
			ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
		case "state":
			ops.state = func(context.Context, *derived.Index) (derived.IndexState, error) { return derived.IndexState{}, boom }
		case "scan":
			ops.scan = func(string, repository.BundleFilter) (repository.RepositoryBundle, error) {
				return repository.RepositoryBundle{}, boom
			}
		case "proposals":
			ops.proposals = func(context.Context, *control.Proposals) ([]control.ProposalRecord, error) { return nil, boom }
		}
		if _, err := runtime.statusSnapshot(context.Background(), 2, ops); !errors.Is(err, boom) {
			t.Fatal(stage, err)
		}
	}
}
func TestRuntimeStatusSnapshotFailures(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	runtime, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.DB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = runtime.StatusSnapshot(ctx, 2); err == nil {
		t.Fatal("closed db")
	}
	_ = runtime.Close(ctx)
}
