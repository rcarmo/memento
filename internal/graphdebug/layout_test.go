package graphdebug

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func layoutNode(i int, namespace string) Node {
	id := "node-" + fmt5(i)
	return Node{ID: id, Path: namespace + id + ".md", Title: "Node " + integerText(int64(i)), Type: "project", Status: "active", Namespace: namespace, UpdatedAt: "2026-07-20T00:00:00Z", MarkdownBytes: int64(100 + i), CombinedBytes: int64(100 + i), ExplicitInDegree: 1, ExplicitOutDegree: 1, Tags: []string{}, ProvenanceKeys: []string{}, AnomalyIDs: []string{}, Embedding: EmbeddingState{Status: "missing"}}
}
func fmt5(i int) string { s := integerText(int64(i)); return "00000"[:5-len(s)] + s }
func layoutEdge(a, b int, kind string, similarity *float64) Edge {
	target := "node-" + fmt5(b)
	return Edge{ID: "edge-" + integerText(int64(a)) + "-" + integerText(int64(b)), Source: "node-" + fmt5(a), Target: &target, RawTarget: "/projects/" + target + ".md", Kind: kind, Canonical: kind == "explicit", Resolution: "resolved", FirstSeenRevision: "rev", LastCheckedRevision: "rev", Similarity: similarity}
}
func TestAggregateLayoutBranches(t *testing.T) {
	merged := mergeGroups([]nodeGroup{{"small", []Node{layoutNode(0, "/s/")}}, {"large", []Node{layoutNode(1, "/l/"), layoutNode(2, "/l/")}}}, 2)
	if len(merged) != 2 {
		t.Fatal(merged)
	}
	orphan := layoutNode(9, "/o/")
	orphan.Orphan = true
	if aggregateNode(nodeGroup{"orphan", []Node{orphan}}, "r", 0, 1).OrphanCount != 1 {
		t.Fatal("orphan")
	}
	if got := aggregateNode(nodeGroup{"empty", nil}, "r", 0, 0); got.Namespace != "/" || got.MemberCount != 0 {
		t.Fatal(got)
	}
	nodes := []Node{layoutNode(0, "/a/"), layoutNode(1, "/b/"), layoutNode(2, "/c/")}
	layout := AggregateLayout(nodes, []Edge{{ID: "nil", Source: "node-00000", Kind: "explicit"}, {ID: "missing", Source: "missing", Target: ptr("node-00000"), Kind: "explicit"}, {ID: "internal", Source: "node-00000", Target: ptr("node-00000"), Kind: "explicit"}}, "r", 2)
	if len(layout.Clusters) != 2 || len(layout.Edges) != 0 {
		t.Fatal(layout)
	}
	score1, score2 := .8, 1.0
	semanticNodes := []Node{layoutNode(0, "/a/"), layoutNode(1, "/b/")}
	edges := []Edge{layoutEdge(0, 1, "semantic_similarity", &score1), layoutEdge(0, 1, "semantic_similarity", &score2)}
	layout = AggregateLayout(semanticNodes, edges, "r", 10)
	if len(layout.Edges) != 1 || layout.Edges[0].Similarity == nil || *layout.Edges[0].Similarity != .9 {
		t.Fatal(layout)
	}
	mixed := []Node{layoutNode(0, "/a/"), layoutNode(1, "/b/"), layoutNode(2, "/c/")}
	score := .7
	mixedEdges := []Edge{layoutEdge(1, 2, "explicit", nil), layoutEdge(0, 2, "explicit", nil), layoutEdge(0, 1, "explicit", nil), layoutEdge(0, 2, "shared_tag", nil), layoutEdge(0, 1, "semantic_similarity", &score)}
	ordered := AggregateLayout(mixed, mixedEdges, "r", 10).Edges
	if len(ordered) != 5 {
		t.Fatal(ordered)
	}
	single := AggregateLayout([]Node{layoutNode(0, "/trash/")}, nil, "r", 1)
	if len(single.Clusters) != 1 || single.Clusters[0].ID != "cluster:trash" {
		t.Fatal(single)
	}
}
func TestAggregateLayoutFixtures(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/graph-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]Layout
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	connected := []Node{layoutNode(0, "/projects/"), layoutNode(1, "/projects/"), layoutNode(2, "/systems/"), layoutNode(3, "/systems/")}
	cases := map[string]Layout{"connected": AggregateLayout(connected, []Edge{layoutEdge(0, 1, "explicit", nil), layoutEdge(1, 2, "explicit", nil), layoutEdge(2, 3, "explicit", nil)}, "rev", 10)}
	sparse := []Node{}
	for i := 0; i < 10; i++ {
		sparse = append(sparse, layoutNode(i, "/skills/"))
	}
	cases["sparse"] = AggregateLayout(sparse, nil, "rev", 20)
	overflow := []Node{}
	for i := 0; i < 5; i++ {
		overflow = append(overflow, layoutNode(i, "/namespace-"+integerText(int64(i))+"/"))
	}
	overflow = append(overflow, layoutNode(5, "/trash/"))
	cases["overflow"] = AggregateLayout(overflow, nil, "revision", 3)
	score := .91
	cases["semantic"] = AggregateLayout([]Node{layoutNode(1, "/a/"), layoutNode(2, "/b/")}, []Edge{layoutEdge(1, 2, "semantic_similarity", &score)}, "revision", 10)
	for name, got := range cases {
		want := fixture[name]
		if len(got.Clusters) != len(want.Clusters) {
			t.Fatal(name, got, want)
		}
		for i := range got.Clusters {
			a, b := got.Clusters[i].CoarsePosition, want.Clusters[i].CoarsePosition
			if math.Abs(a.X-b.X) > 1e-14 || math.Abs(a.Y-b.Y) > 1e-14 || math.Abs(a.Z-b.Z) > 1e-14 {
				t.Fatal(name, a, b)
			}
			got.Clusters[i].CoarsePosition = Position{}
			want.Clusters[i].CoarsePosition = Position{}
		}
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(want)
		if string(a) != string(b) {
			t.Fatal(name, string(a), string(b))
		}
	}
}
