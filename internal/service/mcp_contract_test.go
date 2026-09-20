package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/umcp"
)

// These tests exercise production startup, not an isolated registry or mock
// catalogue. Stub only expensive model inference; storage, auth, discovery,
// validation, execute dispatch and MCP envelopes are the real implementations.
func contractRuntime(t *testing.T, surface string, route, intelligent bool) (*Runtime, *umcp.Server, *routeInferenceStub) {
	t.Helper()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.Limits.MaxConceptBytes = 65536
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{
		"actor":  {Roles: []string{"reader", "proposer", "curator", "admin"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}},
		"reader": {Roles: []string{"reader"}, ReadPrefixes: []string{"/public/"}},
	}}
	seed := t.TempDir()
	installMutationFiles(t, seed, map[string]string{"/public/a.md": mutationConcept, "/private/b.md": strings.ReplaceAll(mutationConcept, "12345678", "87654321")})
	options := ModelsOffRuntimeOptions{Surface: surface, Limits: endpointLimits(), BootstrapSeed: seed,
		Tokens: []BearerPrincipal{{Token: "token", Principal: access.Principal{Name: "actor", Roles: config.Authorization.Principals["actor"].Roles}}, {Token: "reader-token", Principal: access.Principal{Name: "reader", Roles: []string{"reader"}}}},
		Needle: DefaultNeedleRouterConfig(), DeepAnswers: DefaultDeepAnswersConfig(), ModelProposals: DefaultModelProposalsConfig(),
	}
	options.Needle.Enabled = route
	options.DeepAnswers.Enabled = intelligent
	options.ModelProposals.Enabled = intelligent
	if intelligent {
		options.ModelClient = &stubModelClient{}
	}
	stub := &routeInferenceStub{output: `[{"name":"UNKNOWN","arguments":{}}]`}
	ops := defaultModelsOffBuildOps()
	ops.buildRoute = func(NeedleRouterConfig) (NeedleRouteInference, error) { return stub, nil }
	runtime, server, err := buildModelsOffRuntime(context.Background(), config, options, ops)
	if err != nil {
		t.Fatal(err)
	}
	server.SetNotificationOutput(nil)
	t.Cleanup(func() {
		if err := runtime.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return runtime, server, stub
}
func contractRPC(t *testing.T, server *umcp.Server, principal, method string, params map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Process(context.Background(), raw, umcp.RequestContext{Principal: principal})
	if err != nil || response == nil {
		t.Fatalf("%s: %#v %v", method, response, err)
	}
	return jsonNormal(response).(map[string]any)
}
func contractResult(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	if response["error"] != nil {
		t.Fatalf("RPC failure: %#v", response)
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %#v", response)
	}
	return result
}
func contractTool(t *testing.T, server *umcp.Server, principal, name string, args map[string]any) map[string]any {
	t.Helper()
	if args == nil {
		args = map[string]any{}
	}
	result := contractResult(t, contractRPC(t, server, principal, "tools/call", map[string]any{"name": name, "arguments": args}))
	body, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("%s missing structured content: %#v", name, result)
	}
	content := result["content"].([]any)
	if len(content) != 1 || content[0].(map[string]any)["type"] != "text" {
		t.Fatal(result)
	}
	var text any
	if err := json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &text); err != nil || !reflect.DeepEqual(text, body) {
		t.Fatalf("text/structured mismatch: %v %v", result, err)
	}
	if body["status"] != "success" {
		t.Fatalf("%s domain failure: %#v", name, body)
	}
	return body
}
func contractResource(t *testing.T, server *umcp.Server, uri string) map[string]any {
	t.Helper()
	result := contractResult(t, contractRPC(t, server, "actor", "resources/read", map[string]any{"uri": uri}))
	contents := result["contents"].([]any)
	if len(contents) != 1 {
		t.Fatal(result)
	}
	content := contents[0].(map[string]any)
	if content["mimeType"] != "application/json" {
		t.Fatal(content)
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(content["text"].(string)), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func contractNames(t *testing.T, server *umcp.Server, principal string, pageSize int) []string {
	t.Helper()
	names := []string{}
	params := map[string]any{"pageSize": pageSize}
	for pages := 0; pages < 100; pages++ {
		result := contractResult(t, contractRPC(t, server, principal, "tools/list", params))
		for _, raw := range result["tools"].([]any) {
			names = append(names, raw.(map[string]any)["name"].(string))
		}
		cursor, ok := result["nextCursor"]
		if !ok {
			return names
		}
		params["cursor"] = cursor
	}
	t.Fatal("unbounded pagination")
	return nil
}

func TestMCPContractRuntimeSurfaces(t *testing.T) {
	// Independent expected surface sets catch startup registration dropping tools
	// even when isolated Catalog.Register tests still pass.
	surfaces := map[string]string{
		"compact":   "help status search read inventory asset_stage_begin asset_stage_status asset_get execute",
		"read_only": "help status search read list inventory graph answer asset_get",
		"standard":  "help status search read list inventory graph audit answer propose proposal_get proposal_list proposal_asset_get proposal_revise operation_get proposal_review proposal_apply asset_stage_begin asset_stage_status asset_get asset_prune create patch trash restore purge rename",
		"curator":   "help status search read inventory audit proposal_get proposal_list proposal_asset_get proposal_revise operation_get proposal_review proposal_apply asset_stage_begin asset_stage_status asset_get asset_prune trash restore purge execute",
		"admin":     "help status search read list inventory graph audit answer propose proposal_get proposal_list proposal_asset_get proposal_revise operation_get proposal_review proposal_apply asset_stage_begin asset_stage_status asset_get asset_prune create patch trash restore purge rename execute",
	}
	for surface, base := range surfaces {
		for _, route := range []bool{false, true} {
			for _, intelligent := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/route=%t/models=%t", surface, route, intelligent), func(t *testing.T) {
					_, server, _ := contractRuntime(t, surface, route, intelligent)
					want := strings.Fields(base)
					if intelligent && (surface == "compact" || surface == "curator") {
						want = append(want, "answer")
					}
					if intelligent && (surface == "standard" || surface == "admin") {
						want = append(want, "propose_freeform", "propose_update")
					}
					if route && (surface == "compact" || surface == "curator" || surface == "admin") {
						want = append(want, "route")
					}
					for i := range want {
						want[i] = "memory_" + want[i]
					}
					slices.Sort(want)
					names := contractNames(t, server, "actor", 3)
					if len(names) != len(slices.Compact(append([]string{}, names...))) {
						t.Fatal("duplicate discovery", names)
					}
					slices.Sort(names)
					if !reflect.DeepEqual(names, want) {
						t.Fatalf("got %v want %v", names, want)
					}
					whole := contractNames(t, server, "actor", 100)
					slices.Sort(whole)
					if !reflect.DeepEqual(whole, names) {
						t.Fatal("pagination changes discovery")
					}
					help := contractTool(t, server, "actor", "memory_help", nil)["data"].(map[string]any)["mcp"].(map[string]any)
					if help["tool_surface"] != surface || !reflect.DeepEqual(help["direct_tools"], jsonNormal(want)) {
						t.Fatal("help/discovery mismatch", help, want)
					}
					catalog := contractResource(t, server, "memory://catalog")
					direct := []string{}
					for _, raw := range catalog["operations"].([]any) {
						op := raw.(map[string]any)
						if op["direct_tool_available"] != true {
							t.Fatal(op)
						}
						direct = append(direct, op["tool"].(string))
					}
					slices.Sort(direct)
					if !reflect.DeepEqual(direct, want) {
						t.Fatal("catalog/discovery mismatch", direct, want)
					}
					// Hidden read/list/execute operations still dispatch; discovery is not auth.
					for name, args := range map[string]map[string]any{"memory_search": {"query": "Title", "limit": 1}, "memory_read": {"id_or_path": "/public/a.md"}, "memory_inventory": {"path_prefix": "/public/"}, "memory_list": {"path_prefix": "/public/"}, "memory_status": {}, "memory_execute": {"operations": []any{}}} {
						contractTool(t, server, "actor", name, args)
					}
				})
			}
		}
	}
}

func TestMCPContractRoutesExecuteRealPlans(t *testing.T) {
	_, server, stub := contractRuntime(t, "compact", true, false)
	cases := []struct{ name, arguments, request string }{
		{"search_paths", `{"query":"Title","limit":1}`, "find Title"},
		{"search_then_read", `{"query":"Title"}`, "find Title"},
		{"search_then_graph", `{"query":"Title","depth":2}`, "find Title"},
		{"status_field", `{"field":"principal"}`, "show principal"},
		{"read_field", `{"id_or_path":"/public/a.md","field":"title"}`, "show /public/a.md title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub.output = `[{"name":"` + tc.name + `","arguments":` + tc.arguments + `}]`
			preview := contractTool(t, server, "actor", "memory_route", map[string]any{"request": tc.request, "execute": false})["data"].(map[string]any)
			if preview["executed"] != false || preview["expansion"] == nil {
				t.Fatal(preview)
			}
			data := contractTool(t, server, "actor", "memory_route", map[string]any{"request": tc.request})["data"].(map[string]any)
			result := data["result"].(map[string]any)
			if data["executed"] != true || result["status"] != "success" {
				t.Fatalf("nested route failure: %#v", data)
			}
			if tc.name == "search_then_read" || tc.name == "search_then_graph" {
				raw, _ := json.Marshal(result)
				if strings.Contains(string(raw), `"status":"error"`) {
					t.Fatalf("execute operation failed: %s", raw)
				}
			}
		})
	}
	stub.output = `[{"name":"UNKNOWN","arguments":{}}]`
	data := contractTool(t, server, "actor", "memory_route", map[string]any{"request": "unsupported"})["data"].(map[string]any)
	if data["abstained"] != true || data["executed"] != false {
		t.Fatal(data)
	}
}

func TestMCPContractResourcesAndProtocol(t *testing.T) {
	_, server, _ := contractRuntime(t, "compact", true, false)
	init := contractResult(t, contractRPC(t, server, "actor", "initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "contract", "version": "1"}}))
	if init["protocolVersion"] != "2025-03-26" || init["serverInfo"].(map[string]any)["name"] != "memento" {
		t.Fatal(init)
	}
	caps := init["capabilities"].(map[string]any)
	for _, name := range []string{"tools", "resources", "prompts", "completions", "logging"} {
		if caps[name] == nil {
			t.Fatal("capability missing", name)
		}
	}
	for method, key := range map[string]string{"resources/list": "resources", "resources/templates/list": "resourceTemplates", "prompts/list": "prompts"} {
		result := contractResult(t, contractRPC(t, server, "actor", method, map[string]any{"pageSize": 1}))
		if len(result[key].([]any)) != 1 {
			t.Fatal(method, result)
		}
	}
	for _, uri := range []string{"memory://help", "memory://status", "memory://catalog", "memory://catalog/search", "memory://catalog/read", "memory://catalog/inventory", "memory://workflow/inspect", "memory://workflow/asset_pack"} {
		contractResource(t, server, uri)
	}
	prompt := contractResult(t, contractRPC(t, server, "actor", "prompts/get", map[string]any{"name": "publish_asset_pack", "arguments": map[string]any{"target_path": "/skills/test.md", "asset_kind": "skill", "version": "1.0.0"}}))
	if len(prompt["messages"].([]any)) == 0 {
		t.Fatal(prompt)
	}
	for _, tc := range []struct {
		method string
		params map[string]any
	}{
		{"tools/call", map[string]any{"name": "missing"}},
		{"tools/call", map[string]any{"name": "memory_read", "arguments": map[string]any{}}},
		{"tools/call", map[string]any{"name": "memory_search", "arguments": map[string]any{"query": "Title", "unknown": true}}},
		{"tools/list", map[string]any{"pageSize": 0}},
		{"tools/list", map[string]any{"cursor": "invalid"}},
		{"resources/read", map[string]any{"uri": "memory://catalog/missing"}},
		{"prompts/get", map[string]any{"name": "missing"}},
		{"unknown/method", map[string]any{}},
	} {
		response := contractRPC(t, server, "actor", tc.method, tc.params)
		if response["error"] == nil {
			t.Fatal("invalid request accepted", tc, response)
		}
	}
	first := contractResult(t, contractRPC(t, server, "actor", "tools/list", map[string]any{"pageSize": 1}))
	replay := contractRPC(t, server, "reader", "tools/list", map[string]any{"cursor": first["nextCursor"], "pageSize": 1})
	if replay["error"] == nil {
		t.Fatal("cross-principal cursor replay", replay)
	}
}

func TestMCPContractHTTPAuthAndEnvelopes(t *testing.T) {
	runtime, server, _ := contractRuntime(t, "compact", true, false)
	transport, err := umcp.NewStreamableHTTP(server, umcp.DefaultHTTPOptions(), runtime.HTTPHooks)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	rpc := func(token, method string, params map[string]any) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		transport.ServeHTTP(rec, req)
		var value map[string]any
		if rec.Body.Len() > 0 {
			if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
				t.Fatal(rec.Code, rec.Body.String(), err)
			}
		}
		return rec.Code, value
	}
	for _, token := range []string{"", "wrong"} {
		code, _ := rpc(token, "tools/list", nil)
		if code != 401 {
			t.Fatal(code)
		}
	}
	for _, name := range []string{"memory_search", "memory_read", "memory_inventory"} {
		args := map[string]any{}
		if name == "memory_search" {
			args["query"] = "Title"
			args["limit"] = 1
		}
		if name == "memory_read" {
			args["id_or_path"] = "/public/a.md"
		}
		code, response := rpc("reader-token", "tools/call", map[string]any{"name": name, "arguments": args})
		if code != 200 {
			t.Fatal(code, response)
		}
		body := contractResult(t, response)["structuredContent"].(map[string]any)
		if body["status"] != "success" {
			t.Fatal(body)
		}
		raw, _ := json.Marshal(body)
		if strings.Contains(string(raw), "/private/b.md") {
			t.Fatal("scope leak", body)
		}
	}
	for _, args := range []map[string]any{{"id_or_path": "/private/b.md"}, {"id_or_path": "/public/missing.md"}} {
		code, response := rpc("reader-token", "tools/call", map[string]any{"name": "memory_read", "arguments": args})
		if code != 200 {
			t.Fatal(code, response)
		}
		body := contractResult(t, response)["structuredContent"].(map[string]any)
		if body["status"] != "error" || body["error_class"] == nil {
			t.Fatal("missing domain error", body)
		}
	}
}

func TestMCPContractEveryToolSchemaDefaultsAndValidation(t *testing.T) {
	catalog, err := NewCatalog(CatalogConfig{Surface: "admin", AnswerEnabled: true, RouteEnabled: true, ProposalEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	handlers := map[string]CatalogHandler{}
	received := map[string]map[string]any{}
	for _, op := range catalog.source.Operations {
		name := op.Tool
		handlers[name] = func(_ context.Context, args map[string]any) (any, error) {
			received[name] = args
			return map[string]any{"status": "success", "data": map[string]any{}, "warnings": []any{}, "next_tools": []any{}}, nil
		}
	}
	server := umcp.NewServer("schema-contract")
	if err := catalog.Register(server, handlers, nil); err != nil {
		t.Fatal(err)
	}
	var definitions []proposalToolDefinition
	decoder := json.NewDecoder(strings.NewReader(string(catalogToolDefinitions)))
	decoder.UseNumber()
	if err := decoder.Decode(&definitions); err != nil {
		t.Fatal(err)
	}
	for _, definition := range definitions {
		t.Run(definition.Name, func(t *testing.T) {
			properties := definition.Schema["properties"].(map[string]any)
			schemaRequired := map[string]bool{}
			if fields, ok := definition.Schema["required"].([]any); ok {
				for _, raw := range fields {
					schemaRequired[raw.(string)] = true
				}
			}
			args := map[string]any{}
			for _, parameter := range definition.Parameters {
				property, ok := properties[parameter.Name].(map[string]any)
				if !ok {
					t.Fatal("signature property missing", parameter.Name)
				}
				// Retained source contracts distinguish MCP signature defaults from
				// schema defaults: null fields/returns select the service default.
				// compare_manifest inherits a required signature path despite the
				// schema's '/' hint. Pin that exception instead of ignoring drift.
				legacyRequired := definition.Name == "memory_compare_manifest" && parameter.Name == "path_prefix"
				if parameter.Required != (schemaRequired[parameter.Name] || legacyRequired) {
					t.Fatal("required mismatch", parameter.Name)
				}
				if !parameter.Required {
					_, hasSchemaDefault := property["default"]
					serviceDefault := definition.Name == "memory_inventory" && parameter.Name == "fields" || definition.Name == "memory_execute" && parameter.Name == "returns"
					if serviceDefault && parameter.Default != nil {
						t.Fatal("service-default sentinel changed", parameter.Name)
					}
					if hasSchemaDefault && !serviceDefault && !reflect.DeepEqual(jsonNormal(parameter.Default), jsonNormal(property["default"])) {
						t.Fatal("default mismatch", parameter.Name, parameter.Default, property)
					}
					continue
				}
				switch property["type"] {
				case "integer":
					args[parameter.Name] = 1
				case "boolean":
					args[parameter.Name] = true
				case "array":
					args[parameter.Name] = []any{}
				case "object":
					args[parameter.Name] = map[string]any{}
				default:
					args[parameter.Name] = "synthetic"
				}
			}
			contractTool(t, server, "actor", definition.Name, args)
			got := received[definition.Name]
			if len(got) != len(definition.Parameters) {
				t.Fatal("parameter shape", got)
			}
			for _, parameter := range definition.Parameters {
				if !parameter.Required && !reflect.DeepEqual(jsonNormal(got[parameter.Name]), jsonNormal(parameter.Default)) {
					t.Fatal("default not dispatched", parameter.Name, got)
				}
			}
			for _, parameter := range definition.Parameters {
				if parameter.Required {
					without := map[string]any{}
					for k, v := range args {
						if k != parameter.Name {
							without[k] = v
						}
					}
					delete(received, definition.Name)
					response := contractRPC(t, server, "actor", "tools/call", map[string]any{"name": definition.Name, "arguments": without})
					if response["error"] == nil || received[definition.Name] != nil {
						t.Fatal("missing required argument reached handler", parameter.Name, response)
					}
				}
			}
			args["unexpected_contract_field"] = true
			delete(received, definition.Name)
			response := contractRPC(t, server, "actor", "tools/call", map[string]any{"name": definition.Name, "arguments": args})
			if response["error"] == nil || received[definition.Name] != nil {
				t.Fatal("unknown argument reached handler", response)
			}
		})
	}
}

func TestMCPContractMalformedAndCoercedNumbers(t *testing.T) {
	_, server, _ := contractRuntime(t, "compact", true, false)
	// Signature coercion is intentional legacy compatibility, not strict-schema
	// input validation. Preserve it while rejecting malformed nested execute args.
	for _, limit := range []any{1, "1", true} {
		contractTool(t, server, "actor", "memory_search", map[string]any{"query": "Title", "limit": limit})
	}
	for _, limit := range []any{"malformed", []any{}, map[string]any{}, nil} {
		response := contractRPC(t, server, "actor", "tools/call", map[string]any{"name": "memory_search", "arguments": map[string]any{"query": "Title", "limit": limit}})
		if response["error"] != nil {
			continue
		}
		result := contractResult(t, response)["structuredContent"].(map[string]any)
		if result["status"] != "error" {
			t.Fatal("malformed search accepted", limit, result)
		}
	}
	response := contractRPC(t, server, "actor", "tools/call", map[string]any{"name": "memory_execute", "arguments": map[string]any{"operations": []any{map[string]any{"op": "search", "args": map[string]any{"query": "Title", "limit": "malformed"}}}}})
	body := contractResult(t, response)["structuredContent"].(map[string]any)
	raw, _ := json.Marshal(body)
	if body["status"] != "error" || !strings.Contains(string(raw), "args.limit") {
		t.Fatal(body)
	}
	// Execute-only catalogue contracts remain accessible on compact startup.
	for _, name := range []string{"compare_manifest", "asset_metadata", "proposal_rebase"} {
		op := contractResource(t, server, "memory://catalog/"+name)
		if op["direct_tool_available"] != false || op["available_via_execute"] != true {
			t.Fatal(op)
		}
	}
}

func TestMCPContractStartupReclassifiesLegacyLinks(t *testing.T) {
	runtime, _, _ := contractRuntime(t, "compact", false, false)
	ctx := context.Background()
	root := filepath.Dir(runtime.Paths.ControlDB)
	db, err := derived.Connect(ctx, runtime.Paths.DerivedDB)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the old resolver at the same Git revision, without touching the
	// canonical repository or model embeddings.
	if _, err = db.Exec("DELETE FROM index_state WHERE key='link_resolution_version'; UPDATE links SET target_id=NULL,resolution_state='broken'"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	var config RuntimeConfig
	config.Repository.RootPath = root
	rebuilt, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "compact", Limits: endpointLimits(), Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	defer rebuilt.Close(ctx)
	db, err = derived.Connect(ctx, rebuilt.Paths.DerivedDB)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, state string
	if err = db.QueryRow("SELECT value FROM index_state WHERE key='link_resolution_version'").Scan(&version); err != nil || version != derived.LinkResolutionVersion {
		t.Fatal(version, err)
	}
	if err = db.QueryRow("SELECT resolution_state FROM links LIMIT 1").Scan(&state); err != nil || state != "resolved" {
		t.Fatal(state, err)
	}
}
