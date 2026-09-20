package graphdebug

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

func TestExportFormatFixtures(t *testing.T) {
	var fixture map[string]string
	raw, err := os.ReadFile("../../testdata/parity/graph-export.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	nodes := []Node{{ID: "a", Path: "/a", Title: "A & <B>", Type: "concept", Status: "active", Namespace: "/", UpdatedAt: "now", CombinedBytes: 123, Tags: []string{}, ProvenanceKeys: []string{}, AnomalyIDs: []string{}, Embedding: EmbeddingState{Status: "missing"}, CoarsePosition: Position{1, -.5, 0}}, {ID: "b", Path: "/b", Title: "Beta", Type: "concept", Status: "active", Namespace: "/", UpdatedAt: "now", Tags: []string{}, ProvenanceKeys: []string{}, AnomalyIDs: []string{}, Embedding: EmbeddingState{Status: "missing"}, CoarsePosition: Position{-1, .5, 0}}}
	target, missing := "b", "missing"
	edges := []Edge{{ID: "e", Source: "a", Target: &target, RawTarget: "/b", Kind: "explicit", Canonical: true, Resolution: "resolved", FirstSeenRevision: "r", LastCheckedRevision: "r"}, {ID: "missing", Source: "a", Target: &missing, RawTarget: "/missing", Kind: "explicit", Canonical: true, Resolution: "broken", FirstSeenRevision: "r", LastCheckedRevision: "r"}}
	got, err := ExportJSON(nodes, edges, Revisions{Repository: "r", Index: "r"}, map[string]any{"theme": "dark"})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := base64.StdEncoding.DecodeString(fixture["json"])
	if string(got) != string(want) {
		t.Fatal(string(got), string(want))
	}
	got = ExportSVG(nodes, edges, 400, 200)
	want, _ = base64.StdEncoding.DecodeString(fixture["svg"])
	if string(got) != string(want) {
		t.Fatal(string(got), string(want))
	}
}
func TestExportJSONGuards(t *testing.T) {
	if _, err := ExportJSON(nil, nil, Revisions{}, map[string]any{"token_env": "x"}); err == nil {
		t.Fatal("forbidden")
	}
	if _, err := ExportJSON(nil, nil, Revisions{}, map[string]any{"bad": func() {}}); err == nil {
		t.Fatal("marshal")
	}
	deep := any(nil)
	for range 102 {
		deep = []any{deep}
	}
	if _, err := ExportJSON(nil, nil, Revisions{}, map[string]any{"deep": deep}); err == nil {
		t.Fatal("depth")
	}
	if got, err := ExportJSON(nil, nil, Revisions{}, nil); err != nil || len(got) == 0 {
		t.Fatal(string(got), err)
	}
	if len(ExportSVG(nil, nil, 1, 1)) == 0 {
		t.Fatal("svg")
	}
	target := "missing"
	if len(ExportSVG([]Node{{ID: "a"}}, []Edge{{Source: "a"}, {Source: "a", Target: &target}}, 10, 10)) == 0 {
		t.Fatal("missing edges")
	}
}
