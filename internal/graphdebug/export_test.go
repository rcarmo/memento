package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/rcarmo/memento/internal/access"
	"os"
	"reflect"
	"testing"
)

func TestExportSelectionFixture(t *testing.T) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision';DELETE FROM concept_embeddings;INSERT INTO concept_embeddings(concept_id,status,model_id,dimensions,embedding_revision,model_revision,updated_at,error_message,embedding_blob,embedding_norm) VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','ready','model',2,'main','v1','now',NULL,?,NULL),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','ready','model',2,'main','v1','now',NULL,?,NULL)", blob(1, 0), blob(.9, .1))
	seedGraphChunks(t, db)
	db.Close()
	s := NewSnapshotService(root, path, fixtureControlDB(t))
	policy := access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	ids := []string{"6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e", "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e"}
	nodes, edges, revisions, err := s.ExportSelection(context.Background(), ids, 2000, 12000, &policy)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Export struct {
			Nodes     []Node    `json:"nodes"`
			Edges     []Edge    `json:"edges"`
			Revisions Revisions `json:"revisions"`
		} `json:"export_selection"`
	}
	raw, _ := os.ReadFile("../../testdata/parity/graph-snapshot-foundation.json")
	if err = json.Unmarshal(raw, &fixture); err != nil || !reflect.DeepEqual(nodes, fixture.Export.Nodes) || !reflect.DeepEqual(edges, fixture.Export.Edges) || !reflect.DeepEqual(revisions, fixture.Export.Revisions) {
		t.Fatal(nodes, edges, revisions, err)
	}
}
func TestExportSelectionGuards(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	for _, ids := range [][]string{nil, {"a", "b"}} {
		limit := 1
		if ids == nil {
			limit = 10
		}
		if _, _, _, err := s.ExportSelection(context.Background(), ids, limit, 10, nil); err == nil {
			t.Fatal(ids)
		}
	}
	if _, _, _, err := s.ExportSelection(context.Background(), []string{"missing"}, 10, 10, nil); err == nil {
		t.Fatal("unknown")
	}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/private/"}}
	if _, _, _, err := s.ExportSelection(context.Background(), []string{"5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d"}, 10, 10, &policy); err == nil {
		t.Fatal("hidden")
	}
}
