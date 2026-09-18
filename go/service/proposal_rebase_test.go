package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func tableRows(t *testing.T, db *sql.DB, query string) []map[string]any {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	result := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err = rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		item := map[string]any{}
		for i, col := range cols {
			item[col] = values[i]
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}
func installRebaseRecord(t *testing.T, q ProposalQueue, r control.ProposalRecord) {
	t.Helper()
	_, err := q.Proposals.DB.Exec(`UPDATE proposals SET author_principal=?,client_instance_id=?,base_revision=?,intent=?,rationale=?,patch_json=?,patch_hash=?,status=?,created_at=?,updated_at=?,expires_at=?,reviewed_by=?,review_comment=? WHERE proposal_id='proposal'`, r.AuthorPrincipal, r.ClientInstanceID, r.BaseRevision, r.Intent, r.Rationale, r.PatchJSON, r.PatchHash, r.Status, r.CreatedAt, r.UpdatedAt, r.ExpiresAt, r.ReviewedBy, r.ReviewComment)
	if err != nil {
		t.Fatal(err)
	}
}
func TestProposalRebaseReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/proposal-rebase.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario      string
		Before        control.ProposalRecord
		Changed, Tags []string
		Trigger       string
		Steps         []struct {
			Expected                   string `json:"expected_revision"`
			Revision                   string
			Policy                     access.EffectivePolicy
			Response                   map[string]any
			Exception                  string
			After                      control.ProposalRecord
			Events, Operations, Assets []map[string]any
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			q.Now = func() time.Time { return time.Date(2026, 9, 18, 0, 0, 0, 123456000, time.UTC) }
			installRebaseRecord(t, q, c.Before)
			if _, err := q.Proposals.DB.Exec(`INSERT INTO proposal_assets(asset_id,proposal_id,concept_path,asset_kind,version,sha256,blob_bytes,manifest_json,media_type,created_at) VALUES('asset','proposal','/public/a.md','docs','1.0.0','digest',X'0001FF','{}','application/zip','created')`); err != nil {
				t.Fatal(err)
			}
			if c.Trigger != "" {
				if _, err := q.Proposals.DB.Exec(c.Trigger); err != nil {
					t.Fatal(err)
				}
			}
			controls := ProposalControls{Queue: q, Random: bytes.NewReader(make([]byte, 64))}
			repo := fakeRepo(c.Changed)
			repo.read = func(string, string) (repository.BundleEntry, error) {
				return repository.BundleEntry{Document: repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{ID: "12345678", Tags: c.Tags}}}, nil
			}
			client := "client"
			for i, step := range c.Steps {
				repo.main = func(repository.GitRepositoryPaths) (string, error) { return step.Revision, nil }
				if i > 0 && c.Scenario == "replay-after-advance" {
					if _, err := q.Proposals.DB.Exec("UPDATE proposals SET status='applied'"); err != nil {
						t.Fatal(err)
					}
				}
				got, err := controls.rebase(ctx, ProposalActor{Policy: step.Policy, ClientInstanceID: &client}, "proposal", step.Expected, "key", repo)
				switch {
				case step.Exception != "":
					if err == nil {
						t.Fatal("expected SQL error", got)
					}
				case step.Response["status"] == "error":
					if err == nil {
						t.Fatal("expected error", step.Response, got)
					}
					expected := step.Response
					class := ""
					var policy *Error
					var auth *access.AuthorizationError
					var conflict *control.IdempotencyConflictError
					switch {
					case errors.As(err, &policy):
						class = policy.Class
					case errors.As(err, &auth):
						class = "forbidden"
					case errors.As(err, &conflict):
						class = "idempotency_conflict"
					}
					if err.Error() != expected["message"] || class != expected["error_class"] {
						t.Fatal(err, class, expected)
					}
				default:
					if err != nil || !reflect.DeepEqual(jsonNormal(got.Data), step.Response["data"]) || got.OperationID != step.Response["operation_id"] {
						t.Fatal(got, err, step.Response)
					}
				}
				after, err := q.Proposals.Get(ctx, "proposal")
				if err != nil || !reflect.DeepEqual(after, step.After) {
					t.Fatal(after, step.After, err)
				}
				for _, table := range []struct {
					query string
					want  []map[string]any
				}{
					{"SELECT * FROM proposal_events ORDER BY event_id", step.Events},
					{"SELECT * FROM operations ORDER BY op_id", step.Operations},
					{"SELECT asset_id,proposal_id,concept_path,asset_kind,version,sha256,hex(blob_bytes) AS bytes_hex,manifest_json,media_type,created_at FROM proposal_assets", step.Assets},
				} {
					got := tableRows(t, q.Proposals.DB, table.query)
					if !reflect.DeepEqual(jsonNormal(got), jsonNormal(table.want)) {
						t.Fatal(table.query, got, table.want)
					}
				}
			}
		})
	}
}
