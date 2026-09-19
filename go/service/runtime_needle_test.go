package service

import (
	"context"
	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/umcp"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildNeedleRealConstructionBoundaries(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	options := ModelsOffRuntimeOptions{Surface: "compact", Tokens: []BearerPrincipal{}, Needle: DefaultNeedleRouterConfig()}
	options.Needle.Enabled = true
	ops := defaultModelsOffBuildOps()
	ops.loadNeedleModel = func(string) (*needle.Model, error) { return &needle.Model{}, nil }
	ops.loadNeedleTokenizer = func(string) (*needle.Tokenizer, error) { return &needle.Tokenizer{}, nil }
	ops.newNeedleRouter = func(*needle.Model) (*needle.Router, error) { return &needle.Router{}, nil }
	called := false
	ops.registerRoute = func(_ *Jobs, _ *umcp.Server, _ string, _ execute.Limits, inference RouteInference, tokenizer *needle.Tokenizer) error {
		called = inference != nil && tokenizer != nil
		return nil
	}
	runtime, _, err := buildModelsOffRuntime(ctx, config, options, ops)
	if err != nil || !called {
		t.Fatal(runtime, called, err)
	}
	_ = runtime.Close(ctx)
}

func TestBuildNeedleEnabledRuntime(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"actor": {Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}}}
	options := ModelsOffRuntimeOptions{Surface: "compact", Tokens: []BearerPrincipal{{Token: "token", Principal: access.Principal{Name: "actor", Roles: []string{"reader"}}}}, Needle: DefaultNeedleRouterConfig()}
	options.Needle.Enabled = true
	ops := defaultModelsOffBuildOps()
	stub := &routeInferenceStub{output: `[{"name":"UNKNOWN","arguments":{}}]`}
	ops.buildRoute = func(c NeedleRouterConfig) (RouteInference, *needle.Tokenizer, error) {
		if c.ModelPath != options.Needle.ModelPath {
			t.Fatal(c)
		}
		return stub, &needle.Tokenizer{}, nil
	}
	runtime, server, err := buildModelsOffRuntime(ctx, config, options, ops)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	response, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	found := false
	for _, raw := range response.Result.(map[string]any)["tools"].([]any) {
		if raw.(map[string]any)["name"] == "memory_route" {
			found = true
		}
	}
	if !found {
		t.Fatal("route hidden")
	}
	call, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"memory_route","arguments":{"request":"book a flight","execute":false}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || call.Error != nil {
		t.Fatal(call, err)
	}
	if !strings.Contains(stub.query, "book a flight") {
		t.Fatal(stub.query)
	}
}
