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

func fixtureControlDB(t *testing.T) string {
	path := emptyControlDB(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`INSERT INTO proposals VALUES('one','draft','{"path":"/a.md"}','reader'),('two','applied','{"changes":[{"concept_path":"/a.md"},{"new_path":"/b.md"}]}','reader')`); err != nil {
		t.Fatal(err)
	}
	return path
}
func emptyControlDB(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("CREATE TABLE proposals(proposal_id TEXT,status TEXT,patch_json TEXT,author_principal TEXT)"); err != nil {
		t.Fatal(err)
	}
	return path
}
func nodeDB(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "derived.sqlite")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	sql := `CREATE TABLE index_state(key TEXT PRIMARY KEY,value TEXT,updated_at TEXT);INSERT INTO index_state VALUES('repo_revision','main','now'),('index_revision','old','now'),('semantic_embedding_revision','embed','now');CREATE TABLE concepts(id TEXT PRIMARY KEY,path TEXT,type TEXT,title TEXT,status TEXT,tags_json TEXT,updated_at TEXT,repo_revision TEXT,body TEXT,content_hash TEXT);CREATE TABLE graph_metrics(concept_id TEXT PRIMARY KEY,inbound_degree INTEGER,outbound_degree INTEGER,broken_link_count INTEGER,orphan_flag INTEGER);CREATE TABLE concept_embeddings(concept_id TEXT PRIMARY KEY,status TEXT,model_id TEXT,dimensions INTEGER,embedding_revision TEXT,model_revision TEXT,updated_at TEXT,error_message TEXT,embedding_blob BLOB,embedding_norm REAL);CREATE TABLE links(source_id TEXT,target_id TEXT,raw_target TEXT,target_path TEXT,anchor TEXT,link_kind TEXT,resolution_state TEXT,first_seen_revision TEXT,last_checked_revision TEXT);INSERT INTO concepts VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','/a.md','concept','Alpha','active','["one"]','2026-01-02T00:00:00Z','main','Body','ha'),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','/b.md','concept','Beta','active','[]','2026-01-03T00:00:00Z','main','Body','hb');INSERT INTO graph_metrics VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d',9,9,9,0);INSERT INTO concept_embeddings VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','ready','model',3,'main','v1','2026-01-04T00:00:00Z',NULL,NULL,NULL);INSERT INTO links VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','/public/b.md','/public/b.md',NULL,'internal','resolved','r1','r2'),('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d',NULL,'/private/missing.md','/private/missing.md','x','internal','broken','r1','r2'),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e',NULL,'/public/missing.md','/public/missing.md',NULL,'internal','broken','r1','r2');`
	if _, e = db.Exec(sql); e != nil {
		t.Fatal(e)
	}
	files := map[string]string{"a.md": "---\nschema_version: 1\nid: '5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d'\ntype: concept\ntitle: Alpha\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-02T00:00:00Z\nupdated_by: alice\n---\nBody\n", "b.md": "---\nschema_version: 1\nid: '6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e'\ntype: concept\ntitle: Beta\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-03T00:00:00Z\nupdated_by: bob\n---\nBody\n"}
	for name, text := range files {
		if e = os.WriteFile(filepath.Join(root, name), []byte(text), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return root, path
}
func TestNodeFilesystemFields(t *testing.T) {
	root := t.TempDir()
	s := NewSnapshotService(root, "", "")
	if _, err := s.repositoryPath("/../escape"); err == nil {
		t.Fatal("escape")
	}
	if _, err := s.repositoryPath("/missing"); err == nil {
		t.Fatal("missing")
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.repositoryPath("/dir"); err == nil {
		t.Fatal("directory")
	}
	if err := os.WriteFile(filepath.Join(root, "bad.md"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	size, updated, keys, assets := s.filesystemFields("/bad.md", "id")
	if size != 3 || updated != nil || len(keys) != 0 || assets != 0 {
		t.Fatal(size, updated, keys, assets)
	}
	size, updated, keys, assets = s.filesystemFields("/missing.md", "id")
	if size != 0 || updated != nil || len(keys) != 0 || assets != 0 {
		t.Fatal(size, updated, keys, assets)
	}
	concept := "---\nschema_version: 1\nid: '5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d'\ntype: concept\ntitle: Alpha\nstatus: active\nsource_refs:\n  - https://example.com\n  - https://example.com\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-02T00:00:00Z\nupdated_by: alice\n---\nBody"
	if err := os.WriteFile(filepath.Join(root, "good.md"), []byte(concept), 0600); err != nil {
		t.Fatal(err)
	}
	assetDir := filepath.Join(root, ".assets", "id", "docs")
	if err := os.MkdirAll(assetDir, 0700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(assetDir, "1.json"), []byte("{}"), 0600)
	_ = os.WriteFile(filepath.Join(assetDir, "1.zip"), []byte("zip"), 0600)
	_ = os.Mkdir(filepath.Join(assetDir, "ignored.json"), 0700)
	size, updated, keys, assets = s.filesystemFields("/good.md", "id")
	if size == 0 || updated == nil || *updated != "alice" || len(keys) != 1 || assets != 5 {
		t.Fatal(size, updated, keys, assets)
	}
}

func TestNodeQueryVariants(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("INSERT INTO concepts VALUES('trash','/trash/a.md','concept','Trash','active','[]','now','main','Body','h'),('root','','concept','Root','active','[]','now','main','Body','h')")
	db.Close()
	s := NewSnapshotService(root, path, emptyControlDB(t))
	nodes, err := s.Nodes(context.Background(), nil, 10, nil, false)
	if err != nil || len(nodes) != 3 || nodes[2].Namespace != "/" {
		t.Fatal(nodes, err)
	}
	nodes, err = s.Nodes(context.Background(), nil, 10, nil, true)
	if err != nil || len(nodes) != 4 {
		t.Fatal(nodes, err)
	}
	policy := access.EffectivePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/a.md/"}}
	nodes, err = s.Nodes(context.Background(), []string{"5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", "missing"}, 10, &policy, false)
	if err != nil || len(nodes) != 1 {
		t.Fatal(nodes, err)
	}
	nodes, err = s.Nodes(context.Background(), []string{}, 10, nil, false)
	if err != nil || nodes == nil || len(nodes) != 0 {
		t.Fatal(nodes, err)
	}
	db, _ = sql.Open("sqlite", path)
	_, _ = db.Exec("UPDATE concepts SET tags_json='bad' WHERE id='5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d'")
	db.Close()
	if _, err = s.Nodes(context.Background(), nil, 10, nil, false); err == nil {
		t.Fatal("invalid tags")
	}
}

func TestNodesDerivedOpenFailure(t *testing.T) {
	s := NewSnapshotService("", "derived", emptyControlDB(t))
	base := s.open
	s.open = func(ctx context.Context, path string) (*sql.DB, error) {
		if path == "derived" {
			return nil, context.Canceled
		}
		return base(ctx, path)
	}
	if _, err := s.Nodes(context.Background(), nil, 1, nil, false); err == nil {
		t.Fatal("derived")
	}
}
func TestNodesControlFailure(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, filepath.Join(t.TempDir(), "missing.sqlite"))
	if _, err := s.Nodes(context.Background(), nil, 1, nil, false); err == nil {
		t.Fatal("control")
	}
}

func TestNodesFixture(t *testing.T) {
	root, path := nodeDB(t)
	service := NewSnapshotService(root, path, fixtureControlDB(t))
	policy := access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	nodes, e := service.Nodes(context.Background(), nil, 10, &policy, false)
	var fixture struct {
		Nodes       []Node `json:"nodes"`
		ScopedNodes []Node `json:"scoped_nodes"`
		Overview    struct {
			Edges []Edge `json:"edges"`
		} `json:"overview"`
	}
	raw, err := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if e != nil || !reflect.DeepEqual(nodes, fixture.Nodes) {
		a, _ := json.Marshal(nodes)
		b, _ := json.Marshal(fixture.Nodes)
		t.Fatal(string(a), string(b), e)
	}
	edges, err := service.ExplicitEdges(context.Background(), []string{"5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e"}, nil, nil, 10, &policy)
	if err != nil {
		t.Fatal(err)
	}
	scoped := ScopedNodes(nodes, edges)
	if !reflect.DeepEqual(scoped, fixture.ScopedNodes) {
		a, _ := json.Marshal(scoped)
		b, _ := json.Marshal(fixture.ScopedNodes)
		t.Fatal(string(a), string(b))
	}
	overlays := OverlayEdges(scoped, "main", 10)
	if len(overlays) != 1 || !reflect.DeepEqual(overlays[0], fixture.Overview.Edges[2]) {
		t.Fatal(overlays, fixture.Overview.Edges)
	}
}
