package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/rcarmo/memento/go/access"
	"os"
	"testing"
)

func TestOverviewFixture(t *testing.T) {
	root, path := nodeDB(t)
	service := NewSnapshotService(root, path, fixtureControlDB(t))
	policy := access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	got, err := service.Overview(context.Background(), &policy, OverviewOptions{DirectNodeLimit: 2000, EdgeLimit: 12000})
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Overview Overview `json:"overview"`
	}
	raw, err := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(fixture.Overview)
	if string(a) != string(b) {
		t.Fatal(string(a), string(b))
	}
}
func TestSemanticOverviewFixture(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision'; DELETE FROM concept_embeddings")
	_, _ = db.Exec(`INSERT INTO concept_embeddings(concept_id,status,model_id,dimensions,embedding_revision,model_revision,updated_at,error_message,embedding_blob,embedding_norm) VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','ready','model',2,'main','v1','now',NULL,?,NULL),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','ready','model',2,'main','v1','now',NULL,?,NULL)`, blob(1, 0), blob(.9, .1))
	db.Close()
	service := NewSnapshotService(root, path, fixtureControlDB(t))
	policy := access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	got, err := service.Overview(context.Background(), &policy, OverviewOptions{DirectNodeLimit: 2000, EdgeLimit: 12000, Semantic: SemanticConfig{Neighbours: 12, MinSimilarity: .5, NodeLimit: 300, EdgeLimit: 1500}})
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Overview Overview `json:"fresh_overview"`
	}
	raw, _ := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(fixture.Overview)
	if string(a) != string(b) {
		t.Fatal(string(a), string(b))
	}
}
func TestOverviewOrphanMetric(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("DELETE FROM links")
	db.Close()
	service := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := service.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10})
	if err != nil || got.Metrics.OrphanCount != 2 {
		t.Fatal(got, err)
	}
}
func TestOverviewTruncated(t *testing.T) {
	root, path := nodeDB(t)
	service := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := service.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 10, IncludeTrash: true})
	if err == nil || got.SchemaVersion != 0 {
		t.Fatal(got, err)
	}
}
