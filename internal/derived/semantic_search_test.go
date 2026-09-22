package derived

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
)

func semanticBlob(values ...float32) []byte {
	raw := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
	}
	return raw
}

type semanticChunkClientStub struct {
	semanticClientStub
	chunks   []string
	chunkErr error
	empty    bool
}

func (s *semanticChunkClientStub) Chunk(text string, tokens, overlap, chars int) ([]string, error) {
	if s.chunkErr != nil {
		return nil, s.chunkErr
	}
	if s.empty {
		return nil, nil
	}
	if len(s.chunks) != 0 {
		return append([]string{}, s.chunks...), nil
	}
	return []string{text}, nil
}

func semanticSearchBase(t *testing.T) *Index {
	t.Helper()
	root := t.TempDir()
	first := strings.Replace(testConcept, "/a.md", "/a.md", 1)
	second := strings.Replace(strings.Replace(testConcept, "id: 'id'", "id: 'id-2'", 1), "/a.md", "/b.md", 1)
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte(first), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(second), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "index.sqlite")
	index := &Index{Path: path, DeferEmbeddings: true}
	if err := index.Rebuild(context.Background(), root, "main"); err != nil {
		t.Fatal(err)
	}
	index.ConfigureChunkModel(SemanticModelInfo{"m", 2, "v"})
	return index
}

func installSemanticSearchEmbeddings(t *testing.T, index *Index, chunked bool) {
	t.Helper()
	db, err := sql.Open("sqlite", index.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if chunked {
		if _, err = db.Exec(chunkSchema); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec("DELETE FROM concept_embedding_chunks; DELETE FROM concept_embedding_policy"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision';DELETE FROM concept_embeddings;INSERT INTO concept_embeddings(concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,embedding_blob,embedding_norm,updated_at,error_message) SELECT id,path,'h','m',2,'main','ready','v',?,1,'now',NULL FROM concepts WHERE path='/a.md';INSERT INTO concept_embeddings(concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,embedding_blob,embedding_norm,updated_at,error_message) SELECT id,path,'h','m',2,'main','ready','v',?,1,'now',NULL FROM concepts WHERE path='/b.md'`, semanticBlob(1, 0), semanticBlob(0, 1)); err != nil {
		t.Fatal(err)
	}
	if !chunked {
		return
	}
	policy := ChunkPolicy(SemanticModelInfo{"m", 2, "v"}, 4096)
	for _, item := range []struct {
		path   string
		vector []byte
	}{{"/a.md", semanticBlob(1, 0)}, {"/b.md", semanticBlob(0, 1)}} {
		if _, err = db.Exec("INSERT INTO concept_embedding_chunks SELECT id,0,'h','v','Title',?,1 FROM concepts WHERE path=?", item.vector, item.path); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec("INSERT INTO concept_embedding_policy SELECT id,? FROM concepts WHERE path=?", policy, item.path); err != nil {
			t.Fatal(err)
		}
	}
}

func semanticSearchIndex(t *testing.T) *Index {
	t.Helper()
	index := semanticSearchBase(t)
	installSemanticSearchEmbeddings(t, index, true)
	return index
}

func legacySemanticSearchIndex(t *testing.T) *Index {
	t.Helper()
	index := semanticSearchBase(t)
	installSemanticSearchEmbeddings(t, index, false)
	return index
}

func TestSearchSemantic(t *testing.T) {
	ctx := context.Background()
	index := semanticSearchIndex(t)
	client := &semanticChunkClientStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 0}}}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	options := SemanticSearchOptions{SearchOptions: SearchOptions{Query: "Title", Syntax: "plain", Limit: 1}, MaxCandidates: 10}
	page, err := index.SearchSemantic(ctx, policy, options, client)
	if err != nil || len(page.Results) != 1 || page.Results[0].Path != "/a.md" || page.NextCursor == nil {
		t.Fatal(page, err)
	}
	options.Cursor = page.NextCursor
	page, err = index.SearchSemantic(ctx, policy, options, client)
	if err != nil || len(page.Results) != 1 || page.Results[0].Path != "/b.md" {
		t.Fatal(page, err)
	}
	options.Cursor = nil
	options.Hybrid = true
	page, err = index.SearchSemantic(ctx, policy, options, client)
	if err != nil || len(page.Results) != 1 {
		t.Fatal(page, err)
	}
	denied := access.EffectivePolicy{ReadPrefixes: []string{"/missing/"}}
	page, err = index.SearchSemantic(ctx, denied, options, client)
	if err != nil || len(page.Results) != 0 {
		t.Fatal(page, err)
	}
}

func TestSemanticSearchFilters(t *testing.T) {
	ctx := context.Background()
	index := semanticSearchIndex(t)
	client := &semanticChunkClientStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 0}}}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	kind, status, prefix := "concept", "active", "/a"
	options := SemanticSearchOptions{SearchOptions: SearchOptions{Query: "Title", Syntax: "plain", Limit: 10, ConceptType: &kind, Status: &status, PathPrefix: &prefix}, MaxCandidates: 0}
	page, err := index.SearchSemantic(ctx, policy, options, client)
	if err != nil || len(page.Results) != 1 {
		t.Fatal(page, err)
	}
	options.Tags = []string{"missing"}
	page, err = index.SearchSemantic(ctx, policy, options, client)
	if err != nil || len(page.Results) != 0 {
		t.Fatal(page, err)
	}
}

func TestSemanticSearchIgnoresLegacyRowsUntilChunkRefresh(t *testing.T) {
	ctx := context.Background()
	index := legacySemanticSearchIndex(t)
	client := &semanticChunkClientStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 0}}}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	options := SemanticSearchOptions{SearchOptions: SearchOptions{Query: "Title", Syntax: "plain", Limit: 10}, MaxCandidates: 10}
	page, err := index.SearchSemantic(ctx, policy, options, client)
	if err != nil || len(page.Results) != 0 {
		t.Fatal(page, err)
	}
	pending, err := index.PendingEmbeddingPaths(ctx, 10)
	if err != nil || len(pending) != 2 {
		t.Fatal(pending, err)
	}
	if err = index.RefreshEmbeddingPaths(ctx, "main", pending, SemanticRefreshConfig{ModelID: "m", Dimensions: 2, MaxInputChars: 4096, MaxBatch: 16}, client); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	var chunks int
	if err = db.QueryRow("SELECT COUNT(*) FROM concept_embedding_chunks").Scan(&chunks); err != nil || chunks != 2 {
		t.Fatal(chunks, err)
	}
	page, err = index.SearchSemantic(ctx, policy, options, client)
	if err != nil || len(page.Results) != 2 || page.Results[0].Path != "/a.md" {
		t.Fatal(page, err)
	}
}

func TestSemanticSearchFailures(t *testing.T) {
	ctx := context.Background()
	index := semanticSearchIndex(t)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	options := SemanticSearchOptions{SearchOptions: SearchOptions{Query: "x", Syntax: "plain", Limit: 10}, MaxCandidates: 10}
	if _, err := index.SearchSemantic(ctx, policy, options, nil); err == nil {
		t.Fatal("nil")
	}
	badCursor := "bad"
	options.Cursor = &badCursor
	valid := &semanticChunkClientStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 0}}}
	if _, err := index.SearchSemantic(ctx, policy, options, valid); err == nil {
		t.Fatal("cursor")
	}
	options.Cursor = nil
	client := &semanticChunkClientStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1}, err: errors.New("embed")}}
	if _, err := index.SearchSemantic(ctx, policy, options, client); err == nil {
		t.Fatal("embed")
	}
	client.err = nil
	if _, err := index.SearchSemantic(ctx, policy, options, client); err == nil {
		t.Fatal("query vector")
	}
	options.Hybrid = true
	options.Syntax = "bad"
	options.Query = "x"
	client.vector = []float32{1, 0}
	if _, err := index.SearchSemantic(ctx, policy, options, client); err == nil {
		t.Fatal("hybrid syntax")
	}
	options.Syntax = "fts5"
	options.Query = "("
	client.vector = []float32{1, 0}
	if _, err := index.SearchSemantic(ctx, policy, options, client); err == nil {
		t.Fatal("hybrid query")
	}
	options.Hybrid = false
	options.Query = "x"
	options.Syntax = "plain"
	for _, tc := range []struct {
		blob   []byte
		vector []float32
		norm   float64
	}{{[]byte{1}, []float32{1, 0}, 1}, {semanticBlob(0, 0), []float32{1, 0}, 0}, {semanticBlob(float32(math.NaN()), 1), []float32{1, 0}, 1}} {
		db, _ := sql.Open("sqlite", index.Path)
		_, _ = db.Exec("UPDATE concept_embedding_chunks SET embedding_blob=?,embedding_norm=?", tc.blob, tc.norm)
		_ = db.Close()
		client.vector = tc.vector
		if _, err := index.SearchSemantic(ctx, policy, options, client); err == nil {
			t.Fatal(tc)
		}
	}
	for _, mutation := range []string{"DELETE FROM index_state WHERE key='repo_revision'", "DELETE FROM index_state WHERE key='index_revision'", "DROP TABLE concept_embeddings"} {
		index = semanticSearchIndex(t)
		db, _ := sql.Open("sqlite", index.Path)
		_, _ = db.Exec(mutation)
		_ = db.Close()
		client.vector = []float32{1, 0}
		if _, err := index.SearchSemantic(ctx, policy, options, client); err == nil {
			t.Fatal(mutation)
		}
	}
}

func TestSearchChunkPolicyAndLegacyClient(t *testing.T) {
	index := semanticSearchIndex(t)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	client := &semanticChunkClientStub{semanticClientStub: semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 0}}}
	opts := SemanticSearchOptions{SearchOptions: SearchOptions{Query: "hello", Limit: 10}}
	page, err := index.SearchSemantic(t.Context(), policy, opts, client)
	if err != nil || len(page.Results) != 2 {
		t.Fatal(page, err)
	}
	index.MaxInputChars = 512
	page, err = index.SearchSemantic(t.Context(), policy, opts, client)
	if err != nil || len(page.Results) != 0 {
		t.Fatal(page, err)
	}
	if _, err = index.SearchSemantic(t.Context(), policy, opts, &client.semanticClientStub); err == nil {
		t.Fatal("legacy query adapter accepted")
	}
	// Stored vectors with a different checkpoint are never used.
	index.MaxInputChars = 4096
	client.info.Revision = "new-model"
	page, err = index.SearchSemantic(t.Context(), policy, opts, client)
	if err != nil || len(page.Results) != 0 {
		t.Fatal(page, err)
	}
}
