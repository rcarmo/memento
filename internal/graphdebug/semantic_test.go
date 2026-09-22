package graphdebug

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"github.com/rcarmo/memento/internal/derived"
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
	vectors := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "main", "v1", "test-policy"}, "b": {[]float64{.9, .1}, math.Sqrt(.82), "m", "main", "v1", "test-policy"}, "c": {[]float64{0, 1}, 1, "m", "main", "v1", "test-policy"}}
	config := SemanticConfig{Neighbours: 1, MinSimilarity: .5, NodeLimit: 3, EdgeLimit: 3}
	edges := selectSemantic(vectors, Revisions{Repository: "main", Embedding: &revision}, config, 3)
	if len(edges) != 1 || edges[0].ID != "semantic:a:b" || edges[0].RawTarget != "cosine:0.9939" || edges[0].Similarity == nil {
		t.Fatal(edges)
	}
	for _, r := range []Revisions{{Repository: "main"}, {Repository: "main", Embedding: ptr("old")}} {
		if got := selectSemantic(vectors, r, config, 3); len(got) != 1 {
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
	vectors := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "main", "v1", "test-policy"}, "b": {[]float64{1, 0}, 1, "m", "main", "v1", "test-policy"}, "c": {[]float64{.9, .1}, math.Sqrt(.82), "m", "main", "v1", "test-policy"}, "dimension": {[]float64{1}, 1, "m", "main", "v1", "test-policy"}, "model": {[]float64{1, 0}, 1, "other", "main", "v1", "test-policy"}, "revision": {[]float64{1, 0}, 1, "m", "other", "v1", "test-policy"}}
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
	ties := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "main", "v1", "test-policy"}, "b": {[]float64{1, 0}, 1, "m", "main", "v1", "test-policy"}, "c": {[]float64{1, 0}, 1, "m", "main", "v1", "test-policy"}}
	config.Neighbours = 2
	got := selectSemantic(ties, Revisions{Repository: "main", Embedding: &revision}, config, 10)
	if len(got) < 2 || got[0].Source > got[1].Source {
		t.Fatal(got)
	}
}
func TestSemanticFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.sqlite")
	db, _ := sql.Open("sqlite", path)
	_, err := db.Exec(`CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,embedding_blob BLOB,embedding_norm REAL,model_id TEXT,embedding_revision TEXT,model_revision TEXT);INSERT INTO concept_embeddings VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','ready',?,NULL,'model','main','v1'),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','ready',?,NULL,'model','main','v1')`, blob(1, 0), blob(.9, .1))
	if err == nil {
		seedGraphChunks(t, db)
	}
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
	_, err := db.Exec(`CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,embedding_blob BLOB,embedding_norm REAL,model_id TEXT,embedding_revision TEXT,model_revision TEXT);INSERT INTO concept_embeddings VALUES('a','ready',?,NULL,'m','main','v1'),('b','ready',?,1,'m','main','v1'),('bad','ready',X'01',1,'m','main','v1'),('zero','ready',X'00000000',NULL,'m','main','v1')`, blob(1, 0), blob(1, 0))
	if err == nil {
		seedGraphChunks(t, db)
	}
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
	if _, err = db.Exec(`CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,embedding_blob BLOB,embedding_norm REAL,model_id TEXT,embedding_revision TEXT,model_revision TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, status, revision, model string }{
		{"a", "ready", "main", "m"}, {"b", "ready", "main", "m"},
		{"failed", "error", "main", "m"}, {"stale", "stale", "old", "m"},
		{"old-peer", "pending", "old", "m"}, {"other", "ready", "main", "other-model"},
	} {
		if _, err = db.Exec(`INSERT INTO concept_embeddings VALUES(?,?,?,1,?,?,'v1')`, row.id, row.status, blob(1, 0), row.model, row.revision); err != nil {
			t.Fatal(err)
		}
	}
	seedGraphChunks(t, db)
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
		if _, err = db.Exec("INSERT INTO concept_embeddings(concept_id,status,model_id,dimensions,embedding_revision,model_revision,updated_at,error_message,embedding_blob,embedding_norm) VALUES(?,'ready','model',2,'main','v1','now',NULL,?,1)", id, blob(1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	seedGraphChunks(t, db)
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

// Repository revisions are provenance; item status and model identity decide
// eligibility. An unrelated commit must not remove any unchanged pair.
func TestSemanticEdgesItemRelative(t *testing.T) {
	path := filepath.Join(t.TempDir(), "item.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,embedding_blob BLOB,embedding_norm REAL,model_id TEXT,embedding_revision TEXT,model_revision TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, status, revision, modelRevision string }{
		{"a", "ready", "r1", "checkpoint"}, {"b", "ready", "r2", "checkpoint"},
		{"changed", "stale", "r1", "checkpoint"}, {"pending", "pending", "r3", "checkpoint"},
		{"other-model", "ready", "r1", "other-checkpoint"},
	} {
		if _, err = db.Exec("INSERT INTO concept_embeddings VALUES(?,?,?,1,'model',?,?)", row.id, row.status, blob(1, 0), row.revision, row.modelRevision); err != nil {
			t.Fatal(err)
		}
	}
	seedGraphChunks(t, db)
	nodes := []Node{{ID: "a"}, {ID: "b"}, {ID: "changed"}, {ID: "pending"}, {ID: "other-model"}, {ID: "missing"}}
	service := NewSnapshotService("", path, "")
	for _, global := range []*string{nil, ptr("r0"), ptr("partial"), ptr("r3")} {
		edges, err := service.SemanticEdges(t.Context(), nodes, Revisions{Repository: "r3", Embedding: global}, SemanticConfig{Neighbours: 10, NodeLimit: 10, EdgeLimit: 10, MinSimilarity: .5}, 10)
		if err != nil || len(edges) != 1 || edges[0].ID != "semantic:a:b" {
			t.Fatal(global, edges, err)
		}
	}
}

func seedGraphChunks(t *testing.T, db *sql.DB) {
	t.Helper()
	_, _ = db.Exec("ALTER TABLE concept_embeddings ADD COLUMN dimensions INTEGER NOT NULL DEFAULT 2")
	_, _ = db.Exec("CREATE TABLE IF NOT EXISTS concepts(id TEXT PRIMARY KEY,path TEXT)")
	_, _ = db.Exec("INSERT OR IGNORE INTO concepts(id,path) SELECT concept_id,concept_id FROM concept_embeddings")
	_, _ = db.Exec("ALTER TABLE concept_embeddings ADD COLUMN embedding_text_hash TEXT NOT NULL DEFAULT 'test-hash'")
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS concept_embedding_policy(concept_id TEXT PRIMARY KEY,policy TEXT);DELETE FROM concept_embedding_policy;INSERT INTO concept_embedding_policy SELECT concept_id,'test-policy' FROM concept_embeddings;CREATE TABLE IF NOT EXISTS concept_embedding_chunks(concept_id TEXT,ordinal INTEGER,document_hash TEXT,model_revision TEXT,text TEXT,embedding_blob BLOB,embedding_norm REAL);DELETE FROM concept_embedding_chunks;INSERT INTO concept_embedding_chunks SELECT concept_id,0,embedding_text_hash,model_revision,'fixture',embedding_blob,embedding_norm FROM concept_embeddings")
	if err != nil {
		t.Fatal(err)
	}
	if err := derived.PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
}

func TestGraphChunkOnlyEligibilityAndTail(t *testing.T) {
	root, path := nodeDB(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Parent mirrors intentionally disagree with actual chunk vectors.
	_, err = db.Exec(`DELETE FROM concept_embeddings;INSERT INTO concept_embeddings(concept_id,status,model_id,dimensions,embedding_revision,model_revision,embedding_blob,embedding_norm,embedding_text_hash) VALUES('a','ready','m',2,'r1','v1',?,1,'ha'),('b','ready','m',2,'r2','v1',?,1,'hb')`, blob(0, 1), blob(0, 1))
	if err != nil {
		t.Fatal(err)
	}
	seedGraphChunks(t, db)
	_, err = db.Exec("UPDATE concept_embedding_chunks SET embedding_blob=? WHERE concept_id='a'", blob(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	service := NewSnapshotService(root, path, emptyControlDB(t))
	if err := derived.PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	service.EmbeddingPolicy = "test-policy"
	config := SemanticConfig{NodeLimit: 10, Neighbours: 2, EdgeLimit: 10, MinSimilarity: .7}
	nodes := []Node{{ID: "a"}, {ID: "b"}}
	get := func(want int) {
		t.Helper()
		edges, e := service.SemanticEdges(t.Context(), nodes, Revisions{Repository: "r3"}, config, 10)
		if e != nil || len(edges) != want {
			t.Fatal(edges, e)
		}
	}
	get(0)
	if _, err = db.Exec("INSERT INTO concept_embedding_chunks VALUES('a',1,'ha','v1','tail',?,1)", blob(0, 1)); err != nil {
		t.Fatal(err)
	}
	if err := derived.PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	get(1) // tail chunk contributes to the item aggregate, never the parent mirror
	if _, err = db.Exec("UPDATE concept_embedding_policy SET policy='outdated' WHERE concept_id='b'"); err != nil {
		t.Fatal(err)
	}
	get(0)
	if _, err = db.Exec("DELETE FROM concept_embedding_chunks WHERE concept_id='b'"); err != nil {
		t.Fatal(err)
	}
	service.EmbeddingPolicy = ""
	get(0) // legacy row is never a fallback
	if _, err = db.Exec("DROP TABLE concept_embedding_chunks"); err != nil {
		t.Fatal(err)
	}
	get(0)
}

func TestChunkSemanticQueryErrors(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	db, _ := sql.Open("sqlite", path)
	defer db.Close()
	_, err := db.Exec("ALTER TABLE semantic_graph_pairs RENAME COLUMN similarity TO absent")
	if err != nil {
		t.Fatal(err)
	}
	options := OverviewOptions{DirectNodeLimit: 10, RefreshMaxPaths: 10, EdgeLimit: 10, ClusterLimit: 1, Semantic: SemanticConfig{NodeLimit: 10, Neighbours: 1, EdgeLimit: 10}}
	if _, err = s.Overview(t.Context(), nil, options); err == nil {
		t.Fatal("direct semantic error")
	}
	options.DirectNodeLimit = 1
	if _, err = s.Overview(t.Context(), nil, options); err == nil {
		t.Fatal("aggregate semantic error")
	}
	if _, err = s.ExpandCluster(t.Context(), "cluster:overflow", nil, ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 10, ExpansionNodeLimit: 10, ClusterLimit: 1, Semantic: options.Semantic}); err == nil {
		t.Fatal("cluster semantic error")
	}
}

func TestChunkGraphAggregateInvalidAndMixedDimensions(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	defer db.Close()
	_, err := db.Exec("UPDATE concept_embedding_chunks SET embedding_blob=?,embedding_norm=1;INSERT INTO concept_embedding_chunks SELECT concept_id,1,document_hash,model_revision,'opposite',?,1 FROM concept_embedding_chunks WHERE ordinal=0;INSERT INTO concept_embedding_chunks SELECT concept_id,2,document_hash,model_revision,'bad dimensions',?,1 FROM concept_embedding_chunks WHERE ordinal=0", blob(1, 0), blob(-1, 0), blob(1))
	if err != nil {
		t.Fatal(err)
	}
	// Explicit bindings per statement are required by SQLite's multi-statement driver.
	_, err = db.Exec("UPDATE concept_embedding_chunks SET embedding_blob=? WHERE ordinal=1", blob(-1, 0))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("UPDATE concept_embedding_chunks SET embedding_blob=? WHERE ordinal=2", blob(1))
	if err != nil {
		t.Fatal(err)
	}
	s := NewSnapshotService(root, path, emptyControlDB(t))
	edges, err := s.SemanticEdges(t.Context(), []Node{{ID: alphaNodeID}, {ID: betaNodeID}}, Revisions{}, SemanticConfig{NodeLimit: 3, EdgeLimit: 3, Neighbours: 2}, 3)
	if err != nil || len(edges) != 0 {
		t.Fatal(edges, err)
	}
}

func TestSemanticWebLoadsReadCachedScoresOnly(t *testing.T) {
	root, path := nodeDB(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, id := range []string{alphaNodeID, betaNodeID} {
		if _, err = db.Exec(`INSERT INTO concept_embeddings(concept_id,status,model_id,dimensions,embedding_revision,model_revision,embedding_blob,embedding_norm,embedding_text_hash) VALUES(?,'ready','m',2,'old-item-revision','v',?,1,'h') ON CONFLICT(concept_id) DO UPDATE SET model_id='m',dimensions=2,embedding_blob=excluded.embedding_blob,embedding_norm=1,embedding_text_hash='h',model_revision='v'`, id, blob(1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	seedGraphChunks(t, db)
	// Any regression to reading/aggregating chunks or parent vectors now fails SQL.
	for _, query := range []string{
		"ALTER TABLE concept_embedding_chunks RENAME COLUMN embedding_blob TO unavailable",
		"ALTER TABLE concept_embeddings RENAME COLUMN embedding_blob TO unavailable",
		"ALTER TABLE semantic_graph_items RENAME COLUMN embedding_blob TO unavailable",
	} {
		if _, err = db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	for j := 0; j < 3; j++ {
		service := NewSnapshotService(root, path, emptyControlDB(t))
		edges, err := service.SemanticEdges(t.Context(), []Node{{ID: alphaNodeID}, {ID: betaNodeID}}, Revisions{Repository: "unrelated-new-revision"}, SemanticConfig{NodeLimit: 10, Neighbours: 1, EdgeLimit: 10, MinSimilarity: .5}, 10)
		if err != nil || len(edges) != 1 || *edges[0].Similarity != 1 {
			t.Fatal(edges, err)
		}
	}
}

func TestCachedRankingMatchesVectorOracle(t *testing.T) {
	vectors := map[string]semanticVector{"a": {[]float64{1, 0}, 1, "m", "r1", "v", "p"}, "b": {[]float64{.8, .6}, 1, "m", "r2", "v", "p"}, "c": {[]float64{0, 1}, 1, "m", "r3", "v", "p"}}
	scores := []semanticScore{{"a", "b", .8, "m", "r1"}, {"a", "c", 0, "m", "r1"}, {"b", "c", .6, "m", "r2"}}
	revisions := Revisions{Repository: "r4"}
	for _, k := range []int{1, 2, 3} {
		for _, threshold := range []float64{-.1, .5, .7, 1.1} {
			cfg := SemanticConfig{NodeLimit: 10, Neighbours: k, MinSimilarity: threshold, EdgeLimit: 10}
			for _, limit := range []int{0, 1, 10} {
				want := selectSemantic(vectors, revisions, cfg, limit)
				got := selectSemanticScores(scores, revisions, cfg, limit)
				if !reflect.DeepEqual(want, got) {
					t.Fatal(k, threshold, limit, want, got)
				}
			}
		}
	}
	// Hidden candidates must be removed before ranking, not merely clipped later.
	visible := []semanticScore{{"a", "c", 0, "m", "r1"}}
	got := selectSemanticScores(visible, revisions, SemanticConfig{Neighbours: 1, MinSimilarity: -1, EdgeLimit: 10}, 10)
	if len(got) != 1 || got[0].ID != "semantic:a:c" {
		t.Fatal(got)
	}
}

func TestSemanticScoreTiesAndMissingCache(t *testing.T) {
	scores := []semanticScore{{"a", "b", 1, "m", "r"}, {"a", "c", 1, "m", "r"}, {"b", "c", 1, "m", "r"}}
	cfg := SemanticConfig{Neighbours: 3, NodeLimit: 10, EdgeLimit: 10, MinSimilarity: 0}
	got := selectSemanticScores(scores, Revisions{}, cfg, 10)
	if len(got) != 3 || got[0].ID != "semantic:a:b" || got[1].ID != "semantic:a:c" || got[2].ID != "semantic:b:c" {
		t.Fatal(got)
	}
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	defer db.Close()
	_, err := db.Exec("DROP TABLE semantic_graph_pairs")
	if err != nil {
		t.Fatal(err)
	}
	s := NewSnapshotService(root, path, emptyControlDB(t))
	got, err = s.SemanticEdges(t.Context(), []Node{{ID: alphaNodeID}, {ID: betaNodeID}}, Revisions{}, cfg, 10)
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}

func TestCachedNonFiniteScoreExcluded(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	defer db.Close()
	for _, id := range []string{alphaNodeID, betaNodeID} {
		if _, err := db.Exec(`INSERT INTO concept_embeddings(concept_id,status,model_id,dimensions,embedding_revision,model_revision,embedding_blob,embedding_norm) VALUES(?,'ready','m',2,'r','v',?,1) ON CONFLICT(concept_id) DO UPDATE SET model_id='m',model_revision='v',dimensions=2,embedding_blob=excluded.embedding_blob,embedding_norm=1`, id, blob(1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	seedGraphChunks(t, db)
	if _, err := db.Exec("UPDATE semantic_graph_pairs SET similarity=?", math.Inf(1)); err != nil {
		t.Fatal(err)
	}
	s := NewSnapshotService(root, path, emptyControlDB(t))
	edges, err := s.SemanticEdges(t.Context(), []Node{{ID: alphaNodeID}, {ID: betaNodeID}}, Revisions{}, SemanticConfig{NodeLimit: 10, Neighbours: 1, EdgeLimit: 10}, 10)
	if err != nil || len(edges) != 0 {
		t.Fatal(edges, err)
	}
}
