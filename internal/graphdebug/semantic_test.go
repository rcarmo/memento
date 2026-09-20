package graphdebug

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

func blob(values ...float32) []byte {
	out := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out
}
func TestSelectSemantic(t *testing.T) {
	revision := "main"
	vectors := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "main"}, "b": {[]float64{.9, .1}, math.Sqrt(.82), "m", "main"}, "c": {[]float64{0, 1}, 1, "m", "main"}}
	config := SemanticConfig{Neighbours: 1, MinSimilarity: .5, NodeLimit: 3, EdgeLimit: 3}
	edges := selectSemantic(vectors, Revisions{Repository: "main", Embedding: &revision}, config, 3)
	if len(edges) != 1 || edges[0].ID != "semantic:a:b" || edges[0].RawTarget != "cosine:0.9939" || edges[0].Similarity == nil {
		t.Fatal(edges)
	}
	for _, r := range []Revisions{{Repository: "main"}, {Repository: "main", Embedding: ptr("old")}} {
		if got := selectSemantic(vectors, r, config, 3); len(got) != 0 {
			t.Fatal(got)
		}
	}
	config.NodeLimit = 2
	if got := selectSemantic(vectors, Revisions{Repository: "main", Embedding: &revision}, config, 3); len(got) != 0 {
		t.Fatal(got)
	}
	if got := selectSemantic(vectors, Revisions{Repository: "main", Embedding: &revision}, SemanticConfig{Neighbours: 1, MinSimilarity: -1, NodeLimit: 3, EdgeLimit: 1}, 0); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestSemanticRankingBranches(t *testing.T) {
	revision := "main"
	vectors := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "r"}, "b": {[]float64{1, 0}, 1, "m", "r"}, "c": {[]float64{.9, .1}, math.Sqrt(.82), "m", "r"}, "dimension": {[]float64{1}, 1, "m", "r"}, "model": {[]float64{1, 0}, 1, "other", "r"}, "revision": {[]float64{1, 0}, 1, "m", "other"}}
	config := SemanticConfig{Neighbours: 2, MinSimilarity: .5, NodeLimit: 10, EdgeLimit: 1}
	edges := selectSemantic(vectors, Revisions{Repository: "main", Embedding: &revision}, config, 10)
	if len(edges) != 1 || edges[0].ID != "semantic:a:b" {
		t.Fatal(edges)
	}
	config.EdgeLimit = 10
	config.Neighbours = 1
	edges = selectSemantic(vectors, Revisions{Repository: "main", Embedding: &revision}, config, 10)
	if len(edges) == 0 {
		t.Fatal(edges)
	}
	if got := selectSemantic(map[string]semanticVector{"a": vectors["a"]}, Revisions{Repository: "main", Embedding: &revision}, config, 10); len(got) != 0 {
		t.Fatal(got)
	}
	ties := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "r"}, "b": {[]float64{1, 0}, 1, "m", "r"}, "c": {[]float64{1, 0}, 1, "m", "r"}}
	config.Neighbours = 2
	got := selectSemantic(ties, Revisions{Repository: "main", Embedding: &revision}, config, 10)
	if len(got) < 2 || got[0].Source > got[1].Source {
		t.Fatal(got)
	}
}
func TestSemanticFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.sqlite")
	db, _ := sql.Open("sqlite", path)
	_, err := db.Exec(`CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,embedding_blob BLOB,embedding_norm REAL,model_id TEXT,embedding_revision TEXT);INSERT INTO concept_embeddings VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','ready',?,NULL,'model','main'),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','ready',?,NULL,'model','main')`, blob(1, 0), blob(.9, .1))
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	revision := "main"
	s := NewSnapshotService("", path, "")
	nodes := []Node{{ID: "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d"}, {ID: "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e"}}
	got, err := s.SemanticEdges(context.Background(), nodes, Revisions{Repository: "main", Embedding: &revision}, SemanticConfig{Neighbours: 12, MinSimilarity: .5, NodeLimit: 300, EdgeLimit: 1500}, 10)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Edges []Edge `json:"semantic_edges"`
	}
	raw, _ := os.ReadFile("../../testdata/parity/graph-snapshot-foundation.json")
	if err = json.Unmarshal(raw, &fixture); err != nil || !reflect.DeepEqual(got, fixture.Edges) {
		t.Fatal(got, fixture.Edges, err)
	}
}
func TestSemanticEdgesSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.sqlite")
	db, _ := sql.Open("sqlite", path)
	_, err := db.Exec(`CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,embedding_blob BLOB,embedding_norm REAL,model_id TEXT,embedding_revision TEXT);INSERT INTO concept_embeddings VALUES('a','ready',?,NULL,'m','main'),('b','ready',?,1,'m','main'),('bad','ready',X'01',1,'m','main'),('zero','ready',X'00000000',NULL,'m','main')`, blob(1, 0), blob(1, 0))
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	revision := "main"
	s := NewSnapshotService("", path, "")
	edges, err := s.SemanticEdges(context.Background(), []Node{{ID: "a"}, {ID: "b"}, {ID: "bad"}, {ID: "zero"}}, Revisions{Repository: "main", Embedding: &revision}, SemanticConfig{Neighbours: 2, MinSimilarity: .5, NodeLimit: 10, EdgeLimit: 10}, 10)
	if err != nil || len(edges) != 1 {
		t.Fatal(edges, err)
	}
	if got, err := s.SemanticEdges(context.Background(), nil, Revisions{}, SemanticConfig{}, 0); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}
func TestSemanticEdgeFailures(t *testing.T) {
	if snapshotFaultRegistered.CompareAndSwap(false, true) {
		sql.Register("graphdebug-fault", snapshotFaultDriver{})
	}
	revision := "main"
	nodes := []Node{{ID: "a"}, {ID: "b"}}
	config := SemanticConfig{Neighbours: 1, MinSimilarity: 0, NodeLimit: 2, EdgeLimit: 2}
	s := NewSnapshotService("", "derived", "")
	s.open = func(context.Context, string) (*sql.DB, error) { return nil, context.Canceled }
	if _, err := s.SemanticEdges(context.Background(), nodes, Revisions{Repository: "main", Embedding: &revision}, config, 2); err == nil {
		t.Fatal("open")
	}
	for _, mode := range []string{"query", "scan", "rows"} {
		s.open = faultOpen(t, mode)
		if _, err := s.SemanticEdges(context.Background(), nodes, Revisions{Repository: "main", Embedding: &revision}, config, 2); err == nil {
			t.Fatal(mode)
		}
	}
}
func TestFloatFormatting(t *testing.T) {
	for v, w := range map[float64]string{1: "1.0000", .5: "0.5000", -.5: "-0.5000"} {
		if float4(v) != w {
			t.Fatal(v, float4(v))
		}
	}
}
