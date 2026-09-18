package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func TestReviewFailuresAndRollback(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"missing", "key-random", "refresh", "revision", "conflicts", "read-updated", "list-assets", "summary"} {
		t.Run(stage, func(t *testing.T) {
			c, actor := rebaseTest(t)
			actor.Policy.Roles = []string{"curator"}
			id := "proposal"
			key := "key"
			decision := "reject"
			repo := fakeRepo(nil)
			calls := 0
			repo.main = func(repository.GitRepositoryPaths) (string, error) {
				calls++
				if stage == "refresh" || (stage == "revision" && calls == 2) || (stage == "conflicts" && calls == 3) || (stage == "summary" && calls == 3) {
					return "", io.ErrClosedPipe
				}
				return "main", nil
			}
			trigger := ""
			switch stage {
			case "missing":
				id = "missing"
			case "key-random":
				key = ""
				c.Random = bytes.NewReader(nil)
			case "conflicts":
				decision = "approve"
			case "read-updated":
				trigger = `CREATE TRIGGER broken AFTER UPDATE ON proposals WHEN NEW.reviewed_by IS NOT NULL BEGIN UPDATE proposals SET status='invalid' WHERE proposal_id=NEW.proposal_id; END`
			}
			if stage == "list-assets" {
				// A malformed nullable legacy table forces asset row decoding to
				// fail after the successful review update, which must roll back.
				repo.main = func(repository.GitRepositoryPaths) (string, error) {
					calls++
					if calls == 2 {
						if _, err := c.Queue.Proposals.DB.Exec(`DROP TABLE proposal_assets; CREATE TABLE proposal_assets(proposal_id TEXT,asset_id TEXT,concept_path TEXT,asset_kind TEXT,version TEXT,media_type TEXT,sha256 TEXT,blob_bytes BLOB,manifest_json TEXT,created_at TEXT); INSERT INTO proposal_assets(proposal_id,asset_id) VALUES('proposal','asset')`); err != nil {
							t.Fatal(err)
						}
					}
					return "main", nil
				}
			}
			if trigger != "" {
				if _, err := c.Queue.Proposals.DB.Exec(trigger); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := c.review(ctx, actor, id, decision, nil, key, repo); err == nil {
				t.Fatal("missing failure", stage)
			}
			if rows := tableRows(t, c.Queue.Proposals.DB, "SELECT * FROM operations"); len(rows) != 0 {
				t.Fatal(rows)
			}
			if stage != "missing" {
				r, err := c.Queue.Proposals.Get(ctx, "proposal")
				if err != nil || r.ReviewedBy != nil {
					t.Fatal("partial review", r, err)
				}
			}
		})
	}
}
func TestReviewReplayAndApprovalErrors(t *testing.T) {
	ctx := context.Background()
	c, actor := rebaseTest(t)
	actor.Policy.Roles = []string{"curator"}
	raw, _, err := c.controlReplay(ctx, actor.Policy, "key", "memory_proposal_review", map[string]any{"proposal_id": "proposal", "decision": "approve", "comment": nil})
	if err != nil {
		t.Fatal(err)
	}
	op, err := (control.Operations{DB: c.Queue.Proposals.DB}).Create(ctx, control.OperationRequest{OpID: "existing", Principal: actor.Policy.Principal, IdempotencyKey: "key", RequestJSON: raw})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{nil, "{"} {
		if _, err = c.Queue.Proposals.DB.Exec("UPDATE operations SET result_json=?", value); err != nil {
			t.Fatal(err)
		}
		got, err := c.review(ctx, actor, "proposal", "approve", nil, "key", proposalRepository{})
		if value == "{" {
			if err == nil {
				t.Fatal("bad replay")
			}
		} else if err != nil || got.OperationID != op.OpID || got.Data["replayed"] != true {
			t.Fatal(got, err)
		}
	}
	for _, raw := range []string{"{", `{"archival_impact":true}`, `{"archival_impact":[1],"changes":[{"kind":"unknown"}]}`} {
		if err := c.approvalArchival(ctx, actor.Policy, control.ProposalRecord{PatchJSON: raw}, fakeRepo(nil)); err == nil {
			t.Fatal(raw)
		}
	}
	for _, test := range []struct {
		value any
		want  bool
	}{{nil, false}, {false, false}, {true, true}, {"", false}, {"x", true}, {[]any{}, false}, {[]any{1}, true}, {map[string]any{}, false}, {map[string]any{"x": 1}, true}, {json.Number("-0.0"), false}, {json.Number("2"), true}, {1, true}} {
		if jsonTruthy(test.value) != test.want {
			t.Fatal(test)
		}
	}
}
func TestReviewPublicCommitRetryAndConcurrency(t *testing.T) {
	ctx := context.Background()
	c, actor := rebaseTest(t)
	actor.Policy.Roles = []string{"curator"}
	c.Random = nil
	root := t.TempDir()
	q := &c.Queue
	q.Paths = repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}
	boot, err := repository.BootstrapRepository(ctx, q.Paths, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.Proposals.DB.Exec("UPDATE proposals SET base_revision=?", boot.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = q.Proposals.DB.Exec(`CREATE TABLE parent(id INTEGER PRIMARY KEY); CREATE TABLE child(id INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER failed_commit AFTER INSERT ON operations BEGIN INSERT INTO child VALUES(1); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Review(ctx, actor, "proposal", "approve", nil, "key"); err == nil {
		t.Fatal("COMMIT failure")
	}
	r, err := q.Proposals.Get(ctx, "proposal")
	if err != nil || r.Status != control.Submitted || r.ReviewedBy != nil {
		t.Fatal(r, err)
	}
	if rows := tableRows(t, q.Proposals.DB, "SELECT * FROM proposal_events"); len(rows) != 0 {
		t.Fatal(rows)
	}
	if _, err = q.Proposals.DB.Exec("DROP TRIGGER failed_commit"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan ProposalControlResult, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := c.Review(ctx, actor, "proposal", "approve", nil, "key")
			if err != nil {
				t.Error(err)
				return
			}
			results <- got
		}()
	}
	wg.Wait()
	close(results)
	fresh := 0
	id := ""
	for got := range results {
		if got.Data["replayed"] == false {
			fresh++
		}
		if id == "" {
			id = got.OperationID
		}
		if got.OperationID != id {
			t.Fatal("duplicate operation")
		}
	}
	if fresh != 1 {
		t.Fatal(fresh)
	}
	if rows := tableRows(t, q.Proposals.DB, "SELECT * FROM proposal_events"); len(rows) != 1 {
		t.Fatal(rows)
	}
}

func TestReviewArchivalSuccessAndRestartReplay(t *testing.T) {
	ctx := context.Background()
	c, actor := rebaseTest(t)
	actor.Policy.Roles = []string{"curator"}
	c.Random = nil
	root := t.TempDir()
	index := archiveDB(t, root)
	if _, err := index.Exec(`INSERT INTO concepts VALUES('source','/public/source.md');INSERT INTO links VALUES('source','/public/a.md','target','resolved')`); err != nil {
		t.Fatal(err)
	}
	c.Queue.Paths.CurrentDir = root
	c.DerivedIndexPath = filepath.Join(root, "index.sqlite")
	if _, err := c.Queue.Proposals.DB.Exec(`UPDATE proposals SET base_revision='main',patch_json='{"changes":[{"kind":"trash","path":"/public/a.md"}],"archival_impact":[{"path":"/public/a.md"}]}'`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Queue.Proposals.DB.Exec(`INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES('proposal','asset','/public/a.md','docs','1.0.0','application/zip','digest',X'00FF','{"entries":[]}', 'created')`); err != nil {
		t.Fatal(err)
	}
	comment := "safe to archive"
	result, err := c.review(ctx, actor, "proposal", "approve", &comment, "key", archiveRepo())
	if err != nil {
		t.Fatal(err)
	}
	summary := result.Data["proposal"].(map[string]any)
	if summary["asset_count"] != 1 || summary["status"] != "approved" {
		t.Fatal(summary)
	}
	assets, err := c.Queue.Proposals.ListAssets(ctx, control.ProposalAssetQuery{})
	if err != nil || len(assets) != 1 || !bytes.Equal(assets[0].BlobBytes, []byte{0, 255}) {
		t.Fatal(assets, err)
	}
	var databasePath string
	for _, row := range tableRows(t, c.Queue.Proposals.DB, "PRAGMA database_list") {
		if row["name"] == "main" {
			databasePath = row["file"].(string)
		}
	}
	c.Queue.Proposals.DB.Close()
	reopened, err := control.Connect(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	c.Queue.Proposals.DB = reopened
	// Reopened replay still checks the trusted current policy, but does not read
	// Git, derived state or current content after the permission checks succeed.
	got, err := c.review(ctx, actor, "proposal", "approve", &comment, "key", proposalRepository{})
	if err != nil || got.OperationID != result.OperationID || got.Data["replayed"] != true {
		t.Fatal(got, err)
	}
	actor.Policy.WritePrefixes = nil
	if _, err = c.review(ctx, actor, "proposal", "approve", &comment, "key", proposalRepository{}); err == nil {
		t.Fatal("replay bypassed revoked write grant")
	}
}
