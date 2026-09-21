package service

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/gte"
)

type rollbackChunkClient struct{ tok *gte.Tokenizer }

func (c rollbackChunkClient) ModelInfo() derived.SemanticModelInfo {
	return derived.SemanticModelInfo{ModelID: "test", Dimensions: 2, Revision: "test"}
}
func (c rollbackChunkClient) Embed(string) ([]float32, error) { return []float32{1, 0}, nil }
func (c rollbackChunkClient) Chunk(s string, t, o, n int) ([]string, error) {
	return c.tok.Chunk(s, t, o, n)
}

// Run the actual predecessor against a disposable repository written by this
// build, then reopen with this build. No Docker or production state is involved.
func TestChunkActualPredecessorRollback(t *testing.T) {
	old := os.Getenv("MEMENTO_ROLLBACK_BINARY")
	if old == "" {
		t.Skip("set MEMENTO_ROLLBACK_BINARY")
	}
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	installMutationFiles(t, seed, map[string]string{"/public/a.md": mutationConcept})
	var config RuntimeConfig
	config.SchemaVersion = 2
	config.Repository.RootPath = filepath.Join(root, "state")
	config.Repository.BundleRoot = "/"
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{}}
	config.Limits = defaultLimitsConfig()
	config.MCP = defaultMCPConfig()
	config.Observability.GraphExplorer = DefaultGraphExplorerConfig()
	runtime, _, err := BuildModelsOffRuntime(t.Context(), config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}, BootstrapSeed: seed})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())
	state, err := (&derived.Index{Path: runtime.Paths.DerivedDB}).State(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	vocab := make([]string, 110)
	for i := range vocab {
		vocab[i] = "word"
	}
	tok, err := gte.NewTokenizer(vocab, 512)
	if err != nil {
		t.Fatal(err)
	}
	index := &derived.Index{Path: runtime.Paths.DerivedDB, DeferEmbeddings: true}
	if err = index.RefreshEmbeddingPaths(t.Context(), state.RepoRevision, []string{"/public/a.md"}, derived.SemanticRefreshConfig{ModelID: "test", Dimensions: 2, MaxInputChars: 4096, MaxBatch: 1}, rollbackChunkClient{tok}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Use a minimal rollback-compatible config; newer Go-only defaults are not
	// needed for this models-disabled state-format check.
	file := filepath.Join(root, "config.json")
	raw, _ := json.Marshal(map[string]any{"schema_version": 2, "repository": map[string]any{"root_path": config.Repository.RootPath}, "authorization": map[string]any{"principals": map[string]any{}}})
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if model := os.Getenv("GTE_MODEL_PATH"); model != "" {
		worker, _ := filepath.Abs("../../build/memento-embed-go")
		raw, _ = json.Marshal(map[string]any{"schema_version": 2, "repository": map[string]any{"root_path": config.Repository.RootPath}, "authorization": map[string]any{"principals": map[string]any{}}, "intelligent_tiers": map[string]any{"semantic_search": map[string]any{"enabled": true, "model_path": model, "worker_mode": "subprocess", "worker_path": worker, "refresh_on_startup": false}}})
		if err = os.WriteFile(file, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, op := range []string{"status", "rebuild-index", "status"} {
		cmd := exec.CommandContext(t.Context(), old, "--config", file, op)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("predecessor %s: %v %s", op, err, output)
		}
		t.Logf("predecessor %s passed", op)
	}
	runtime, _, err = BuildModelsOffRuntime(t.Context(), config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(t.Context())
	reopened, err := index.State(t.Context())
	if err != nil || reopened.RepoRevision != state.RepoRevision {
		t.Fatal(reopened, err)
	}
}
