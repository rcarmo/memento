package derived

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"github.com/rcarmo/memento/go/access"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func semanticBlob(values ...float32) []byte {
	raw := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
	}
	return raw
}
func semanticSearchIndex(t *testing.T) *Index {
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
	db, _ := sql.Open("sqlite", path)
	_, err := db.Exec(`UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision';DELETE FROM concept_embeddings;INSERT INTO concept_embeddings(concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,embedding_blob,embedding_norm,updated_at,error_message) SELECT id,path,'h','m',2,'main','ready','v',?,1,'now',NULL FROM concepts WHERE path='/a.md';INSERT INTO concept_embeddings(concept_id,path,embedding_text_hash,model_id,dimensions,embedding_revision,status,model_revision,embedding_blob,embedding_norm,updated_at,error_message) SELECT id,path,'h','m',2,'main','ready','v',?,1,'now',NULL FROM concepts WHERE path='/b.md'`, semanticBlob(1, 0), semanticBlob(0, 1))
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}
	return index
}
func TestSearchSemantic(t *testing.T) {
	ctx := context.Background()
	index := semanticSearchIndex(t)
	client := &semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 0}}
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
	client := &semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1, 0}}
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
	if _, err := index.SearchSemantic(ctx, policy, options, &semanticClientStub{}); err == nil {
		t.Fatal("cursor")
	}
	options.Cursor = nil
	client := &semanticClientStub{info: SemanticModelInfo{"m", 2, "v"}, vector: []float32{1}, err: errors.New("embed")}
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
	}{{[]byte{1}, []float32{1, 0}}, {semanticBlob(0, 0), []float32{1, 0}}, {semanticBlob(float32(math.NaN()), 1), []float32{1, 0}}} {
		db, _ := sql.Open("sqlite", index.Path)
		_, _ = db.Exec("UPDATE concept_embeddings SET embedding_blob=?", tc.blob)
		_ = db.Close()
		client.vector = tc.vector
		if _, err := index.SearchSemantic(ctx, policy, options, client); err == nil {
			t.Fatal(tc)
		}
	}
	if _, err := semanticCosine([]float32{1}, []float32{1, 2}); err == nil {
		t.Fatal("dimension")
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
