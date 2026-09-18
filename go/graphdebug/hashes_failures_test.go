package graphdebug

import (
	"context"
	"database/sql"
	"testing"
)

func TestContentHashFailures(t *testing.T) {
	if snapshotFaultRegistered.CompareAndSwap(false, true) {
		sql.Register("graphdebug-fault", snapshotFaultDriver{})
	}
	s := NewSnapshotService("", "derived", "")
	s.open = func(context.Context, string) (*sql.DB, error) { return nil, context.Canceled }
	if _, err := s.ContentHashes(context.Background(), nil); err == nil {
		t.Fatal("open")
	}
	for _, mode := range []string{"query", "scan", "rows"} {
		s.open = faultOpen(t, mode)
		if _, err := s.ContentHashes(context.Background(), nil); err == nil {
			t.Fatal(mode)
		}
	}
	s.open = faultOpen(t, "ok")
	got, err := s.ContentHashes(context.Background(), nil)
	if err != nil || got["a"] != "hash" {
		t.Fatal(got, err)
	}
}
