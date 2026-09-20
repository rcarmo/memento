package derived

import (
	"reflect"
	"testing"
)

func TestRankSemanticCandidates(t *testing.T) {
	items := []SemanticCandidate{{"b", "/b", .9}, {"a", "/a", .9}, {"c", "/c", .8}}
	got := RankSemanticCandidates(items, nil, false, 0, 10)
	if !reflect.DeepEqual([]string{got[0].ID, got[1].ID, got[2].ID}, []string{"a", "b", "c"}) {
		t.Fatal(got)
	}
	hybrid := RankSemanticCandidates(items, []string{"c", "b"}, true, 0, 10)
	if hybrid[0].ID != "b" || hybrid[1].ID != "c" {
		t.Fatal(hybrid)
	}
	page := RankSemanticCandidates(items, nil, false, 1, 1)
	if len(page) != 2 || page[0].ID != "b" {
		t.Fatal(page)
	}
	for _, tc := range []struct{ offset, limit int }{{9, 1}, {0, -1}, {-1, 0}} {
		got = RankSemanticCandidates(items, nil, false, tc.offset, tc.limit)
		if tc.offset == 9 || tc.limit < 0 {
			if len(got) != 0 {
				t.Fatal(tc, got)
			}
		} else if len(got) != 1 {
			t.Fatal(tc, got)
		}
	}
	if len(items) != 3 || items[0].ID != "b" {
		t.Fatal("mutated")
	}
}
func TestHybridCandidateLessBranches(t *testing.T) {
	left, right := SemanticCandidate{"a", "/a", 1}, SemanticCandidate{"b", "/b", 1}
	if !hybridCandidateLess(left, right, map[string]int{}, map[string]int{"a": 1, "b": 2}) {
		t.Fatal("score")
	}
	if !hybridCandidateLess(left, SemanticCandidate{"b", "/b", .5}, map[string]int{"a": 1, "b": 1}, map[string]int{"a": 1, "b": 1}) {
		t.Fatal("cosine")
	}
	if hybridCandidateLess(left, right, map[string]int{"a": 192, "b": 164}, map[string]int{"a": 3, "b": 4}) {
		t.Fatal("lexical")
	}
	if !hybridCandidateLess(left, right, map[string]int{}, map[string]int{"a": 1, "b": 1}) {
		t.Fatal("path")
	}
	right.Path = "/a"
	if !hybridCandidateLess(left, right, map[string]int{}, map[string]int{"a": 1, "b": 1}) {
		t.Fatal("id")
	}
}
func TestRankSemanticTieBreaks(t *testing.T) {
	items := []SemanticCandidate{{"z", "/same", 1}, {"a", "/same", 1}, {"x", "/x", 1}}
	got := RankSemanticCandidates(items, []string{"none"}, true, 0, 10)
	if got[0].ID != "a" || got[1].ID != "z" {
		t.Fatal(got)
	}
	got = RankSemanticCandidates(items, []string{"z", "a"}, true, 0, 10)
	if got[0].ID != "a" {
		t.Fatal(got)
	}
}
