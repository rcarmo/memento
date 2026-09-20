package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

func TestRegisteredAssetReadHTTP(t *testing.T) {
	ctx := context.Background()
	j, revision := jobsTest(t)
	files, _ := assetGetFixture(t)
	installAssetFiles(t, j.Controls.Queue.Paths.CurrentDir, files)
	var err error
	j.Identity, err = NewIdentity([]BearerPrincipal{{"token", access.Principal{Name: "reader", Roles: []string{"reader"}}}, {"denied", access.Principal{Name: "actor", Roles: []string{"proposer"}}}}, access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"reader": {Roles: []string{"reader"}, ReadPrefixes: []string{"/public/"}}, "actor": {Roles: []string{"proposer"}, ReadPrefixes: []string{"/"}}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("accepted-assets")
	server.SetNotificationOutput(nil)
	if err = j.RegisterAssetReadProposalTools(server, nil); err != nil {
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
	host.Client().Timeout = 5 * time.Second
	defer j.Workers.Drain(ctx)
	call := func(token string, args map[string]any) (map[string]any, error) {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "memory_asset_get", "arguments": args}})
		request, _ := http.NewRequest("POST", host.URL+"/mcp", bytes.NewReader(raw))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := host.Client().Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		var result map[string]any
		err = json.NewDecoder(response.Body).Decode(&result)
		return result, err
	}
	envelope := func(token string, args map[string]any) map[string]any {
		t.Helper()
		result, err := call(token, args)
		if err != nil || result["error"] != nil {
			t.Fatal(result, err)
		}
		return result["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	args := map[string]any{"id_or_path": "/public/a.md", "asset_kind": "docs"}
	value := envelope("token", args)
	if value["status"] != "success" || value["repo_revision"] != revision {
		t.Fatal(value)
	}
	data := value["data"].(map[string]any)
	raw, err := base64.StdEncoding.DecodeString(data["zip_base64"].(string))
	if err != nil || !bytes.Equal(raw, files["/.assets/12345678/docs/1.10.0.zip"]) {
		t.Fatal(data, err)
	}
	digest := data["zip_sha256"]
	for _, overrides := range []map[string]any{{"offset": 2, "limit": 3, "version": "1.10.0", "expected_sha256": digest}, {"view": "file", "file_path": "binary.bin"}, {"view": "manifest"}} {
		a := map[string]any{"id_or_path": "12345678", "asset_kind": "docs"}
		for key, v := range overrides {
			a[key] = v
		}
		if result := envelope("token", a); result["status"] != "success" {
			t.Fatal(result)
		}
	}
	for _, key := range []string{"offset", "limit"} {
		a := map[string]any{"id_or_path": "/public/a.md", "asset_kind": "docs", key: true}
		if v := envelope("token", a); v["error_class"] != "validation_error" {
			t.Fatal(v)
		}
	}
	result, err := call("token", map[string]any{"id_or_path": 4, "asset_kind": "docs"})
	if err != nil || result["error"] == nil {
		t.Fatal(result, err)
	}
	// Reader/shape failures precede the lock; valid reads wait rather than
	// observing a checkout replacement. No recursive outer Jobs lock is taken.
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
	if v := envelope("denied", args); v["error_class"] != "forbidden" {
		t.Fatal(v)
	}
	bad := map[string]any{"id_or_path": "/public/a.md", "asset_kind": "docs", "view": "bad"}
	if v := envelope("token", bad); v["error_class"] != "validation_error" {
		t.Fatal(v)
	}
	finished := make(chan error, 1)
	go func() {
		result, err := call("token", args)
		if err == nil {
			if result["error"] != nil {
				err = fmt.Errorf("%v", result)
			} else if content := result["result"].(map[string]any)["structuredContent"].(map[string]any); content["status"] != "success" || content["repo_revision"] != revision {
				err = fmt.Errorf("%v", content)
			}
		}
		finished <- err
	}()
	select {
	case err := <-finished:
		t.Fatal("read escaped writer lock", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
