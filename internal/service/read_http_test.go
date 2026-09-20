package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

func TestRegisteredReadHTTP(t *testing.T) {
	j, base := jobsTest(t)
	ctx := context.Background()
	root := j.Controls.Queue.Paths.CurrentDir
	installMutationFiles(t, root, map[string]string{"/private/p.md": strings.Replace(mutationConcept, "'12345678'", "'private'", 1), "/trash/a.md": strings.Replace(mutationConcept, "'12345678'", "'trash'", 1)})
	index := &derived.Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	if err := index.Rebuild(ctx, root, base); err != nil {
		t.Fatal(err)
	}
	j.Controls.Index = index
	j.Identity.authorization.ProtectedReadPrefixes = []string{"/private/"}
	server := umcp.NewServer("memento-read-subset")
	server.SetNotificationOutput(nil)
	if err := j.RegisterReadProposalTools(server, nil); err != nil {
		t.Fatal(err)
	}
	transport, err := umcp.NewStreamableHTTP(server, umcp.DefaultHTTPOptions(), j.Identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	defer j.Workers.Drain(ctx)
	call := func(method string, args map[string]any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": method, "arguments": args}})
		req, err := http.NewRequest("POST", host.URL+"/mcp", strings.NewReader(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var parsed map[string]any
		if err = json.NewDecoder(response.Body).Decode(&parsed); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 || parsed["error"] != nil {
			t.Fatal(response.StatusCode, parsed)
		}
		return parsed["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	for _, id := range []string{"/a.md", "12345678"} {
		got := call("memory_read", map[string]any{"id_or_path": id})
		if got["status"] != "success" || got["data"].(map[string]any)["path"] != "/a.md" {
			t.Fatal(got)
		}
	}
	if got := call("memory_read", map[string]any{"id_or_path": "/private/p.md"}); got["error_class"] != "forbidden" {
		t.Fatal(got)
	}
	if got := call("memory_read", map[string]any{"id_or_path": "private"}); got["error_class"] != "not_found" {
		t.Fatal(got)
	}
	listed := call("memory_list", map[string]any{})["data"].(map[string]any)["entries"].([]any)
	if len(listed) != 1 {
		t.Fatal(listed)
	}
	trash := call("memory_list", map[string]any{"path_prefix": "/trash/"})["data"].(map[string]any)["entries"].([]any)
	if len(trash) != 1 {
		t.Fatal(trash)
	}
	// Source read methods do not use @_serialized. They must complete even while
	// another writer owns the mutation lock; admission remains a separate gate.
	locked, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- repository.WithTransactionLock(ctx, j.Controls.Queue.Paths, func() error { close(locked); <-release; return nil })
	}()
	<-locked
	once := false
	unblock := func() {
		if !once {
			close(release)
			once = true
		}
	}
	defer unblock()
	host.Client().Timeout = 2 * time.Second
	if got := call("memory_read", map[string]any{"id_or_path": "/a.md"}); got["status"] != "success" {
		t.Fatal(got)
	}
	unblock()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if err = index.SetRepoRevision(ctx, "advanced"); err != nil {
		t.Fatal(err)
	}
	search := call("memory_search", map[string]any{"query": "target", "search_mode": "hybrid", "limit": "1"})
	if search["repo_revision"] != "advanced" || search["index_revision"] != base || search["index_stale"] != false || len(search["warnings"].([]any)) != 1 {
		t.Fatal(search)
	}
	graph := call("memory_graph", map[string]any{"id_or_path": "/a.md", "depth": "2"})
	if graph["status"] != "success" || graph["repo_revision"] != "advanced" {
		t.Fatal(graph)
	}
	if err := os.WriteFile(filepath.Join(root, "private/bad.md"), []byte("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := call("memory_list", map[string]any{}); got["status"] != "success" {
		t.Fatal("hidden malformed content leaked", got)
	}
	// Non-model explicit fallback only; never claim semantic inference occurred.
	j.Controls.DefaultSearchMode = "semantic"
	if got := call("memory_search", map[string]any{"query": "target"}); got["data"].(map[string]any)["search_mode"] != "semantic" {
		t.Fatal(got)
	}
}
