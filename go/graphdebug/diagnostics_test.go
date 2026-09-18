package graphdebug

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestDiagnosticFoundationFixture(t *testing.T) {
	var fixture struct {
		Overview struct {
			Diagnostics []Diagnostic `json:"diagnostics"`
		} `json:"overview"`
		Scoped    []Node    `json:"scoped_nodes"`
		Revisions Revisions `json:"revisions"`
		Edges     []Edge    `json:"edges"`
	}
	raw, err := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	got := DiagnoseFoundation(fixture.Scoped, fixture.Edges, fixture.Revisions)
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(fixture.Overview.Diagnostics)
	if string(a) != string(b) {
		t.Fatal(string(a), string(b))
	}
}
func TestDiagnosticFoundationBranches(t *testing.T) {
	stale := "old"
	failed := "boom"
	nodes := []Node{{ID: "a", Namespace: "/a/", Orphan: true, Embedding: EmbeddingState{Status: "error", Error: &failed}}, {ID: "b", Namespace: "/b/", ExplicitOutDegree: 30, Embedding: EmbeddingState{Status: "ready", EmbeddingRevision: &stale}}}
	for i := 0; i < 20; i++ {
		nodes = append(nodes, Node{ID: fmt.Sprintf("low-%02d", i), Namespace: "/a/", ExplicitOutDegree: 1, Embedding: EmbeddingState{Status: "ready", EmbeddingRevision: ptr("main")}})
	}
	got := DiagnoseFoundation(nodes, nil, Revisions{Repository: "main"})
	rules := map[string]bool{}
	for _, item := range got {
		rules[item.Rule] = true
	}
	for _, rule := range []string{"orphan", "isolated_cluster", "high_degree", "embedding_failed", "embedding_stale"} {
		if !rules[rule] {
			t.Fatal(rule, got)
		}
	}
	if percentile(nil, .95) != 0 || percentile([]int{1, 2, 100}, .95) != 100 {
		t.Fatal("percentile")
	}
	_ = DiagnoseFoundation([]Node{{ID: "x", ExplicitOutDegree: 21, Embedding: EmbeddingState{Status: "ready", EmbeddingRevision: ptr("main")}}}, nil, Revisions{Repository: "main"})
	d := diagnostic("x", "info", []string{"b", "a"}, "m", map[string]any{}, map[string]any{}, false)
	if !reflect.DeepEqual(d.ConceptIDs, []string{"a", "b"}) {
		t.Fatal(d)
	}
}
