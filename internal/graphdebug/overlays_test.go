package graphdebug

import (
	"reflect"
	"testing"
)

func TestOverlayEdges(t *testing.T) {
	nodes := []Node{{ID: "b", Tags: []string{"x", "y", "x"}, Namespace: "/n/", Type: "concept", ProvenanceKeys: []string{"p", "p"}}, {ID: "a", Tags: []string{"x", "y"}, Namespace: "/n/", Type: "concept", ProvenanceKeys: []string{"p"}}}
	got := OverlayEdges(nodes, "main", 10)
	kinds := []string{}
	for _, edge := range got {
		kinds = append(kinds, edge.Kind)
		if edge.Target == nil || edge.FirstSeenRevision != "main" || edge.Canonical {
			t.Fatal(edge)
		}
	}
	if !reflect.DeepEqual(kinds, []string{"shared_tag", "shared_namespace", "shared_type", "shared_provenance"}) {
		t.Fatal(kinds)
	}
	limited := OverlayEdges(nodes, "main", 2)
	if len(limited) != 2 || limited[0].Kind != "shared_tag" || limited[1].Kind != "shared_namespace" {
		t.Fatal(limited)
	}
	if got := OverlayEdges(nodes, "main", 0); got == nil || len(got) != 0 {
		t.Fatal(got)
	}
	if got := OverlayEdges(nil, "main", 10); got == nil || len(got) != 0 {
		t.Fatal(got)
	}
}
