package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/umcp"
)

func TestRegisteredProposalToolsHTTP(t *testing.T) {
	j, base := jobsTest(t)
	ctx := context.Background()
	j.Controls.Staging = &assets.StagingStore{DB: j.Controls.Queue.Proposals.DB, Now: j.Controls.Queue.Now}
	server := umcp.NewServer("memento-proposal-subset")
	server.SetNotificationOutput(nil)
	var mu sync.Mutex
	notifications := []string{}
	if err := j.RegisterProposalTools(server, func(_ context.Context, method string, _ map[string]any, _ []string) error {
		mu.Lock()
		notifications = append(notifications, method)
		mu.Unlock()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	options := umcp.DefaultHTTPOptions()
	options.AsyncReference = true
	transport, err := umcp.NewStreamableHTTP(server, options, j.Identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	defer j.Workers.Drain(ctx)
	serial := 0
	session := ""
	rpc := func(method string, params map[string]any) map[string]any {
		t.Helper()
		serial++
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": serial, "method": method, "params": params})
		req, err := http.NewRequest("POST", host.URL+"/mcp", strings.NewReader(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		response, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		content, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 {
			t.Fatal(response.StatusCode, string(content))
		}
		if value := response.Header.Get("Mcp-Session-Id"); value != "" {
			session = value
		}
		var parsed map[string]any
		if err = json.Unmarshal(content, &parsed); err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		response := rpc("tools/call", map[string]any{"name": name, "arguments": args})
		if response["error"] != nil {
			t.Fatal(name, response)
		}
		result := response["result"].(map[string]any)
		return result["structuredContent"].(map[string]any)
	}
	success := func(name string, args map[string]any) map[string]any {
		t.Helper()
		result := call(name, args)
		if result["status"] != "success" {
			t.Fatal(name, result)
		}
		return result
	}
	rpc("initialize", map[string]any{"protocolVersion": "2025-03-26"})
	rpc("resources/subscribe", map[string]any{"uri": "memory://status"})
	listed := rpc("tools/list", map[string]any{})
	if len(listed["result"].(map[string]any)["tools"].([]any)) != 9 {
		t.Fatal(listed)
	}
	created := success("memory_propose", map[string]any{"intent": "HTTP flow", "base_revision": base, "changes": []any{map[string]any{"kind": "create", "path": "/new.md", "concept_type": "concept", "title": "New", "body": "synthetic"}}})
	id := created["data"].(map[string]any)["proposal"].(map[string]any)["proposal_id"].(string)
	for _, view := range []string{"detailed", "summary", "unknown"} {
		success("memory_proposal_get", map[string]any{"proposal_id": id, "view": view})
	}
	page := success("memory_proposal_list", map[string]any{"limit": "1"})["data"].(map[string]any)
	if page["next_cursor"] != nil {
		success("memory_proposal_list", map[string]any{"limit": 1, "cursor": page["next_cursor"]})
	}
	reviewed := success("memory_proposal_review", map[string]any{"proposal_id": id, "decision": "approve", "idempotency_key": "review"})
	operationID := reviewed["operation_id"].(string)
	success("memory_operation_get", map[string]any{"operation_id": operationID})
	applied := success("memory_proposal_apply", map[string]any{"proposal_id": id, "expected_revision": base, "idempotency_key": "apply"})
	revision := applied["repo_revision"].(string)
	replay := success("memory_proposal_apply", map[string]any{"proposal_id": id, "expected_revision": base, "idempotency_key": "apply"})
	if replay["data"].(map[string]any)["replayed"] != true {
		t.Fatal(replay)
	}
	reconciled := success("memory_operation_get", map[string]any{"idempotency_key": "apply"})
	if reconciled["data"].(map[string]any)["final_state"] != "committed" {
		t.Fatal(reconciled)
	}
	if _, err := os.Stat(filepath.Join(j.Controls.Queue.Paths.CurrentDir, "new.md")); err != nil {
		t.Fatal(err)
	}
	// The original fixture proposal targets /a.md, clean but now at an old base.
	revised := success("memory_proposal_revise", map[string]any{"proposal_id": "proposal", "selected_change_indexes": []int{0, 0}, "expected_revision": revision})
	if revised["data"].(map[string]any)["source_proposal_id"] != "proposal" {
		t.Fatal(revised)
	}
	success("memory_proposal_rebase", map[string]any{"proposal_id": "proposal", "expected_revision": revision, "idempotency_key": "rebase"})
	// Fetch a real copied proposal asset through the same registered surface.
	blob := submitZIP(t)
	upload, _, err := j.Controls.Staging.Put(ctx, "actor", "stage", "docs", "1.0.0", blob)
	if err != nil {
		t.Fatal(err)
	}
	assetProposal := success("memory_propose", map[string]any{"intent": "asset", "base_revision": revision, "changes": []any{map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": "docs", "version": "1.0.0", "staged_asset_id": upload.StagedAssetID}}})["data"].(map[string]any)["proposal"].(map[string]any)
	assetID := assetProposal["changes"].([]any)[0].(map[string]any)["asset_id"].(string)
	success("memory_proposal_asset_get", map[string]any{"proposal_id": assetProposal["proposal_id"], "asset_id": assetID})
	success("memory_proposal_asset_get", map[string]any{"proposal_id": assetProposal["proposal_id"], "asset_id": assetID, "file_path": "document.txt", "offset": "0", "limit": "8"})
	if invalid := rpc("tools/call", map[string]any{"name": "memory_proposal_get", "arguments": map[string]any{"proposal_id": id, "principal": "admin"}}); invalid["error"].(map[string]any)["code"] != float64(-32602) {
		t.Fatal(invalid)
	}
	if failure := call("memory_operation_get", map[string]any{}); failure["error_class"] != "validation_error" {
		t.Fatal(failure)
	}
	mu.Lock()
	defer mu.Unlock()
	if fmt.Sprint(notifications) != "[notifications/resources/list_changed notifications/resources/updated notifications/resources/list_changed notifications/resources/updated]" {
		t.Fatal(notifications)
	}
}
