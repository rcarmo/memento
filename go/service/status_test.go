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

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
)

func TestStatusDispatchReference(t *testing.T) {
	testToolReference(t, "status-tool-dispatch.json", statusToolDefinitions)
}
func TestStatusReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/service-status.json")
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
			if err != nil || !reflect.DeepEqual(jsonNormal(result), tc.Expected) {
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
