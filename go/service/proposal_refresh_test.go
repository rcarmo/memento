package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func queueTest(t *testing.T) (ProposalQueue, control.ProposalRecord) {
	t.Helper()
	db, err := control.Connect(context.Background(), filepath.Join(t.TempDir(), "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = control.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC) }
	p := control.Proposals{DB: db, Now: clock}
	r, err := p.Create(context.Background(), control.ProposalRequest{ProposalID: "proposal", AuthorPrincipal: "proposer", BaseRevision: "base", Intent: "synthetic", Patch: map[string]any{"changes": []any{map[string]any{"kind": "patch", "path": "/a.md"}}}})
	if err != nil {
		t.Fatal(err)
	}
	return ProposalQueue{Proposals: p, Now: clock}, r
}
func fakeRepo(changed []string) proposalRepository {
	return proposalRepository{main: func(repository.GitRepositoryPaths) (string, error) { return "main", nil }, diff: func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
		return changed, nil
	}, read: func(string, string) (repository.BundleEntry, error) { return repository.BundleEntry{}, os.ErrNotExist }}
}
func jsonNormal(value any) any {
	raw, _ := json.Marshal(value)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}
func TestProposalRefreshPythonReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/proposal-refresh.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Status        control.ProposalStatus
		Scenario      string
		Changed       []string
		Before, After control.ProposalRecord
		Conflicts     []ProposalConflict
		Events        []map[string]any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(string(c.Status)+"/"+c.Scenario, func(t *testing.T) {
			q, _ := queueTest(t)
			r := c.Before
			_, err := q.Proposals.DB.Exec(`UPDATE proposals SET client_instance_id=?,base_revision=?,rationale=?,patch_json=?,patch_hash=?,status=?,created_at=?,updated_at=?,expires_at=?,reviewed_by=?,review_comment=? WHERE proposal_id='proposal'`, r.ClientInstanceID, r.BaseRevision, r.Rationale, r.PatchJSON, r.PatchHash, r.Status, r.CreatedAt, r.UpdatedAt, r.ExpiresAt, r.ReviewedBy, r.ReviewComment)
			if err != nil {
				t.Fatal(err)
			}
			repo := fakeRepo(c.Changed)
			if c.Scenario == "missing-base" {
				repo.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
					return nil, &repository.GitError{Message: "missing"}
				}
			}
			if c.Scenario == "asset" {
				repo.read = func(string, string) (repository.BundleEntry, error) {
					return repository.BundleEntry{Document: repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{ID: "12345678"}}}, nil
				}
			}
			got, err := q.conflicts(context.Background(), r, "", nil, nil, repo)
			if err != nil || !reflect.DeepEqual(got, c.Conflicts) {
				t.Fatal(got, c.Conflicts, err)
			}
			updated, err := q.refresh(context.Background(), r, "", nil, repo)
			if err != nil || !reflect.DeepEqual(updated, c.After) {
				t.Fatal(updated, c.After, err)
			}
			if _, err = q.refresh(context.Background(), updated, "main", nil, repo); err != nil {
				t.Fatal(err)
			}
			rows, err := q.Proposals.DB.Query("SELECT event_id,proposal_id,actor,action,from_status,to_status,base_revision,repo_revision,details_json,created_at FROM proposal_events ORDER BY event_id")
			if err != nil {
				t.Fatal(err)
			}
			events := []map[string]any{}
			cols, _ := rows.Columns()
			for rows.Next() {
				values := make([]any, len(cols))
				pointers := make([]any, len(cols))
				for i := range values {
					pointers[i] = &values[i]
				}
				if err = rows.Scan(pointers...); err != nil {
					t.Fatal(err)
				}
				event := map[string]any{}
				for i, name := range cols {
					event[name] = values[i]
				}
				events = append(events, event)
			}
			rows.Close()
			if !reflect.DeepEqual(jsonNormal(events), jsonNormal(c.Events)) {
				t.Fatal(events, c.Events)
			}
		})
	}
}
func TestProposalRefreshCachePagesAndErrors(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	repo := fakeRepo(nil)
	calls := 0
	repo.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
		calls++
		return nil, nil
	}
	for i := range 205 {
		if _, err := q.Proposals.Create(ctx, control.ProposalRequest{ProposalID: fmt.Sprintf("p-%03d", i), AuthorPrincipal: "p", BaseRevision: "base", Intent: "x", Patch: map[string]any{"changes": []any{}}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.refreshAll(ctx, repo); err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	for _, limit := range []int{0, 1, -1, 100} {
		if _, err := q.conflicts(ctx, r, "main", &limit, RevisionDiffs{}, repo); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"{", `{"changes":1}`, `{"changes":[1]}`, `{"changes":[{"path":null}]}`, `{"changes":[{"kind":"rename","new_path":1}]}`} {
		copy := r
		copy.PatchJSON = raw
		if _, err := q.conflicts(ctx, copy, "main", nil, nil, repo); err == nil {
			t.Fatal(raw)
		}
	}
	copy := r
	copy.PatchJSON = `{}`
	if got, err := q.conflicts(ctx, copy, "main", nil, nil, repo); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	copy.PatchJSON = `{"changes":[{}]}`
	if _, err := q.conflicts(ctx, copy, "main", nil, nil, repo); err != nil {
		t.Fatal(err)
	}
	if (ProposalQueue{}).now() == "" {
		t.Fatal("clock")
	}
	q.Now = func() time.Time { return time.Date(2026, 9, 18, 0, 0, 0, 123456000, time.UTC) }
	if q.now() != "2026-09-18T00:00:00.123456Z" {
		t.Fatal(q.now())
	}
}

func TestProposalRefreshFailuresAndRollback(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	broken := fakeRepo(nil)
	broken.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, err := q.conflicts(ctx, r, "", nil, nil, broken); err == nil {
		t.Fatal("main read")
	}
	if _, err := q.refresh(ctx, r, "", nil, broken); err == nil {
		t.Fatal("refresh main")
	}
	if err := q.refreshAll(ctx, broken); err == nil {
		t.Fatal("queue main")
	}
	for _, cause := range []error{context.Canceled, io.ErrClosedPipe} {
		broken.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
			return nil, cause
		}
		if _, err := q.conflicts(ctx, r, "main", nil, nil, broken); err == nil {
			t.Fatal(cause)
		}
	}
	copy := r
	copy.PatchJSON = `{"changes":[{"kind":"attach_asset_pack","path":"/asset.md"}]}`
	broken = fakeRepo(nil)
	broken.read = func(string, string) (repository.BundleEntry, error) {
		return repository.BundleEntry{}, io.ErrClosedPipe
	}
	if _, err := q.conflicts(ctx, copy, "main", nil, nil, broken); err == nil {
		t.Fatal("unexpected read failure")
	}
	copy.PatchJSON = "{"
	if _, err := q.refresh(ctx, copy, "main", nil, fakeRepo(nil)); err == nil {
		t.Fatal("refresh bad patch")
	}
	for _, table := range []string{"proposal_events", "proposals"} {
		q, r := queueTest(t)
		verb := "INSERT"
		if table == "proposals" {
			verb = "UPDATE"
		}
		if _, err := q.Proposals.DB.Exec("CREATE TRIGGER blocked BEFORE " + verb + " ON " + table + " BEGIN SELECT RAISE(ABORT,'synthetic'); END"); err != nil {
			t.Fatal(err)
		}
		if _, err := q.refresh(ctx, r, "main", nil, fakeRepo(nil)); err == nil {
			t.Fatal(table)
		}
		var count int
		_ = q.Proposals.DB.QueryRow("SELECT COUNT(*) FROM proposal_events").Scan(&count)
		got, err := q.Proposals.Get(ctx, r.ProposalID)
		if err != nil || count != 0 || got.Status != r.Status {
			t.Fatal("partial refresh", got, count, err)
		}
	}
	if err := control.WithTransaction(ctx, q.Proposals.DB, func(tx *sql.Tx) error { return q.event(ctx, tx, r, "actor", "test", control.Submitted, "main", nil) }); err != nil {
		t.Fatal(err)
	}
	if err := control.WithTransaction(ctx, q.Proposals.DB, func(tx *sql.Tx) error {
		return q.event(ctx, tx, r, "actor", "test", control.Submitted, "main", map[string]any{"bad": make(chan int)})
	}); err == nil {
		t.Fatal("event JSON")
	}
	if _, err := q.refreshPage(ctx, &fakeIDs{scanErr: io.ErrClosedPipe}, "main", nil, fakeRepo(nil)); err == nil {
		t.Fatal("page scan")
	}
	if _, err := q.refreshPage(ctx, &fakeIDs{id: "missing"}, "main", nil, fakeRepo(nil)); err == nil {
		t.Fatal("missing proposal")
	}
	broken = fakeRepo(nil)
	broken.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
		return nil, context.Canceled
	}
	if _, err := q.refreshPage(ctx, &fakeIDs{id: r.ProposalID}, "main", nil, broken); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	q.Proposals.DB.Close()
	if err := q.refreshAll(ctx, fakeRepo(nil)); err == nil {
		t.Fatal("closed queue")
	}
}

type fakeIDs struct {
	id      string
	done    bool
	scanErr error
}

func (f *fakeIDs) Close() error { return nil }
func (f *fakeIDs) Next() bool {
	if f.done {
		return false
	}
	f.done = true
	return true
}
func (f *fakeIDs) Scan(values ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	*(values[0].(*string)) = f.id
	return nil
}
func (f *fakeIDs) Err() error { return nil }

func TestProposalRefreshRealGitAndExpiry(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	root := t.TempDir()
	q.Paths = repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}
	seed := t.TempDir()
	if err := os.WriteFile(filepath.Join(seed, "a.md"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	boot, err := repository.BootstrapRepository(ctx, q.Paths, seed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.Proposals.DB.Exec("UPDATE proposals SET base_revision=? WHERE proposal_id=?", boot.Revision, r.ProposalID); err != nil {
		t.Fatal(err)
	}
	r, err = q.Proposals.Get(ctx, r.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	conflicts, err := q.Conflicts(ctx, r, "", nil, nil)
	if err != nil || len(conflicts) != 1 || conflicts[0].Status != "clean" {
		t.Fatal(conflicts, err)
	}
	work, err := repository.CreateOperationWorktree(ctx, q.Paths, "mutation", boot.Revision)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(work.Path, "a.md"), []byte("changed"), 0600)
	identity := repository.GitCommitIdentity{Name: "Test", Email: "test@example.invalid", When: time.Now()}
	commit, err := repository.CommitExactPaths(ctx, work, []string{"/a.md"}, "change", identity, identity)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := repository.PublishMainCompareAndSwap(q.Paths, boot.Revision, commit.Revision); err != nil || !ok {
		t.Fatal(ok, err)
	}
	updated, err := q.Refresh(ctx, r, "", nil)
	if err != nil || updated.Status != control.Conflicted {
		t.Fatal(updated, err)
	}
	if err = q.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	// Expiry is strictly < now, and applied proposals never expire. All other
	// statuses can expire without altering patch/author/review metadata.
	_, _ = q.Proposals.DB.Exec("UPDATE proposals SET expires_at='2000-01-01T00:00:00Z'")
	updated, err = q.Proposals.Get(ctx, r.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = q.Refresh(ctx, updated, "", nil)
	if err != nil || updated.Status != control.Expired {
		t.Fatal(updated, err)
	}
}
