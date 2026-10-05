package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/derived"
)

func TestStatusDispatchReference(t *testing.T) {
	testToolReference(t, "status-tool-dispatch.json", statusToolDefinitions)
}
func TestStatusReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/service-status.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Files map[string]string
		Cases []struct {
			Scenario      string
			Policy        access.EffectivePolicy
			State         derived.IndexState
			Snapshot      derived.StatusSnapshot
			Mutation      *struct{ Path, Text string }
			Before, After []control.ProposalRecord
			Events        []map[string]any
			Expected      map[string]any
		}
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for i, tc := range f.Cases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.Scenario), func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			q.Paths.CurrentDir = t.TempDir()
			files := map[string][]byte{}
			for p, v := range f.Files {
				b, err := base64.StdEncoding.DecodeString(v)
				if err != nil {
					t.Fatal(err)
				}
				files[p] = b
			}
			installAssetFiles(t, q.Paths.CurrentDir, files)
			index := &derived.Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
			if err = index.Rebuild(ctx, q.Paths.CurrentDir, "main"); err != nil {
				t.Fatal(err)
			}
			db, err := derived.Connect(ctx, index.Path)
			if err != nil {
				t.Fatal(err)
			}
			for k, v := range map[string]string{"repo_revision": tc.State.RepoRevision, "index_revision": tc.State.IndexRevision, "status": tc.State.Status} {
				if _, err = db.Exec("UPDATE index_state SET value=? WHERE key=?", v, k); err != nil {
					t.Fatal(err)
				}
			}
			db.Close()
			if tc.Mutation != nil {
				installMutationFiles(t, q.Paths.CurrentDir, map[string]string{tc.Mutation.Path: tc.Mutation.Text})
			}
			if _, err = q.Proposals.DB.Exec("DELETE FROM proposals"); err != nil {
				t.Fatal(err)
			}
			for _, record := range tc.Before {
				patch, err := record.Patch()
				if err != nil {
					t.Fatal(err)
				}
				if _, err = q.Proposals.Create(ctx, control.ProposalRequest{ProposalID: record.ProposalID, AuthorPrincipal: record.AuthorPrincipal, BaseRevision: record.BaseRevision, Intent: record.Intent, Patch: patch}); err != nil {
					t.Fatal(err)
				}
				if _, err = q.Proposals.DB.Exec("UPDATE proposals SET status=?,created_at=?,updated_at=?,expires_at=? WHERE proposal_id=?", record.Status, record.CreatedAt, record.UpdatedAt, record.ExpiresAt, record.ProposalID); err != nil {
					t.Fatal(err)
				}
			}
			meta, err := NewModelsOffMetadata("compact")
			if err != nil {
				t.Fatal(err)
			}
			c := &ProposalControls{Queue: q, Metadata: meta, Index: index}
			data, options, err := c.status(ctx, ProposalActor{Policy: tc.Policy}, fakeRepo(nil))
			var result any
			if err != nil {
				result, err = FailureEnvelope(err)
			} else {
				result, err = q.successEnvelope(data, options, fakeRepo(nil))
			}
			applyExpectedStatusSummary(t, tc.Expected, tc.Policy, tc.After)
			if err != nil || !reflect.DeepEqual(jsonNormal(result), jsonNormal(tc.Expected)) {
				t.Fatal(result, tc.Expected, err)
			}
			after, err := q.Proposals.List(ctx, control.ProposalQuery{})
			if err != nil || !reflect.DeepEqual(after, tc.After) {
				t.Fatal(after, tc.After, err)
			}
			rows := tableRows(t, q.Proposals.DB, "SELECT * FROM proposal_events ORDER BY event_id")
			if !reflect.DeepEqual(jsonNormal(rows), jsonNormal(tc.Events)) {
				t.Fatal(rows, tc.Events)
			}
		})
	}
}

func applyExpectedStatusSummary(t *testing.T, expected map[string]any, policy access.EffectivePolicy, proposals []control.ProposalRecord) {
	t.Helper()
	data, ok := expected["data"].(map[string]any)
	if !ok {
		return
	}
	backlog, unresolved, counts, err := summarizeVisibleProposalStatuses(policy, proposals)
	if err != nil {
		t.Fatal(err)
	}
	data["proposal_backlog"] = float64(backlog)
	data["proposal_unresolved"] = float64(unresolved)
	encoded := map[string]any{}
	for status, count := range counts {
		encoded[status] = float64(count)
	}
	data["proposal_counts"] = encoded
}

func insertStatusRecord(t *testing.T, q ProposalQueue, id, author string, status control.ProposalStatus, path string) {
	t.Helper()
	_, err := q.Proposals.Create(context.Background(), control.ProposalRequest{ProposalID: id, AuthorPrincipal: author, BaseRevision: "main", Intent: "synthetic", Patch: map[string]any{"changes": []any{map[string]any{"kind": "patch", "path": path}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.Proposals.DB.Exec("UPDATE proposals SET status=? WHERE proposal_id=?", status, id); err != nil {
		t.Fatal(err)
	}
}

func insertMalformedStatusRecord(t *testing.T, q ProposalQueue, id, author string, status control.ProposalStatus, rawPatch string) {
	t.Helper()
	insertStatusRecord(t, q, id, author, status, "/public/a.md")
	if _, err := q.Proposals.DB.Exec("UPDATE proposals SET patch_json=?,patch_hash='bad' WHERE proposal_id=?", rawPatch, id); err != nil {
		t.Fatal(err)
	}
}

func TestStatusProposalCountsVisibilityAndNormalization(t *testing.T) {
	ctx := context.Background()
	q, _ := queueTest(t)
	if _, err := q.Proposals.DB.Exec("DELETE FROM proposals"); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id, author, path string
		status           control.ProposalStatus
	}{
		{"submitted", "actor", "/public/submitted.md", control.Submitted},
		{"approved", "actor", "/public/approved.md", control.Approved},
		{"draft", "actor", "/public/draft.md", control.Draft},
		{"needs-rebase", "actor", "/public/rebase.md", control.NeedsRebase},
		{"conflicted", "actor", "/public/conflicted.md", control.Conflicted},
		{"stale", "actor", "/public/stale.md", control.Stale},
		{"rejected", "actor", "/public/rejected.md", control.Rejected},
		{"applied", "actor", "/public/applied.md", control.Applied},
		{"expired", "actor", "/public/expired.md", control.Expired},
		{"hidden-author", "other", "/public/hidden-author.md", control.Submitted},
		{"hidden-private", "actor", "/private/hidden.md", control.Submitted},
	} {
		insertStatusRecord(t, q, item.id, item.author, item.status, item.path)
	}
	insertMalformedStatusRecord(t, q, "hidden-malformed", "other", control.Stale, "{}")
	meta, err := NewModelsOffMetadata("compact")
	if err != nil {
		t.Fatal(err)
	}
	c := &ProposalControls{Queue: q, Metadata: meta, Index: failingStatusIndex{result: derived.StatusSnapshot{State: derived.IndexState{Status: "ready", RepoRevision: "main", IndexRevision: "main"}, VisibleConcepts: 2}}}
	actor := ProposalActor{Policy: access.EffectivePolicy{Principal: "actor", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}}
	data, _, err := c.status(ctx, actor, fakeRepo(nil))
	if err != nil {
		t.Fatal(err)
	}
	if data["proposal_backlog"] != 2 || data["proposal_unresolved"] != 6 {
		t.Fatal(data)
	}
	if !reflect.DeepEqual(data["proposal_counts"], map[string]int{"approved": 1, "applied": 1, "conflicted": 1, "draft": 1, "expired": 1, "needs_rebase": 2, "rejected": 1, "submitted": 1}) {
		t.Fatal(data["proposal_counts"])
	}
}
