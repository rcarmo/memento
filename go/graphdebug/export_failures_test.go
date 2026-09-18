package graphdebug

import (
	"context"
	"database/sql"
	"testing"
)

func TestExportSelectionFailures(t *testing.T) {
	root, path := nodeDB(t)
	control := emptyControlDB(t)
	ids := []string{"5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d"}
	for _, failure := range []int{1, 3, 4} {
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
		if _, _, _, err := s.ExportSelection(context.Background(), ids, 10, 10, nil); err == nil {
			t.Fatal(failure, calls)
		}
	}
	if (&SnapshotError{"x"}).Error() != "x" {
		t.Fatal("error")
	}
}
