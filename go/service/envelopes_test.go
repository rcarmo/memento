package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

func TestServiceEnvelopeReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/service-envelopes.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Failures []struct {
			Name, Propagated string
			Expected         map[string]any
		}
		Keys []struct {
			Key      string
			Expected map[string]any
		}
		Successes []struct {
			Options struct {
				RepoRevision  *string `json:"repo_revision"`
				IndexRevision *string `json:"index_revision"`
				IndexStale    bool    `json:"index_stale"`
				OperationID   *string `json:"operation_id"`
				Warnings      []string
				NextTools     []string `json:"next_tools"`
			}
			Expected map[string]any
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	failures := map[string]error{"auth": &access.AuthorizationError{Message: "denied"}, "forbidden": &Error{"forbidden", "denied"}, "not-found": &Error{"not_found", "missing"}, "conflict": &Error{"conflict", "conflict"}, "needs-rebase": &Error{"needs_rebase", "stale"}, "service": &Error{"validation_error", "bad request"}, "bundle": &repository.BundleError{Message: "bundle"}, "frontmatter": &repository.FrontmatterError{Message: "frontmatter"}, "path": &repository.PathSafetyError{Message: "path"}, "asset": &assets.ReadError{Message: "asset"}, "staged": &assets.StagedAssetError{Message: "staged"}, "pack": &assets.ValidationError{Message: "pack"}, "search": &derived.SearchError{Message: "query"}, "idempotency": &control.IdempotencyConflictError{}, "transaction": &repository.TransactionConflictError{Message: "transaction"}, "git": &repository.GitError{Message: "git"}, "unavailable": &derived.UnavailableError{Message: "busy"}, "file": &assets.FileNotDeclaredError{}, "asset-id": &control.ProposalNotFoundError{ProposalID: "proposal", AssetID: "asset"}, "os": io.ErrClosedPipe, "runtime": errors.New("unexpected"), "corruption": &derived.CorruptionError{Message: "corrupt"}, "timeout": context.DeadlineExceeded}
	for _, c := range fixture.Failures {
		got, err := FailureEnvelope(failures[c.Name])
		if c.Propagated != "" {
			if err != failures[c.Name] {
				t.Fatal(c.Name, got, err)
			}
		} else if err != nil || !reflect.DeepEqual(jsonNormal(got), c.Expected) {
			t.Fatal(c.Name, got, c.Expected, err)
		}
	}
	for _, c := range fixture.Keys {
		for _, failure := range []error{&control.ProposalNotFoundError{ProposalID: c.Key}, &control.OperationNotFoundError{OpID: c.Key}, &derived.ConceptNotFoundError{ConceptID: c.Key}} {
			got, err := FailureEnvelope(failure)
			if err != nil || !reflect.DeepEqual(jsonNormal(got), c.Expected) {
				t.Fatal(c.Key, got, c.Expected, err)
			}
		}
	}
	for _, c := range fixture.Successes {
		o := c.Options
		got, err := (ProposalQueue{}).successEnvelope(map[string]any{"items": []any{}, "nothing": nil, "large": json.Number("9007199254740993")}, SuccessOptions{RepoRevision: o.RepoRevision, IndexRevision: o.IndexRevision, IndexStale: o.IndexStale, OperationID: o.OperationID, Warnings: o.Warnings, NextTools: o.NextTools}, fakeRepo(nil))
		if err != nil || !reflect.DeepEqual(jsonNormal(got), c.Expected) {
			t.Fatal(got, c.Expected, err)
		}
		value, err := MCPEnvelope(got)
		if err != nil {
			t.Fatal(err)
		}
		result, err := umcp.FormatToolResult(value, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := result["structuredContent"]; !ok {
			t.Fatal("lost structured envelope")
		}
		text := result["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(text, "9007199254740993") {
			t.Fatal("large integer rounded", text)
		}
	}
}
func TestEnvelopeErrorsAndLiveRevision(t *testing.T) {
	repo := fakeRepo(nil)
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, err := (ProposalQueue{}).successEnvelope(nil, SuccessOptions{}, repo); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	root := t.TempDir()
	q := ProposalQueue{Paths: repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}}
	boot, err := repository.BootstrapRepository(context.Background(), q.Paths, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.SuccessEnvelope(nil, SuccessOptions{})
	if err != nil || got.RepoRevision != boot.Revision {
		t.Fatal(got, err)
	}
	if _, err := FailureEnvelope(&ChangeValidationError{Message: ""}); err == nil {
		t.Fatal("invalid empty failure")
	}
	var bad any
	syntax := json.Unmarshal([]byte("{"), &bad)
	mapped, err := FailureEnvelope(syntax)
	if err != nil || mapped.ErrorClass != "validation_error" {
		t.Fatal(mapped, err)
	}
	mapped, err = FailureEnvelope(&ChangeValidationError{"bad"})
	if err != nil || mapped.Message != "bad" {
		t.Fatal(mapped, err)
	}
	for _, value := range []any{math.NaN(), make(chan int)} {
		if _, err := MCPEnvelope(value); err == nil {
			t.Fatal("unsupported value")
		}
	}
}
