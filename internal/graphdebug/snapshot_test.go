package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

func snapshotDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "derived.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{"CREATE TABLE index_state(key TEXT PRIMARY KEY,value TEXT NOT NULL,updated_at TEXT NOT NULL)", "CREATE TABLE concepts(id TEXT PRIMARY KEY,path TEXT NOT NULL)", "INSERT INTO index_state VALUES('repo_revision','main','now'),('index_revision','old','now'),('semantic_embedding_revision','embed','now')", "INSERT INTO concepts VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','/a.md'),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','/b.md')"} {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return path
}
func TestSnapshotRevisionsAndPaths(t *testing.T) {
	var fixture struct {
		Revisions  Revisions                       `json:"revisions"`
		PathsCases []struct{ IDs, Paths []string } `json:"paths_cases"`
	}
	raw, err := os.ReadFile("../../testdata/parity/graph-snapshot-foundation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	service := NewSnapshotService("root", snapshotDB(t), "control")
	revisions, err := service.Revisions(ctx)
	if err != nil || !reflect.DeepEqual(revisions, fixture.Revisions) {
		t.Fatal(revisions, fixture.Revisions, err)
	}
	for _, tc := range fixture.PathsCases {
		paths, err := service.PathsForIDs(ctx, tc.IDs)
		if err != nil || !reflect.DeepEqual(paths, tc.Paths) {
			t.Fatal(tc, paths, err)
		}
	}
}
func TestSnapshotMissingStateAndDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	path := snapshotDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("DELETE FROM index_state")
	db.Close()
	service := NewSnapshotService("root", path, "control")
	r, err := service.Revisions(ctx)
	if err != nil || r.Embedding != nil || r.Stale {
		t.Fatal(r, err)
	}
	missing := NewSnapshotService("root", filepath.Join(t.TempDir(), "missing.sqlite"), "control")
	if _, err = missing.Revisions(ctx); err == nil {
		t.Fatal("missing database")
	}
	if _, err = missing.PathsForIDs(ctx, []string{"a"}); err == nil {
		t.Fatal("missing database")
	}
}
