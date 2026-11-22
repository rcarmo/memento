package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcarmo/memento/umcp"
)

type failingFileRandom struct{}

func (failingFileRandom) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestMCPFilesReadOnlyAndCustomEndpoint(t *testing.T) {
	h := newFileHarness(t)
	ctx := context.Background()
	server := umcp.NewServer("read-only")
	f, err := NewFileTransfers(ctx, h.jobs, server, false)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": fileParams(fileUploadTool, fileArguments(submitZIP(t), "disabled"))})
	trusted := umcp.RequestContext{Transport: "streamable-http", ProtocolVersion: fileProtocol, Principal: "actor"}
	response, err := server.Process(ctx, request, trusted)
	if err != nil || response.Error == nil || response.Error.Code != -32601 {
		t.Fatal(response, err)
	}
	request = []byte(`{"jsonrpc":"2.0","id":1,"method":"server/discover"}`)
	response, err = server.Process(ctx, request, trusted)
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	settings := response.Result.(map[string]any)["capabilities"].(map[string]any)["extensions"].(map[string]any)[fileExtension].(map[string]any)
	if _, exists := settings["upload"]; exists {
		t.Fatal(settings)
	}
	r, err := f.handle(ctx, "POST", "/files/upload", map[string]string{"authorization": "Bearer actor"}, 0, nil)
	if err != nil || r.Status != 403 {
		t.Fatal(r, err)
	}
	// An adapter instance behind another MCP path resolves relative targets there.
	options := umcp.DefaultHTTPOptions()
	options.Endpoint = "/nested/mcp"
	options.MaxRequestBytes = 72 << 20
	hooks := h.jobs.Identity.HTTPHooks()
	hooks.BodyRoute = h.files.HTTPRoute()
	transport, err := umcp.NewStreamableHTTP(h.server, options, hooks)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	raw := submitZIP(t)
	state, headers := h.begin(fileArguments(raw, "custom-path"))
	body, discoverHeaders := modernFileRequest("server/discover", nil)
	// Reuse the harness request helper against the second host.
	other := *h
	other.host = host
	status, _ := other.request("POST", "/nested/mcp", "actor", body, discoverHeaders)
	if status != 200 {
		t.Fatal(status)
	}
	status, _ = other.request("POST", "/nested/mcp/files/upload", "actor", raw, headers)
	if status != 200 {
		t.Fatal(status, state)
	}
}
func TestMCPFilesConstructorAndStorageFailures(t *testing.T) {
	h := newFileHarness(t)
	ctx := context.Background()
	server := umcp.NewServer("filter")
	server.Tools.Visible = func(context.Context, umcp.Tool) bool { return false }
	if _, err := NewFileTransfers(ctx, h.jobs, server, true); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{"_meta": fileMeta(true, true)}})
	response, err := server.Process(ctx, request, umcp.RequestContext{Transport: "streamable-http", ProtocolVersion: fileProtocol, Principal: "actor"})
	if err != nil || response.Error != nil || len(response.Result.(map[string]any)["tools"].([]any)) != 0 {
		t.Fatal(response, err)
	}
	if _, err := newFileTransfers(ctx, h.jobs, umcp.NewServer("bad"), true, []umcp.Tool{{Name: "invalid"}}); err == nil {
		t.Fatal("invalid tool metadata")
	}
	h.files.random = failingFileRandom{}
	if _, _, err := h.files.issue(ctx, "actor", fileUploadTool, "{}", "true/true", 1); err == nil {
		t.Fatal("entropy failure")
	}
	// Closed storage fails without producing a usable target.
	if err := h.jobs.Controls.Staging.DB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileTransfers(ctx, h.jobs, umcp.NewServer("closed"), true); err == nil {
		t.Fatal("closed database")
	}
	h.files.random = strings.NewReader(strings.Repeat("r", 32))
	if _, _, err := h.files.issue(ctx, "actor", fileUploadTool, "{}", "true/true", 1); err == nil {
		t.Fatal("closed storage")
	}
}
func TestMCPFilesProtocolAndCancellationFailures(t *testing.T) {
	h := newFileHarness(t)
	ctx := context.Background()
	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"server/discover"}`)
	response, err := h.server.Process(ctx, request, umcp.RequestContext{Transport: "streamable-http", ProtocolVersion: "2025-03-26", Principal: "actor"})
	if err != nil || response.Error == nil || response.Error.Code != -32601 {
		t.Fatal(response, err)
	}
	for _, tool := range []string{fileUploadTool, fileDownloadTool} {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": fileParams(tool, fileArguments(submitZIP(t), "untrusted"))})
		r, e := h.server.Process(ctx, body, umcp.RequestContext{Transport: "streamable-http", ProtocolVersion: fileProtocol, Principal: "unconfigured"})
		if e != nil || r.Error != nil || r.Result.(map[string]any)["isError"] != true {
			t.Fatal(r, e)
		}
	}
	state, headers := h.begin(fileArguments(submitZIP(t), "cancel"))
	_ = state
	h.files.gate <- struct{}{}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	rejected, err := h.files.handle(cancelled, "POST", "/files/upload", map[string]string{"authorization": "Bearer actor", fileTokenHeader: headers["X-Memento-File-Token"]}, 0, nil)
	<-h.files.gate
	if err != nil || rejected.Status < 400 {
		t.Fatal(rejected, err)
	}
}

func TestMCPFilesCompletedReceiptSurvivesConsumption(t *testing.T) {
	h := newFileHarness(t)
	state, args := h.acceptPack()
	p := fileParams(fileUploadTool, args)
	p["requestState"] = state
	result := fileResult(t, h.rpc("tools/call", "actor", p))
	if result["isError"] == true {
		t.Fatal(result)
	}
	stage := result["structuredContent"].(map[string]any)["staged_asset_id"]
	var size int64
	if err := h.jobs.Controls.Staging.DB.QueryRow("SELECT length(blob_bytes) FROM staged_assets WHERE staged_asset_id=?", stage).Scan(&size); err != nil || size != 0 {
		t.Fatal(size, err)
	}
	// Receipt replay is independent of staging/proposal byte lifecycle.
	server := umcp.NewServer("restart")
	if _, err := NewFileTransfers(context.Background(), h.jobs, server, true); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": p})
	r, err := server.Process(context.Background(), raw, umcp.RequestContext{Transport: "streamable-http", ProtocolVersion: fileProtocol, Principal: "actor"})
	if err != nil || r.Error != nil || r.Result.(map[string]any)["isError"] == true {
		t.Fatal(r, err)
	}
	if value, _, _ := completedFileTransfer("invalid-json"); value.(map[string]any)["isError"] != true {
		t.Fatal(value)
	}
}
