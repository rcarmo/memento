package derived

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/repository"
)

func TestAssetLinkLoadingFailures(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".assets/12345678/docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".assets/12345678/docs/1.0.0.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	bundle := repository.RepositoryBundle{Entries: []repository.BundleEntry{{Document: repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{ID: "12345678"}}}}}
	if _, err := assetPathsForBundle(root, bundle); err == nil {
		t.Fatal("bad manifest")
	}
	text := "---\nid: '12345678'\ntype: concept\ntitle: Link\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: test\n---\n[asset](a.txt)\n"
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	store := testStore(t)
	if err := store.Rebuild(context.Background(), root, "r"); err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatal(err)
	}
	if _, err := store.DB.Exec("DROP TABLE concepts;CREATE TABLE concepts(id TEXT);INSERT INTO concepts VALUES(NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := assetPathsForConceptRows(context.Background(), root, store.DB); err == nil {
		t.Fatal("invalid concept row")
	}
}
