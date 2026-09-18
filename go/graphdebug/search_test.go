package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/rcarmo/memento/go/access"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func searchDB(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "search.sqlite")
	db, _ := sql.Open("sqlite", p)
	defer db.Close()
	_, err := db.Exec(`CREATE TABLE concepts(id TEXT PRIMARY KEY,path TEXT,title TEXT,type TEXT,tags_json TEXT);CREATE VIRTUAL TABLE concept_fts USING fts5(concept_id UNINDEXED,title,description,aliases,tags,body,path);INSERT INTO concepts VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','/a.md','Alpha','concept','["one"]'),('private','/private/b.md','Beta','concept','[]'),('trash','/trash/a.md','Trash','concept','[]');INSERT INTO concept_fts VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','Alpha','','','one','hello   world','/a.md'),('private','Beta','','','','hello world','/private/b.md'),('trash','Trash','','','','hello world','/trash/a.md')`)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestSearchFixture(t *testing.T) {
	s := NewSnapshotService("", searchDB(t), "")
	policy := access.EffectivePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	got, err := s.Search(context.Background(), " hello world ", &policy)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Search SearchResults `json:"search"`
	}
	raw, _ := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err = json.Unmarshal(raw, &fixture); err != nil || !reflect.DeepEqual(got, fixture.Search) {
		t.Fatal(got, fixture.Search, err)
	}
}
func TestSearchHelpers(t *testing.T) {
	if _, err := plainFTSQuery("---"); err == nil {
		t.Fatal("words")
	}
	if _, err := NewSnapshotService("", "", "").Search(context.Background(), "---", nil); err == nil {
		t.Fatal("search words")
	}
	query, err := plainFTSQuery("hello 世界")
	if err != nil || query != `"hello" "世界"` {
		t.Fatal(query, err)
	}
	if graphSnippet(" a \n b ") != "a b" {
		t.Fatal("compact")
	}
	long := strings.Repeat("😀", 241)
	if got := graphSnippet(long); len([]rune(got)) != 240 {
		t.Fatal(len([]rune(got)))
	}
}
func TestSearchFaultBranches(t *testing.T) {
	if snapshotFaultRegistered.CompareAndSwap(false, true) {
		sql.Register("graphdebug-fault", snapshotFaultDriver{})
	}
	s := NewSnapshotService("", "derived", "")
	for _, mode := range []string{"query", "scan", "rows"} {
		s.open = faultOpen(t, mode)
		if _, err := s.Search(context.Background(), "word", nil); err == nil {
			t.Fatal(mode)
		}
	}
	s.open = faultOpen(t, "ok")
	got, err := s.Search(context.Background(), "word", nil)
	if err != nil || len(got.Results) != 1 || got.Results[0].Snippet != "Title" {
		t.Fatal(got, err)
	}
}
func TestSearchErrors(t *testing.T) {
	s := NewSnapshotService("", filepath.Join(t.TempDir(), "missing"), "")
	if _, err := s.Search(context.Background(), "word", nil); err == nil {
		t.Fatal("open")
	}
	p := searchDB(t)
	db, _ := sql.Open("sqlite", p)
	_, _ = db.Exec("UPDATE concepts SET tags_json='bad'")
	db.Close()
	s = NewSnapshotService("", p, "")
	if _, err := s.Search(context.Background(), "hello", nil); err == nil {
		t.Fatal("tags")
	}
}
