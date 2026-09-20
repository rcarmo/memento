package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

func TestRegisteredDirectMutationHTTP(t *testing.T) {
	ctx := context.Background()
	j, base := jobsTest(t)
	server := umcp.NewServer("direct-mutations")
	server.SetNotificationOutput(nil)
	notifications := []string{}
	if err := j.RegisterMutationTools(server, func(_ context.Context, method string, _ map[string]any, _ []string) error {
		notifications = append(notifications, method)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	opts := umcp.DefaultHTTPOptions()
	opts.AsyncReference = true
	transport, err := umcp.NewStreamableHTTP(server, opts, j.Identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	defer j.Workers.Drain(ctx)
	rpc := func(method string, params map[string]any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		request := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(string(raw)))
		request.Header.Set("Authorization", "Bearer token")
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response := httptest.NewRecorder()
		transport.ServeHTTP(response, request)
		var result map[string]any
		if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 {
			t.Fatal(response.Code, result, err)
		}
		return result
	}
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		result := rpc("tools/call", map[string]any{"name": name, "arguments": args})
		if result["error"] != nil {
			t.Fatal(result)
		}
		return result["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	success := func(name string, args map[string]any) map[string]any {
		t.Helper()
		result := call(name, args)
		if result["status"] != "success" {
			t.Fatal(result)
		}
		return result
	}
	list := rpc("tools/list", nil)
	if len(list["result"].(map[string]any)["tools"].([]any)) != 20 {
		t.Fatal(list)
	}
	if _, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/subscribe","params":{"uri":"memory://status"}}`), umcp.RequestContext{SessionID: "observer"}); err != nil {
		t.Fatal(err)
	}
	created := success("memory_create", map[string]any{"path": "/new.md", "concept_type": "concept", "title": "New", "body": "synthetic", "expected_revision": base, "idempotency_key": "create", "tags": []string{"z", "a", "z"}})
	if len(created["warnings"].([]any)) != 1 {
		t.Fatal(created)
	}
	revision := created["repo_revision"].(string)
	patch := map[string]any{"path": "/new.md", "body": "updated", "expected_revision": revision, "idempotency_key": "patch"}
	patched := success("memory_patch", patch)
	revision = patched["repo_revision"].(string)
	// Preview uses already-published content: body patch produces no diff.
	if patched["data"].(map[string]any)["diff"] != "" {
		t.Fatal(patched)
	}
	repeated := success("memory_patch", patch)
	if repeated["data"].(map[string]any)["replayed"] != true {
		t.Fatal(repeated)
	}
	renamed := success("memory_rename", map[string]any{"path": "/new.md", "new_path": "/renamed.md", "expected_revision": revision, "idempotency_key": "rename"})
	if renamed["repo_revision"] == revision {
		t.Fatal(renamed)
	}
	if _, err = os.Stat(filepath.Join(j.Controls.Queue.Paths.CurrentDir, "renamed.md")); err != nil {
		t.Fatal(err)
	}
	for _, args := range []map[string]any{{"path": "/renamed.md", "status": "invalid", "expected_revision": revision, "idempotency_key": "bad"}, {"path": "/renamed.md", "tags": []any{1}, "expected_revision": revision, "idempotency_key": "bad"}} {
		result := rpc("tools/call", map[string]any{"name": "memory_patch", "arguments": args})
		if result["error"] == nil || result["error"].(map[string]any)["code"] != float64(-32602) {
			t.Fatal(result)
		}
		if args["status"] == "invalid" && result["error"].(map[string]any)["message"] != "'invalid' is not a valid ConceptStatus" {
			t.Fatal(result)
		}
	}
	result := call("memory_patch", map[string]any{"path": "/renamed.md", "body": "different", "expected_revision": revision, "idempotency_key": "patch"})
	if result["error_class"] != "idempotency_conflict" {
		t.Fatal(result)
	}
	if len(notifications) != 8 {
		t.Fatal(notifications)
	}
	for i, method := range notifications {
		expected := "notifications/resources/list_changed"
		if i%2 != 0 {
			expected = "notifications/resources/updated"
		}
		if method != expected {
			t.Fatal(notifications)
		}
	}
}
func TestDirectConcurrentReplay(t *testing.T) {
	c, actor, base := realApplyTest(t)
	var wg sync.WaitGroup
	out := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, options, err := c.CommitConceptChange(context.Background(), actor, map[string]any{"kind": "create", "path": "/concurrent.md", "title": "Concurrent", "concept_type": "concept", "body": "synthetic"}, base, "concurrent")
			if err == nil && (options.OperationID == nil || len(data["changed_paths"].([]string)) != 1) {
				err = fmt.Errorf("%v", data)
			}
			out <- err
		}()
	}
	wg.Wait()
	close(out)
	for err := range out {
		if err != nil {
			t.Fatal(err)
		}
	}
	entry, err := repository.ReadBundleEntry(c.Queue.Paths.CurrentDir, "/concurrent.md")
	if err != nil || entry.Document.Body != "synthetic" {
		t.Fatal(entry, err)
	}
}
