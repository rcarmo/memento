package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/repository"
)

func submitZIP(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, err := z.Create("document.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("public fixture")); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func submitAsset(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": "docs", "version": "1.0.0", "zip_base64": base64.StdEncoding.EncodeToString(submitZIP(t))}
}
func TestPrepareInputAndIOFailures(t *testing.T) {
	ctx := context.Background()
	c, actor := rebaseTest(t)
	for _, raw := range []any{nil, map[string]any{"kind": "rename", "path": 1}, map[string]any{"kind": "attach_asset_pack", "path": 1}, map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": 1}, map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": "../bad"}, map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": "docs", "version": 1}} {
		if _, err := c.prepareAssets(ctx, []any{raw}, actor.Policy.Principal, fakeRepo(nil)); err == nil {
			t.Fatal(raw)
		}
	}
	for _, stage := range []string{"staging-db", "asset-query", "existing-row", "existing-refresh", "asset-id"} {
		t.Run(stage, func(t *testing.T) {
			c, actor := rebaseTest(t)
			root := t.TempDir()
			installMutationFiles(t, root, map[string]string{"/a.md": mutationConcept})
			c.Queue.Paths.CurrentDir = root
			repo := fakeRepo(nil)
			repo.read = repository.ReadBundleEntry
			change := submitAsset(t)
			switch stage {
			case "staging-db":
				c.Staging = &assets.StagingStore{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}
				delete(change, "zip_base64")
				change["staged_asset_id"] = "id"
				c.Queue.Proposals.DB.Close()
			case "asset-query":
				if _, err := c.Queue.Proposals.DB.Exec("DROP TABLE proposal_assets"); err != nil {
					t.Fatal(err)
				}
			case "existing-row", "existing-refresh":
				if _, err := c.Queue.Proposals.DB.Exec(`INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES('proposal','asset','/a.md','docs','1.0.0','application/zip','digest',X'00','{}','now')`); err != nil {
					t.Fatal(err)
				}
				if stage == "existing-row" {
					if _, err := c.Queue.Proposals.DB.Exec("UPDATE proposals SET status='invalid'"); err != nil {
						t.Fatal(err)
					}
				} else {
					repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
				}
			case "asset-id":
				c.Random = bytes.NewReader(nil)
			}
			if _, err := c.prepareAssets(ctx, []any{change}, actor.Policy.Principal, repo); err == nil {
				t.Fatal("missing failure")
			}
		})
	}
	c.Random = bytes.NewReader(nil)
	if _, err := c.propose(ctx, actor, "intent", "main", nil, nil, fakeRepo(nil)); err == nil {
		t.Fatal("proposal id failure")
	}
}
func TestSubmitPublicLifecycleAndConsumption(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer", "curator"}
	if _, err := c.Queue.Proposals.DB.Exec("DELETE FROM proposals"); err != nil {
		t.Fatal(err)
	}
	c.Staging = &assets.StagingStore{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}
	archive := submitZIP(t)
	upload, _, err := c.Staging.Put(ctx, actor.Policy.Principal, "upload", "docs", "1.0.0", archive)
	if err != nil {
		t.Fatal(err)
	}
	raw := map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": "docs", "version": "1.0.0", "staged_asset_id": upload.StagedAssetID}
	var wg sync.WaitGroup
	results := make(chan map[string]any, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.Propose(ctx, actor, "attach docs", base, []any{raw}, nil)
			if err != nil {
				errs <- err
				return
			}
			results <- got
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	if len(results) != 1 || len(errs) != 1 {
		t.Fatal("consumption not single-winner", len(results), len(errs))
	}
	got := <-results
	proposal := got["proposal"].(map[string]any)
	id := proposal["proposal_id"].(string)
	staged, err := c.Staging.Get(ctx, actor.Policy.Principal, upload.StagedAssetID, false)
	if err != nil || staged.State != "consumed" || len(staged.BlobBytes) != 0 || staged.ProposalID == nil || *staged.ProposalID != id {
		t.Fatal(staged, err)
	}
	if _, err = c.Review(ctx, actor, id, "approve", nil, "review"); err != nil {
		t.Fatal(err)
	}
	applied, err := c.Apply(ctx, actor, id, base, "apply")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(c.Queue.Paths.CurrentDir, ".assets/12345678/docs/1.0.0.zip"))
	if err != nil || !bytes.Equal(stored, archive) {
		t.Fatal(err)
	}
	if _, err = c.Propose(ctx, actor, "new version", applied.Revision, []any{raw}, nil); err == nil {
		t.Fatal("consumed upload reused")
	}
	// Unlike durable review/apply keys, repeated plain submissions produce IDs.
	a, err := c.Propose(ctx, actor, "empty", applied.Revision, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.Propose(ctx, actor, "empty", applied.Revision, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(a, b) {
		t.Fatal("submission unexpectedly replayed")
	}
}
func TestSubmitArchivalAndPostCommitFailure(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer", "curator"}
	raw := []any{map[string]any{"kind": "trash", "path": "/a.md"}}
	if _, err := c.Propose(ctx, actor, "archive", base, raw, nil); err == nil {
		t.Fatal("missing index accepted")
	}
	root := t.TempDir()
	index := archiveDB(t, root)
	if _, err := index.Exec("UPDATE index_state SET value=? WHERE key='index_revision'", base); err != nil {
		t.Fatal(err)
	}
	c.DerivedIndexPath = filepath.Join(root, "index.sqlite")
	result, err := c.Propose(ctx, actor, "archive", base, raw, nil)
	if err != nil || len(result["proposal"].(map[string]any)["archival_impact"].([]any)) != 1 {
		t.Fatal(result, err)
	}
	// At the final post-commit revision check, surface the failure while retaining
	// the submitted record. No HTTP failure may silently imply rollback here.
	q, _ := queueTest(t)
	q.Paths.CurrentDir = c.Queue.Paths.CurrentDir
	c.Queue = q
	c.Random = nil
	c.Staging = nil
	repo := fakeRepo(nil)
	repo.read = repository.ReadBundleEntry
	calls := 0
	repo.main = func(repository.GitRepositoryPaths) (string, error) {
		calls++
		if calls == 6 {
			return "", io.ErrClosedPipe
		}
		return base, nil
	}
	before := len(tableRows(t, q.Proposals.DB, "SELECT * FROM proposals"))
	if _, err = c.propose(ctx, actor, "archive", base, raw, nil, repo); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err, calls)
	}
	if len(tableRows(t, q.Proposals.DB, "SELECT * FROM proposals")) != before+1 {
		t.Fatal("postcommit failure lost proposal")
	}
}
