package graphdebug

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

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
