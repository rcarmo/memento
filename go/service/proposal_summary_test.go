package service

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/internal/pyjson"
	"github.com/rcarmo/memento/go/repository"
)

func TestProposalSummaryReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/proposal-summary.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario  string
		Include   bool `json:"include_conflicts"`
		Record    map[string]any
		Assets    []map[string]any
		Expected  map[string]any
		ErrorType string `json:"error_type"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		recordBytes, err := json.Marshal(c.Record)
		if err != nil {
			t.Fatal(err)
		}
		var record control.ProposalRecord
		if err = json.Unmarshal(recordBytes, &record); err != nil {
			t.Fatal(err)
		}
		record.PatchJSON, err = pyjson.Dumps(c.Record["patch"])
		if err != nil {
			t.Fatal(err)
		}
		assets := []control.ProposalAssetRecord{}
		for _, raw := range c.Assets {
			data, _ := json.Marshal(raw)
			var asset control.ProposalAssetRecord
			_ = json.Unmarshal(data, &asset)
			asset.ManifestJSON, err = pyjson.Dumps(raw["manifest"])
			if err != nil {
				t.Fatal(err)
			}
			assets = append(assets, asset)
		}
		repo := fakeRepo([]string{"/a.md"})
		repo.read = func(string, string) (repository.BundleEntry, error) {
			switch c.Scenario {
			case "missing-body":
				return repository.BundleEntry{}, os.ErrNotExist
			case "bad-body":
				return repository.BundleEntry{}, &repository.FrontmatterError{Message: "invalid"}
			}
			return repository.BundleEntry{Document: repository.ConceptDocument{Body: "current \n", Frontmatter: repository.ConceptFrontmatter{ID: "12345678"}}}, nil
		}
		if c.Scenario == "missing-base" {
			repo.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
				return nil, &repository.GitError{Message: "missing base"}
			}
		}
		got, err := (ProposalQueue{}).summary(context.Background(), record, assets, c.Include, repo)
		if c.ErrorType != "" {
			if err == nil {
				t.Fatal(c.Scenario, "expected error")
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(jsonNormal(got), c.Expected) {
			t.Fatal(c.Scenario, c.Include, got, c.Expected, err)
		}
		if _, err = pyjson.Dumps(got); err != nil {
			t.Fatal("summary not journal-safe", err)
		}
	}
}

func TestProposalSummaryFailuresAndPublic(t *testing.T) {
	ctx := context.Background()
	q, r := queueTest(t)
	repo := fakeRepo(nil)
	for _, raw := range []string{"{", `{"changes":null}`} {
		r.PatchJSON = raw
		if _, err := q.summary(ctx, r, nil, false, repo); err == nil {
			t.Fatal(raw)
		}
	}
	r.PatchJSON = `{}`
	if _, err := q.summary(ctx, r, nil, false, repo); err != nil {
		t.Fatal(err)
	}
	assets := []control.ProposalAssetRecord{{ProposalAssetInput: control.ProposalAssetInput{ConceptPath: "/a.md", ManifestJSON: "{"}}}
	if _, err := q.summary(ctx, r, assets, false, repo); err == nil {
		t.Fatal("bad manifest")
	}
	assets[0].ManifestJSON = `{"entries":null}`
	repo.read = func(string, string) (repository.BundleEntry, error) { return repository.BundleEntry{}, nil }
	if _, err := q.summary(ctx, r, assets, false, repo); err == nil {
		t.Fatal("null entries")
	}
	r.PatchJSON = `{"changes":[null]}`
	assets[0].ManifestJSON = `{}`
	if _, err := q.summary(ctx, r, assets, false, repo); err == nil {
		t.Fatal("malformed change")
	}
	r.PatchJSON = `{}`
	repo.read = func(string, string) (repository.BundleEntry, error) {
		return repository.BundleEntry{}, io.ErrClosedPipe
	}
	if _, err := q.summary(ctx, r, assets, false, repo); err == nil {
		t.Fatal("read failure")
	}
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, err := q.summary(ctx, r, nil, false, repo); err == nil {
		t.Fatal("main failure")
	}
	root := t.TempDir()
	q.Paths = repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}
	boot, err := repository.BootstrapRepository(ctx, q.Paths, "")
	if err != nil {
		t.Fatal(err)
	}
	r.BaseRevision = boot.Revision
	if _, err = q.Summary(ctx, r, true); err != nil {
		t.Fatal(err)
	}
	q.Proposals.DB.Close()
	if _, err = q.Summary(ctx, r, false); err == nil {
		t.Fatal("closed DB")
	}
}
