package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"

	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/umcp"
)

func TestMCPStagingUploadProposalFlow(t *testing.T) {
	ctx := context.Background()
	j, base := jobsTest(t)
	j.Controls.Staging = &assets.StagingStore{DB: j.Controls.Queue.Proposals.DB, Now: j.Controls.Queue.Now}
	server := umcp.NewServer("staging-subset")
	server.SetNotificationOutput(nil)
	if err := j.RegisterStagingReadProposalTools(server, nil); err != nil {
		t.Fatal(err)
	}
	hooks := j.Identity.HTTPHooks()
	hooks.Route = (StagingHTTP{Store: j.Controls.Staging, Authenticate: j.Identity.AuthenticateHeaders}).Handle
	options := umcp.DefaultHTTPOptions()
	options.AsyncReference = true
	options.MaxRequestBytes = assets.MaxZIPBytes + 1024
	transport, err := umcp.NewStreamableHTTP(server, options, hooks)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	host.Client().Timeout = 3 * time.Second
	defer j.Workers.Drain(ctx)
	request := func(path string, body []byte, headers map[string]string) (int, map[string]any) {
		t.Helper()
		req, err := http.NewRequest("POST", host.URL+path, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		response, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var parsed map[string]any
		if err = json.NewDecoder(response.Body).Decode(&parsed); err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, parsed
	}
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		code, response := request("/mcp", raw, map[string]string{"Authorization": "Bearer token", "Content-Type": "application/json", "Mcp-Protocol-Version": "2025-03-26"})
		if code != 200 || response["error"] != nil {
			t.Fatal(code, response)
		}
		return response["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	// Source direct staging calls bypass the worker busy gate.
	j.Workers.SetExecuteBusy(true)
	locked, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		finished <- repository.WithTransactionLock(ctx, j.Controls.Queue.Paths, func() error { close(locked); <-release; return nil })
	}()
	<-locked
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	begin := call("memory_asset_stage_begin", map[string]any{"asset_kind": "docs", "version": "1.0.0", "idempotency_key": "ticket"})
	if begin["status"] != "success" {
		t.Fatal(begin)
	}
	ticket := begin["data"].(map[string]any)["upload_ticket"].(string)
	status := call("memory_asset_stage_status", map[string]any{"idempotency_key": "ticket"})
	if status["data"].(map[string]any)["state"] != "pending" {
		t.Fatal(status)
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	j.Workers.SetExecuteBusy(false)
	code, uploaded := request("/assets/staging/upload", submitZIP(t), map[string]string{"Content-Type": "application/zip", "X-Memento-Upload-Ticket": ticket})
	if code != 201 {
		t.Fatal(code, uploaded)
	}
	status = call("memory_asset_stage_status", map[string]any{"idempotency_key": "ticket"})
	data := status["data"].(map[string]any)
	if data["state"] != "uploaded" || data["staged_asset"].(map[string]any)["state"] != "ready" {
		t.Fatal(status)
	}
	proposed := call("memory_propose", map[string]any{"intent": "ticket upload", "base_revision": base, "changes": []any{map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": "docs", "version": "1.0.0", "staged_asset_id": uploaded["staged_asset_id"]}}})
	if proposed["status"] != "success" {
		t.Fatal(proposed)
	}
	status = call("memory_asset_stage_status", map[string]any{"idempotency_key": "ticket"})
	data = status["data"].(map[string]any)
	if data["state"] != "uploaded" || data["staged_asset"].(map[string]any)["state"] != "consumed" {
		t.Fatal(status)
	}
	repeated := call("memory_asset_stage_begin", map[string]any{"asset_kind": "docs", "version": "1.0.0", "idempotency_key": "ticket"})
	if repeated["error_class"] != "validation_error" {
		t.Fatal(repeated)
	}
}
func TestStagingDirectMissingContext(t *testing.T) {
	j, _ := jobsTest(t)
	if _, err := j.callStagingOrProposalTool(context.Background(), "memory_asset_stage_status", map[string]any{}); err == nil {
		t.Fatal("missing context")
	}
}

func TestStagingPrincipalRolesAndHTTPFailures(t *testing.T) {
	ctx := context.Background()
	j, _ := jobsTest(t)
	j.Controls.Staging = &assets.StagingStore{DB: j.Controls.Queue.Proposals.DB, Now: j.Controls.Queue.Now}
	// Namespace policy roles do not grant staging. Source reads principal roles
	// directly; managed-only principals are resolved afresh on every call.
	managed, err := access.OpenStore(ctx, j.Controls.Queue.Proposals.DB, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := managed.Create(ctx, "bootstrap", "dynamic", []string{"proposer"}, []string{"/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	j.Identity, err = NewIdentity([]BearerPrincipal{{"unused", access.Principal{Name: "dynamic", Roles: []string{"reader"}}}}, access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"dynamic": {Roles: []string{"proposer"}, ReadPrefixes: []string{"/"}}}}, managed, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("staging-identity")
	server.SetNotificationOutput(nil)
	if err = j.RegisterStagingReadProposalTools(server, nil); err != nil {
		t.Fatal(err)
	}
	opts := umcp.DefaultHTTPOptions()
	opts.AsyncReference = true
	transport, err := umcp.NewStreamableHTTP(server, opts, j.Identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		request := httptest.NewRequest("POST", "http://localhost/mcp", bytes.NewReader(raw))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response := httptest.NewRecorder()
		transport.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		var value map[string]any
		if err = json.Unmarshal(response.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	envelope := func(value map[string]any) map[string]any {
		t.Helper()
		if value["error"] != nil {
			t.Fatal(value)
		}
		return value["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	beginArgs := map[string]any{"idempotency_key": "key", "asset_kind": "docs", "version": "1.0.0"}
	value := envelope(call("memory_asset_stage_begin", beginArgs))
	if value["message"] != "proposer role is required" {
		t.Fatal(value)
	}
	delete(j.Identity.names, "dynamic") // now resolve this managed-only principal
	value = envelope(call("memory_asset_stage_begin", beginArgs))
	if value["status"] != "success" {
		t.Fatal(value)
	}
	_, err = managed.Update(ctx, "bootstrap", "dynamic", []string{"reader"}, []string{"/"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	value = envelope(call("memory_asset_stage_status", map[string]any{"idempotency_key": "key"}))
	if value["message"] != "proposer role is required" {
		t.Fatal(value)
	}
	_, err = managed.Update(ctx, "bootstrap", "dynamic", []string{"proposer"}, []string{"/"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A tool argument never selects the identity, nor changes the stored ticket.
	value = call("memory_asset_stage_status", map[string]any{"idempotency_key": "key", "principal": "other"})
	if value["error"] == nil {
		t.Fatal(value)
	}
	j.Controls.Staging = nil
	value = envelope(call("memory_asset_stage_status", map[string]any{"idempotency_key": "key"}))
	if value["message"] != "asset staging is unavailable" {
		t.Fatal(value)
	}
	// Store failures reach uMCP redaction rather than becoming validation errors.
	q, _ := queueTest(t)
	q.Proposals.DB.Close()
	j.Controls.Staging = &assets.StagingStore{DB: q.Proposals.DB}
	value = call("memory_asset_stage_status", map[string]any{"idempotency_key": "key"})
	if value["error"] == nil {
		t.Fatal(value)
	}
	rpc := value["error"].(map[string]any)
	if rpc["code"] != float64(-32603) || rpc["message"] != "Tool execution failed" {
		t.Fatal(value)
	}
}
