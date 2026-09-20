package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func TestAssetPruneReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/asset-prune.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario  string
		Files     map[string]string
		Proposals []control.ProposalRecord
		Policy    access.EffectivePolicy
		Arguments struct {
			ID       string `json:"id_or_path"`
			Kind     string `json:"asset_kind"`
			Keep     any
			Expected string `json:"expected_revision"`
			Key      string `json:"idempotency_key"`
		}
		MissingZIP *string `json:"missing_zip"`
		Steps      []struct {
			Expected           map[string]any
			Exception          *string
			Remaining, Changed []string
			Requests           []map[string]any
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			root := t.TempDir()
			q.Paths.CurrentDir = root
			files := map[string][]byte{}
			for path, encoded := range tc.Files {
				data, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					t.Fatal(err)
				}
				files[path] = data
			}
			installAssetFiles(t, root, files)
			for _, p := range tc.Proposals {
				installRebaseRecord(t, q, p)
				if _, err = q.Proposals.DB.Exec("UPDATE proposals SET proposal_id=? WHERE proposal_id='proposal'", p.ProposalID); err != nil {
					t.Fatal(err)
				}
				if _, err = q.Proposals.DB.Exec("INSERT INTO proposal_assets VALUES(?,?,?,?,?,?,?,?,?,?)", p.ProposalID, "asset", "/public/a.md", "docs", "1.9.0", "application/zip", string(bytes.Repeat([]byte{'a'}, 64)), []byte("synthetic"), "{}", "created"); err != nil {
					t.Fatal(err)
				}
			}
			if tc.MissingZIP != nil {
				if err = os.Remove(filepath.Join(root, *tc.MissingZIP)); err != nil {
					t.Fatal(err)
				}
			}
			if tc.Scenario == "unlink-failure" {
				path := filepath.Join(root, ".assets/12345678/docs/1.9.0.zip")
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err = os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			c := &ProposalControls{Queue: q, Random: bytes.NewReader(make([]byte, 64))}
			revision := "main"
			changed := []string{}
			requests := []map[string]any{}
			repo := fakeRepo(nil)
			repo.main = func(repository.GitRepositoryPaths) (string, error) { return revision, nil }
			transaction := func(ctx context.Context, r repository.TransactionRequest, mutate repository.MutationCallback) (repository.TransactionResult, error) {
				requests = append(requests, map[string]any{"operation": map[string]any{"op_id": r.Operation.OpID, "principal": r.Operation.Principal, "idempotency_key": r.Operation.IdempotencyKey, "tool_name": r.Operation.ToolName, "request_json": r.Operation.RequestJSON, "client_instance_id": r.Operation.ClientInstanceID, "mcp_session_id": r.Operation.MCPSessionID, "source_chat": r.Operation.SourceChat}, "expected_revision": r.ExpectedRevision, "commit_message": r.CommitMessage, "author_name": r.AuthorName, "author_email": r.AuthorEmail})
				if tc.Scenario == "transaction-error" {
					return repository.TransactionResult{}, &Error{"conflict", "synthetic transaction failure"}
				}
				if tc.Scenario == "replay" {
					return repository.TransactionResult{Replayed: true, ResultRevision: "previous", Operation: control.OperationRecord{OpID: "original"}}, nil
				}
				if r.ExpectedRevision != revision {
					return repository.TransactionResult{}, &Error{"conflict", "synthetic expected revision mismatch"}
				}
				paths, err := mutate(ctx, root)
				if err != nil {
					return repository.TransactionResult{}, err
				}
				changed = paths
				revision = "result"
				return repository.TransactionResult{Operation: control.OperationRecord{OpID: r.Operation.OpID}, ResultRevision: revision, ChangedPaths: changed}, nil
			}
			client, session, chat := "client", "session", "chat"
			actor := ProposalActor{Policy: tc.Policy, ClientInstanceID: &client, MCPSessionID: &session, SourceChat: &chat}
			a := tc.Arguments
			if n, ok := a.Keep.(float64); ok {
				a.Keep = int(n)
			}
			for _, step := range tc.Steps {
				data, options, err := c.assetPrune(ctx, actor, a.ID, a.Kind, a.Keep, a.Expected, a.Key, transaction, defaultMutationIO())
				if step.Exception != nil {
					if err == nil {
						t.Fatal("missing exception")
					}
				} else {
					var result any
					if err != nil {
						result, err = FailureEnvelope(err)
					} else {
						result, err = q.successEnvelope(data, options, repo)
					}
					if err != nil || !reflect.DeepEqual(jsonNormal(result), step.Expected) {
						t.Fatal(result, step.Expected, err)
					}
				}
				remaining := []string{}
				if err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !entry.IsDir() {
						relative, _ := filepath.Rel(root, path)
						remaining = append(remaining, "/"+relative)
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				sort.Strings(remaining)
				if !reflect.DeepEqual(remaining, step.Remaining) || !reflect.DeepEqual(changed, step.Changed) || !reflect.DeepEqual(jsonNormal(requests), jsonNormal(step.Requests)) {
					t.Fatal(remaining, step.Remaining, changed, step.Changed, requests, step.Requests)
				}
			}
		})
	}
}
