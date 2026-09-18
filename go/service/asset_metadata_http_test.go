package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

func TestRegisteredAssetMetadataHTTP(t *testing.T) {
	j, base := jobsTest(t)
	revision := seedPruneAssets(t, j, base)
	ctx := context.Background()
	var err error
	j.Identity, err = NewIdentity([]BearerPrincipal{{"token", access.Principal{Name: "reader", Roles: []string{"reader"}}}}, access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"reader": {Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("asset-metadata")
	server.SetNotificationOutput(nil)
	if err = j.RegisterMetadataTools(server, nil); err != nil {
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
		req := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(string(raw)))
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response := httptest.NewRecorder()
		transport.ServeHTTP(response, req)
		var v map[string]any
		if err = json.Unmarshal(response.Body.Bytes(), &v); err != nil || response.Code != 200 {
			t.Fatal(v, err)
		}
		return v
	}
	call := func(args map[string]any) map[string]any {
		t.Helper()
		v := rpc("tools/call", map[string]any{"name": "memory_asset_metadata", "arguments": args})
		if v["error"] != nil {
			t.Fatal(v)
		}
		return v["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	if v := rpc("tools/list", nil); len(v["result"].(map[string]any)["tools"].([]any)) != 26 {
		t.Fatal(v)
	}
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
	finished := make(chan map[string]any, 1)
	go func() {
		finished <- call(map[string]any{"id_or_path": "/a.md", "version_limit": 2, "include_files": "yes", "file_limit": 1})
	}()
	var result map[string]any
	select {
	case result = <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("metadata blocked by writer lock")
	}
	if result["status"] != "success" || result["repo_revision"] != revision {
		t.Fatal(result)
	}
	data := result["data"].(map[string]any)
	asset := data["entries"].([]any)[0].(map[string]any)["assets"].([]any)[0].(map[string]any)
	if fmt.Sprint(asset["versions"]) != "[1.9.0 1.10.0]" || asset["version_metadata_truncated"] != true {
		t.Fatal(asset)
	}
	details := asset["version_metadata"].([]any)
	if details[0].(map[string]any)["version"] != "1.10.0" || details[1].(map[string]any)["version"] != "1.9.0" {
		t.Fatal(details)
	}
	for _, detail := range details {
		if detail.(map[string]any)["created_at"] != modelTimestamp(j.Controls.Queue.Now()) {
			t.Fatal("git author timestamp", detail)
		}
	}
	missing := call(map[string]any{"id_or_path": "/a.md", "asset_kind": "docs", "version": "9.0.0"})
	if missing["status"] != "success" || missing["data"].(map[string]any)["entries"].([]any)[0].(map[string]any)["assets"].([]any)[0].(map[string]any)["requested_version_present"] != false {
		t.Fatal(missing)
	}
	invalid := rpc("tools/call", map[string]any{"name": "memory_asset_metadata", "arguments": map[string]any{"id_or_path": 1}})
	if invalid["error"] == nil {
		t.Fatal(invalid)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestMetadataConcurrentReads(t *testing.T) {
	j, base := jobsTest(t)
	seedPruneAssets(t, j, base)
	actor := ProposalActor{Policy: access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}}
	id := "/a.md"
	var wg sync.WaitGroup
	errors := make(chan error, 12)
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			data, _, err := j.Controls.AssetMetadata(context.Background(), actor, AssetMetadataOptions{IDOrPath: &id, Limit: 20, VersionLimit: i%3 + 1, FileLimit: 1, IncludeFiles: i%2 == 0})
			if err == nil {
				a := data["entries"].([]any)[0].(map[string]any)["assets"].([]any)[0].(map[string]any)
				if len(a["version_metadata"].([]any)) != i%3+1 {
					err = fmt.Errorf("projection leaked")
				}
			}
			errors <- err
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
}
