package main

import (
	"encoding/json"
	"fmt"
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
	if err = validateCorpus(c); err != nil {
		t.Fatal(err)
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
	if r.Recall1 != .5 || r.Recall3 != 1 || r.Recall10 != 1 || r.MRR != .75 || math.Abs(r.NDCG10-(1+1/math.Log2(3))/2) > 1e-12 || r.EnglishRecall1 != 1 || r.MultilingualRecall1 != 0 || r.Ranks[1].Rank != 2 {
		t.Fatal(r)
	}
}

func TestExpandedCorpus(t *testing.T) {
	b, err := os.ReadFile("corpus-expanded.json")
	if err != nil {
		t.Fatal(err)
	}
	var c corpus
	if err = json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	if err = validateCorpus(c); err != nil {
		t.Fatal(err)
	}
	if len(c.Documents) != 60 || len(c.Queries) != 80 || len(c.Workloads) != 3 {
		t.Fatal("expanded scope changed")
	}
	languages := map[string]int{}
	categories := map[string]int{}
	for _, q := range c.Queries {
		languages[q.Language]++
		categories[q.Category]++
		if len(q.HardNegatives) != 2 {
			t.Fatal(q.ID)
		}
	}
	if len(categories) != 20 || len(languages) != 6 || languages["en"] != 40 {
		t.Fatal(languages, categories)
	}
	for _, lang := range []string{"pt", "es", "fr", "de", "ja"} {
		if languages[lang] != 8 {
			t.Fatal(lang, languages)
		}
	}
}

func TestInvalidCorpus(t *testing.T) {
	valid := func() corpus {
		return corpus{
			Documents: []document{{ID: "a", Text: "answer"}, {ID: "b", Text: "negative"}},
			Queries:   []query{{ID: "q", Language: "en", Text: "question", Relevant: "a", HardNegatives: []string{"b"}}},
			Workloads: []workload{{Name: "test", Role: "query", Text: "text", Repeats: 3}},
		}
	}
	for name, mutate := range map[string]func(*corpus){
		"missing documents":    func(c *corpus) { c.Documents = nil },
		"missing queries":      func(c *corpus) { c.Queries = nil },
		"missing workload":     func(c *corpus) { c.Workloads = nil },
		"duplicate document":   func(c *corpus) { c.Documents[1].ID = "a" },
		"unknown relevance":    func(c *corpus) { c.Queries[0].Relevant = "missing" },
		"unknown negative":     func(c *corpus) { c.Queries[0].HardNegatives = []string{"missing"} },
		"positive as negative": func(c *corpus) { c.Queries[0].HardNegatives = []string{"a"} },
		"duplicate negative":   func(c *corpus) { c.Queries[0].HardNegatives = []string{"b", "b"} },
		"missing language":     func(c *corpus) { c.Queries[0].Language = "" },
		"duplicate query":      func(c *corpus) { c.Queries = append(c.Queries, c.Queries[0]) },
		"invalid repeats":      func(c *corpus) { c.Workloads[0].Repeats = 0 },
		"invalid role":         func(c *corpus) { c.Workloads[0].Role = "invalid" },
	} {
		t.Run(name, func(t *testing.T) {
			c := valid()
			mutate(&c)
			if validateCorpus(c) == nil {
				t.Fatal("accepted invalid corpus")
			}
		})
	}
}

func TestMonolingualRetrieval(t *testing.T) {
	c := corpus{Documents: []document{{ID: "a"}}, Queries: []query{{ID: "q", Language: "en", Relevant: "a"}}}
	r := evaluate(c, [][]float32{{1, 0}}, [][]float32{{1, 0}}, 2)
	if r.MultilingualRecall1 != 0 || r.Recall1 != 1 {
		t.Fatal(r)
	}
	if _, err := json.Marshal(r); err != nil {
		t.Fatal(err)
	}
	for _, v := range [][]float32{{0, 0}, {float32(math.Inf(1)), 0}, {float32(math.NaN()), 0}, {1}} {
		if validVector(v, 2) == nil {
			t.Fatal("accepted", v)
		}
	}
	if err := validVector([]float32{1, 0}, 2); err != nil {
		t.Fatal(err)
	}
}

func TestRetrievalCutoffAndStableTies(t *testing.T) {
	c := corpus{Queries: []query{{ID: "q", Language: "pt", Relevant: "target"}}}
	var docs [][]float32
	for i := 0; i < 11; i++ {
		c.Documents = append(c.Documents, document{ID: fmt.Sprint(i)})
		docs = append(docs, []float32{1, 0})
	}
	c.Documents[10].ID = "target"
	r := evaluate(c, docs, [][]float32{{1, 0}}, 2)
	if r.Ranks[0].Rank != 11 || r.Recall10 != 0 || r.NDCG10 != 0 || math.Abs(r.MRR-1.0/11) > 1e-12 {
		t.Fatal(r)
	}
}

func FuzzCorpusValidation(f *testing.F) {
	f.Add([]byte(`{"documents":[{"id":"a","text":"answer"}],"queries":[{"id":"q","language":"en","text":"question","relevant":"a"}],"workloads":[{"name":"query","role":"query","text":"text","repeats":3}]}`))
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 64<<10 {
			t.Skip()
		}
		var c corpus
		if json.Unmarshal(raw, &c) != nil {
			return
		}
		if err := validateCorpus(c); err != nil {
			return
		}
		if len(c.Documents) == 0 || len(c.Queries) == 0 || len(c.Workloads) == 0 {
			t.Fatal("accepted empty fixture")
		}
		ids := map[string]bool{}
		for _, d := range c.Documents {
			if ids[d.ID] || d.ID == "" {
				t.Fatal(d.ID)
			}
			ids[d.ID] = true
		}
		for _, q := range c.Queries {
			if !ids[q.Relevant] {
				t.Fatal(q.ID)
			}
		}
	})
}
