package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func TestProposalQueueFailureStopsAtRecord(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	if _, err := q.Proposals.DB.Exec(`UPDATE proposals SET patch_json='{"changes":null}'`); err != nil {
		t.Fatal(err)
	}
	if err := q.refreshAll(ctx, fakeRepo(nil)); err == nil {
		t.Fatal("invalid persisted patch was ignored")
	}
	got, err := q.Proposals.Get(ctx, r.ProposalID)
	if err != nil || got.Status != r.Status {
		t.Fatal(got, err)
	}
	var count int
	if err = q.Proposals.DB.QueryRow("SELECT COUNT(*) FROM proposal_events").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestProposalRefreshCommitFailureRetry(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	// The event and status update both succeed, but SQLite rejects COMMIT.
	// The pinned connection must be rolled back before the next refresh.
	_, err := q.Proposals.DB.Exec(`CREATE TABLE parent(id INTEGER PRIMARY KEY);
 CREATE TABLE child(id INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED);
 CREATE TRIGGER fail_commit AFTER UPDATE ON proposals BEGIN INSERT INTO child VALUES(1); END;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.refresh(ctx, r, "main", nil, fakeRepo(nil)); err == nil {
		t.Fatal("deferred COMMIT failure missing")
	}
	got, err := q.Proposals.Get(ctx, r.ProposalID)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, r, err)
	}
	var count int
	if err = q.Proposals.DB.QueryRow("SELECT COUNT(*) FROM proposal_events").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if _, err = q.Proposals.DB.Exec("DROP TRIGGER fail_commit"); err != nil {
		t.Fatal(err)
	}
	got, err = q.refresh(ctx, r, "main", nil, fakeRepo(nil))
	if err != nil || got.Status != control.NeedsRebase {
		t.Fatal(got, err)
	}
	if err = q.Proposals.DB.QueryRow("SELECT COUNT(*) FROM proposal_events").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestProposalConflictCacheAndReadErrors(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	r.PatchJSON = `{"changes":[{"kind":"attach_asset_pack","path":"/asset.md"},{"kind":"patch","path":"/asset.md"}]}`
	cached := map[string]bool{"/.assets/12345678/docs/1.0.0.json": true}
	diffs := RevisionDiffs{"base": cached}
	repo := fakeRepo(nil)
	repo.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
		t.Fatal("ignored cached diff")
		return nil, nil
	}
	repo.read = func(string, string) (repository.BundleEntry, error) {
		return repository.BundleEntry{Document: repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{ID: "12345678"}}}, nil
	}
	got, err := q.conflicts(ctx, r, "main", nil, diffs, repo)
	if err != nil || got[0].Status != "conflict" || got[1].Status != "conflict" {
		t.Fatal(got, err)
	}
	if len(cached) != 1 || cached["/asset.md"] {
		t.Fatal("asset expansion mutated shared diff", cached)
	}
	r.PatchJSON = `{"changes":[{"kind":"patch","path":"/asset.md"}]}`
	got, err = q.conflicts(ctx, r, "main", nil, diffs, repo)
	if err != nil || got[0].Status != "clean" {
		t.Fatal("asset conflict leaked to another proposal", got, err)
	}
	r.PatchJSON = `{"changes":[{"kind":"attach_asset_pack","path":"/asset.md"}]}`
	for _, cause := range []error{&repository.BundleError{Message: "invalid"}, &repository.FrontmatterError{Message: "invalid"}, &repository.PathSafetyError{Message: "invalid"}, io.ErrUnexpectedEOF} {
		repo.read = func(string, string) (repository.BundleEntry, error) {
			return repository.BundleEntry{}, fmt.Errorf("wrapped: %w", cause)
		}
		got, err = q.conflicts(ctx, r, "main", nil, diffs, repo)
		if cause == io.ErrUnexpectedEOF {
			if !errors.Is(err, cause) {
				t.Fatal(err)
			}
			continue
		}
		if err != nil || got[0].Status != "clean" {
			t.Fatal(cause, got, err)
		}
	}
	repo = fakeRepo(nil)
	repo.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
		return nil, context.DeadlineExceeded
	}
	if _, err = q.conflicts(ctx, r, "main", nil, nil, repo); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestProposalRefreshQueueSkipsTerminalAndPreservesRecords(t *testing.T) {
	ctx := context.Background()
	q, _ := queueTest(t)
	// Multiple complete pages and a tail, including terminal and inactive states.
	statuses := []control.ProposalStatus{control.Applied, control.Expired, control.Draft, control.Rejected, control.Submitted}
	for i := range 215 {
		id := fmt.Sprintf("p-%03d", i)
		_, err := q.Proposals.Create(ctx, control.ProposalRequest{ProposalID: id, AuthorPrincipal: "p", BaseRevision: "base", Intent: "x", Patch: map[string]any{"changes": []any{}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = q.Proposals.DB.Exec("UPDATE proposals SET status=?,reviewed_by='reviewer',review_comment='keep' WHERE proposal_id=?", statuses[i%len(statuses)], id); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.refreshAll(ctx, fakeRepo(nil)); err != nil {
		t.Fatal(err)
	}
	for i := range 215 {
		r, err := q.Proposals.Get(ctx, fmt.Sprintf("p-%03d", i))
		want := statuses[i%len(statuses)]
		if want == control.Submitted {
			want = control.NeedsRebase
		}
		if err != nil || r.Status != want || r.ReviewedBy == nil || *r.ReviewedBy != "reviewer" || r.ReviewComment == nil || *r.ReviewComment != "keep" {
			t.Fatal(r, want, err)
		}
	}
	var count int
	if err := q.Proposals.DB.QueryRow("SELECT COUNT(*) FROM proposals").Scan(&count); err != nil || count != 216 {
		t.Fatal(count, err)
	}
	if err := q.Proposals.DB.QueryRow("SELECT COUNT(*) FROM proposal_events").Scan(&count); err != nil || count != 44 {
		t.Fatal(count, err)
	}
}
