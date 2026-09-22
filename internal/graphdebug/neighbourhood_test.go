package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/rcarmo/memento/internal/access"
	"os"
	"testing"
)

func freshFixtureService(t *testing.T) (*SnapshotService, *access.EffectivePolicy) {
	root, path := nodeDB(t)
	db, _ := sql.Open("sqlite", path)
	_, _ = db.Exec("UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision';DELETE FROM concept_embeddings;INSERT INTO concept_embeddings(concept_id,status,model_id,dimensions,embedding_revision,model_revision,updated_at,error_message,embedding_blob,embedding_norm) VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','ready','model',2,'main','v1','now',NULL,?,NULL),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','ready','model',2,'main','v1','now',NULL,?,NULL)", blob(1, 0), blob(.9, .1))
	seedGraphChunks(t, db)
	db.Close()
	policy := &access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	return NewSnapshotService(root, path, fixtureControlDB(t)), policy
}
func TestNeighbourhoodFixture(t *testing.T) {
	s, policy := freshFixtureService(t)
	o := NeighbourhoodOptions{Depth: 1, EdgeLimit: 12000, SemanticNodeLimit: 300, SemanticEdgeLimit: 1500, ExpansionNodeLimit: 2000, Semantic: SemanticConfig{Neighbours: 12, MinSimilarity: .5, NodeLimit: 300, EdgeLimit: 1500}}
	got, err := s.Neighbourhood(context.Background(), "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", policy, o)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Neighbourhood Neighbourhood `json:"neighbourhood"`
	}
	raw, _ := os.ReadFile("../../testdata/parity/graph-snapshot-foundation.json")
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	// Diagnostics are additive and tested by TestFreshSelectionDiagnostics.
	if len(got.Diagnostics) == 0 {
		t.Fatal("missing scoped diagnostics")
	}
	got.Diagnostics = nil
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(fixture.Neighbourhood)
	if string(a) != string(b) {
		t.Fatal(string(a), string(b))
	}
}
func TestNeighbourhoodGuards(t *testing.T) {
	s, policy := freshFixtureService(t)
	o := NeighbourhoodOptions{Depth: 2}
	if _, err := s.Neighbourhood(context.Background(), "x", policy, o); err == nil {
		t.Fatal("depth")
	}
	o.Depth = 1
	o.SemanticNodeLimit = 10
	o.ExpansionNodeLimit = 10
	o.EdgeLimit = 10
	o.SemanticEdgeLimit = 10
	o.Semantic = SemanticConfig{NodeLimit: 10, EdgeLimit: 10, Neighbours: 1}
	if _, err := s.Neighbourhood(context.Background(), "missing", policy, o); err == nil {
		t.Fatal("unknown")
	}
	db, _ := sql.Open("sqlite", s.DerivedDBPath)
	_, _ = db.Exec("INSERT INTO links VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d',NULL,'/missing','/missing',NULL,'internal','broken','r','r')")
	db.Close()
	o.ExpansionNodeLimit = 1
	got, err := s.Neighbourhood(context.Background(), "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e", policy, o)
	if err != nil || len(got.Nodes) != 1 {
		t.Fatal(got, err)
	}
}
