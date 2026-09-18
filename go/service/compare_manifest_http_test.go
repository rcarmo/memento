package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

func TestRegisteredManifestHTTP(t *testing.T) {
	ctx := context.Background()
	j, revision := jobsTest(t)
	files, _ := assetGetFixture(t)
	installAssetFiles(t, j.Controls.Queue.Paths.CurrentDir, files)
	j.Identity, _ = NewIdentity([]BearerPrincipal{{"token", access.Principal{Name: "reader", Roles: []string{"reader"}}}}, access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"reader": {Roles: []string{"reader"}, ReadPrefixes: []string{"/public/"}}}}, nil, nil)
	server := umcp.NewServer("manifest-subset")
	server.SetNotificationOutput(nil)
	if err := j.RegisterManifestTools(server, nil); err != nil {
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
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer token")
		request.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response := httptest.NewRecorder()
		transport.ServeHTTP(response, request)
		var value map[string]any
		if err = json.Unmarshal(response.Body.Bytes(), &value); err != nil || response.Code != 200 {
			t.Fatal(value, err)
		}
		return value
	}
	call := func(args map[string]any) map[string]any {
		t.Helper()
		v := rpc("tools/call", map[string]any{"name": "memory_compare_manifest", "arguments": args})
		if v["error"] != nil {
			t.Fatal(v)
		}
		return v["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	listed := rpc("tools/list", nil)
	if len(listed["result"].(map[string]any)["tools"].([]any)) != 25 {
		t.Fatal(listed)
	}
	// Never take the writer lock for this source read operation.
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
	item := manifestTestItem()
	huge := json.Number("100000000000000000000000000000000000000000000000001")
	item["local_bytes"] = huge
	delete(item, "memento_path")
	args := map[string]any{"path_prefix": "/public/", "items": []any{item}, "match": map[string]any{"path_template": "/public/{name}.md"}, "include_asset_metadata": "yes"}
	finished := make(chan map[string]any, 1)
	go func() { finished <- call(args) }()
	var result map[string]any
	select {
	case result = <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("read blocked by writer")
	}
	if result["status"] != "success" || result["repo_revision"] != revision {
		t.Fatal(result)
	}
	// Structured results are inspected with UseNumber separately to ensure the
	// service/uMCP conversion never rounds arbitrary-sized local byte counts.
	j.Workers.SetExecuteBusy(true)
	if busy := call(args); busy["error_class"] != "busy" {
		t.Fatal(busy)
	}
	j.Workers.SetExecuteBusy(false)
	for _, bad := range []map[string]any{{"path_prefix": "/public/", "items": 1}, {"path_prefix": "/public/", "items": []any{item}, "match": 1}} {
		v := rpc("tools/call", map[string]any{"name": "memory_compare_manifest", "arguments": bad})
		if v["error"] == nil {
			t.Fatal(v)
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	value, _, err := j.Controls.CompareManifest(ctx, ProposalActor{Policy: access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/public/"}}}, "/public/", []any{item}, args["match"].(map[string]any), true)
	if err != nil {
		t.Fatal(err)
	}
	converted, err := MCPEnvelope(value)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(converted)
	if !strings.Contains(string(encoded), string(huge)) {
		t.Fatal("rounded integer", string(encoded))
	}
}
func TestManifestWorkerRechecksManagedPolicy(t *testing.T) {
	ctx := context.Background()
	j, _ := jobsTest(t)
	files, _ := assetGetFixture(t)
	installAssetFiles(t, j.Controls.Queue.Paths.CurrentDir, files)
	store, err := access.OpenStore(ctx, j.Controls.Queue.Proposals.DB, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Create(ctx, "bootstrap", "actor", []string{"reader"}, []string{"/public/"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	j.Identity.managed = store
	// The outer service actor keeps the old grant; the nested inventory must use
	// the worker-owned resolver to see the update, not that captured policy.
	result, err := j.run(ctx, access.Principal{Name: "actor", Roles: []string{"reader"}}, nil, "memory_compare_manifest", func(ctx context.Context, c *ProposalControls, actor ProposalActor) (map[string]any, SuccessOptions, error) {
		if _, err := store.Update(ctx, "bootstrap", "actor", []string{"reader"}, []string{"/else/"}, nil); err != nil {
			return nil, SuccessOptions{}, err
		}
		return c.CompareManifest(ctx, actor, "/public/", []any{manifestTestItem()}, nil, false)
	}, control.Connect)
	if err != nil || jsonNormal(result).(map[string]any)["error_class"] != "forbidden" {
		t.Fatal(result, err)
	}
}
func TestManifestConcurrentReadIsolation(t *testing.T) {
	c, actor, _ := realApplyTest(t)
	actor.Policy.Roles = []string{"reader"}
	files, _ := assetGetFixture(t)
	installAssetFiles(t, c.Queue.Paths.CurrentDir, files)
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			item := manifestTestItem()
			item["name"] = fmt.Sprint(i)
			data, _, err := c.CompareManifest(context.Background(), actor, "/public/", []any{item}, nil, i%2 == 0)
			if err == nil {
				first := data["differing"].([]any)[0].(map[string]any)
				if first["name"] != fmt.Sprint(i) {
					err = io.ErrUnexpectedEOF
				}
			}
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
}
