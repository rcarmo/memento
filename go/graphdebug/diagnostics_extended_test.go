package graphdebug

import (
	"encoding/json"
	"os"
	"testing"
)

func TestExtendedDiagnosticsFixture(t *testing.T) {
	var fixture struct {
		Nodes       []Node            `json:"nodes"`
		Edges       []Edge            `json:"edges"`
		Diagnostics []Diagnostic      `json:"diagnostics"`
		Hashes      map[string]string `json:"content_hashes"`
	}
	raw, err := os.ReadFile("../testdata/parity/graph-diagnostics-extended.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	got := DiagnoseGraph(fixture.Nodes, fixture.Edges, Revisions{Repository: "r", Index: "r"}, fixture.Hashes)
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(fixture.Diagnostics)
	if string(a) != string(b) {
		t.Fatal(string(a), string(b))
	}
}
func TestExtendedDiagnosticBranches(t *testing.T) {
	if median(nil) != 0 || median([]int64{3, 1, 2}) != 2 || median([]int64{4, 2}) != 3 {
		t.Fatal("median")
	}
	if absFloat(-2) != 2 || absFloat(2) != 2 || maxFloat(1, 2) != 2 || maxFloat(2, 1) != 2 {
		t.Fatal("numbers")
	}
	nodes := []Node{{ID: "a", Namespace: "/n/", CombinedBytes: 1, Embedding: EmbeddingState{Status: "ready", EmbeddingRevision: ptr("r")}}, {ID: "b", Namespace: "/n/", CombinedBytes: 1, Embedding: EmbeddingState{Status: "ready", EmbeddingRevision: ptr("r")}}, {ID: "c", Namespace: "/n/", CombinedBytes: 1, Embedding: EmbeddingState{Status: "ready", EmbeddingRevision: ptr("r")}}}
	got := DiagnoseGraph(nodes, nil, Revisions{Repository: "r", Index: "r"}, map[string]string{"a": "one"})
	for _, item := range got {
		if item.Rule == "size_outlier" || item.Rule == "tag_drift" || item.Rule == "exact_duplicate" || item.Rule == "namespace_outlier" {
			t.Fatal(item)
		}
	}
}
