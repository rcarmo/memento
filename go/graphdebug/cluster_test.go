package graphdebug

import (
	"context"
	"encoding/json"
	"github.com/rcarmo/memento/go/access"
	"math"
	"os"
	"testing"
)

func TestClusterExpansionFixture(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, fixtureControlDB(t))
	policy := &access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	o := ClusterOptions{RefreshMaxPaths: 2000, EdgeLimit: 12000, ExpansionNodeLimit: 1, ClusterLimit: 1, Semantic: SemanticConfig{Neighbours: 12, MinSimilarity: .75, NodeLimit: 300, EdgeLimit: 1500}}
	got, err := s.ExpandCluster(context.Background(), "cluster:overflow", policy, o)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Expansion ClusterExpansion `json:"cluster_expansion"`
	}
	raw, _ := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	a, b := got.ParentPosition, fixture.Expansion.ParentPosition
	if math.Abs(a.X-b.X) > 1e-14 || math.Abs(a.Y-b.Y) > 1e-14 || math.Abs(a.Z-b.Z) > 1e-14 {
		t.Fatal(a, b)
	}
	got.ParentPosition = Position{}
	fixture.Expansion.ParentPosition = Position{}
	ga, _ := json.Marshal(got)
	fa, _ := json.Marshal(fixture.Expansion)
	if string(ga) != string(fa) {
		t.Fatal(string(ga), string(fa))
	}
}
func TestClusterExpansionGuards(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	if _, err := s.ExpandCluster(context.Background(), "x", nil, ClusterOptions{Cursor: -1}); err == nil {
		t.Fatal("cursor")
	}
	o := ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 10, ExpansionNodeLimit: 1, ClusterLimit: 10, Semantic: SemanticConfig{NodeLimit: 10, EdgeLimit: 10, Neighbours: 1}}
	if _, err := s.ExpandCluster(context.Background(), "missing", nil, o); err == nil {
		t.Fatal("cluster")
	}
	overview, _ := s.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 10, RefreshMaxPaths: 10, ClusterLimit: 10})
	got, err := s.ExpandCluster(context.Background(), overview.Clusters[0].ID, nil, ClusterOptions{Cursor: 99, RefreshMaxPaths: 10, EdgeLimit: 10, ExpansionNodeLimit: 1, ClusterLimit: 10})
	if err != nil || len(got.Nodes) != 0 || got.NextCursor != nil {
		t.Fatal(got, err)
	}
}
