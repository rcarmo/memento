package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func mutationSnapshot(t *testing.T, root string) (map[string]string, []string) {
	t.Helper()
	files := map[string]string{}
	dirs := []string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = "/" + filepath.ToSlash(relative)
		if entry.IsDir() {
			dirs = append(dirs, relative)
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[relative] = string(data)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files, dirs
}
func installMutationFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, text := range files {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
func TestWorktreeMutationReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/worktree-mutations.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Kind       string
		Initial, Files       map[string]string
		Directories, Changed []string
		Change               map[string]any
		Limit                int
		Policy               access.EffectivePolicy
		ErrorType            string `json:"error_type"`
		Error                string
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			root := t.TempDir()
			installMutationFiles(t, root, c.Initial)
			m := WorktreeMutator{MaxConceptBytes: c.Limit, Now: func() time.Time { return time.Date(2026, 9, 18, 5, 0, 0, 123456000, time.UTC) }, Random: bytes.NewReader(make([]byte, 16))}
			changes, err := NormalizeProposalChanges([]any{c.Change})
			if err != nil {
				t.Fatal(err)
			}
			change := changes[0]
			var changed []string
			switch c.Kind {
			case "create":
				err = m.Create(root, change, "actor")
				changed = []string{change["path"].(string)}
			case "patch":
				err = m.Patch(root, change, "actor")
				changed = []string{change["path"].(string)}
			case "rename":
				changed, err = m.Rename(root, change, "actor", c.Policy)
			case "trash":
				changed, err = m.Trash(root, change, c.Policy)
			}
			if c.ErrorType != "" {
				if err == nil {
					t.Fatal("missing error", c.ErrorType)
				}
				if c.ErrorType == "ConflictError" || c.ErrorType == "ServiceError" || c.ErrorType == "AuthorizationError" || c.ErrorType == "NotFoundError" {
					if err.Error() != c.Error {
						t.Fatal(err, c.Error)
					}
				}
			} else if err != nil || !reflect.DeepEqual(changed, c.Changed) {
				t.Fatal(changed, c.Changed, err)
			}
			files, dirs := mutationSnapshot(t, root)
			if !reflect.DeepEqual(files, c.Files) || !reflect.DeepEqual(dirs, c.Directories) {
				t.Fatal("state mismatch", files, c.Files, dirs, c.Directories)
			}
		})
	}
}

func TestWorktreeMutationsThroughGitTransaction(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	seed := t.TempDir()
	installMutationFiles(t, seed, map[string]string{"/public/a.md": mutationConcept, "/public/ref.md": mutationConcept})
	paths := repository.GitRepositoryPaths{BareDir: filepath.Join(root, "repo.git"), CurrentDir: filepath.Join(root, "current"), WorktreesDir: filepath.Join(root, "worktrees")}
	boot, err := repository.BootstrapRepository(ctx, paths, seed)
	if err != nil {
		t.Fatal(err)
	}
	db, err := control.Connect(ctx, filepath.Join(root, "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = control.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	manager := repository.TransactionManager{Paths: paths, Operations: control.Operations{DB: db}}
	request := repository.TransactionRequest{Operation: control.OperationRequest{OpID: "mutation", Principal: "actor", IdempotencyKey: "key", ToolName: "synthetic", RequestJSON: "{}"}, ExpectedRevision: boot.Revision, CommitMessage: "synthetic mutations", AuthorName: "Synthetic Actor", AuthorEmail: "actor@example.invalid"}
	mutator := WorktreeMutator{MaxConceptBytes: 65536, Proposals: control.Proposals{DB: db}}
	create := normalizedMutation(t, map[string]any{"kind": "create", "path": "/public/new.md", "concept_type": "concept", "title": "New", "body": "new"})
	patch := normalizedMutation(t, map[string]any{"kind": "patch", "path": "/public/new.md", "body": "changed"})
	rename := normalizedMutation(t, map[string]any{"kind": "rename", "path": "/public/a.md", "new_path": "/public/moved.md"})
	trash := normalizedMutation(t, map[string]any{"kind": "trash", "path": "/public/ref.md"})
	manifest := map[string]any{"entries": []any{}, "sha256": strings.Repeat("a", 64), "total_uncompressed_bytes": json.Number("0"), "file_count": json.Number("0")}
	if _, err = mutator.Proposals.Create(ctx, control.ProposalRequest{ProposalID: "proposal", AuthorPrincipal: "actor", BaseRevision: boot.Revision, Intent: "synthetic", Patch: map[string]any{}, Assets: []control.ProposalAssetInput{{AssetID: "asset", ConceptPath: "/public/new.md", AssetKind: "docs", Version: "1.0.0", MediaType: "application/zip", SHA256: strings.Repeat("a", 64), BlobBytes: []byte{0, 1, 255}, ManifestJSON: "{}"}}}); err != nil {
		t.Fatal(err)
	}
	asset := normalizedMutation(t, map[string]any{"kind": "attach_asset_pack", "path": "/public/new.md", "asset_kind": "docs", "version": "1.0.0", "asset_id": "asset", "zip_sha256": strings.Repeat("a", 64), "manifest": manifest})
	proposalID := "proposal"
	mutate := func(ctx context.Context, worktree string) ([]string, error) {
		return mutator.ApplyChanges(ctx, worktree, []ProposalChange{create, patch, asset, rename, trash}, "actor", archivePolicy(), &proposalID)
	}
	result, err := manager.Apply(ctx, request, mutate)
	if err != nil || result.Replayed {
		t.Fatal(result, err)
	}
	if result.ResultRevision == boot.Revision {
		t.Fatal("no publication")
	}
	entry, err := repository.ReadBundleEntry(paths.CurrentDir, "/public/new.md")
	if err != nil || entry.Document.Body != "changed" {
		t.Fatal(entry, err)
	}
	zip, err := os.ReadFile(filepath.Join(paths.CurrentDir, ".assets", entry.Document.Frontmatter.ID, "docs", "1.0.0.zip"))
	if err != nil || !bytes.Equal(zip, []byte{0, 1, 255}) {
		t.Fatal(zip, err)
	}
	ref, err := repository.ReadBundleEntry(paths.CurrentDir, "/trash/public/ref.md")
	if err != nil || !strings.Contains(ref.Document.Body, "/public/moved.md") {
		t.Fatal(ref, err)
	}
	replay, err := manager.Apply(ctx, request, func(context.Context, string) ([]string, error) { t.Fatal("replayed mutation"); return nil, nil })
	if err != nil || !replay.Replayed || replay.ResultRevision != result.ResultRevision {
		t.Fatal(replay, err)
	}
	// Partial worktree writes from a failing mutation must never reach main.
	request.Operation.OpID = "failed"
	request.Operation.IdempotencyKey = "failed"
	request.ExpectedRevision = result.ResultRevision
	create["path"] = "/public/failure.md"
	patch["path"] = "/public/failure.md"
	_, err = manager.Apply(ctx, request, func(_ context.Context, worktree string) ([]string, error) {
		if err := mutator.Create(worktree, create, "actor"); err != nil {
			return nil, err
		}
		mutator.MaxConceptBytes = 1
		return nil, mutator.Patch(worktree, patch, "actor")
	})
	if err == nil {
		t.Fatal("oversize mutation accepted")
	}
	current, err := repository.GetMainRevision(paths)
	if err != nil || current != result.ResultRevision {
		t.Fatal(current, err)
	}
	if _, err = os.Stat(filepath.Join(paths.CurrentDir, "public/failure.md")); !os.IsNotExist(err) {
		t.Fatal("partial mutation published", err)
	}
}
