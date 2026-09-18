package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/repository"
)

func TestTrashMutationFailures(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	if _, _, err := c.TrashMutation(ctx, actor, "bad", "/a.md", base, "key", true); err == nil {
		t.Fatal("invalid action")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := c.TrashMutation(canceled, actor, "trash", "/a.md", base, "key", true); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	never := func(context.Context, repository.TransactionRequest, repository.MutationCallback) (repository.TransactionResult, error) {
		t.Error("unexpected transaction")
		return repository.TransactionResult{}, nil
	}
	if _, _, err := c.trashMutation(ctx, actor, "trash", "/a.md", string([]byte{255}), "key", true, fakeRepo(nil), never, defaultMutationIO()); err == nil {
		t.Fatal("JSON")
	}
	c.Random = bytes.NewReader(nil)
	if _, _, err := c.trashMutation(ctx, actor, "trash", "/a.md", base, "key", true, fakeRepo(nil), never, defaultMutationIO()); err == nil {
		t.Fatal("random")
	}
	for _, scenario := range []string{"source-write", "target-write", "mkdir", "rename", "kinds", "versions", "asset-write", "asset-remove", "source-remove"} {
		t.Run(scenario, func(t *testing.T) {
			files, _ := assetGetFixture(t)
			root := t.TempDir()
			installAssetFiles(t, root, files)
			ops := defaultMutationIO()
			action, path, destination := "trash", "/public/a.md", "/trash/public/a.md"
			if scenario == "source-write" {
				ops.read = func(root, path string) (repository.BundleEntry, error) {
					return repository.ReadBundleEntry(root, "/public/a.md")
				}
				path = "/index.md"
			}
			if scenario == "target-write" {
				destination = "/index.md"
			}
			if scenario == "mkdir" {
				ops.mkdir = func(string, string) error { return io.ErrClosedPipe }
			}
			if scenario == "rename" {
				ops.rename = func(string, string, string) error { return io.ErrClosedPipe }
			}
			if scenario == "kinds" || scenario == "versions" || scenario == "asset-write" || scenario == "asset-remove" || scenario == "source-remove" {
				action = "purge"
			}
			switch scenario {
			case "kinds":
				if err := os.Mkdir(filepath.Join(root, ".assets/12345678/INVALID"), 0700); err != nil {
					t.Fatal(err)
				}
			case "versions":
				if err := os.WriteFile(filepath.Join(root, ".assets/12345678/docs/bad.json"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "asset-write":
				target := filepath.Join(root, ".assets/12345678/docs/1.9.0.zip")
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("/outside", target); err != nil {
					t.Fatal(err)
				}
			case "asset-remove":
				ops.remove = func(string, string) error { return io.ErrClosedPipe }
			case "source-remove":
				original := ops.remove
				ops.remove = func(root, path string) error {
					if path == "/public/a.md" {
						return io.ErrClosedPipe
					}
					return original(root, path)
				}
			}
			if _, err := mutateTrash(root, action, path, destination, ops); err == nil {
				t.Fatal("expected failure")
			}
			if scenario == "source-remove" {
				if _, err := os.Stat(filepath.Join(root, "public/a.md")); err != nil {
					t.Fatal("source must survive", err)
				}
				if _, err := os.Stat(filepath.Join(root, ".assets/12345678/docs/1.10.0.zip")); !os.IsNotExist(err) {
					t.Fatal("asset partial deletion", err)
				}
			}
		})
	}
}
