package service

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func TestProposalApplyReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-apply.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Initial, Trigger string
		Before                     control.ProposalRecord
		Steps                      []struct {
			Expected  string
			Policy    access.EffectivePolicy
			Response  map[string]any
			Exception string
			After     control.ProposalRecord
			Operation *control.OperationRecord
			Revision  string
			Changed   []string
			Calls     int `json:"transaction_calls"`
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			root := t.TempDir()
			installMutationFiles(t, root, map[string]string{"/a.md": c.Initial})
			q.Paths.CurrentDir = root
			installRebaseRecord(t, q, c.Before)
			if c.Trigger != "" {
				if _, err := q.Proposals.DB.Exec(c.Trigger); err != nil {
					t.Fatal(err)
				}
			}
			controls := ProposalControls{Queue: q, Random: bytes.NewReader(make([]byte, 64)), MaxConceptBytes: 65536}
			revision := "main"
			changed := []string{}
			if c.Scenario == "stale-conflict" {
				changed = []string{"/a.md"}
			}
			calls := 0
			repo := fakeRepo(nil)
			repo.main = func(repository.GitRepositoryPaths) (string, error) { return revision, nil }
			repo.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
				return changed, nil
			}
			transaction := func(ctx context.Context, request repository.TransactionRequest, mutate repository.MutationCallback) (repository.TransactionResult, error) {
				calls++
				operations := control.Operations{DB: q.Proposals.DB, Now: q.Now}
				op, err := operations.Create(ctx, request.Operation)
				if err != nil {
					return repository.TransactionResult{}, err
				}
				if c.Scenario == "transaction-failure" {
					return repository.TransactionResult{}, &Error{"conflict", "synthetic transaction failure"}
				}
				if request.ExpectedRevision != revision {
					return repository.TransactionResult{}, &Error{"conflict", "synthetic expected revision mismatch"}
				}
				changed, err = mutate(ctx, root)
				if err != nil {
					return repository.TransactionResult{}, err
				}
				revision = "result"
				op, err = operations.MarkSucceeded(ctx, op.OpID, revision, map[string]any{"changed_paths": changed})
				if err != nil {
					return repository.TransactionResult{}, err
				}
				if c.Scenario == "refresh-failure" {
					if _, err = q.Proposals.DB.Exec(`INSERT INTO proposals SELECT 'broken',author_principal,client_instance_id,'base',intent,rationale,'{',patch_hash,'approved',NULL,NULL,NULL,NULL,created_at,updated_at,expires_at FROM proposals WHERE proposal_id='proposal'`); err != nil {
						t.Fatal(err)
					}
				}
				if request.AuthorName != "Rui Carmo" || request.AuthorEmail != "rui.carmo@gmail.com" || request.Operation.SourceChat == nil || *request.Operation.SourceChat != "chat" {
					t.Fatal(request)
				}
				return repository.TransactionResult{Operation: op, ChangedPaths: changed, ResultRevision: revision}, nil
			}
			client, session, chat := "client", "session", "chat"
			for i, step := range c.Steps {
				if i > 0 && c.Scenario == "replay-no-state" {
					if _, err := q.Proposals.DB.Exec("UPDATE operations SET state='failed'"); err != nil {
						t.Fatal(err)
					}
				}
				got, err := controls.apply(ctx, ProposalActor{Policy: step.Policy, ClientInstanceID: &client, MCPSessionID: &session, SourceChat: &chat}, "proposal", step.Expected, "key", repo, transaction)
				if step.Exception != "" || step.Response["status"] == "error" {
					if err == nil {
						t.Fatal("expected error", got, step)
					}
					if step.Response["status"] == "error" && !strings.Contains(c.Scenario, "preview") && c.Scenario != "refresh-failure" {
						if err.Error() != step.Response["message"] {
							t.Fatal(err, step.Response)
						}
					}
				} else if err != nil || !reflect.DeepEqual(jsonNormal(got.Data), step.Response["data"]) || got.OperationID != step.Response["operation_id"] || got.Revision != step.Response["repo_revision"] {
					t.Fatal(got, err, step.Response)
				}
				after, err := q.Proposals.Get(ctx, "proposal")
				if err != nil || !reflect.DeepEqual(after, step.After) {
					t.Fatal(after, step.After, err)
				}
				op, err := (control.Operations{DB: q.Proposals.DB}).ByIdempotency(ctx, "author", "key")
				if err != nil || !reflect.DeepEqual(op, step.Operation) {
					t.Fatal(op, step.Operation, err)
				}
				if calls != step.Calls || revision != step.Revision || !reflect.DeepEqual(changed, step.Changed) {
					t.Fatal(calls, revision, changed, step)
				}
			}
		})
	}
}
