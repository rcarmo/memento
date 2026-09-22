package graphdebug

import (
	"context"
	"database/sql"
	"testing"
)

func TestClusterExpansionFailureStages(t *testing.T) {
	for _, failure := range []int{1, 2, 3, 4, 5, 6} {
		root, path := nodeDB(t)
		s := NewSnapshotService(root, path, emptyControlDB(t))
		base := s.open
		calls := 0
		s.open = func(ctx context.Context, path string) (*sql.DB, error) {
			calls++
			if calls == failure {
				return nil, context.Canceled
			}
			return base(ctx, path)
		}
		o := ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 10, ExpansionNodeLimit: 10, ClusterLimit: 1, Semantic: SemanticConfig{NodeLimit: 10, EdgeLimit: 10, Neighbours: 1}}
		if _, err := s.ExpandCluster(context.Background(), "cluster:overflow", nil, o); err == nil {
			t.Fatal(failure, calls)
		}
	}
}
func TestClusterExpansionSemanticFailure(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision';DROP TABLE concept_embedding_chunks;CREATE TABLE concept_embedding_chunks(concept_id TEXT,document_hash TEXT,model_revision TEXT,embedding_blob BLOB,embedding_norm REAL,ordinal INTEGER);DROP TABLE concept_embeddings;CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,model_id TEXT,dimensions INTEGER,embedding_revision TEXT,model_revision TEXT,updated_at TEXT,error_message TEXT)")
	db.Close()
	s := NewSnapshotService(root, path, emptyControlDB(t))
	o := ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 10, ExpansionNodeLimit: 10, ClusterLimit: 1, Semantic: SemanticConfig{NodeLimit: 10, EdgeLimit: 10, Neighbours: 1}}
	if _, err := s.ExpandCluster(context.Background(), "cluster:overflow", nil, o); err == nil {
		t.Fatal("semantic")
	}
}
func TestClusterExpansionPageEdges(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("INSERT INTO links VALUES('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','/a.md','/a.md',NULL,'internal','resolved','r','r')")
	db.Close()
	s := NewSnapshotService(root, path, emptyControlDB(t))
	o := ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 3, ExpansionNodeLimit: 10, ClusterLimit: 1}
	got, err := s.ExpandCluster(context.Background(), "cluster:overflow", nil, o)
	if err != nil || len(got.Edges) != 2 {
		t.Fatal(got, err)
	}
}
