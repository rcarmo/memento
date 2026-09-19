package service

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestSequenceMatcherPython(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/dream-sequence.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		A, B  string
		Ratio float64
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		got := sequenceRatio(c.A, c.B)
		if math.Abs(got-c.Ratio) > 1e-15 {
			t.Fatal(c.A, c.B, got, c.Ratio)
		}
	}
}
func TestSequenceMatcherBranchCorpus(t *testing.T) {
	values := []string{"", "a", "b", "aa", "ab", "ba", "bb", "aba", "bab", "aab", "bba", "abab", "baba", "xabc", "abcx", "xabcy"}
	for _, a := range values {
		for _, b := range values {
			ratio := sequenceRatio(a, b)
			if ratio < 0 || ratio > 1 {
				t.Fatal(a, b, ratio)
			}
		}
	}
}
func TestMergeMatchBlocks(t *testing.T) {
	got := mergeMatchBlocks([]matchBlock{{2, 5, 1}, {0, 3, 1}, {1, 4, 1}, {2, 4, 1}})
	want := []matchBlock{{0, 3, 2}, {2, 4, 1}, {2, 5, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
}
func TestLongestSequenceMatch(t *testing.T) {
	a, b := []rune("abc"), []rune("zabx")
	index := map[rune][]int{'z': {0}, 'a': {1}, 'b': {2}, 'x': {3}}
	block := longestSequenceMatch(a, b, index, 0, len(a), 0, len(b))
	if block != (matchBlock{0, 1, 2}) {
		t.Fatal(block)
	}
	backward := longestSequenceMatch([]rune("aab"), []rune("aab"), map[rune][]int{'b': {2}}, 0, 3, 0, 3)
	if backward != (matchBlock{0, 0, 3}) {
		t.Fatal(backward)
	}
	if sequenceRatio("", "") != 1 || sequenceRatio("a", "") != 0 {
		t.Fatal("empty")
	}
}
