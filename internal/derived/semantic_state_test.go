package derived

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddingRevision(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	index := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	if err := index.Rebuild(ctx, root, "r"); err != nil {
		t.Fatal(err)
	}
	revision, err := index.EmbeddingRevision(ctx)
	if err != nil || revision != "disabled" {
		t.Fatal(revision, err)
	}
	index.Path = filepath.Join(t.TempDir(), "invalid.sqlite")
	if err = os.WriteFile(index.Path, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = index.EmbeddingRevision(ctx); err == nil {
		t.Fatal("invalid")
	}
}
