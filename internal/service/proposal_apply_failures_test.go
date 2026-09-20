package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func realApplyTest(t *testing.T) (*ProposalControls, ProposalActor, string) {
	t.Helper()
	ctx := context.Background()
	c, actor := rebaseTest(t)
	actor.Policy.Roles = []string{"curator"}
	c.Random = nil
	c.MaxConceptBytes = 65536
	root := t.TempDir()
	seed := t.TempDir()
	installMutationFiles(t, seed, map[string]string{"/a.md": mutationConcept})
	c.Queue.Paths = repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}
	boot, err := repository.BootstrapRepository(ctx, c.Queue.Paths, seed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Queue.Proposals.DB.Exec(`UPDATE proposals SET base_revision=?,status='approved',reviewed_by='reviewer',review_comment='keep',patch_json='{"changes":[{"kind":"patch","path":"/a.md","body":"changed"}]}'`, boot.Revision); err != nil {
		t.Fatal(err)
	}
	return c, actor, boot.Revision
}
func TestPublicApplyConcurrentReplay(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	derived, tracked := 0, 0
	c.DerivedUpdate = func(_ context.Context, root, revision string, paths []string) error {
		derived++
		entry, err := repository.ReadBundleEntry(root, "/a.md")
		if err != nil || entry.Document.Body != "changed" || revision == base || !reflect.DeepEqual(paths, []string{"/a.md"}) {
			t.Fatal(entry, revision, paths, err)
		}
		return nil
	}
	c.ChangedConcepts = func(_ context.Context, _ access.EffectivePolicy, paths []string) error {
		tracked++
		if !reflect.DeepEqual(paths, []string{"/a.md"}) {
			t.Fatal(paths)
		}
		return nil
	}
	var wg sync.WaitGroup
	results := make(chan ProposalApplyResult, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := c.Apply(ctx, actor, "proposal", base, "key")
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
	revision, op := "", ""
	for r := range results {
		if r.Data["replayed"] == false {
			fresh++
		}
		if op == "" {
			op = r.OperationID
			revision = r.Revision
		}
		if r.OperationID != op || r.Revision != revision {
			t.Fatal("duplicate publication", r)
		}
	}
	if fresh != 1 || derived != 1 || tracked != 1 {
		t.Fatal(fresh, derived, tracked)
	}
	record, err := c.Queue.Proposals.Get(ctx, "proposal")
	if err != nil || record.Status != control.Applied || record.ReviewComment == nil || *record.ReviewComment != "keep" {
		t.Fatal(record, err)
	}
	if _, err = c.Apply(ctx, actor, "proposal", base, "different"); err == nil {
		t.Fatal("applied with another key")
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
	replay, err := c.Apply(ctx, actor, "proposal", base, "key")
	if err != nil || replay.OperationID != op || replay.Data["replayed"] != true {
		t.Fatal(replay, err)
	}
}
func TestApplyPostPublicationFailures(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"proposal-update", "tracking", "payload"} {
		t.Run(stage, func(t *testing.T) {
			c, actor, base := realApplyTest(t)
			switch stage {
			case "proposal-update":
				if _, err := c.Queue.Proposals.DB.Exec(`CREATE TRIGGER blocked BEFORE UPDATE ON proposals WHEN NEW.status='applied' BEGIN SELECT RAISE(ABORT,'synthetic'); END`); err != nil {
					t.Fatal(err)
				}
			case "tracking":
				c.ChangedConcepts = func(context.Context, access.EffectivePolicy, []string) error { return io.ErrClosedPipe }
			case "payload":
				c.ChangedConcepts = func(context.Context, access.EffectivePolicy, []string) error {
					_, err := c.Queue.Proposals.DB.Exec("DROP TABLE proposal_events")
					return err
				}
			}
			if _, err := c.Apply(ctx, actor, "proposal", base, "key"); err == nil {
				t.Fatal("missing post-publication failure")
			}
			head, err := repository.GetMainRevision(c.Queue.Paths)
			if err != nil || head == base {
				t.Fatal(head, err)
			}
			op, err := (control.Operations{DB: c.Queue.Proposals.DB}).ByIdempotency(ctx, actor.Policy.Principal, "key")
			if err != nil || op == nil || op.State != control.Succeeded {
				t.Fatal(op, err)
			}
			if stage == "proposal-update" {
				r, err := c.Queue.Proposals.Get(ctx, "proposal")
				if err != nil || r.Status != control.Approved {
					t.Fatal(r, err)
				}
				if _, err = c.Queue.Proposals.DB.Exec("DROP TRIGGER blocked"); err != nil {
					t.Fatal(err)
				}
				if _, err = c.Apply(ctx, actor, "proposal", base, "key"); err == nil {
					t.Fatal("source must conflict on retry of old approved base")
				}
			}
		})
	}
}
func TestApplyEarlyAndReplayFailures(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"missing", "refresh", "lookup", "request-json", "conflicts", "main", "random"} {
		t.Run(stage, func(t *testing.T) {
			c, actor := rebaseTest(t)
			actor.Policy.Roles = []string{"curator"}
			c.MaxConceptBytes = 65536
			repo := fakeRepo(nil)
			id, expected := "proposal", "main"
			calls := 0
			if _, err := c.Queue.Proposals.DB.Exec("UPDATE proposals SET base_revision='main',status='approved'"); err != nil {
				t.Fatal(err)
			}
			repo.main = func(repository.GitRepositoryPaths) (string, error) {
				calls++
				if stage == "refresh" || (stage == "conflicts" && calls == 2) || (stage == "main" && calls == 3) {
					return "", io.ErrClosedPipe
				}
				if stage == "lookup" {
					c.Queue.Proposals.DB.Close()
				}
				return "main", nil
			}
			if stage == "missing" {
				id = "missing"
			}
			if stage == "request-json" {
				expected = "\xff"
			}
			if stage == "random" {
				c.Random = bytes.NewReader(nil)
			}
			if _, err := c.apply(ctx, actor, id, expected, "key", repo, nil); err == nil {
				t.Fatal("missing failure")
			}
		})
	}
	c, actor := rebaseTest(t)
	mutator := WorktreeMutator{MaxConceptBytes: 65536}
	revision := "result"
	record := control.ProposalRecord{PatchJSON: `{"changes":[]}`}
	bad := "{"
	op := control.OperationRecord{ResultRevision: &revision, ResultJSON: &bad}
	if _, err := c.replayApplied(ctx, record, op, fakeRepo(nil), mutator); err == nil {
		t.Fatal("bad replay JSON")
	}
	op.ResultJSON = nil
	record.PatchJSON = "{"
	if _, err := c.replayApplied(ctx, record, op, fakeRepo(nil), mutator); err == nil {
		t.Fatal("bad replay proposal")
	}
	for _, raw := range []string{"{", `{"changes":[{"kind":"patch","path":"/denied/a.md"}]}`, `{"changes":[{"kind":"trash","path":"/public/a.md"}]}`} {
		if _, err := c.prepareAppliedChanges(ctx, archivePolicy(), control.ProposalRecord{PatchJSON: raw}, "old", fakeRepo(nil)); err == nil {
			t.Fatal(raw)
		}
	}
	if _, err := c.applyPayload(ctx, record, nil, nil, true, "op", "rev", fakeRepo(nil), mutator); err == nil {
		t.Fatal("payload failure")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := c.Apply(canceled, actor, "proposal", "main", "key"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestApplyArchivalAndReplayPathShapes(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	if _, err := c.Queue.Proposals.DB.Exec(`UPDATE proposals SET patch_json='{"changes":[{"kind":"trash","path":"/a.md"}]}'`); err != nil {
		t.Fatal(err)
	}
	c.DerivedIndexPath = filepath.Join(t.TempDir(), "missing.sqlite")
	if _, err := c.Apply(ctx, actor, "proposal", base, "key"); err == nil {
		t.Fatal("archival without fresh index")
	}
	indexRoot := t.TempDir()
	index := archiveDB(t, indexRoot)
	if _, err := index.Exec("UPDATE index_state SET value=? WHERE key='index_revision'", base); err != nil {
		t.Fatal(err)
	}
	c.DerivedIndexPath = filepath.Join(indexRoot, "index.sqlite")
	got, err := c.Apply(ctx, actor, "proposal", base, "key")
	if err != nil || got.Data["replayed"] != false {
		t.Fatal(got, err)
	}
	if _, err := repository.ReadBundleEntry(c.Queue.Paths.CurrentDir, "/trash/a.md"); err != nil {
		t.Fatal(err)
	}
	for _, result := range []any{nil, `{"changed_paths":"bad"}`, `{"changed_paths":["/a.md",1,null,"/trash/a.md"]}`} {
		if _, err := c.Queue.Proposals.DB.Exec("UPDATE operations SET result_json=?", result); err != nil {
			t.Fatal(err)
		}
		replay, err := c.Apply(ctx, actor, "proposal", base, "key")
		if err != nil {
			t.Fatal(err)
		}
		paths := replay.Data["changed_paths"].([]string)
		if result == nil || result == `{"changed_paths":"bad"}` {
			if len(paths) != 0 {
				t.Fatal(paths)
			}
		} else if !reflect.DeepEqual(paths, []string{"/a.md", "/trash/a.md"}) {
			t.Fatal(paths)
		}
	}
}
