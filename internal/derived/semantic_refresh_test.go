package derived

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type semanticClientStub struct {
	info   SemanticModelInfo
	vector []float32
	err    error
	text   string
}

func (s *semanticClientStub) ModelInfo() SemanticModelInfo { return s.info }
func (s *semanticClientStub) Embed(text string) ([]float32, error) {
	s.text = text
	return s.vector, s.err
}
func TestRefreshEmbeddingPaths(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	index := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite"), DeferEmbeddings: true}
	if err := index.Rebuild(ctx, root, "r1"); err != nil {
		t.Fatal(err)
	}
	pending, _ := index.PendingEmbeddingPaths(ctx, 10)
	client := &semanticChunkClientStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{3, 4}}}
	config := SemanticRefreshConfig{ModelID: "m", Dimensions: 2, MaxInputChars: 4096, MaxBatch: 16}
	if err := index.RefreshEmbeddingPaths(ctx, "r1", pending, config, client); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	var status string
	var blob []byte
	var norm float64
	if err := db.QueryRow("SELECT status,embedding_blob,embedding_norm FROM concept_embeddings").Scan(&status, &blob, &norm); err != nil || status != "ready" || len(blob) != 8 || norm != 5 {
		t.Fatal(status, blob, norm, err)
	}
	client.err = errors.New("failed")
	// An unchanged refresh failure preserves prior ready chunks. Change the item
	// first so this case exercises error publication instead.
	_, _ = db.Exec("UPDATE concepts SET body='changed input'")
	if err := index.RefreshEmbeddingPaths(ctx, "r2", pending, config, client); err == nil {
		t.Fatal("expected inference error")
	}
	var message string
	if err := db.QueryRow("SELECT status,error_message FROM concept_embeddings").Scan(&status, &message); err != nil || status != "error" || message != "failed" {
		t.Fatal(status, message, err)
	}
}
func TestRefreshEmbeddingSQLFailures(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"degraded", "ready", "advance"} {
		root := t.TempDir()
		installConcept(t, root)
		index := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite"), DeferEmbeddings: true}
		_ = index.Rebuild(ctx, root, "r")
		pending, _ := index.PendingEmbeddingPaths(ctx, 1)
		db, _ := sql.Open("sqlite", index.Path)
		if kind == "advance" {
			_, _ = db.Exec(`CREATE TRIGGER fail_advance BEFORE UPDATE ON index_state WHEN NEW.key='semantic_embedding_revision' BEGIN SELECT RAISE(ABORT,'advance');END`)
		} else {
			_, _ = db.Exec("DROP TABLE concept_embeddings")
		}
		_ = db.Close()
		client := &semanticChunkClientStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 1}}}
		if kind == "degraded" {
			client.err = errors.New("embed")
		}
		if err := index.RefreshEmbeddingPaths(ctx, "r", pending, SemanticRefreshConfig{ModelID: "m", Dimensions: 2, MaxBatch: 16}, client); err == nil {
			t.Fatal(kind)
		}
	}
}
func TestRefreshEmbeddingBatchIntegration(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	second := strings.Replace(testConcept, "id: 'id'", "id: 'id-2'", 1)
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(second), 0600); err != nil {
		t.Fatal(err)
	}
	index := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite"), DeferEmbeddings: true}
	if err := index.Rebuild(ctx, root, "r"); err != nil {
		t.Fatal(err)
	}
	paths, _ := index.PendingEmbeddingPaths(ctx, 10)
	client := &itemChunkBatch{semanticBatchStub: semanticBatchStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}}, batchVectors: [][]float32{{1, 0}}}}
	if err := index.RefreshEmbeddingPaths(ctx, "r", paths, SemanticRefreshConfig{ModelID: "m", Dimensions: 2, MaxInputChars: 4096, MaxBatch: 10}, client); err != nil {
		t.Fatal(err)
	}
	if len(client.batches) != 2 || len(client.batches[0]) != 1 || len(client.singles) != 0 {
		t.Fatal(client.batches, client.singles)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	var ready int
	if err := db.QueryRow("SELECT COUNT(*) FROM concept_embeddings WHERE status='ready'").Scan(&ready); err != nil || ready != 2 {
		t.Fatal(ready, err)
	}
}
func TestRefreshEmbeddingFailures(t *testing.T) {
	ctx := context.Background()
	index := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	root := t.TempDir()
	installConcept(t, root)
	_ = index.Rebuild(ctx, root, "r")
	config := SemanticRefreshConfig{ModelID: "m", Dimensions: 2, MaxInputChars: 2, MaxBatch: 16}
	if err := index.RefreshEmbeddingPaths(ctx, "r", nil, config, nil); err == nil {
		t.Fatal("nil")
	}
	client := &semanticClientStub{info: SemanticModelInfo{"bad", 2, "v"}, vector: []float32{1, 1}}
	if err := index.RefreshEmbeddingPaths(ctx, "r", nil, config, client); err == nil {
		t.Fatal("metadata")
	}
	client.info.ModelID = "m"
	for _, tc := range []struct {
		path   string
		vector []float32
	}{{"/missing.md", []float32{1, 1}}, {"", []float32{1}}, {"", []float32{0, 0}}, {"", []float32{float32(math.NaN()), 1}}} {
		client.vector = tc.vector
		paths := []string{tc.path}
		if tc.path == "" {
			pending, _ := index.PendingEmbeddingPaths(ctx, 1)
			paths = pending
		}
		if err := index.RefreshEmbeddingPaths(ctx, "r", paths, config, client); err == nil {
			t.Fatal(tc)
		}
	}
}
func TestEmbeddingTextAndVector(t *testing.T) {
	description := " desc "
	if got := embeddingText(" title ", &description, " body "); got != "title\n\ndesc\n\nbody" {
		t.Fatal(got)
	}
	if got := embeddingText(" ", nil, " "); got != "" {
		t.Fatal(got)
	}
	blob, norm, err := validateSemanticVector([]float32{3, 4}, 2)
	if err != nil || len(blob) != 8 || norm != 5 {
		t.Fatal(blob, norm, err)
	}
}

type itemChunkBatch struct{ semanticBatchStub }

func (c *itemChunkBatch) Chunk(text string, _, _, _ int) ([]string, error) {
	return []string{text}, nil
}

func TestRefreshRejectsLegacyClient(t *testing.T) {
	i := semanticSearchIndex(t)
	client := &semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 0}}
	if err := i.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("single-vector adapter accepted")
	}
}

func TestNonFiniteChunkVectorRejected(t *testing.T) {
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		if _, _, err := validateSemanticVector([]float32{value}, 1); err == nil {
			t.Fatal(value)
		}
	}
}
