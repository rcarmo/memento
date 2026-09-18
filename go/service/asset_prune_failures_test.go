package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/repository"
)

func TestAssetPruneFailurePaths(t *testing.T) {
	for _, scenario := range []string{"read", "asset-query", "proposal-get", "json", "random", "keep-type"} {
		t.Run(scenario, func(t *testing.T) {
			c, actor, _ := realApplyTest(t)
			files, _ := assetGetFixture(t)
			installAssetFiles(t, c.Queue.Paths.CurrentDir, files)
			expected := "main"
			var keep any = 1
			switch scenario {
			case "read":
				if err := os.WriteFile(filepath.Join(c.Queue.Paths.CurrentDir, "a.md"), []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			case "asset-query":
				c.Queue.Proposals.DB.Close()
			case "proposal-get":
				if _, err := c.Queue.Proposals.DB.Exec("PRAGMA foreign_keys=OFF"); err != nil {
					t.Fatal(err)
				}
				if _, err := c.Queue.Proposals.DB.Exec("INSERT INTO proposal_assets VALUES('missing','asset','/a.md','docs','1.9.0','application/zip','digest',X'00','{}','created')"); err != nil {
					t.Fatal(err)
				}
			case "json":
				expected = string([]byte{0xff})
			case "random":
				c.Random = bytes.NewReader(nil)
			case "keep-type":
				keep = "invalid"
			}
			_, _, err := c.assetPrune(context.Background(), actor, "/a.md", "docs", keep, expected, "key", func(context.Context, repository.TransactionRequest, repository.MutationCallback) (repository.TransactionResult, error) {
				t.Error("unexpected transaction")
				return repository.TransactionResult{}, nil
			}, defaultMutationIO())
			if err == nil {
				t.Fatal("expected failure")
			}
		})
	}
	root := t.TempDir()
	ops := defaultMutationIO()
	if _, err := pruneAssetFiles(root, "bad", "docs", []string{"1.0.0"}, ops); err == nil {
		t.Fatal("invalid asset identity")
	}
	ops.stat = func(string) (fs.FileInfo, error) { return nil, io.ErrClosedPipe }
	if _, err := pruneAssetFiles(root, "12345678", "docs", []string{"1.0.0"}, ops); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	files, _ := assetGetFixture(t)
	installAssetFiles(t, root, files)
	ops = defaultMutationIO()
	ops.remove = func(string, string) error { return io.ErrClosedPipe }
	if _, err := pruneAssetFiles(root, "12345678", "docs", []string{"1.9.0"}, ops); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	// Missing worktree members are skipped, not reported as changed.
	got, err := pruneAssetFiles(t.TempDir(), "12345678", "docs", []string{"1.9.0"}, defaultMutationIO())
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	// Public lock path must release even after caller cancellation.
	c, actor, base := realApplyTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := c.AssetPrune(ctx, actor, "/a.md", "docs", 1, base, "key"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, options, err := c.AssetPrune(context.Background(), actor, "/a.md", "docs", 1, "stale", "key"); err != nil || options.OperationID != nil {
		t.Fatal(options, err)
	}
}
