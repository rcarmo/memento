package main

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestFixedCorpus(t *testing.T) {
	b, err := os.ReadFile("corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var c corpus
	if err = json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Documents) != 18 || len(c.Queries) != 16 || len(c.Workloads) != 3 {
		t.Fatal("corpus changed; review evaluation scope")
	}
	ids := map[string]bool{}
	for _, d := range c.Documents {
		if d.ID == "" || d.Text == "" || ids[d.ID] {
			t.Fatal(d.ID)
		}
		ids[d.ID] = true
	}
	queries := map[string]bool{}
	for _, q := range c.Queries {
		if !ids[q.Relevant] || q.Text == "" || queries[q.ID] {
			t.Fatal(q.ID)
		}
		queries[q.ID] = true
	}
	for _, w := range c.Workloads {
		if w.Repeats < 3 || w.Text == "" {
			t.Fatal(w.Name)
		}
	}
}
func TestDistribution(t *testing.T) {
	r := distribution([]float64{5, 1, 3}, 300, 9)
	if r.MedianMS != 3 || r.MinMS != 1 || r.MaxMS != 5 || r.P95MS != 5 || r.AllocBytesPerOp != 100 || r.AllocsPerOp != 3 {
		t.Fatal(r)
	}
}
func TestCosineAndRetrieval(t *testing.T) {
	if math.Abs(cosine([]float32{2, 0}, []float32{7, 0}, 2)-1) > 1e-12 {
		t.Fatal("cosine")
	}
	c := corpus{Documents: []document{{ID: "a"}, {ID: "b"}}, Queries: []query{{ID: "q1", Language: "en", Relevant: "a"}, {ID: "q2", Language: "pt", Relevant: "b"}}}
	r := evaluate(c, [][]float32{{1, 0}, {0, 1}}, [][]float32{{1, 0}, {.9, .1}}, 2)
	if r.Recall1 != .5 || r.Recall3 != 1 || r.MRR != .75 || r.EnglishRecall1 != 1 || r.MultilingualRecall1 != 0 || r.Ranks[1].Rank != 2 {
		t.Fatal(r)
	}
}
