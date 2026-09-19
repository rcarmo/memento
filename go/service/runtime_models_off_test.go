package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/derived"
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

func TestBuildConfiguredIntelligentRuntime(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"actor": {Roles: []string{"reader", "proposer"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}}}
	proposals := DefaultModelProposalsConfig()
	proposals.Enabled = true
	runtime, server, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{{Token: "token", Principal: access.Principal{Name: "actor", Roles: []string{"reader", "proposer"}}}}, ModelClient: &stubModelClient{}, ModelProposals: proposals, DeepAnswers: DefaultDeepAnswersConfig(), ExactCache: DefaultExactAnswerCacheConfig(), HotMemory: DefaultHotWorkingMemoryConfig()})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	response, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	names := map[string]bool{}
	for _, raw := range response.Result.(map[string]any)["tools"].([]any) {
		names[raw.(map[string]any)["name"].(string)] = true
	}
	if !names["memory_propose_freeform"] || !names["memory_propose_update"] || !names["memory_answer"] || names["memory_route"] {
		t.Fatalf("tools=%v", names)
	}
	for question, source := range map[string]string{"What is the password?": "policy_abstention", "What is this?": "disabled"} {
		body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"memory_answer","arguments":{"question":` + strconv.Quote(question) + `}}}`
		answer, callErr := server.Process(ctx, []byte(body), umcp.RequestContext{Principal: "actor"})
		if callErr != nil || answer.Error != nil {
			t.Fatalf("answer=%#v err=%v", answer, callErr)
		}
		structured := jsonNormal(answer.Result).(map[string]any)["structuredContent"].(map[string]any)
		if structured["data"].(map[string]any)["answer_source"] != source {
			t.Fatalf("source=%#v", structured)
		}
	}
	var table string
	if err = runtime.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='answer_cache'`).Scan(&table); err != nil || table != "answer_cache" {
		t.Fatalf("table=%q err=%v", table, err)
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

func TestBuildManagedModelsOffRuntime(t *testing.T) {
	t.Setenv("MEMENTO_ADMIN_MASTER_KEY", "master")
	t.Setenv("SANDBOX_TOKEN", "sandbox-token")
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"piclaw-workspace": {TokenEnv: "SANDBOX_TOKEN", Roles: []string{"reader", "proposer", "curator"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}}}
	options := ModelsOffRuntimeOptions{Surface: "standard", Graph: GraphHTTPConfig{Enabled: true, RoutePrefix: "/graph"}}
	runtime, server, err := BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := runtime.Jobs.Identity.AuthenticateHeaders(ctx, map[string]string{"authorization": "Bearer sandbox-token"})
	if err != nil || principal == nil || principal.Name != "sandbox" {
		t.Fatal(principal, err)
	}
	if value, err := runtime.Jobs.Identity.AuthenticateHeaders(ctx, map[string]string{"authorization": "Bearer other"}); err != nil || value != nil {
		t.Fatal(value, err)
	}
	response, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "sandbox"})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	count := 0
	for _, raw := range response.Result.(map[string]any)["tools"].([]any) {
		if strings.HasPrefix(raw.(map[string]any)["name"].(string), "access_") {
			count++
		}
	}
	if count != 10 {
		t.Fatal(count)
	}
	principals, listErr := runtime.AuditPrincipals(ctx)
	if listErr != nil || len(principals) != 1 || principals[0].Name != "sandbox" {
		t.Fatal(principals, listErr)
	}
	graphResponse, err := runtime.HTTPHooks.Route(ctx, "GET", "/graph/api/v1/principals", nil, nil, "")
	if err != nil || graphResponse.Status != 200 || !bytes.Contains(graphResponse.Body, []byte(`"sandbox"`)) {
		t.Fatal(graphResponse, err)
	}
	if err = runtime.DB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, listErr = runtime.AuditPrincipals(ctx); listErr == nil {
		t.Fatal("closed managed list")
	}
	runtime.DB = nil
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	runtime, _, err = BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	principal, err = runtime.Jobs.Identity.AuthenticateHeaders(ctx, map[string]string{"authorization": "Bearer sandbox-token"})
	if err != nil || principal == nil {
		t.Fatal(principal, err)
	}
}

func TestModelsOffRuntimeSemanticWorker(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	worker := derived.NewSemanticWorker(adapterIndex{}, adapterClient{}, derived.SemanticRefreshConfig{})
	options := ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}, Graph: GraphHTTPConfig{Enabled: true, RoutePrefix: "/graph", Cluster: graphdebug.ClusterOptions{RefreshMaxPaths: 10}, Overview: graphdebug.OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10}}, SemanticWorker: worker}
	runtime, _, err := BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GraphRefresh == nil || runtime.SemanticWorker != worker {
		t.Fatal(runtime)
	}
	response, err := runtime.HTTPHooks.Route(ctx, "POST", "/graph/api/v1/embeddings/refresh", nil, []byte(`{"scope":"full","confirm_full":true}`), "")
	if err != nil || response.Status != 202 {
		t.Fatal(response, err)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if worker.State().Alive {
		t.Fatal("worker alive")
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
