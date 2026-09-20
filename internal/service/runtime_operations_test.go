package service

import (
	"context"
	"errors"
	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/repository"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type rebuildRefreshIndex struct{ called chan struct{} }

func (r *rebuildRefreshIndex) PendingEmbeddingPaths(context.Context, int) ([]string, error) {
	return []string{"/x"}, nil
}
func (r *rebuildRefreshIndex) RefreshEmbeddingPaths(context.Context, string, []string, derived.SemanticRefreshConfig, derived.SemanticClient) error {
	select {
	case r.called <- struct{}{}:
	default:
	}
	return nil
}

type rebuildClient struct{}

func (rebuildClient) ModelInfo() derived.SemanticModelInfo {
	return derived.SemanticModelInfo{ModelID: "m", Dimensions: 1}
}
func (rebuildClient) Embed(string) ([]float32, error) { return []float32{1}, nil }
func TestRuntimeAuditRepository(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "current"), 0700); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{Paths: RuntimePaths{Repository: repository.GitRepositoryPaths{CurrentDir: filepath.Join(root, "current")}}, ProtectedReadPrefixes: []string{"/private/"}}
	runtime.AuditPrincipals = StaticAuditPrincipals(access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"z": {Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}, "admin": {Roles: []string{"admin"}, ReadPrefixes: []string{"/"}}}})
	payload, err := runtime.AuditRepository(ctx, nil)
	if err != nil || payload["ok"] != true || len(payload["warnings"].([]map[string]any)) != 1 || payload["warnings"].([]map[string]any)[0]["principal"] != "z" {
		t.Fatal(payload, err)
	}
	concept := `---
id: '12345678'
type: concept
title: Broken
status: active
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
updated_by: test
---
[missing](/missing.md)
`
	if err = os.WriteFile(filepath.Join(root, "current", "x.md"), []byte(concept), 0600); err != nil {
		t.Fatal(err)
	}
	wanted := "/x.md"
	payload, err = runtime.AuditRepository(ctx, &wanted)
	if err != nil || payload["ok"] != false || len(payload["issues"].([]repository.AuditIssue)) != 1 {
		t.Fatal(payload, err)
	}
	wanted = "/other.md"
	payload, err = runtime.AuditRepository(ctx, &wanted)
	if err != nil || len(payload["issues"].([]repository.AuditIssue)) != 0 {
		t.Fatal(payload, err)
	}
	runtime.AuditPrincipals = func(context.Context) ([]AuditPrincipal, error) { return nil, errors.New("principals") }
	if _, err = runtime.AuditRepository(ctx, nil); err == nil {
		t.Fatal("principals")
	}
	if err = os.WriteFile(filepath.Join(root, "current", "bad.md"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	runtime.AuditPrincipals = nil
	if _, err = runtime.AuditRepository(ctx, nil); err == nil {
		t.Fatal("parse")
	}
}
func TestRuntimeRebuildIndex(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	runtime, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := runtime.RebuildIndex(ctx)
	if err != nil || payload["parity_matches"] != true || payload["parity_details"] != "match" || payload["repo_revision"] != payload["index_revision"] {
		t.Fatal(payload, err)
	}
	refresh := &rebuildRefreshIndex{called: make(chan struct{}, 1)}
	runtime.SemanticWorker = derived.NewSemanticWorker(refresh, rebuildClient{}, derived.SemanticRefreshConfig{})
	payload, err = runtime.RebuildIndex(ctx)
	if err != nil {
		t.Fatal(payload, err)
	}
	select {
	case <-refresh.called:
	case <-time.After(time.Second):
		t.Fatal("semantic refresh not enqueued")
	}
	runtime.SemanticWorker.Close()
	runtime.SemanticWorker = nil
	_ = runtime.Close(ctx)
}
func TestRuntimeRebuildIndexFailures(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	runtime := &Runtime{Paths: RuntimePaths{Root: t.TempDir()}}
	base := runtimeRebuildOps{revision: func(repository.GitRepositoryPaths) (string, error) { return "r", nil }, rebuild: func(context.Context, *derived.Index, string, string) error { return nil }, temp: func(string, string) (string, error) { return t.TempDir(), nil }, parity: func(context.Context, *derived.Index, *derived.Index, string) (derived.ParityReport, error) {
		return derived.ParityReport{}, nil
	}, remove: func(string) error { return nil }}
	for _, stage := range []string{"revision", "live", "temp", "clean", "parity"} {
		ops := base
		calls := 0
		switch stage {
		case "revision":
			ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
		case "live":
			ops.rebuild = func(context.Context, *derived.Index, string, string) error { return boom }
		case "temp":
			ops.temp = func(string, string) (string, error) { return "", boom }
		case "clean":
			ops.rebuild = func(context.Context, *derived.Index, string, string) error {
				calls++
				if calls == 2 {
					return boom
				}
				return nil
			}
		case "parity":
			ops.parity = func(context.Context, *derived.Index, *derived.Index, string) (derived.ParityReport, error) {
				return derived.ParityReport{}, boom
			}
		}
		if _, err := runtime.rebuildIndex(ctx, ops); !errors.Is(err, boom) {
			t.Fatal(stage, err)
		}
	}
}
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
	if snapshot["service_version"] != "0.5.9" || snapshot["schema_version"] != 2 || snapshot["visible_concepts"] != 0 || snapshot["proposal_backlog"] != 1 || snapshot["closed"] != false {
		t.Fatal(snapshot)
	}
	runtime.ServiceVersion = "1.0.0"
	snapshot, err = runtime.StatusSnapshot(ctx, 2)
	if err != nil || snapshot["service_version"] != "1.0.0" {
		t.Fatal(snapshot, err)
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
	runtime.Jobs.Controls.RuntimeCapabilities = RuntimeCapabilities{SemanticEnabled: true, SemanticLoaded: true, SemanticModelID: "m", SemanticDimensions: 2, NeedleEnabled: true, NeedleLoaded: true, NeedleModelPath: "/n", NeedleRuntime: "go-mmap-subprocess"}
	runtime.SemanticWorker = derived.NewSemanticWorker(nil, nil, derived.SemanticRefreshConfig{})
	snapshot, err := runtime.StatusSnapshot(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	semantic := snapshot["semantic_search"].(map[string]any)
	needle := snapshot["needle_router"].(map[string]any)
	if semantic["model_id"] != "m" || semantic["dimensions"] != 2 || semantic["worker"] == nil || needle["runtime"] != "go-mmap-subprocess" {
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
