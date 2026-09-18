package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

func TestRegisteredInventoryHTTP(t *testing.T) {
	ctx := context.Background()
	j, revision := jobsTest(t)
	files, _ := assetGetFixture(t)
	installAssetFiles(t, j.Controls.Queue.Paths.CurrentDir, files)
	var err error
	j.Identity, err = NewIdentity([]BearerPrincipal{{"reader", access.Principal{Name: "reader", Roles: []string{"reader"}}}, {"nested", access.Principal{Name: "nested", Roles: []string{"reader"}}}}, access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"reader": {Roles: []string{"reader"}, ReadPrefixes: []string{"/public/"}}, "nested": {Roles: []string{"reader"}, ReadPrefixes: []string{"/public/nested/"}}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Unreadable malformed siblings and unreadable symlink trees do not leak.
	if err = os.WriteFile(filepath.Join(j.Controls.Queue.Paths.CurrentDir, "private/bad.md"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(t.TempDir(), filepath.Join(j.Controls.Queue.Paths.CurrentDir, "private/link")); err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("inventory-subset")
	server.SetNotificationOutput(nil)
	if err = j.RegisterInventoryTools(server, nil); err != nil {
		t.Fatal(err)
	}
	opts := umcp.DefaultHTTPOptions()
	opts.AsyncReference = true
	transport, err := umcp.NewStreamableHTTP(server, opts, j.Identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	host.Client().Timeout = 3 * time.Second
	defer j.Workers.Drain(ctx)
	rpc := func(token, method string, params map[string]any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest("POST", host.URL+"/mcp", strings.NewReader(string(raw)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var value map[string]any
		if err = json.NewDecoder(response.Body).Decode(&value); err != nil || response.StatusCode != 200 {
			t.Fatal(response.StatusCode, value, err)
		}
		return value
	}
	call := func(token string, args map[string]any) map[string]any {
		t.Helper()
		v := rpc(token, "tools/call", map[string]any{"name": "memory_inventory", "arguments": args})
		if v["error"] != nil {
			t.Fatal(v)
		}
		return v["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	listed := rpc("reader", "tools/list", nil)
	if len(listed["result"].(map[string]any)["tools"].([]any)) != 24 {
		t.Fatal(listed)
	}
	// Inventory is not @_serialized. A held writer lock cannot block reads.
	locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- repository.WithTransactionLock(ctx, j.Controls.Queue.Paths, func() error { close(locked); <-release; return nil })
	}()
	<-locked
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	first := call("reader", map[string]any{"path_prefix": "/public/", "limit": 1, "fields": []string{"path", "assets", "body_bytes", "path"}})
	if first["status"] != "success" || first["repo_revision"] != revision {
		t.Fatal(first)
	}
	data := first["data"].(map[string]any)
	if len(data["fields"].([]any)) != 3 || data["next_cursor"] != "/public/a.md" {
		t.Fatal(data)
	}
	next := call("reader", map[string]any{"path_prefix": "/public/", "limit": 1, "cursor": data["next_cursor"], "fields": []string{"path"}})
	if next["data"].(map[string]any)["next_cursor"] != nil {
		t.Fatal(next)
	}
	restricted := call("nested", map[string]any{})
	if restricted["status"] != "success" || len(restricted["data"].(map[string]any)["entries"].([]any)) != 0 {
		t.Fatal(restricted)
	}
	denied := call("nested", map[string]any{"path_prefix": "/private/"})
	if denied["error_class"] != "forbidden" {
		t.Fatal(denied)
	}
	empty := call("reader", map[string]any{"fields": []string{}})
	if empty["error_class"] != "validation_error" {
		t.Fatal(empty)
	}
	stringFields := call("reader", map[string]any{"fields": "path"})
	if stringFields["message"] != "unsupported inventory fields: a, h, p, t" {
		t.Fatal(stringFields)
	}
	for _, fields := range []any{1, []any{1}} {
		v := rpc("reader", "tools/call", map[string]any{"name": "memory_inventory", "arguments": map[string]any{"fields": fields}})
		if v["error"] == nil {
			t.Fatal(v)
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestInventoryConcurrentProjection(t *testing.T) {
	c, actor, _ := realApplyTest(t)
	actor.Policy.Roles = []string{"reader"}
	files, _ := assetGetFixture(t)
	installAssetFiles(t, c.Queue.Paths.CurrentDir, files)
	var wg sync.WaitGroup
	out := make(chan error, 16)
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			fields := []string{"path"}
			if i%2 == 0 {
				fields = nil
			}
			data, _, err := c.Inventory(context.Background(), actor, "/public/", fields, 1, nil)
			if err == nil {
				entry := data["entries"].([]any)[0].(map[string]any)
				if (fields == nil && entry["assets"] == nil) || (fields != nil && len(entry) != 1) {
					err = fmt.Errorf("projection leaked: %v", entry)
				}
			}
			out <- err
		}(i)
	}
	wg.Wait()
	close(out)
	for err := range out {
		if err != nil {
			t.Fatal(err)
		}
	}
}
