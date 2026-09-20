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
	vectors := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "main"}, "b": {[]float64{1, 0}, 1, "m", "main"}, "c": {[]float64{.9, .1}, math.Sqrt(.82), "m", "main"}, "dimension": {[]float64{1}, 1, "m", "main"}, "model": {[]float64{1, 0}, 1, "other", "main"}, "revision": {[]float64{1, 0}, 1, "m", "other"}}
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
	ties := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "main"}, "b": {[]float64{1, 0}, 1, "m", "main"}, "c": {[]float64{1, 0}, 1, "m", "main"}}
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

func TestSemanticEdgesPartialRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,embedding_blob BLOB,embedding_norm REAL,model_id TEXT,embedding_revision TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, status, revision, model string }{
		{"a", "ready", "main", "m"}, {"b", "ready", "main", "m"},
		{"failed", "error", "main", "m"}, {"stale", "ready", "old", "m"},
		{"old-peer", "ready", "old", "m"}, {"other", "ready", "main", "other-model"},
	} {
		if _, err = db.Exec(`INSERT INTO concept_embeddings VALUES(?,?,?,1,?,?)`, row.id, row.status, blob(1, 0), row.model, row.revision); err != nil {
			t.Fatal(err)
		}
	}
	nodes := []Node{{ID: "a"}, {ID: "b"}, {ID: "failed"}, {ID: "stale"}, {ID: "old-peer"}, {ID: "other"}}
	s := NewSnapshotService("", path, "")
	config := SemanticConfig{Neighbours: 5, MinSimilarity: .5, NodeLimit: 10, EdgeLimit: 10}
	for _, revision := range []string{"partial", "main"} {
		edges, err := s.SemanticEdges(context.Background(), nodes, Revisions{Repository: "main", Embedding: &revision}, config, 10)
		if err != nil || len(edges) != 1 || edges[0].ID != "semantic:a:b" || *edges[0].EmbeddingRevision != "main" {
			t.Fatal(revision, edges, err)
		}
	}
	// A partial repository still respects visibility and the quadratic-work cap.
	for _, subset := range [][]Node{{{ID: "a"}, {ID: "failed"}}, {{ID: "a"}, {ID: "stale"}}} {
		edges, err := s.SemanticEdges(context.Background(), subset, Revisions{Repository: "main", Embedding: ptr("partial")}, config, 10)
		if err != nil || len(edges) != 0 {
			t.Fatal(edges, err)
		}
	}
	config.NodeLimit = 2
	edges, err := s.SemanticEdges(context.Background(), nodes, Revisions{Repository: "main", Embedding: ptr("partial")}, config, 10)
	if err != nil || len(edges) != 0 {
		t.Fatal(edges, err)
	}
}

func TestSemanticOverviewPartialRevision(t *testing.T) {
	root, path := nodeDB(t)
	service := NewSnapshotService(root, path, fixtureControlDB(t))
	db, err := sql.Open("sqlite", service.DerivedDBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("UPDATE index_state SET value='partial' WHERE key='semantic_embedding_revision'; DELETE FROM concept_embeddings"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e"} {
		if _, err = db.Exec("INSERT INTO concept_embeddings VALUES(?,'ready','model',2,'main','v1','now',NULL,?,1)", id, blob(1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	payload, err := service.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 2000, EdgeLimit: 12000, Semantic: SemanticConfig{Neighbours: 12, MinSimilarity: .5, NodeLimit: 300, EdgeLimit: 1500}})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, edge := range payload.Edges {
		if edge.Kind == "semantic_similarity" {
			count++
		}
	}
	if count != 1 || payload.Revisions.Embedding == nil || *payload.Revisions.Embedding != "partial" {
		t.Fatal(payload)
	}
}
