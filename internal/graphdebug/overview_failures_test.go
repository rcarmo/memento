package graphdebug

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOverviewSemanticFailure(t *testing.T) {
	root, path := nodeDB(t)
	control := emptyControlDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision'; DROP TABLE concept_embedding_chunks;CREATE TABLE concept_embedding_chunks(concept_id TEXT); DROP TABLE concept_embeddings; CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,model_id TEXT,dimensions INTEGER,embedding_revision TEXT,model_revision TEXT,updated_at TEXT,error_message TEXT)")
	db.Close()
	s := NewSnapshotService(root, path, control)
	if _, err := s.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10, Semantic: SemanticConfig{NodeLimit: 10, EdgeLimit: 10, Neighbours: 1}}); err == nil {
		t.Fatal("semantic")
	}
}
func TestOverviewAggregateSemanticFailure(t *testing.T) {
	root, path := nodeDB(t)
	control := emptyControlDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision';DROP TABLE concept_embedding_chunks;CREATE TABLE concept_embedding_chunks(concept_id TEXT,document_hash TEXT,model_revision TEXT,embedding_blob BLOB,embedding_norm REAL,ordinal INTEGER);DROP TABLE concept_embeddings;CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,model_id TEXT,dimensions INTEGER,embedding_revision TEXT,model_revision TEXT,updated_at TEXT,error_message TEXT)")
	db.Close()
	s := NewSnapshotService(root, path, control)
	if _, err := s.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 10, Semantic: SemanticConfig{NodeLimit: 10, EdgeLimit: 10, Neighbours: 1}}); err == nil {
		t.Fatal("aggregate semantic")
	}
}
func TestOverviewAggregateFailures(t *testing.T) {
	for _, failure := range []int{7, 8, 9} {
		root, path := nodeDB(t)
		control := emptyControlDB(t)
		s := NewSnapshotService(root, path, control)
		base := s.open
		calls := 0
		s.open = func(ctx context.Context, path string) (*sql.DB, error) {
			calls++
			if calls == failure {
				return nil, context.Canceled
			}
			return base(ctx, path)
		}
		semantic := SemanticConfig{}
		if failure == 8 {
			db, _ := sql.Open("sqlite", path)
			_, _ = db.Exec("UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision'")
			db.Close()
			semantic = SemanticConfig{NodeLimit: 10, EdgeLimit: 10, Neighbours: 1}
		}
		if _, err := s.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 10, Semantic: semantic}); err == nil {
			t.Fatal(failure, calls)
		}
	}
}
func TestOverviewFailures(t *testing.T) {
	ctx := context.Background()
	cases := []func(*SnapshotService, string){func(s *SnapshotService, _ string) { s.DerivedDBPath = filepath.Join(t.TempDir(), "missing") }, func(s *SnapshotService, _ string) { s.ControlDBPath = filepath.Join(t.TempDir(), "missing") }, func(s *SnapshotService, path string) {
		db, _ := sql.Open("sqlite", path)
		_, _ = db.Exec("DROP TABLE links")
		db.Close()
	}, func(s *SnapshotService, path string) {
		db, _ := sql.Open("sqlite", path)
		_, _ = db.Exec("ALTER TABLE concepts RENAME COLUMN content_hash TO missing_hash")
		db.Close()
	}}
	for _, mutate := range cases {
		root, path := nodeDB(t)
		control := emptyControlDB(t)
		s := NewSnapshotService(root, path, control)
		mutate(s, path)
		if _, err := s.Overview(ctx, nil, OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10}); err == nil {
			t.Fatal("failure")
		}
	}
}
