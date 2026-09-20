package service

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func TestProposalPreviewReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-preview.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Previews []struct {
			Name, Initial, Expected string
			Changes                 []any
			Limit                   int
			ErrorType               string `json:"error_type"`
		}
		Diffs    []struct{ Before, After, Expected string }
		Payloads []struct {
			Record   control.ProposalRecord
			Events   []map[string]any
			Expected map[string]any
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Previews {
		t.Run(c.Name, func(t *testing.T) {
			root := t.TempDir()
			installMutationFiles(t, root, map[string]string{"/a.md": c.Initial})
			changes, err := NormalizeProposalChanges(c.Changes)
			if err != nil {
				t.Fatal(err)
			}
			got, err := (WorktreeMutator{MaxConceptBytes: c.Limit}).PreviewChanges(root, changes)
			if c.ErrorType != "" {
				if err == nil {
					t.Fatal("missing error")
				}
			} else if err != nil || got != c.Expected {
				t.Fatalf("%q != %q: %v", got, c.Expected, err)
			}
		})
	}
	for _, c := range fixture.Diffs {
		if got := proposalDiff(c.Before, c.After, "/a.md"); got != c.Expected {
			t.Fatalf("%q != %q", got, c.Expected)
		}
	}
	for _, c := range fixture.Payloads {
		q, _ := queueTest(t)
		for _, e := range c.Events {
			if _, err := q.Proposals.DB.Exec(`INSERT INTO proposal_events(event_id,proposal_id,actor,action,from_status,to_status,base_revision,repo_revision,details_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, e["event_id"], "proposal", e["actor"], e["action"], e["from_status"], e["to_status"], e["base_revision"], e["repo_revision"], e["details_json"], e["created_at"]); err != nil {
				t.Fatal(err)
			}
		}
		id := c.Record.ProposalID
		c.Record.ProposalID = "proposal"
		got, err := q.payload(context.Background(), c.Record, "preview text", fakeRepo([]string{"/b.md"}))
		if err != nil {
			t.Fatal(err)
		}
		got["proposal_id"] = id
		if !reflect.DeepEqual(jsonNormal(got), c.Expected) {
			t.Fatal(got, c.Expected)
		}
	}
}
func TestProposalPayloadFailures(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	repo := fakeRepo(nil)
	for _, raw := range []string{"{", `{}`, `{"changes":null}`} {
		copy := r
		copy.PatchJSON = raw
		if _, err := q.payload(ctx, copy, "", repo); err == nil {
			t.Fatal(raw)
		}
	}
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, err := q.payload(ctx, r, "", repo); err == nil {
		t.Fatal("main")
	}
	for _, rows := range []*failedArchivalRows{{scan: io.ErrClosedPipe}, {err: io.ErrClosedPipe}} {
		if _, err := proposalHistory(rows); err == nil || !rows.closed {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	q.Paths = repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}
	boot, err := repository.BootstrapRepository(ctx, q.Paths, "")
	if err != nil {
		t.Fatal(err)
	}
	r.BaseRevision = boot.Revision
	if _, err = q.Payload(ctx, r, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = q.Proposals.DB.Exec(`DROP TABLE proposal_events; CREATE TABLE proposal_events(proposal_id TEXT,event_id INTEGER,actor TEXT,action TEXT,from_status TEXT,to_status TEXT,base_revision TEXT,repo_revision TEXT,details_json TEXT,created_at TEXT);INSERT INTO proposal_events(proposal_id,event_id) VALUES('proposal',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = q.Payload(ctx, r, ""); err == nil {
		t.Fatal("malformed history row")
	}
	// Query failure after successful revision lookup.
	q.Proposals.DB.Close()
	if _, err = q.Payload(ctx, r, ""); err == nil {
		t.Fatal("closed DB")
	}
	if got := pythonDiffLines("a\r\nb\r\nx\rend"); !reflect.DeepEqual(got, []string{"a\r\n", "b\r\n", "x\r", "end"}) {
		t.Fatal(got)
	}
}
