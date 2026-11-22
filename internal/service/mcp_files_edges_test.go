package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

func TestMCPFilesRuntimeWiring(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	config.MCP = defaultMCPConfig()
	config.MCP.ExperimentalFiles = true
	config.Authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"actor": {Roles: []string{"reader", "proposer"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}}}
	options := ModelsOffRuntimeOptions{Surface: "compact", Tokens: []BearerPrincipal{{Token: "actor", Principal: access.Principal{Name: "actor", Roles: []string{"reader", "proposer"}}}}}
	runtime, server, err := BuildModelsOffRuntime(ctx, config, options)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	transportOptions := umcp.DefaultHTTPOptions()
	transportOptions.MaxRequestBytes = config.MCP.MaxRequestBytes
	transport, err := umcp.NewStreamableHTTP(server, transportOptions, runtime.HTTPHooks)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	host.Client().Timeout = 5 * time.Second
	h := &fileHarness{t: t, jobs: runtime.Jobs, server: server, host: host}
	raw := submitZIP(t)
	args := fileArguments(raw, "runtime")
	state, headers := h.begin(args)
	status, _ := h.request("POST", "/mcp/files/upload", "actor", raw, headers)
	if status != 200 {
		t.Fatal(status)
	}
	p := fileParams(fileUploadTool, args)
	p["requestState"] = state
	if r := fileResult(t, h.rpc("tools/call", "actor", p)); r["isError"] == true {
		t.Fatal(r)
	}
	if r, e := runtime.HTTPHooks.Route(ctx, "GET", "/not-a-file-target", nil, nil, ""); r == nil || r.Status != 404 || e != nil {
		t.Fatal(r, e)
	}
	config.MCP.MaxRequestBytes = 4 << 20
	if config.MCP.Validate() == nil {
		t.Fatal("undersized transport accepted")
	}
	config.MCP.ExperimentalFiles = false
	if err := config.MCP.Validate(); err != nil {
		t.Fatal(err)
	}
	rawConfig, _ := json.Marshal(config.MCP)
	if strings.Contains(string(rawConfig), "experimental_files") {
		t.Fatal("default configuration changed", string(rawConfig))
	}
}
func TestMCPFilesBothHTTPModes(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(fmt.Sprint(async), func(t *testing.T) {
			h := newFileHarness(t, async)
			raw := submitZIP(t)
			args := fileArguments(raw, "transport")
			state, headers := h.begin(args)
			status, _ := h.request("POST", "/mcp/files/upload", "actor", raw, headers)
			if status != 200 {
				t.Fatal(status)
			}
			p := fileParams(fileUploadTool, args)
			p["requestState"] = state
			if r := fileResult(t, h.rpc("tools/call", "actor", p)); r["isError"] == true {
				t.Fatal(r)
			}
		})
	}
}
func TestMCPFilesErrorsAndQuotas(t *testing.T) {
	h := newFileHarness(t)
	ctx := context.Background()
	if _, err := NewFileTransfers(ctx, nil, h.server, true); err == nil {
		t.Fatal("nil jobs")
	}
	if _, err := NewFileTransfers(ctx, h.jobs, nil, true); err == nil {
		t.Fatal("nil server")
	}
	raw := submitZIP(t)
	args := fileArguments(raw, "errors")
	state, headers := h.begin(args)
	status, _ := h.request("GET", "/mcp/files/upload", "actor", nil, headers)
	if status != 405 {
		t.Fatal(status)
	}
	status, _ = h.request("OPTIONS", "/mcp/files/upload", "", nil, nil)
	if status != 204 {
		t.Fatal(status)
	}
	badHeaders := map[string]string{"X-Memento-File-Token": state, "Content-Type": "application/pdf"}
	status, _ = h.request("POST", "/mcp/files/upload", "actor", raw, badHeaders)
	if status != 415 {
		t.Fatal(status)
	}
	status, _ = h.request("GET", "/mcp/files/download", "actor", nil, headers)
	if status != 404 {
		t.Fatal(status)
	}
	p := fileParams(fileUploadTool, args)
	p["requestState"] = false
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32602)
	p["arguments"] = false
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32602)
	delete(p, "requestState")
	if r := fileResult(t, h.rpc("tools/call", "actor", p)); r["isError"] != true {
		t.Fatal(r)
	}
	p = fileParams(fileUploadTool, args)
	p["_meta"] = map[string]any{"io.modelcontextprotocol/clientCapabilities": map[string]any{"extensions": map[string]any{fileExtension: map[string]any{"revision": "unrecognised", "upload": map[string]any{}}}}}
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32021)
	for i := 0; i < 3; i++ {
		_, _ = h.begin(fileArguments(raw, fmt.Sprint(i)))
	}
	if r := fileResult(t, h.rpc("tools/call", "actor", fileParams(fileUploadTool, fileArguments(raw, "quota")))); r["isError"] != true {
		t.Fatal(r)
	}
	if _, _, err := h.files.issue(ctx, "other", fileUploadTool, "{}", "true/true", 2*50*1024*1024); err == nil {
		t.Fatal("byte reservation quota")
	}
	// No session context and unsupported transports cannot acquire file targets.
	if _, rpcErr, err := h.server.Tools.Call(ctx, map[string]any{"name": fileUploadTool}); err != nil || rpcErr == nil {
		t.Fatal(rpcErr, err)
	}
	if _, _, err := h.files.call(ctx, fileParams(fileUploadTool, args)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.files.record(ctx, "short"); err == nil {
		t.Fatal("short state")
	}
	// Expired transfer metadata is pruned at the next issuance.
	h.files.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, _, err := h.files.issue(ctx, "actor", fileUploadTool, "{}", "true/true", 1); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := h.jobs.Controls.Staging.DB.QueryRow("SELECT COUNT(*) FROM mcp_file_transfers").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
func TestMCPFilesDownloadValidation(t *testing.T) {
	h := newFileHarness(t)
	for _, args := range []map[string]any{{}, {"id_or_path": "/a.md", "asset_kind": "docs", "unknown": true}, {"id_or_path": "/a.md", "asset_kind": "docs", "version": false}, {"id_or_path": "/missing.md", "asset_kind": "docs"}} {
		r := fileResult(t, h.rpc("tools/call", "actor", fileParams(fileDownloadTool, args)))
		if r["isError"] != true {
			t.Fatal(r)
		}
	}
	p := fileParams(fileDownloadTool, map[string]any{"id_or_path": "/a.md", "asset_kind": "docs"})
	p["requestState"] = "unexpected"
	fileRPCError(t, h.rpc("tools/call", "actor", p), -32602)
}
func TestMCPFilesMetadataEdges(t *testing.T) {
	raw := submitZIP(t)
	mutate := []func(map[string]any){
		func(a map[string]any) { a["unknown"] = true }, func(a map[string]any) { a["asset_kind"] = "../bad" }, func(a map[string]any) { a["version"] = "bad" }, func(a map[string]any) { a["idempotency_key"] = " " }, func(a map[string]any) { a["file"] = nil },
		func(a map[string]any) { a["file"].(map[string]any)["id"] = "" }, func(a map[string]any) { a["file"].(map[string]any)["name"] = false }, func(a map[string]any) { a["file"].(map[string]any)["size"] = false }, func(a map[string]any) { a["file"].(map[string]any)["size"] = json.Number(strings.Repeat("1", 65)) },
		func(a map[string]any) { a["file"].(map[string]any)["digest"] = nil }, func(a map[string]any) { a["file"].(map[string]any)["digest"].(map[string]any)["value"] = false }, func(a map[string]any) {
			a["file"].(map[string]any)["digest"].(map[string]any)["value"] = base64.RawURLEncoding.EncodeToString([]byte("short"))
		},
	}
	for i, change := range mutate {
		args := fileArguments(raw, "edge")
		change(args)
		if _, _, err := parseFileArguments(args); err == nil {
			t.Fatal("accepted", i)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (checkedFileReader{cancelled, strings.NewReader("x")}).Read(make([]byte, 1)); err == nil {
		t.Fatal("cancelled read")
	}
}
func BenchmarkMCPFileMetadata(b *testing.B) {
	args := fileArguments(make([]byte, 1<<20), "benchmark")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := parseFileArguments(args); err != nil {
			b.Fatal(err)
		}
	}
}
