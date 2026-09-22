package derived

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type itemChunkClient struct{ semanticClientStub }

func (c *itemChunkClient) Chunk(text string, tokens, overlap, chars int) ([]string, error) {
	return []string{text}, nil
}

type itemVector struct {
	Hash, Revision, Updated, Model, Status, Path string
	Blob                                         []byte
	Chunks                                       int
}

func readItemVector(t *testing.T, index *Index, id string) itemVector {
	t.Helper()
	db, err := sql.Open("sqlite", index.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v itemVector
	if err = db.QueryRow("SELECT embedding_text_hash,embedding_revision,updated_at,model_revision,status,path,embedding_blob FROM concept_embeddings WHERE concept_id=?", id).Scan(&v.Hash, &v.Revision, &v.Updated, &v.Model, &v.Status, &v.Path, &v.Blob); err != nil {
		t.Fatal(err)
	}
	exists, err := chunkTableExists(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		if err = db.QueryRow("SELECT COUNT(*) FROM concept_embedding_chunks WHERE concept_id=?", id).Scan(&v.Chunks); err != nil {
			t.Fatal(err)
		}
	}
	return v
}
func TestItemRelativeEmbeddings(t *testing.T) {
	t.Run("chunks", func(t *testing.T) {
		ctx := t.Context()
		root := t.TempDir()
		installConcept(t, root)
		initial := filepath.Join(root, "a.md")
		if _, err := os.Stat(initial); err != nil {
			t.Fatal(err)
		}
		second := strings.Replace(testConcept, "id: 'id'", "id: 'peer'", 1)
		if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(second), 0600); err != nil {
			t.Fatal(err)
		}
		index := &Index{Path: filepath.Join(t.TempDir(), "derived.sqlite"), DeferEmbeddings: true}
		if err := index.Rebuild(ctx, root, "r1"); err != nil {
			t.Fatal(err)
		}
		client := &itemChunkClient{semanticClientStub{info: SemanticModelInfo{"m", 2, "v1"}, vector: []float32{1, 1}}}
		config := SemanticRefreshConfig{ModelID: "m", Dimensions: 2, MaxBatch: 1, MaxInputChars: 4096}
		if err := index.RefreshEmbeddingPaths(ctx, "r1", []string{"/a.md", "/b.md"}, config, client); err != nil {
			t.Fatal(err)
		}
		before := readItemVector(t, index, "id")
		peer := readItemVector(t, index, "peer")
		// An unrelated item edit must retain the original blob, timestamp and provenance.
		if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(second+"\nchanged peer input\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := index.UpdatePaths(ctx, root, "r2", []string{"/b.md"}); err != nil {
			t.Fatal(err)
		}
		if got := readItemVector(t, index, "id"); !reflect.DeepEqual(before, got) {
			t.Fatal("unrelated edit invalidated item", before, got)
		}
		if got := readItemVector(t, index, "peer"); got.Status != "stale" || got.Revision != peer.Revision {
			t.Fatal("changed peer not invalidated", got)
		}
		pending, err := index.PendingEmbeddingPaths(ctx, 10)
		if err != nil || !reflect.DeepEqual(pending, []string{"/b.md"}) {
			t.Fatal(pending, err)
		}
		if err = index.RefreshEmbeddingPaths(ctx, "r2", pending, config, client); err != nil {
			t.Fatal(err)
		}
		if got := readItemVector(t, index, "id"); !reflect.DeepEqual(before, got) {
			t.Fatal("peer refresh rewrote item", got)
		}
		// Metadata outside the embedding input and stable-ID renames need no inference.
		metadata := strings.Replace(testConcept, "status: active", "status: active\ntags: [metadata-only]", 1)
		if err = os.WriteFile(initial, []byte(metadata), 0600); err != nil {
			t.Fatal(err)
		}
		if err = index.UpdatePaths(ctx, root, "r3", []string{"/a.md"}); err != nil {
			t.Fatal(err)
		}
		if got := readItemVector(t, index, "id"); !reflect.DeepEqual(before, got) {
			t.Fatal("metadata invalidated item", got)
		}
		if err = os.Rename(initial, filepath.Join(root, "z.md")); err != nil {
			t.Fatal(err)
		}
		if err = index.UpdatePaths(ctx, root, "r4", []string{"/a.md", "/z.md"}); err != nil {
			t.Fatal(err)
		}
		before.Path = "/z.md"
		if got := readItemVector(t, index, "id"); !reflect.DeepEqual(before, got) {
			t.Fatal("rename invalidated item", got)
		}
		if err = index.Rebuild(ctx, root, "r5"); err != nil {
			t.Fatal(err)
		}
		if got := readItemVector(t, index, "id"); !reflect.DeepEqual(before, got) {
			t.Fatal("rebuild rewrote provenance", got)
		}
		pending, err = index.PendingEmbeddingPaths(ctx, 10)
		if err != nil || len(pending) != 0 {
			t.Fatal(pending, err)
		}
		if err = os.Remove(filepath.Join(root, "z.md")); err != nil {
			t.Fatal(err)
		}
		if err = index.UpdatePaths(ctx, root, "r6", []string{"/z.md"}); err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite", index.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var count int
		if err = db.QueryRow("SELECT COUNT(*) FROM concept_embeddings WHERE concept_id='id'").Scan(&count); err != nil || count != 0 {
			t.Fatal(count, err)
		}
		if err = db.QueryRow("SELECT COUNT(*) FROM concept_embedding_chunks WHERE concept_id='id'").Scan(&count); err != nil || count != 0 {
			t.Fatal(count, err)
		}
	})
}

func TestItemUpdateFailureIsAtomic(t *testing.T) {
	index, client, _ := chunkFixture(t)
	root := t.TempDir()
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	before := readItemVector(t, index, "id")
	db, err := sql.Open("sqlite", index.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	snapshotBefore := normal(snapshot(t, db))
	if err = os.WriteFile(filepath.Join(root, "a.md"), []byte(testConcept+"\nchanged input"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "z-bad.md"), []byte("not a concept"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = index.UpdatePaths(t.Context(), root, "bad-revision", []string{"/a.md", "/z-bad.md"}); err == nil {
		t.Fatal("accepted invalid document")
	}
	if got := normal(snapshot(t, db)); !reflect.DeepEqual(snapshotBefore, got) {
		t.Fatal("failed update partially committed")
	}
	if got := readItemVector(t, index, "id"); !reflect.DeepEqual(before, got) {
		t.Fatal(before, got)
	}
}
