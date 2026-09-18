package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

func TestRegisteredModelsOffStatusHTTP(t *testing.T) {
	ctx := context.Background()
	j, base := jobsTest(t)
	j.Controls.Metadata, _ = NewModelsOffMetadata("compact")
	index := &derived.Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	if err := index.Rebuild(ctx, j.Controls.Queue.Paths.CurrentDir, base); err != nil {
		t.Fatal(err)
	}
	j.Controls.Index = index
	server := umcp.NewServer("status-subset")
	server.SetNotificationOutput(nil)
	if err := j.RegisterStatusTools(server, nil); err != nil {
		t.Fatal(err)
	}
	opts := umcp.DefaultHTTPOptions()
	opts.AsyncReference = true
	transport, err := umcp.NewStreamableHTTP(server, opts, j.Identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	rpc := func(method string, params map[string]any) map[string]any {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		rec := httptest.NewRecorder()
		transport.ServeHTTP(rec, req)
		var value map[string]any
		if rec.Code != 200 {
			t.Errorf("HTTP %d: %s", rec.Code, rec.Body.String())
			return map[string]any{"error": rec.Code}
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
			t.Error(err)
		}
		return value
	}
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		if args == nil {
			args = map[string]any{}
		}
		v := rpc("tools/call", map[string]any{"name": name, "arguments": args})
		if v["error"] != nil {
			t.Fatal(v)
		}
		return v["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	if v := rpc("tools/list", nil); len(v["result"].(map[string]any)["tools"].([]any)) != 28 {
		t.Fatal(v)
	}
	resourceList := rpc("resources/list", nil)
	if len(resourceList["result"].(map[string]any)["resources"].([]any)) != 2 {
		t.Fatal(resourceList)
	}
	for _, name := range []string{"help", "status"} {
		v := rpc("resources/read", map[string]any{"uri": "memory://" + name})
		if v["error"] != nil {
			t.Fatal(v)
		}
	}
	j.Workers.SetExecuteBusy(true)
	status := call("memory_status", nil)
	if status["status"] != "success" || status["repo_revision"] != base || status["index_stale"] != false {
		t.Fatal(status)
	}
	data := status["data"].(map[string]any)
	if data["proposal_backlog"] != float64(1) || data["visible_concepts"] != float64(1) || data["readiness"].(map[string]any)["needle_router"].(map[string]any)["runtime"] != nil {
		t.Fatal(data)
	}
	help := call("memory_help", nil)
	if help["status"] != "success" || help["data"].(map[string]any)["mcp"].(map[string]any)["tool_surface"] != "compact" {
		t.Fatal(help)
	}
	busy := call("memory_read", map[string]any{"id_or_path": "/a.md"})
	if busy["error_class"] != "busy" {
		t.Fatal(busy)
	}
	// Help is unlocked, while status queues behind the mutation lock even though
	// both bypass worker admission. Fallback callbacks cannot hold the lock twice.
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
	help = call("memory_help", nil)
	if help["status"] != "success" {
		t.Fatal(help)
	}
	pending := make(chan map[string]any, 1)
	go func() { pending <- rpc("tools/call", map[string]any{"name": "memory_status"}) }()
	select {
	case result := <-pending:
		t.Fatal("status escaped lock", result)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	result := <-pending
	if result["error"] != nil {
		t.Fatal(result)
	}
	if err := index.SetRepoRevision(ctx, "advanced"); err != nil {
		t.Fatal(err)
	}
	// State.repo_revision alone does not make service status stale: it compares
	// index_revision with live Git main, independently of index repo_revision.
	status = call("memory_status", nil)
	if status["index_stale"] != false {
		t.Fatal(status)
	}
	j.Controls.Index = nil
	status = call("memory_status", nil)
	if status["error_class"] != "derived_index_unavailable" {
		t.Fatal(status)
	}
}
func TestStatusDirectFailures(t *testing.T) {
	ctx := context.Background()
	j, _ := jobsTest(t)
	server := umcp.NewServer("failure")
	server.SetNotificationOutput(nil)
	if err := j.RegisterStatusTools(server, nil); err != nil {
		t.Fatal(err)
	}
	process := func(name, principal string) *umcp.Response {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name}})
		r, err := server.Process(ctx, raw, umcp.RequestContext{Principal: principal, Transport: "streamable-http"})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if err := j.registerStatusTools(umcp.NewServer("invalid"), nil, []byte("{")); err == nil {
		t.Fatal("bad definitions")
	}
	resource, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"memory://help"}}`), umcp.RequestContext{Principal: "missing", Transport: "streamable-http"})
	if err != nil || resource.Error == nil {
		t.Fatal(resource, err)
	}
	if r := process("memory_help", "missing"); r.Error == nil {
		t.Fatal(r)
	}
	if r := process("memory_status", "missing"); r.Error == nil {
		t.Fatal(r)
	}
	if r := process("memory_help", "actor"); r.Error == nil {
		t.Fatal(r)
	}
	j.Controls.Metadata, _ = NewModelsOffMetadata("compact")
	// Help policy failures are outside the Python exception catch; status maps
	// the same live-policy denial into a service envelope.
	j.Identity.authorization.Principals = nil
	if r := process("memory_help", "actor"); r.Error == nil {
		t.Fatal(r)
	}
	r := process("memory_status", "actor")
	if r.Error != nil || jsonNormal(r.Result).(map[string]any)["structuredContent"].(map[string]any)["error_class"] != "forbidden" {
		t.Fatal(r)
	}
	// Restore policy for the unexpected index failure/redaction assertion.
	j.Identity.authorization = access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"actor": {Roles: []string{"curator"}, ReadPrefixes: []string{"/"}}}}
	j.Controls.Index = failingStatusIndex{err: io.ErrClosedPipe}
	if r := process("memory_status", "actor"); r.Error == nil {
		t.Fatal(r)
	}
}
func TestStatusConcurrentSnapshots(t *testing.T) {
	j, base := jobsTest(t)
	j.Controls.Metadata, _ = NewModelsOffMetadata("compact")
	index := &derived.Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	ctx := context.Background()
	if err := index.Rebuild(ctx, j.Controls.Queue.Paths.CurrentDir, base); err != nil {
		t.Fatal(err)
	}
	j.Controls.Index = index
	actor, err := actorContext(t, j.Identity, "actor", "")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, _, err := j.Controls.Status(ctx, actor)
			if err == nil && data["proposal_backlog"] != 1 {
				err = io.ErrUnexpectedEOF
			}
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
}
