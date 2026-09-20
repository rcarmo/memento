package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

func TestRegisteredAuditHTTP(t *testing.T) {
	ctx := context.Background()
	j, revision := jobsTest(t)
	j.Controls.Metadata, _ = NewModelsOffMetadata("compact")
	server := umcp.NewServer("audit")
	server.SetNotificationOutput(nil)
	if err := j.RegisterAuditTools(server, nil); err != nil {
		t.Fatal(err)
	}
	var scopes []access.EffectivePolicy
	j.Controls.AuditGraph = auditProvider(func(_ context.Context, policy access.EffectivePolicy) (AuditOverview, error) {
		scopes = append(scopes, policy)
		return AuditOverview{IndexRevision: revision, Nodes: []AuditGraphNode{{"a", "/a.md"}}, Diagnostics: []AuditDiagnostic{{ID: "first", Severity: "error", Rule: "broken", ConceptIDs: []string{"a"}, Message: "one"}, {ID: "second", Severity: "warning", Rule: "embedding_health", ConceptIDs: []string{"a"}, Message: "two"}}}, nil
	})
	opts := umcp.DefaultHTTPOptions()
	opts.AsyncReference = true
	transport, err := umcp.NewStreamableHTTP(server, opts, j.Identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	defer j.Workers.Drain(ctx)
	rpc := func(method string, params map[string]any) map[string]any {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		request := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(string(raw)))
		request.Header.Set("Authorization", "Bearer token")
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response := httptest.NewRecorder()
		transport.ServeHTTP(response, request)
		var v map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &v); err != nil {
			t.Error(err)
		}
		return v
	}
	call := func(args map[string]any) map[string]any {
		t.Helper()
		v := rpc("tools/call", map[string]any{"name": "memory_audit", "arguments": args})
		if v["error"] != nil {
			t.Fatal(v)
		}
		return v["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	if v := rpc("tools/list", nil); len(v["result"].(map[string]any)["tools"].([]any)) != 29 {
		t.Fatal(v)
	}
	if v := rpc("resources/list", nil); len(v["result"].(map[string]any)["resources"].([]any)) != 2 {
		t.Fatal(v)
	}
	if v := rpc("resources/read", map[string]any{"uri": "memory://help"}); v["error"] != nil {
		t.Fatal(v)
	}
	// Audit is a worker, but not writer-serialized. Revision/graph consistency
	// is checked after repository inspection, not held under a write lock.
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
	pending := make(chan map[string]any, 1)
	go func() { pending <- call(map[string]any{"limit": 1}) }()
	var first map[string]any
	select {
	case first = <-pending:
	case <-time.After(3 * time.Second):
		t.Fatal("audit blocked by writer")
	}
	if first["status"] != "success" || first["repo_revision"] != revision {
		t.Fatal(first)
	}
	data := first["data"].(map[string]any)
	graph := data["graph_diagnostics"].(map[string]any)
	cursor := graph["next_cursor"].(string)
	second := call(map[string]any{"limit": 1, "cursor": cursor})
	g := second["data"].(map[string]any)["graph_diagnostics"].(map[string]any)
	if g["next_cursor"] != nil || g["diagnostics"].([]any)[0].(map[string]any)["id"] != "second" {
		t.Fatal(second)
	}
	changed := call(map[string]any{"limit": 1, "cursor": cursor, "rule": "broken"})
	if changed["error_class"] != "validation_error" {
		t.Fatal(changed)
	}
	if len(scopes) < 3 || len(scopes[0].WritePrefixes) == 0 {
		t.Fatal(scopes)
	}
	j.Workers.SetExecuteBusy(true)
	busy := call(map[string]any{})
	if busy["error_class"] != "busy" {
		t.Fatal(busy)
	}
	j.Workers.SetExecuteBusy(false)
	// No configured graph service returns a real unavailable payload and ignores
	// cursor errors, matching the source's early return.
	j.Controls.AuditGraph = nil
	v := call(map[string]any{"cursor": "bad"})
	if v["status"] != "success" || v["data"].(map[string]any)["graph_diagnostics"].(map[string]any)["reason"] != "not_configured" {
		t.Fatal(v)
	}
	v = call(map[string]any{"limit": 0})
	if v["error_class"] != "validation_error" {
		t.Fatal(v)
	}
	raw := rpc("tools/call", map[string]any{"name": "memory_audit", "arguments": map[string]any{"path": 1}})
	if raw["error"] == nil {
		t.Fatal(raw)
	}
	help := rpc("tools/call", map[string]any{"name": "memory_help"})
	if help["error"] != nil {
		t.Fatal(help)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestAuditConcurrentScopes(t *testing.T) {
	c, actor, _ := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer"}
	c.AuditGraph = auditProvider(func(_ context.Context, policy access.EffectivePolicy) (AuditOverview, error) {
		for _, path := range policy.ReadPrefixes {
			if path != "/" {
				return AuditOverview{}, io.ErrUnexpectedEOF
			}
		}
		revision, _ := repository.GetMainRevision(c.Queue.Paths)
		return AuditOverview{IndexRevision: revision}, nil
	})
	var wg sync.WaitGroup
	out := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, _, err := c.Audit(context.Background(), actor, AuditOptions{Limit: 1})
			// realApplyTest's /a.md links to absent /public/a.md. The
			// repository issue must survive an otherwise empty graph snapshot.
			if err == nil {
				issues := data["issues"].([]repository.AuditIssue)
				if data["ok"] != false || len(issues) != 1 || issues[0].Code != "broken_link" {
					err = io.ErrUnexpectedEOF
				}
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
}

// Cursor bytes intentionally contain no principal. On every continuation the
// provider still receives the current writable scope; the cursor grants nothing.
func TestAuditCursorFreshScope(t *testing.T) {
	c, actor, revision := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer"}
	c.AuditGraph = auditProvider(func(_ context.Context, policy access.EffectivePolicy) (AuditOverview, error) {
		overview := AuditOverview{IndexRevision: revision}
		for _, node := range []AuditGraphNode{{"a", "/public/a.md"}, {"b", "/private/b.md"}} {
			if readable(policy, node.Path) {
				overview.Nodes = append(overview.Nodes, node)
				overview.Diagnostics = append(overview.Diagnostics, AuditDiagnostic{ID: node.ID, Rule: "orphans", Severity: "warning", ConceptIDs: []string{node.ID}})
			}
		}
		return overview, nil
	})
	first, _, err := c.Audit(context.Background(), actor, AuditOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	graph := first["graph_diagnostics"].(map[string]any)
	cursor := graph["next_cursor"].(string)
	actor.Policy.ReadPrefixes = []string{"/public/"}
	actor.Policy.WritePrefixes = []string{"/public/"}
	after, _, err := c.Audit(context.Background(), actor, AuditOptions{Limit: 1, Cursor: &cursor})
	if err != nil {
		t.Fatal(err)
	}
	if got := after["graph_diagnostics"].(map[string]any)["diagnostics"].([]any); len(got) != 0 {
		t.Fatal("cursor leaked revoked scope", got)
	}
}
