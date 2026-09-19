package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/derived"
)

type runtimeSemanticClient struct{}

func (runtimeSemanticClient) ModelInfo() derived.SemanticModelInfo {
	return derived.SemanticModelInfo{ModelID: "m", Dimensions: 2, Revision: "v"}
}
func (runtimeSemanticClient) Embed(string) ([]float32, error) { return []float32{1, 1}, nil }
func TestSemanticRefreshNeeded(t *testing.T) {
	if semanticRefreshNeeded("r", "r") || !semanticRefreshNeeded("r", "") {
		t.Fatal("needed")
	}
}
func TestSemanticPathRequired(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	semantic := DefaultSemanticSearchConfig()
	semantic.Enabled = true
	if runtime, server, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}, Semantic: semantic}); err == nil || runtime != nil || server != nil {
		t.Fatal(runtime, server, err)
	}
}
func TestBuildSemanticEnabledRuntime(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	semantic := DefaultSemanticSearchConfig()
	semantic.Enabled = true
	model := "/model"
	semantic.ModelPath = &model
	semantic.ModelID = "m"
	semantic.Dimensions = 2
	semantic.RefreshOnStartup = true
	seed := t.TempDir()
	installMutationFiles(t, seed, map[string]string{"/a.md": mutationConcept})
	options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}, BootstrapSeed: seed, Semantic: semantic, Graph: GraphHTTPConfig{Enabled: true, RoutePrefix: "/graph"}}
	ops := defaultModelsOffBuildOps()
	ops.buildSemantic = func(SemanticSearchConfig) (derived.SemanticClient, error) { return runtimeSemanticClient{}, nil }
	runtime, _, err := buildModelsOffRuntime(ctx, config, options, ops)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.SemanticWorker == nil || runtime.GraphRefresh == nil {
		t.Fatal(runtime)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
