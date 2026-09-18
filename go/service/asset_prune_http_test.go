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

	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

// Publish fixture versions through the real transaction layer, never hand-edit
// the materialised checkout as a substitute for accepted Git state.
func seedPruneAssets(t *testing.T, j *Jobs, base string) string {
	t.Helper()
	blob := submitZIP(t)
	pack, err := assets.ValidateAssetPack("docs", "1.0.0", blob, "", "")
	if err != nil {
		t.Fatal(err)
	}
	manager := repository.TransactionManager{Paths: j.Controls.Queue.Paths, Operations: control.Operations{DB: j.Controls.Queue.Proposals.DB, Now: j.Controls.Queue.Now}, Now: j.Controls.Queue.Now}
	result, err := manager.Apply(context.Background(), repository.TransactionRequest{Operation: control.OperationRequest{OpID: "seed-assets", Principal: "actor", IdempotencyKey: "seed", ToolName: "seed", RequestJSON: "{}"}, ExpectedRevision: base, CommitMessage: "seed synthetic assets", AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}, func(_ context.Context, root string) ([]string, error) {
		changed := []string{}
		for _, version := range []string{"1.0.0", "1.9.0", "1.10.0"} {
			paths, err := assets.WriteAssetVersion(root, assets.AcceptedVersion{ConceptID: "12345678", ConceptPath: "/a.md", AssetKind: "docs", Version: version, ZIPBytes: blob, Manifest: pack.Manifest, AcceptedBy: "actor", SourceProposalID: "proposal"})
			if err != nil {
				return nil, err
			}
			changed = append(changed, paths...)
		}
		return changed, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result.ResultRevision
}
func TestRegisteredAssetPruneHTTP(t *testing.T) {
	j, base := jobsTest(t)
	revision := seedPruneAssets(t, j, base)
	ctx := context.Background()
	defer j.Workers.Drain(ctx)
	server := umcp.NewServer("asset-prune")
	server.SetNotificationOutput(nil)
	notifications := []string{}
	if err := j.RegisterAssetReadProposalTools(server, func(_ context.Context, method string, _ map[string]any, _ []string) error {
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
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		var result map[string]any
		if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil || result["error"] != nil {
			t.Fatal(result, err)
		}
		return result["result"].(map[string]any)
	}
	// A source prune success only notifies status, even when files were removed.
	if _, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"resources/subscribe","params":{"uri":"memory://status"}}`), umcp.RequestContext{SessionID: "observer"}); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"id_or_path": "/a.md", "asset_kind": "docs", "keep": true, "expected_revision": revision, "idempotency_key": "prune"}
	call := func() map[string]any {
		return rpc("tools/call", map[string]any{"name": "memory_asset_prune", "arguments": args})["structuredContent"].(map[string]any)
	}
	listed := rpc("tools/list", nil)
	if len(listed["tools"].([]any)) != 17 {
		t.Fatal(listed)
	}
	result := call()
	if result["status"] != "success" || result["operation_id"] == nil || result["repo_revision"] == revision {
		t.Fatal(result)
	}
	published := result["repo_revision"]
	data := result["data"].(map[string]any)
	if fmt.Sprint(data["pruned_versions"]) != "[1.9.0 1.0.0]" || data["changed_paths"] != nil || data["replayed"] != false {
		t.Fatal(data)
	}
	for _, version := range []string{"1.0.0", "1.9.0"} {
		for _, extension := range []string{".json", ".zip"} {
			if _, err := os.Stat(filepath.Join(j.Controls.Queue.Paths.CurrentDir, ".assets/12345678/docs/"+version+extension)); !os.IsNotExist(err) {
				t.Fatal(version, err)
			}
		}
	}
	// After pruning the same key/old revision takes the no-op path, not replay.
	repeated := call()
	if repeated["status"] != "success" || repeated["operation_id"] != nil || repeated["repo_revision"] != published || repeated["data"].(map[string]any)["replayed"] != nil {
		t.Fatal(repeated)
	}
	args["keep"] = 0
	if v := call(); v["error_class"] != "validation_error" {
		t.Fatal(v)
	}
	if fmt.Sprint(notifications) != "[notifications/resources/updated notifications/resources/updated]" {
		t.Fatal(notifications)
	}
	op, err := (control.Operations{DB: j.Controls.Queue.Proposals.DB}).ByIdempotency(ctx, "actor", "prune")
	if err != nil || op == nil || op.State != control.Succeeded {
		t.Fatal(op, err)
	}
	expectedRequest := fmt.Sprintf(`{"asset_kind": "docs", "concept_path": "/a.md", "expected_revision": %q, "keep": true}`, revision)
	if op.RequestHash != (control.OperationRequest{RequestJSON: expectedRequest}).RequestHash() {
		t.Fatal("boolean retention hash", op)
	}
}
func TestPublicAssetPruneConcurrentNoopAndRollback(t *testing.T) {
	j, base := jobsTest(t)
	revision := seedPruneAssets(t, j, base)
	actor, err := actorContext(t, j.Identity, "actor", "")
	if err != nil {
		t.Fatal(err)
	}
	// A post-mutation/pre-publication failure must leave accepted files intact.
	failed := func(ctx context.Context, r repository.TransactionRequest, mutate repository.MutationCallback) (repository.TransactionResult, error) {
		manager := repository.TransactionManager{Paths: j.Controls.Queue.Paths, Operations: control.Operations{DB: j.Controls.Queue.Proposals.DB}, Checkpoint: func(name string) error {
			if name == "mutation_applied" {
				return fmt.Errorf("synthetic fault")
			}
			return nil
		}}
		return manager.ApplyUnderLock(ctx, r, mutate)
	}
	err = repository.WithTransactionLock(context.Background(), j.Controls.Queue.Paths, func() error {
		_, _, err := j.Controls.assetPrune(context.Background(), actor, "/a.md", "docs", 1, revision, "failed", failed, defaultMutationIO())
		return err
	})
	if err == nil {
		t.Fatal("checkpoint did not fail")
	}
	versions, err := assets.ListAssetVersions(j.Controls.Queue.Paths.CurrentDir, "12345678", "docs")
	if err != nil || len(versions) != 3 {
		t.Fatal(versions, err)
	}
	var wg sync.WaitGroup
	results := make(chan SuccessOptions, 8)
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, options, err := j.Controls.AssetPrune(context.Background(), actor, "/a.md", "docs", 1, revision, "shared")
			results <- options
			failures <- err
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	count := 0
	for options := range results {
		if options.OperationID != nil {
			count++
		}
	}
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Fatal("mutating operations", count)
	}
}
