package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func TestArchivalIOAndIndexFailures(t *testing.T) {
	ctx := context.Background()
	changes := []ProposalChange{{"kind": "trash", "path": "/public/a.md"}}
	if got, err := (ProposalQueue{}).ArchivalImpact(ctx, archivePolicy(), nil, "main", "missing"); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	for _, stage := range []string{"main", "second-main", "open", "missing-index", "bad-index", "bad-state", "bad-links", "bad-row", "unsafe-target", "kinds", "versions", "metadata"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			db := archiveDB(t, root)
			q := ProposalQueue{Paths: repository.GitRepositoryPaths{CurrentDir: root}}
			repo := archiveRepo()
			index := filepath.Join(root, "index.sqlite")
			open := openArchivalIndex
			calls := 0
			repo.main = func(repository.GitRepositoryPaths) (string, error) {
				calls++
				if stage == "main" || (stage == "second-main" && calls == 2) {
					return "", io.ErrClosedPipe
				}
				return "main", nil
			}
			var query string
			switch stage {
			case "open":
				open = func(string) (*sql.DB, error) { return nil, io.ErrClosedPipe }
			case "missing-index":
				index = filepath.Join(root, "missing.sqlite")
			case "bad-index":
				query = "DROP TABLE index_state"
			case "bad-state":
				query = "UPDATE index_state SET value=NULL"
			case "bad-links":
				query = "DROP TABLE links"
			case "bad-row":
				query = "INSERT INTO concepts VALUES('id','/public/source.md');INSERT INTO links VALUES('id','/public/a.md',NULL,'resolved')"
			case "unsafe-target":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "trash")); err != nil {
					t.Fatal(err)
				}
			case "kinds":
				if err := os.WriteFile(filepath.Join(root, ".assets"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "versions":
				writeArchiveAssets(t, root, map[string][]string{"docs": {"nonsense"}})
			case "metadata":
				writeArchiveAssets(t, root, map[string][]string{"docs": {"1.0.0"}})
				if err := os.WriteFile(filepath.Join(root, ".assets/12345678/docs/1.0.0.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if query != "" {
				if _, err := db.Exec(query); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := q.archivalImpact(ctx, archivePolicy(), changes, "main", index, repo, open); err == nil {
				t.Fatal("expected failure", stage)
			}
			if stage == "missing-index" {
				if _, err := os.Stat(index); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("read created index", err)
				}
			}
		})
	}
	if err := checkArchivalTarget("path", func(string) (os.FileInfo, error) { return nil, io.ErrClosedPipe }); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	for _, fail := range []*failedArchivalRows{{scan: io.ErrClosedPipe}, {err: io.ErrClosedPipe}} {
		if err := checkArchivalStateRows(fail, "main"); !errors.Is(err, io.ErrClosedPipe) || !fail.closed {
			t.Fatal(err, fail)
		}
		fail.closed = false
		if _, err := archivalReferenceRows(fail, archivePolicy()); !errors.Is(err, io.ErrClosedPipe) || !fail.closed {
			t.Fatal(err, fail)
		}
	}
}

type failedArchivalRows struct {
	scan, err error
	closed    bool
}

func (r *failedArchivalRows) Next() bool        { return r.scan != nil }
func (r *failedArchivalRows) Scan(...any) error { return r.scan }
func (r *failedArchivalRows) Err() error        { return r.err }
func (r *failedArchivalRows) Close() error      { r.closed = true; return nil }

func TestVisibleArchivalMalformedAndPublic(t *testing.T) {
	ctx := context.Background()
	q := ProposalQueue{}
	policy := archivePolicy()
	repo := archiveRepo()
	for _, raw := range []string{"{", `{"archival_impact":1}`, `{"archival_impact":[null]}`, `{"archival_impact":[{}]}`, `{"archival_impact":[{"path":"/public/a.md"}]}`, `{"archival_impact":[{"path":"/public/a.md","inbound_references":[null]}]}`, `{"archival_impact":[{"path":"/public/a.md","inbound_references":[{}]}]}`} {
		if _, err := q.visibleArchivalImpact(ctx, control.ProposalRecord{PatchJSON: raw}, policy, "unused", repo); err == nil {
			t.Fatal(raw)
		}
	}
	raw := `{"archival_impact":[{"path":"/public/a.md","inbound_references":[]}]}`
	r := control.ProposalRecord{PatchJSON: raw, Status: control.Submitted, BaseRevision: "main"}
	if _, err := q.visibleArchivalImpact(ctx, r, policy, "unused", repo); err == nil {
		t.Fatal("missing changes on recompute")
	}
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, err := q.visibleArchivalImpact(ctx, r, policy, "unused", repo); err == nil {
		t.Fatal("main failure")
	}
	r.Status = control.Rejected
	if got, err := q.VisibleArchivalImpact(ctx, r, policy, "unused"); err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	r.PatchJSON = `{}`
	if got, err := q.VisibleArchivalImpact(ctx, r, policy, "unused"); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}
