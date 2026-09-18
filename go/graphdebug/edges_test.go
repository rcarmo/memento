package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/access"
	_ "modernc.org/sqlite"
)

func edgeDB(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "edges.sqlite")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.Exec(`CREATE TABLE links(source_id TEXT,target_id TEXT,raw_target TEXT,target_path TEXT,anchor TEXT,link_kind TEXT,resolution_state TEXT,first_seen_revision TEXT,last_checked_revision TEXT);INSERT INTO links VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','/public/b.md','/public/b.md',NULL,'internal','resolved','r1','r2'),('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d',NULL,'/private/missing.md','/private/missing.md','x','internal','broken','r1','r2'),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e',NULL,'/public/missing.md','/public/missing.md',NULL,'internal','broken','r1','r2')`)
	if e != nil {
		t.Fatal(e)
	}
	return path
}
func TestExplicitEdges(t *testing.T) {
	s := NewSnapshotService("", edgeDB(t), "")
	reader := access.EffectivePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	edges, e := s.ExplicitEdges(context.Background(), []string{"5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e"}, nil, nil, 10, &reader)
	var fixture struct {
		Edges []Edge `json:"edges"`
	}
	raw, err := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if e != nil || !reflect.DeepEqual(edges, fixture.Edges) {
		t.Fatal(edges, fixture.Edges, e)
	}
	source := "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d"
	target := "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e"
	edges, e = s.ExplicitEdges(context.Background(), nil, &source, &target, 1, nil)
	if e != nil || len(edges) != 1 {
		t.Fatal(edges, e)
	}
	edges, e = s.ExplicitEdges(context.Background(), nil, &source, nil, 10, nil)
	if e != nil || len(edges) != 2 || edges[0].Anchor == nil || *edges[0].Anchor != "x" {
		t.Fatal(edges, e)
	}
	edges, e = s.ExplicitEdges(context.Background(), []string{}, nil, nil, 1, nil)
	if e != nil || len(edges) != 0 || edges == nil {
		t.Fatal(edges, e)
	}
}
func TestExplicitEdgeFailures(t *testing.T) {
	s := NewSnapshotService("", "", "")
	s.open = func(context.Context, string) (*sql.DB, error) { return nil, context.Canceled }
	if _, e := s.ExplicitEdges(context.Background(), nil, nil, nil, 1, nil); e == nil {
		t.Fatal("open")
	}
	s = NewSnapshotService("", snapshotDB(t), "")
	if _, e := s.ExplicitEdges(context.Background(), nil, nil, nil, 1, nil); e == nil {
		t.Fatal("query")
	}
}
func TestIntegerText(t *testing.T) {
	for n, w := range map[int64]string{0: "0", 12: "12", -3: "-3"} {
		if integerText(n) != w {
			t.Fatal(n, integerText(n))
		}
	}
}
