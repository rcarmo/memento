package service

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func rebaseTest(t *testing.T) (*ProposalControls, ProposalActor) {
	t.Helper()
	q, r := queueTest(t)
	return &ProposalControls{Queue: q, Random: bytes.NewReader(make([]byte, 64))}, ProposalActor{Policy: access.EffectivePolicy{Principal: r.AuthorPrincipal, Roles: []string{"proposer"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}}
}

func TestRebaseFailures(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"missing", "refresh-main", "second-main", "conflict-main", "conflict-diff", "refresh-write", "normalization", "write-permission", "random"} {
		t.Run(stage, func(t *testing.T) {
			c, actor := rebaseTest(t)
			q := c.Queue
			repo := fakeRepo(nil)
			id := "proposal"
			calls := 0
			repo.main = func(repository.GitRepositoryPaths) (string, error) {
				calls++
				if stage == "refresh-main" || (stage == "second-main" && calls == 2) || (stage == "conflict-main" && calls == 3) {
					return "", io.ErrClosedPipe
				}
				return "main", nil
			}
			if stage == "conflict-diff" {
				diffs := 0
				repo.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
					diffs++
					if diffs == 2 {
						return nil, context.Canceled
					}
					return nil, nil
				}
			}
			if stage == "missing" {
				id = "missing"
			}
			if stage == "random" {
				c.Random = bytes.NewReader(nil)
			}
			trigger := ""
			switch stage {
			case "refresh-write":
				trigger = `CREATE TRIGGER broken BEFORE UPDATE ON proposals BEGIN SELECT RAISE(ABORT,'blocked'); END`
			case "normalization":
				trigger = `CREATE TRIGGER broken AFTER UPDATE ON proposals WHEN NEW.status='needs_rebase' BEGIN UPDATE proposals SET patch_json='{"changes":[{"kind":"patch","path":"/a.md","title":1}]}' WHERE proposal_id=NEW.proposal_id; END`
			case "write-permission":
				actor.Policy.ReadPrefixes = []string{"/allowed/"}
				actor.Policy.WritePrefixes = []string{"/allowed/"}
				if _, err := q.Proposals.DB.Exec(`UPDATE proposals SET patch_json='{"changes":[{"kind":"patch","path":"/allowed/a.md"}]}'`); err != nil {
					t.Fatal(err)
				}
				trigger = `CREATE TRIGGER broken AFTER UPDATE ON proposals WHEN NEW.status='needs_rebase' BEGIN UPDATE proposals SET patch_json='{"changes":[{"kind":"patch","path":"/forbidden/a.md"}]}' WHERE proposal_id=NEW.proposal_id; END`
			}
			if trigger != "" {
				if _, err := q.Proposals.DB.Exec(trigger); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := c.rebase(ctx, actor, id, "main", "key", repo); err == nil {
				t.Fatal("missing failure", stage)
			}
			if rows := tableRows(t, q.Proposals.DB, "SELECT * FROM operations"); len(rows) != 0 {
				t.Fatal(rows)
			}
		})
	}
}

func TestRebaseReplayBoundaries(t *testing.T) {
	ctx := context.Background()
	c, actor := rebaseTest(t)
	raw, _, err := c.controlReplay(ctx, actor.Policy, "key", "memory_proposal_rebase", map[string]any{"proposal_id": "proposal", "expected_revision": "main"})
	if err != nil {
		t.Fatal(err)
	}
	operations := control.Operations{DB: c.Queue.Proposals.DB}
	op, err := operations.Create(ctx, control.OperationRequest{OpID: "existing", Principal: actor.Policy.Principal, IdempotencyKey: "key", RequestJSON: raw, ToolName: "memory_proposal_rebase"})
	if err != nil {
		t.Fatal(err)
	}
	// The source replays any existing state; it does not re-check succeeded here.
	for _, value := range []any{nil, "[]", "{", `{"old":true}`} {
		if _, err = c.Queue.Proposals.DB.Exec("UPDATE operations SET result_json=? WHERE op_id=?", value, op.OpID); err != nil {
			t.Fatal(err)
		}
		got, err := c.rebase(ctx, actor, "proposal", "main", "key", proposalRepository{})
		if value == "{" {
			if err == nil {
				t.Fatal("bad JSON")
			}
			continue
		}
		if err != nil || got.OperationID != op.OpID || got.Data["replayed"] != true {
			t.Fatal(got, err)
		}
	}
	if _, _, err = c.controlReplay(ctx, actor.Policy, "key", "method", map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("bad request JSON")
	}
	c.Queue.Proposals.DB.Close()
	if _, _, err = c.controlReplay(ctx, actor.Policy, "key", "method", nil); err == nil {
		t.Fatal("closed DB")
	}
}

func TestSkillBindingsOrderAndFallback(t *testing.T) {
	c, _ := rebaseTest(t)
	repo := fakeRepo(nil)
	attach := map[string]any{"kind": "attach_asset_pack", "path": "/a.md", "asset_kind": "skill", "asset_id": "a", "version": "1.0.0", "zip_sha256": "s", "manifest": map[string]any{}}
	create := map[string]any{"kind": "create", "path": "/a.md", "concept_type": "concept", "title": "x", "body": "x", "tags": []any{"skill"}}
	patch := map[string]any{"kind": "patch", "path": "/a.md", "tags": []any{}}
	other := map[string]any{"kind": "create", "path": "/other.md", "concept_type": "concept", "title": "x", "body": "x"}
	for i, raw := range [][]any{{attach}, {attach, other, create}, {attach, create, patch}, {attach, patch, create}} {
		changes, err := NormalizeProposalChanges(raw)
		if err != nil {
			t.Fatal(err)
		}
		err = c.validateSkillBindings(changes, repo)
		if i == 0 {
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatal("missing fallback")
			}
		} else if i == 2 {
			if err == nil {
				t.Fatal("last tag setter ignored")
			}
		} else if err != nil {
			t.Fatal(i, err)
		}
	}
	if err := control.WithTransaction(context.Background(), c.Queue.Proposals.DB, func(tx *sql.Tx) error {
		_, err := c.journalControl(context.Background(), tx, ProposalActor{}, "key", "method", "{}", "main", map[string]any{"bad": make(chan int)})
		return err
	}); err == nil {
		t.Fatal("bad result JSON")
	}
}

func TestRebasePublicConcurrentReplayAndCommitRetry(t *testing.T) {
	ctx := context.Background()
	c, actor := rebaseTest(t)
	q := &c.Queue
	root := t.TempDir()
	q.Paths = repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}
	boot, err := repository.BootstrapRepository(ctx, q.Paths, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.Proposals.DB.Exec("UPDATE proposals SET base_revision=?,status='needs_rebase'", boot.Revision); err != nil {
		t.Fatal(err)
	}
	_, err = q.Proposals.DB.Exec(`CREATE TABLE parent(id INTEGER PRIMARY KEY); CREATE TABLE child(id INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER failed_commit AFTER INSERT ON operations BEGIN INSERT INTO child VALUES(1); END;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Rebase(ctx, actor, "proposal", boot.Revision, "key"); err == nil {
		t.Fatal("expected COMMIT failure")
	}
	r, err := q.Proposals.Get(ctx, "proposal")
	if err != nil || r.Status != control.NeedsRebase {
		t.Fatal(r, err)
	}
	if rows := tableRows(t, q.Proposals.DB, "SELECT * FROM proposal_events"); len(rows) != 0 {
		t.Fatal(rows)
	}
	if _, err = q.Proposals.DB.Exec("DROP TRIGGER failed_commit"); err != nil {
		t.Fatal(err)
	}
	c.Random = nil
	// Separate service/DB handles must share the repository lock, not merely
	// serialise methods on one ProposalControls instance.
	var databasePath string
	for _, row := range tableRows(t, q.Proposals.DB, "PRAGMA database_list") {
		if row["name"] == "main" {
			databasePath = row["file"].(string)
		}
	}
	otherDB, err := control.Connect(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer otherDB.Close()
	otherQueue := *q
	otherQueue.Proposals.DB = otherDB
	other := &ProposalControls{Queue: otherQueue}
	var wg sync.WaitGroup
	results := make(chan ProposalControlResult, 12)
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			service := c
			if i%2 == 1 {
				service = other
			}
			result, err := service.Rebase(ctx, actor, "proposal", boot.Revision, "key")
			if err != nil {
				t.Error(err)
				return
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	fresh := 0
	id := ""
	for result := range results {
		if result.Data["replayed"] == false {
			fresh++
		}
		if id == "" {
			id = result.OperationID
		}
		if result.OperationID != id {
			t.Fatal("duplicate journal", result)
		}
	}
	if fresh != 1 || len(id) != 36 || id[14] != '4' || !strings.ContainsRune("89ab", rune(id[19])) {
		t.Fatal(fresh, id)
	}
	if rows := tableRows(t, q.Proposals.DB, "SELECT * FROM operations"); len(rows) != 1 {
		t.Fatal(rows)
	}
	if rows := tableRows(t, q.Proposals.DB, "SELECT * FROM proposal_events"); len(rows) != 1 {
		t.Fatal(rows)
	}
}
