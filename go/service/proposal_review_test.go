package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
)

func reviewRandom() *bytes.Reader {
	raw := make([]byte, 64)
	for i := range 4 {
		raw[i*16+15] = byte(i + 1)
	}
	return bytes.NewReader(raw)
}
func TestProposalReviewReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/proposal-review.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Decision, Trigger string
		Before                      control.ProposalRecord
		Changed                     []string
		Steps                       []struct {
			Policy             access.EffectivePolicy
			Key                *string
			Comment            string
			Response           map[string]any
			Exception          string
			After              control.ProposalRecord
			Events, Operations []map[string]any
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Decision+"/"+c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			installRebaseRecord(t, q, c.Before)
			controls := ProposalControls{Queue: q, Random: reviewRandom()}
			if c.Scenario == "archival" {
				root := t.TempDir()
				db := archiveDB(t, root)
				if _, err := db.Exec("UPDATE index_state SET value='old' WHERE key='index_revision'"); err != nil {
					t.Fatal(err)
				}
				controls.DerivedIndexPath = filepath.Join(root, "index.sqlite")
			}
			if c.Trigger != "" {
				if _, err := q.Proposals.DB.Exec(c.Trigger); err != nil {
					t.Fatal(err)
				}
			}
			client := "client"
			for _, step := range c.Steps {
				key := ""
				if step.Key != nil {
					key = *step.Key
				}
				got, err := controls.review(ctx, ProposalActor{Policy: step.Policy, ClientInstanceID: &client}, "proposal", c.Decision, &step.Comment, key, fakeRepo(c.Changed))
				switch {
				case step.Exception != "":
					if err == nil {
						t.Fatal("expected SQL error", got)
					}
				case step.Response["status"] == "error":
					if err == nil {
						t.Fatal("expected error", got, step.Response)
					}
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
					if err.Error() != step.Response["message"] || class != step.Response["error_class"] {
						t.Fatal(err, class, step.Response)
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
				}{{"SELECT * FROM proposal_events ORDER BY event_id", step.Events}, {"SELECT * FROM operations ORDER BY op_id", step.Operations}} {
					got := tableRows(t, q.Proposals.DB, table.query)
					if !reflect.DeepEqual(jsonNormal(got), jsonNormal(table.want)) {
						t.Fatal(table.query, got, table.want)
					}
				}
			}
		})
	}
}
