package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/umcp"
)

type runtimeSemanticClient struct{}

func (runtimeSemanticClient) ModelInfo() derived.SemanticModelInfo {
	return derived.SemanticModelInfo{ModelID: "m", Dimensions: 2, Revision: "v"}
}
func (runtimeSemanticClient) Embed(string) ([]float32, error) { return []float32{1, 1}, nil }
func (runtimeSemanticClient) Chunk(text string, _, _, _ int) ([]string, error) {
	return []string{text}, nil
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
func TestBuildProgressiveSemanticRuntime(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	semantic := DefaultSemanticSearchConfig()
	semantic.Enabled = true
	semantic.ProgressiveEnabled = true
	model := "/model"
	semantic.ModelPath = &model
	options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}, Semantic: semantic}
	ops := defaultModelsOffBuildOps()
	ops.buildSemantic = func(SemanticSearchConfig) (derived.SemanticClient, error) { return runtimeSemanticClient{}, nil }
	called := false
	ops.newProgressiveWorker = func(index derived.SemanticRefreshIndex, client derived.SemanticClient, config derived.SemanticRefreshConfig, policy derived.SemanticWorkerPolicy, idle func() time.Duration, cpu func() *float64, now func() time.Time) *derived.SemanticWorker {
		called = policy.Enabled && idle != nil && cpu != nil && now != nil
		return derived.NewSemanticWorker(index, client, config)
	}
	runtime, _, err := buildModelsOffRuntime(ctx, config, options, ops)
	if err != nil || !called {
		t.Fatal(runtime, called, err)
	}
	_ = runtime.Close(ctx)
}
func TestBuildSemanticEnabledRuntime(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"actor": {Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}}}
	semantic := DefaultSemanticSearchConfig()
	semantic.Enabled = true
	model := "/model"
	semantic.ModelPath = &model
	semantic.ModelID = "m"
	semantic.Dimensions = 2
	semantic.RefreshOnStartup = true
	seed := t.TempDir()
	installMutationFiles(t, seed, map[string]string{"/a.md": mutationConcept})
	options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{{Token: "token", Principal: access.Principal{Name: "actor", Roles: []string{"reader"}}}}, BootstrapSeed: seed, Semantic: semantic, Graph: GraphHTTPConfig{Enabled: true, RoutePrefix: "/graph"}}
	ops := defaultModelsOffBuildOps()
	ops.buildSemantic = func(SemanticSearchConfig) (derived.SemanticClient, error) { return runtimeSemanticClient{}, nil }
	runtime, server, err := buildModelsOffRuntime(ctx, config, options, ops)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.SemanticWorker == nil || runtime.GraphRefresh == nil {
		t.Fatal(runtime)
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err = runtime.SemanticWorker.WaitIdle(waitCtx); err != nil {
		t.Fatal(err)
	}
	response, callErr := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_search","arguments":{"query":"x","search_mode":"semantic","query_syntax":"plain","limit":5}}}`), umcp.RequestContext{Principal: "actor"})
	if callErr != nil || response.Error != nil {
		t.Fatal(response, callErr)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestChunkPreparationFailureStopsRuntime(t *testing.T) {
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}}
	options.Semantic = DefaultSemanticSearchConfig()
	options.Semantic.Enabled = true
	options.Semantic.ModelID = "m"
	options.Semantic.Dimensions = 2
	modelPath := "unused"
	options.Semantic.ModelPath = &modelPath
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ops := defaultModelsOffBuildOps()
	ops.buildSemantic = func(SemanticSearchConfig) (derived.SemanticClient, error) {
		cancel()
		return runtimeSemanticClient{}, nil
	}
	if _, _, err := buildModelsOffRuntime(ctx, config, options, ops); err == nil {
		t.Fatal("migration error ignored")
	}
}
