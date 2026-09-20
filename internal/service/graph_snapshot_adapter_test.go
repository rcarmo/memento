package service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/graphdebug"
	_ "modernc.org/sqlite"
)

func controlDBAdapter(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "control.sqlite")
	db, _ := sql.Open("sqlite", p)
	defer db.Close()
	_, err := db.Exec("CREATE TABLE proposals(proposal_id TEXT,status TEXT,patch_json TEXT,author_principal TEXT)")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func nodeDBAdapter(t *testing.T) (string, string) {
	root := t.TempDir()
	p := filepath.Join(root, "derived.sqlite")
	db, _ := sql.Open("sqlite", p)
	defer db.Close()
	_, err := db.Exec(`CREATE TABLE index_state(key TEXT PRIMARY KEY,value TEXT,updated_at TEXT);INSERT INTO index_state VALUES('repo_revision','main','now'),('index_revision','main','now');CREATE TABLE concepts(id TEXT PRIMARY KEY,path TEXT,type TEXT,title TEXT,status TEXT,tags_json TEXT,updated_at TEXT,repo_revision TEXT,body TEXT,content_hash TEXT);INSERT INTO concepts VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','/a.md','concept','A','active','[]','now','main','body','h');CREATE TABLE graph_metrics(concept_id TEXT PRIMARY KEY,inbound_degree INTEGER,outbound_degree INTEGER,broken_link_count INTEGER,orphan_flag INTEGER);CREATE TABLE concept_embeddings(concept_id TEXT PRIMARY KEY,status TEXT,model_id TEXT,dimensions INTEGER,embedding_revision TEXT,model_revision TEXT,updated_at TEXT,error_message TEXT);CREATE TABLE links(source_id TEXT,target_id TEXT,raw_target TEXT,target_path TEXT,anchor TEXT,link_kind TEXT,resolution_state TEXT,first_seen_revision TEXT,last_checked_revision TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(root, "a.md"), []byte("bad"), 0600)
	return root, p
}
func TestGraphSnapshotAdapter(t *testing.T) {
	root, path := nodeDBAdapter(t)
	adapter := GraphSnapshotAdapter{Service: graphdebug.NewSnapshotService(root, path, controlDBAdapter(t))}
	overview, err := adapter.Overview(context.Background(), access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}})
	if err != nil || len(overview.Nodes) != 1 || overview.IndexRevision != "main" || overview.Stale {
		t.Fatal(overview, err)
	}
	if _, err = (GraphSnapshotAdapter{}).Overview(context.Background(), access.EffectivePolicy{}); err == nil {
		t.Fatal("nil")
	}
	adapter.DirectNodeLimit = 0
	adapter.EdgeLimit = 0
	if _, err = (GraphSnapshotAdapter{Service: adapter.Service, DirectNodeLimit: 0, EdgeLimit: 0}).Overview(context.Background(), access.EffectivePolicy{ReadPrefixes: []string{"/"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = (GraphSnapshotAdapter{Service: adapter.Service, DirectNodeLimit: 0, EdgeLimit: 1}).Overview(context.Background(), access.EffectivePolicy{ReadPrefixes: []string{"/"}}); err != nil {
		t.Fatal(err)
	}
}
func TestGraphSnapshotAdapterAggregation(t *testing.T) {
	root, path := nodeDBAdapter(t)
	adapter := GraphSnapshotAdapter{Service: graphdebug.NewSnapshotService(root, path, controlDBAdapter(t)), DirectNodeLimit: 0}
	adapter.DirectNodeLimit = 1
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("INSERT INTO concepts VALUES('other','/b.md','concept','B','active','[]','now','main','body','h')")
	db.Close()
	if _, err := adapter.Overview(context.Background(), access.EffectivePolicy{ReadPrefixes: []string{"/"}}); err == nil {
		t.Fatal("aggregation")
	}
}
func TestGraphSnapshotAdapterFailures(t *testing.T) {
	root, path := nodeDBAdapter(t)
	adapter := GraphSnapshotAdapter{Service: graphdebug.NewSnapshotService(root, path, controlDBAdapter(t)), DirectNodeLimit: 0, EdgeLimit: 1}
	if _, err := adapter.Overview(context.Background(), access.EffectivePolicy{ReadPrefixes: []string{"/"}}); err != nil {
		t.Fatal(err)
	}
	adapter.DirectNodeLimit = 1
	dbPath := path
	_ = dbPath
	adapter.Service = graphdebug.NewSnapshotService(root, filepath.Join(t.TempDir(), "missing"), controlDBAdapter(t))
	if _, err := adapter.Overview(context.Background(), access.EffectivePolicy{}); err == nil {
		t.Fatal("db")
	}
}
