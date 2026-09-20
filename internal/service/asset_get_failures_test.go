package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/repository"
)

func TestAssetGetFailurePaths(t *testing.T) {
	files, cases := assetGetFixture(t)
	c, _ := rebaseTest(t)
	root := t.TempDir()
	c.Queue.Paths.CurrentDir = root
	actor := ProposalActor{Policy: cases[0].Policy}
	o := assetOptions(cases[0].Arguments)
	open := func(path string) (assetSource, error) { return os.Open(path) }
	run := func(stat func(string) (fs.FileInfo, error), opener func(string) (assetSource, error), repo proposalRepository) error {
		_, _, err := c.assetGet(context.Background(), actor, o, stat, opener, repo)
		return err
	}
	for _, name := range []string{"metadata", "manifest", "archive", "stat", "open", "revision", "versions", "missing-after-validation", "read"} {
		t.Run(name, func(t *testing.T) {
			installAssetFiles(t, root, files)
			zipPath := filepath.Join(root, ".assets/12345678/docs/1.10.0.zip")
			metaPath := filepath.Join(root, ".assets/12345678/docs/1.10.0.json")
			stat, opener, repo := os.Stat, open, fakeRepo(nil)
			switch name {
			case "metadata":
				if err := os.WriteFile(metaPath, []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "manifest":
				var value map[string]any
				if err := json.Unmarshal(files["/.assets/12345678/docs/1.10.0.json"], &value); err != nil {
					t.Fatal(err)
				}
				value["manifest"] = nil
				raw, _ := json.Marshal(value)
				if err := os.WriteFile(metaPath, raw, 0600); err != nil {
					t.Fatal(err)
				}
			case "archive":
				if err := os.Remove(zipPath); err != nil {
					t.Fatal(err)
				}
			case "stat":
				stat = func(string) (fs.FileInfo, error) { return nil, io.ErrClosedPipe }
			case "open":
				opener = func(string) (assetSource, error) { return nil, io.ErrClosedPipe }
			case "revision":
				repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
			case "versions":
				stat = func(path string) (fs.FileInfo, error) {
					info, err := os.Stat(path)
					if failure := os.WriteFile(filepath.Join(root, ".assets/12345678/docs/bad.json"), nil, 0600); failure != nil {
						t.Fatal(failure)
					}
					return info, err
				}
				defer os.Remove(filepath.Join(root, ".assets/12345678/docs/bad.json"))
			case "missing-after-validation":
				stat = func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }
			case "read":
				opener = func(string) (assetSource, error) { return brokenAssetSource{}, nil }
			}
			if err := run(stat, opener, repo); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
	installAssetFiles(t, root, files)
	// Cancellation is checked after acquiring the source-compatible blocking
	// lock; it must leave the lock available for the next read.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := c.AssetGet(ctx, actor, o)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// A valid decimal digest stored as a JSON integer follows Python str().
	meta := filepath.Join(root, ".assets/12345678/docs/1.10.0.json")
	var value map[string]any
	_ = json.Unmarshal(files["/.assets/12345678/docs/1.10.0.json"], &value)
	digest := "1234567890123456789012345678901234567890123456789012345678901234"
	value["zip_sha256"] = json.Number(digest)
	value["manifest"].(map[string]any)["sha256"] = digest
	raw, _ := json.Marshal(value)
	if err = os.WriteFile(meta, raw, 0600); err != nil {
		t.Fatal(err)
	}
	o.View = "manifest"
	if err = run(os.Stat, open, fakeRepo(nil)); err != nil {
		t.Fatal(err)
	}
	// Larger archives omit inline manifest and choose the default bounded slice.
	blob := make([]byte, assets.MaxInlineArchiveBytes+1)
	file := filepath.Join(root, ".assets/12345678/docs/1.10.0.zip")
	if err = os.WriteFile(file, blob, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(blob)
	digest = fmt.Sprintf("%x", sum)
	value["zip_sha256"] = digest
	value["manifest"].(map[string]any)["sha256"] = digest
	raw, _ = json.Marshal(value)
	if err = os.WriteFile(meta, raw, 0600); err != nil {
		t.Fatal(err)
	}
	o.View = "archive"
	data, _, err := c.assetGet(context.Background(), actor, o, os.Stat, open, fakeRepo(nil))
	if err != nil || data["manifest"] != nil {
		t.Fatal(data, err)
	}
}

type brokenAssetSource struct{}

func (brokenAssetSource) Read([]byte) (int, error)          { return 0, io.ErrClosedPipe }
func (brokenAssetSource) ReadAt([]byte, int64) (int, error) { return 0, io.ErrClosedPipe }
func (brokenAssetSource) Close() error                      { return nil }
