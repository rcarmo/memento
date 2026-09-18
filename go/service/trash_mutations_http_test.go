package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rcarmo/memento/go/access"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

func TestRegisteredTrashLifecycleHTTP(t *testing.T) {
	j, base := jobsTest(t)
	revision := seedPruneAssets(t, j, base)
	ctx := context.Background()
	defer j.Workers.Drain(ctx)
	before, _ := mutationSnapshot(t, j.Controls.Queue.Paths.CurrentDir)
	server := umcp.NewServer("trash-lifecycle")
	server.SetNotificationOutput(nil)
	notifications := []string{}
	if err := j.RegisterTrashMutationTools(server, func(_ context.Context, method string, _ map[string]any, _ []string) error {
		notifications = append(notifications, method)
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
	rpc := func(method string, params map[string]any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		request := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(string(raw)))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer token")
		request.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		response := httptest.NewRecorder()
		transport.ServeHTTP(response, request)
		var result map[string]any
		if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 {
			t.Fatal(result, err)
		}
		return result
	}
	call := func(name, path, expected, key string, confirm any) map[string]any {
		t.Helper()
		args := map[string]any{"path": path, "expected_revision": expected, "idempotency_key": key}
		if name == "memory_purge" {
			args["confirm"] = confirm
		}
		result := rpc("tools/call", map[string]any{"name": name, "arguments": args})
		if result["error"] != nil {
			t.Fatal(result)
		}
		return result["result"].(map[string]any)["structuredContent"].(map[string]any)
	}
	if got := rpc("tools/list", nil); len(got["result"].(map[string]any)["tools"].([]any)) != 23 {
		t.Fatal(got)
	}
	if _, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/subscribe","params":{"uri":"memory://status"}}`), umcp.RequestContext{SessionID: "observer"}); err != nil {
		t.Fatal(err)
	}
	trashed := call("memory_trash", "/a.md", revision, "trash", nil)
	if trashed["status"] != "success" {
		t.Fatal(trashed)
	}
	after, _ := mutationSnapshot(t, j.Controls.Queue.Paths.CurrentDir)
	if after["/trash/a.md"] != before["/a.md"] || after["/a.md"] != "" {
		t.Fatal(after)
	}
	for path, content := range before {
		if strings.HasPrefix(path, "/.assets/") && after[path] != content {
			t.Fatal("asset moved", path)
		}
	}
	replay := call("memory_trash", "/a.md", revision, "trash", nil)
	if replay["status"] != "success" || replay["data"].(map[string]any)["replayed"] != true {
		t.Fatal(replay)
	}
	conflict := call("memory_trash", "/a.md", trashed["repo_revision"].(string), "trash", nil)
	if conflict["error_class"] != "idempotency_conflict" {
		t.Fatal(conflict)
	}
	restored := call("memory_restore", "/trash/a.md", trashed["repo_revision"].(string), "restore", nil)
	if restored["status"] != "success" {
		t.Fatal(restored)
	}
	after, _ = mutationSnapshot(t, j.Controls.Queue.Paths.CurrentDir)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("restore bytes changed")
	}
	revision = restored["repo_revision"].(string)
	again := call("memory_trash", "/a.md", revision, "trash-again", nil)
	if again["status"] != "success" {
		t.Fatal(again)
	}
	revision = again["repo_revision"].(string)
	for _, confirm := range []any{false, 1, nil, []any{true}} {
		v := call("memory_purge", "/trash/a.md", revision, "purge", confirm)
		if v["error_class"] != "validation_error" {
			t.Fatal(v)
		}
	}
	purged := call("memory_purge", "/trash/a.md", revision, "purge", "yes")
	if purged["status"] != "success" {
		t.Fatal(purged)
	}
	data := purged["data"].(map[string]any)
	if data["destination"] != nil || data["history_retained"] != true || len(data["changed_paths"].([]any)) != 7 {
		t.Fatal(purged)
	}
	if _, err = os.Stat(filepath.Join(j.Controls.Queue.Paths.CurrentDir, "trash/a.md")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	replay = call("memory_purge", "/trash/a.md", revision, "purge", true)
	if replay["status"] != "success" || replay["data"].(map[string]any)["replayed"] != true {
		t.Fatal(replay)
	}
	rejected := call("memory_purge", "/trash/a.md", revision, "purge", false)
	if rejected["error_class"] != "validation_error" {
		t.Fatal(rejected)
	}
	// Confirmation denial precedes _policy, while still requiring trusted
	// identity and worker admission. No namespace config is needed to deny it.
	j.Identity.authorization = access.AuthorizationConfig{}
	unconfirmed := call("memory_purge", "/trash/a.md", revision, "unconfirmed", false)
	if unconfirmed["error_class"] != "validation_error" {
		t.Fatal(unconfirmed)
	}
	confirmed := call("memory_purge", "/trash/a.md", revision, "confirmed", true)
	if confirmed["error_class"] != "forbidden" {
		t.Fatal(confirmed)
	}
	// The historical commit still contains the trashed concept and all assets.
	historic := filepath.Join(t.TempDir(), "historic")
	if _, err = repository.MaterializeCurrentCheckout(ctx, repository.GitRepositoryPaths{BareDir: j.Controls.Queue.Paths.BareDir, CurrentDir: historic}, revision); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(historic, "trash/a.md"))
	if err != nil || string(content) != before["/a.md"] {
		t.Fatal("history", err)
	}
	if len(notifications) != 12 {
		t.Fatal(notifications)
	}
	for i, method := range notifications {
		want := "notifications/resources/list_changed"
		if i%2 == 1 {
			want = "notifications/resources/updated"
		}
		if method != want {
			t.Fatal(notifications)
		}
	}
}
func TestPublicTrashConcurrentAndPostCommitFailure(t *testing.T) {
	c, actor, base := realApplyTest(t)
	ctx := context.Background()
	c.ChangedConcepts = func(context.Context, access.EffectivePolicy, []string) error { return io.ErrClosedPipe }
	if _, _, err := c.TrashMutation(ctx, actor, "trash", "/a.md", base, "key", nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	op, err := (control.Operations{DB: c.Queue.Proposals.DB}).ByIdempotency(ctx, actor.Policy.Principal, "key")
	if err != nil || op == nil || op.State != control.Succeeded {
		t.Fatal(op, err)
	}
	c.ChangedConcepts = nil
	var wg sync.WaitGroup
	out := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, _, err := c.TrashMutation(ctx, actor, "trash", "/a.md", base, "key", nil)
			if err == nil && data["replayed"] != true {
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
	denied := actor
	denied.Policy.WritePrefixes = nil
	if _, _, err = c.TrashMutation(ctx, denied, "trash", "/a.md", base, "key", nil); err == nil {
		t.Fatal("revoked replay")
	}
}

func TestTrashPurgeRollbackAndConcurrentPublication(t *testing.T) {
	j, base := jobsTest(t)
	revision := seedPruneAssets(t, j, base)
	ctx := context.Background()
	actor, err := actorContext(t, j.Identity, "actor", "")
	if err != nil {
		t.Fatal(err)
	}
	_, options, err := j.Controls.TrashMutation(ctx, actor, "trash", "/a.md", revision, "trash", nil)
	if err != nil {
		t.Fatal(err)
	}
	revision = *options.RepoRevision
	before, _ := mutationSnapshot(t, j.Controls.Queue.Paths.CurrentDir)
	manager := repository.TransactionManager{Paths: j.Controls.Queue.Paths, Operations: control.Operations{DB: j.Controls.Queue.Proposals.DB}, Checkpoint: func(name string) error {
		if name == "mutation_applied" {
			return fmt.Errorf("synthetic fault")
		}
		return nil
	}}
	err = repository.WithTransactionLock(ctx, j.Controls.Queue.Paths, func() error {
		_, _, err := j.Controls.trashMutation(ctx, actor, "purge", "/trash/a.md", revision, "failed-purge", true, defaultProposalRepository(), manager.ApplyUnderLock, defaultMutationIO())
		return err
	})
	if err == nil {
		t.Fatal("fault skipped")
	}
	after, _ := mutationSnapshot(t, j.Controls.Queue.Paths.CurrentDir)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed purge changed accepted files")
	}
	var wg sync.WaitGroup
	out := make(chan error, 8)
	first := make(chan bool, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, _, err := j.Controls.TrashMutation(ctx, actor, "purge", "/trash/a.md", revision, "purge", true)
			if err == nil {
				first <- data["replayed"] == false
			}
			out <- err
		}()
	}
	wg.Wait()
	close(out)
	close(first)
	for err := range out {
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for fresh := range first {
		if fresh {
			count++
		}
	}
	if count != 1 {
		t.Fatal("publications", count)
	}
}
