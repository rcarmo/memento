package derived

import (
	"database/sql"
	"math"
	"reflect"
	"testing"
)

func graphCacheFixture(t *testing.T) (*Index, *sql.DB) {
	t.Helper()
	index := semanticSearchIndex(t)
	db, err := sql.Open("sqlite", index.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	return index, db
}
func cacheRows(t *testing.T, db *sql.DB) (int, int) {
	t.Helper()
	var items, pairs int
	if err := db.QueryRow("SELECT COUNT(*) FROM semantic_graph_items").Scan(&items); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM semantic_graph_pairs").Scan(&pairs); err != nil {
		t.Fatal(err)
	}
	return items, pairs
}
func TestSemanticGraphCachePersistsAndInvalidates(t *testing.T) {
	_, db := graphCacheFixture(t)
	if i, p := cacheRows(t, db); i != 2 || p != 1 {
		t.Fatal(i, p)
	}
	// Startup and unrelated metadata/revision changes must not rewrite the cache.
	if _, err := db.Exec(`CREATE TRIGGER detect_recompute BEFORE INSERT ON semantic_graph_items BEGIN SELECT RAISE(ABORT,'recomputed');END; UPDATE concept_embeddings SET embedding_revision='unrelated',updated_at='later',path=path`); err != nil {
		t.Fatal(err)
	}
	if err := PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TRIGGER detect_recompute; UPDATE concept_embeddings SET status='stale' WHERE concept_id='id'"); err != nil {
		t.Fatal(err)
	}
	if i, p := cacheRows(t, db); i != 1 || p != 0 {
		t.Fatal(i, p)
	}
	if _, err := db.Exec("UPDATE concept_embeddings SET status='ready' WHERE concept_id='id'"); err != nil {
		t.Fatal(err)
	}
	if err := PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE concept_embedding_policy SET policy='changed' WHERE concept_id='id'"); err != nil {
		t.Fatal(err)
	}
	if i, p := cacheRows(t, db); i != 1 || p != 0 {
		t.Fatal(i, p)
	}
	if err := PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	if i, p := cacheRows(t, db); i != 2 || p != 0 {
		t.Fatal("mixed policies", i, p)
	}
	if _, err := db.Exec("DELETE FROM concept_embeddings WHERE concept_id='id-2'"); err != nil {
		t.Fatal(err)
	}
	if i, p := cacheRows(t, db); i != 1 || p != 0 {
		t.Fatal(i, p)
	}
}
func TestSemanticGraphCacheUsesAllChunksAndSurvivesRestart(t *testing.T) {
	index, db := graphCacheFixture(t)
	if _, err := db.Exec("INSERT INTO concept_embedding_chunks VALUES('id',1,'h','v','tail',?,1)", semanticBlob(0, 1)); err != nil {
		t.Fatal(err)
	}
	if i, p := cacheRows(t, db); i != 1 || p != 0 {
		t.Fatal(i, p)
	}
	if err := PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	var score float64
	if err := db.QueryRow("SELECT similarity FROM semantic_graph_pairs").Scan(&score); err != nil || math.Abs(score-math.Sqrt(.5)) > 1e-6 {
		t.Fatal(score, err)
	}
	db.Close()
	again, err := sql.Open("sqlite", index.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if err := again.QueryRow("SELECT similarity FROM semantic_graph_pairs").Scan(&score); err != nil || math.Abs(score-math.Sqrt(.5)) > 1e-6 {
		t.Fatal(score, err)
	}
	// Only affected pairs disappear; untouched aggregate survives byte-for-byte.
	var before, after []byte
	if err := again.QueryRow("SELECT embedding_blob FROM semantic_graph_items WHERE concept_id='id-2'").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := again.Exec("DELETE FROM concept_embedding_chunks WHERE concept_id='id'"); err != nil {
		t.Fatal(err)
	}
	if err := PrepareSemanticGraphCache(t.Context(), again); err != nil {
		t.Fatal(err)
	}
	if err := again.QueryRow("SELECT embedding_blob FROM semantic_graph_items WHERE concept_id='id-2'").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("unrelated aggregate changed")
	}
}
func TestSemanticGraphCacheSQLFaults(t *testing.T) {
	// Inject each cache-build SQL error inside the same transaction used by
	// publication. Failure must not publish partial pairs or item vectors.
	s, fault := faultStore(t)
	if err := s.Migrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	db := s.DB
	if _, err := db.Exec(chunkSchema + ";" + semanticGraphSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO concepts(id,path,type,title,status,tags_json,aliases_json,body,content_hash,updated_at,repo_revision) VALUES('a','/a','concept','a','active','[]','[]','','h','now','r'),('b','/b','concept','b','active','[]','[]','','h','now','r');INSERT INTO concept_embeddings SELECT id,path,'h','m',2,'r','ready','v',NULL,NULL,'now',NULL FROM concepts;INSERT INTO concept_embedding_policy SELECT id,'policy' FROM concepts`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if _, err := db.Exec("INSERT INTO concept_embedding_chunks VALUES(?,0,'h','v','body',?,1)", id, semanticBlob(1, 0)); err != nil {
			t.Fatal(err)
		}
	}
	fault.count = 0
	if err := PrepareSemanticGraphCache(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	stages := fault.count
	for n := 1; n <= stages; n++ {
		fault.remaining = 0
		if _, err := db.Exec("DELETE FROM semantic_graph_items"); err != nil {
			t.Fatal(err)
		}
		fault.fired = false
		fault.remaining = n
		err := PrepareSemanticGraphCache(t.Context(), db)
		fault.remaining = 0
		if !fault.fired {
			t.Fatal("missing stage", n)
		}
		if err == nil && n < stages {
			t.Fatal("ignored SQL fault", n)
		}
		i, p := cacheRows(t, db)
		if err != nil && (i != 0 || p != 0) {
			t.Fatal("partial cache", n, i, p)
		}
	}
	db.Close()
	if err := PrepareSemanticGraphCache(t.Context(), db); err == nil {
		t.Fatal("closed DB")
	}
}

func TestSemanticGraphAggregateValidation(t *testing.T) {
	_, db := graphCacheFixture(t)
	for _, b := range [][]byte{nil, {1}, semanticBlob(0, 0), semanticBlob(float32(math.NaN()), 0), semanticBlob(float32(math.Inf(1)), 0)} {
		sum := []float64{0, 0}
		semanticGraphAddNormalizedChunk(sum, b)
		if sum[0] != 0 || sum[1] != 0 {
			t.Fatal(sum)
		}
	}
	for _, values := range [][]float64{{0, 0}, {math.Inf(1), 0}, {1e308, 1e308}} {
		if _, norm := semanticGraphEncode(values); norm != 0 {
			t.Fatal(norm)
		}
	}
	if _, ok := semanticGraphCosine([]float32{1, 0}, 1, []byte{1}, 1, 2); ok {
		t.Fatal("invalid vector")
	}
	if _, ok, err := semanticGraphIdentityRow(t.Context(), db, "missing"); err != nil || ok {
		t.Fatal(ok, err)
	}
	if err := refreshSemanticGraphItem(t.Context(), db, "missing"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE concept_embedding_chunks SET embedding_blob=?,embedding_norm=1 WHERE concept_id='id'", semanticBlob(0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := refreshSemanticGraphItem(t.Context(), db, "id"); err != nil {
		t.Fatal(err)
	}
	// Bad cached peer vector is rejected; no dot-product result is persisted.
	if _, err := db.Exec("UPDATE semantic_graph_items SET embedding_blob=x'00' WHERE concept_id='id-2';UPDATE concept_embedding_chunks SET embedding_blob=x'0000803f00000000' WHERE concept_id='id'"); err != nil {
		t.Fatal(err)
	}
	if err := refreshSemanticGraphItem(t.Context(), db, "id"); err != nil {
		t.Fatal(err)
	}
	if _, pairs := cacheRows(t, db); pairs != 0 {
		t.Fatal(pairs)
	}
	if _, err := db.Exec("DELETE FROM concept_embedding_chunks WHERE concept_id='id'"); err != nil {
		t.Fatal(err)
	}
	if err := refreshSemanticGraphItem(t.Context(), db, "id"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE concept_embeddings SET dimensions=0 WHERE concept_id='id-2'"); err != nil {
		t.Fatal(err)
	}
	if err := refreshSemanticGraphItem(t.Context(), db, "id-2"); err != nil {
		t.Fatal(err)
	}
}
