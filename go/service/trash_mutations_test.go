package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func TestTrashToolDispatchReference(t *testing.T) {
	testToolReference(t, "trash-tool-dispatch.json", trashToolDefinitions)
}
func TestTrashMutationReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/trash-mutations.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Action string
		Policy           access.EffectivePolicy
		Files            map[string]string
		Steps            []struct {
			Arguments map[string]any
			Expected  map[string]any
			Exception *string
			Files     map[string]string
			Operation *control.OperationRecord
			Requests  []map[string]any
			Recorded  [][]string
			Refreshes int
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			if _, err = q.Proposals.DB.Exec("DELETE FROM proposals"); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			q.Paths.CurrentDir = root
			files := map[string][]byte{}
			for path, encoded := range tc.Files {
				blob, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					t.Fatal(err)
				}
				files[path] = blob
			}
			installAssetFiles(t, root, files)
			if tc.Scenario == "purge-unlink-failure" {
				path := filepath.Join(root, ".assets/12345678/docs/1.9.0.zip")
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err = os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			c := &ProposalControls{Queue: q, Random: bytes.NewReader(make([]byte, 128))}
			revision := "main"
			requests := []map[string]any{}
			recorded := [][]string{}
			refreshes := 0
			repo := fakeRepo(nil)
			repo.main = func(repository.GitRepositoryPaths) (string, error) {
				refreshes++
				if tc.Scenario == "refresh-failure" {
					return "", &Error{"conflict", "synthetic refresh failure"}
				}
				return revision, nil
			}
			c.ChangedConcepts = func(_ context.Context, _ access.EffectivePolicy, paths []string) error {
				recorded = append(recorded, paths)
				if tc.Scenario == "tracking-failure" {
					return &Error{"conflict", "synthetic tracking failure"}
				}
				return nil
			}
			operations := control.Operations{DB: q.Proposals.DB, Now: q.Now}
			transaction := func(ctx context.Context, r repository.TransactionRequest, mutate repository.MutationCallback) (repository.TransactionResult, error) {
				requests = append(requests, map[string]any{"operation": map[string]any{"op_id": r.Operation.OpID, "principal": r.Operation.Principal, "idempotency_key": r.Operation.IdempotencyKey, "tool_name": r.Operation.ToolName, "request_json": r.Operation.RequestJSON, "client_instance_id": r.Operation.ClientInstanceID, "mcp_session_id": r.Operation.MCPSessionID, "source_chat": r.Operation.SourceChat}, "expected_revision": r.ExpectedRevision, "commit_message": r.CommitMessage, "author_name": r.AuthorName, "author_email": r.AuthorEmail})
				existing, err := operations.ByIdempotency(ctx, "actor", "key")
				if err != nil {
					return repository.TransactionResult{}, err
				}
				if existing != nil {
					if existing.RequestHash != r.Operation.RequestHash() {
						return repository.TransactionResult{}, &control.IdempotencyConflictError{}
					}
					payload, err := existing.ReplayPayload()
					if err != nil {
						return repository.TransactionResult{}, err
					}
					paths := []string{}
					for _, p := range payload["changed_paths"].([]any) {
						paths = append(paths, p.(string))
					}
					return repository.TransactionResult{Operation: *existing, ResultRevision: *existing.ResultRevision, ChangedPaths: paths, Replayed: true}, nil
				}
				op, err := operations.Create(ctx, r.Operation)
				if err != nil {
					return repository.TransactionResult{}, err
				}
				if tc.Scenario == "transaction-failure" {
					return repository.TransactionResult{}, &Error{"conflict", "synthetic transaction failure"}
				}
				if r.ExpectedRevision != revision {
					return repository.TransactionResult{}, &Error{"conflict", "synthetic expected revision mismatch"}
				}
				paths, err := mutate(ctx, root)
				if err != nil {
					return repository.TransactionResult{}, err
				}
				revision = "result"
				op, err = operations.MarkSucceeded(ctx, op.OpID, revision, map[string]any{"changed_paths": paths})
				if err != nil {
					return repository.TransactionResult{}, err
				}
				return repository.TransactionResult{Operation: op, ResultRevision: revision, ChangedPaths: paths}, nil
			}
			client, session, chat := "client", "session", "chat"
			actor := ProposalActor{Policy: tc.Policy, ClientInstanceID: &client, MCPSessionID: &session, SourceChat: &chat}
			for _, step := range tc.Steps {
				a := step.Arguments
				data, options, err := c.trashMutation(ctx, actor, tc.Action, a["path"].(string), a["expected_revision"].(string), "key", a["confirm"], repo, transaction, defaultMutationIO())
				if step.Exception != nil {
					if err == nil {
						t.Fatal("expected exception")
					}
				} else {
					var result any
					if err != nil {
						result, err = FailureEnvelope(err)
					} else {
						result, err = q.successEnvelope(data, options, fakeRepo(nil))
					}
					if err != nil || !reflect.DeepEqual(jsonNormal(result), step.Expected) {
						t.Fatal(result, step.Expected, err)
					}
				}
				gotFiles := map[string]string{}
				snapshot, _ := mutationSnapshot(t, root)
				for path, content := range snapshot {
					gotFiles[path] = base64.StdEncoding.EncodeToString([]byte(content))
				}
				op, err := operations.ByIdempotency(ctx, "actor", "key")
				if err != nil || !reflect.DeepEqual(op, step.Operation) {
					t.Fatal(op, step.Operation, err)
				}
				if !reflect.DeepEqual(gotFiles, step.Files) || !reflect.DeepEqual(jsonNormal(requests), jsonNormal(step.Requests)) || !reflect.DeepEqual(recorded, step.Recorded) || refreshes != step.Refreshes {
					t.Fatal(gotFiles, step.Files, requests, step.Requests, recorded, step.Recorded, refreshes, step.Refreshes)
				}
			}
		})
	}
}
