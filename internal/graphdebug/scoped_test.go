package graphdebug

import (
	"reflect"
	"testing"
)

func ptr(value string) *string { return &value }
func TestScopedNodes(t *testing.T) {
	nodes := []Node{{ID: "a", ExplicitInDegree: 9, ExplicitOutDegree: 9, BrokenLinkCount: 9}, {ID: "b"}, {ID: "c"}}
	edges := []Edge{{Source: "a", Target: ptr("b"), Kind: "explicit", Resolution: "resolved", RawTarget: "/b"}, {Source: "a", Kind: "explicit", Resolution: "broken", RawTarget: "/missing"}, {Source: "b", Kind: "explicit", Resolution: "broken", RawTarget: " HTTPS://example.com "}, {Source: "c", Target: ptr("a"), Kind: "semantic_similarity", Resolution: "derived"}, {Source: "b", Target: ptr("c"), Kind: "explicit", Resolution: "stale", RawTarget: "/c"}}
	edges = append(edges, Edge{Source: "a", Kind: "explicit", Resolution: "asset", RawTarget: "references/guide.md"})
	got := ScopedNodes(nodes, edges)
	want := []Node{{ID: "a", ExplicitOutDegree: 1, BrokenLinkCount: 1}, {ID: "b", ExplicitInDegree: 1, BrokenLinkCount: 1}, {ID: "c", Orphan: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
	if nodes[0].ExplicitInDegree != 9 {
		t.Fatal("mutated input")
	}
}
func TestExternalLink(t *testing.T) {
	for _, value := range []string{"http://x", "HTTPS://x", " mailto:x", "tel:x", "data:x", "memory://x", "//host/a"} {
		if !externalLink(value) {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"/a.md", "relative.md", ""} {
		if externalLink(value) {
			t.Fatal(value)
		}
	}
}
