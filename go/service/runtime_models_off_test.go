package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/graphdebug"
	"github.com/rcarmo/memento/go/umcp"
)

func TestBuildModelsOffRuntime(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{
		"admin": {TokenEnv: "TOKEN", Roles: []string{"admin", "reader", "proposer", "curator"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}},
	}}
	options := ModelsOffRuntimeOptions{
		Surface: "standard",
		Limits:  execute.Limits{MaxOperations: 10, MaxIntermediates: 10, MaxRecords: 10, MaxOutputBytes: "65536", MaxTimeSeconds: 3},
		Tokens:  []BearerPrincipal{{Token: "token", Principal: access.Principal{Name: "admin", Roles: []string{"admin", "reader", "proposer", "curator"}}}},
	}
	runtime, server, err := BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	request := umcp.RequestContext{Principal: "admin"}
	response, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), request)
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	tools := response.Result.(map[string]any)["tools"].([]any)
	if len(tools) == 0 {
		t.Fatal("empty discovery")
	}
	response, err = server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_status","arguments":{}}}`), request)
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	runtime, server, err = BuildModelsOffRuntime(ctx, config, options)
	if err != nil || server == nil {
		t.Fatal(err)
	}
	if err = runtime.Jobs.Controls.DerivedUpdate(ctx, runtime.Paths.Repository.CurrentDir, "next", []string{}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Jobs.Controls.DerivedUpdate(ctx, runtime.Paths.Repository.CurrentDir, "next", []string{"/missing.md"}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestModelsOffRuntimeHTTPHooks(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"actor": {Roles: []string{"reader", "proposer"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}}}
	options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{{Token: "token", Principal: access.Principal{Name: "actor", Roles: []string{"reader", "proposer"}}}}, Graph: GraphHTTPConfig{Enabled: true, RoutePrefix: "/graph", Overview: graphdebug.OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10}}}
	runtime, server, err := BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	transport, err := umcp.NewStreamableHTTP(server, umcp.DefaultHTTPOptions(), runtime.HTTPHooks)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	request := func(method, path, token string, body []byte) int {
		req, _ := http.NewRequest(method, host.URL+path, bytes.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		response, e := host.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		return response.StatusCode
	}
	if status := request("GET", "/graph/api/v1/status", "", nil); status != 200 {
		t.Fatal(status)
	}
	if status := request("GET", "/assets/staging/missing", "", nil); status != 401 {
		t.Fatal(status)
	}
	rpc := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	if status := request("POST", "/mcp", "", rpc); status != 401 {
		t.Fatal(status)
	}
	if status := request("POST", "/mcp", "token", rpc); status != 200 {
		t.Fatal(status)
	}
}

func TestBuildModelsOffRuntimeFailures(t *testing.T) {
	ctx := context.Background()
	token := []BearerPrincipal{{Token: "one", Principal: access.Principal{Name: "same"}}, {Token: "two", Principal: access.Principal{Name: "same"}}}
	for _, tc := range []struct {
		name, surface string
		tokens        []BearerPrincipal
		prepare       func(RuntimePaths)
	}{{"storage", "standard", token[:1], func(p RuntimePaths) { _ = os.WriteFile(p.Root, []byte("x"), 0600) }}, {"revision", "standard", token[:1], func(p RuntimePaths) { _ = os.MkdirAll(p.Repository.BareDir, 0700) }}, {"identity", "standard", token, nil}, {"surface", "bad", token[:1], nil}} {
		t.Run(tc.name, func(t *testing.T) {
			var config RuntimeConfig
			config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
			if tc.prepare != nil {
				tc.prepare(RuntimePathsFor(config))
			}
			runtime, server, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: tc.surface, Tokens: tc.tokens})
			if err == nil || runtime != nil || server != nil {
				t.Fatal(runtime, server, err)
			}
		})
	}
}

func TestBuildModelsOffRuntimeTokenFailure(t *testing.T) {
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"a": {TokenEnv: "MISSING"}}}
	runtime, server, err := BuildModelsOffRuntime(context.Background(), config, ModelsOffRuntimeOptions{Surface: "standard"})
	if err == nil || runtime != nil || server != nil {
		t.Fatal(runtime, server, err)
	}
}
