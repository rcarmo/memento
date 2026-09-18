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
	_, _ = db.Exec("UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision'; DROP TABLE concept_embeddings; CREATE TABLE concept_embeddings(concept_id TEXT,status TEXT,model_id TEXT,dimensions INTEGER,embedding_revision TEXT,model_revision TEXT,updated_at TEXT,error_message TEXT)")
	db.Close()
	s := NewSnapshotService(root, path, control)
	if _, err := s.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10, Semantic: SemanticConfig{NodeLimit: 10, EdgeLimit: 10, Neighbours: 1}}); err == nil {
		t.Fatal("semantic")
	}
}
func TestOverviewFailures(t *testing.T) {
	ctx := context.Background()
	root, path := nodeDB(t)
	control := emptyControlDB(t)
	cases := []func(*SnapshotService){func(s *SnapshotService) { s.DerivedDBPath = filepath.Join(t.TempDir(), "missing") }, func(s *SnapshotService) { s.ControlDBPath = filepath.Join(t.TempDir(), "missing") }, func(s *SnapshotService) {
		db, _ := sql.Open("sqlite", path)
		_, _ = db.Exec("DROP TABLE links")
		db.Close()
	}}
	for _, mutate := range cases {
		s := NewSnapshotService(root, path, control)
		mutate(s)
		if _, err := s.Overview(ctx, nil, OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10}); err == nil {
			t.Fatal("failure")
		}
	}
}
