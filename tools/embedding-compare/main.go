// Offline host-local comparison. Runtime services do not import this package.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcarmo/memento/internal/embeddinggemma"
	"github.com/rcarmo/memento/internal/gte"
)

type document struct{ ID, Title, Text string }
type query struct {
	ID, Language, Text, Relevant, Category string
	HardNegatives                          []string
}
type workload struct {
	Name, Role, Text string
	Repeats          int
}
type corpus struct {
	Description string
	Documents   []document
	Queries     []query
	Workloads   []workload
}
type memory struct{ HeapBytes, HeapObjects, RSSBytes, PeakRSSBytes uint64 }
type timing struct {
	SamplesMS                        []float64
	MedianMS, MinMS, MaxMS, P95MS    float64
	AllocBytesPerOp, AllocsPerOp     float64
	CPUUserMSPerOp, CPUSystemMSPerOp float64
}
type workResult struct {
	Name                           string
	Characters, Tokens, Dimensions int
	Warm                           timing
	Memory                         memory
}
type retrievalResult struct {
	Dimensions                                                                   int
	Queries, EnglishQueries, MultilingualQueries                                 int
	Recall1, Recall3, Recall10, MRR, NDCG10, EnglishRecall1, MultilingualRecall1 float64
	Ranks                                                                        []rank
}
type rank struct {
	Query, Language, Relevant, Category string
	Rank                                int
	Top                                 []hit
}
type hit struct {
	ID    string
	Score float64
}
type report struct {
	GemmaWorkers                                                    int
	Description, AllowedCPUs                                        string
	CPUAccountingAvailable                                          bool
	Engine, Revision, Toolchain, Backend, CorpusSHA256, ModelSHA256 string
	ModelBytes                                                      int64
	GOMAXPROCS                                                      int
	Config                                                          any
	LoadMS, FirstEmbedMS                                            float64
	LoadedMemory, AfterGCMemory                                     memory
	Workloads                                                       []workResult
	Retrieval                                                       []retrievalResult
	RetrievalEmbedMS                                                float64
	RetrievalCPUUserMS, RetrievalCPUSystemMS                        float64
	EmbeddedTexts                                                   int
}

// Reject invalid fixtures before loading either model. Labels are frozen inputs,
// never inferred from model rankings.
func validateCorpus(c corpus) error {
	if len(c.Documents) == 0 || len(c.Queries) == 0 || len(c.Workloads) == 0 {
		return fmt.Errorf("corpus requires documents, queries and workloads")
	}
	ids := map[string]bool{}
	for _, d := range c.Documents {
		if d.ID == "" || strings.TrimSpace(d.Text) == "" || ids[d.ID] {
			return fmt.Errorf("invalid or duplicate document %q", d.ID)
		}
		ids[d.ID] = true
	}
	queries := map[string]bool{}
	for _, q := range c.Queries {
		if q.ID == "" || q.Language == "" || strings.TrimSpace(q.Text) == "" || queries[q.ID] || !ids[q.Relevant] {
			return fmt.Errorf("invalid query or relevance label %q", q.ID)
		}
		queries[q.ID] = true
		seen := map[string]bool{}
		for _, id := range q.HardNegatives {
			if !ids[id] || id == q.Relevant || seen[id] {
				return fmt.Errorf("invalid hard negative %q for %q", id, q.ID)
			}
			seen[id] = true
		}
	}
	for _, w := range c.Workloads {
		if w.Name == "" || w.Text == "" || w.Repeats < 3 || (w.Role != "query" && w.Role != "document") {
			return fmt.Errorf("invalid workload %q", w.Name)
		}
	}
	return nil
}

func validVector(v []float32, dimensions int) error {
	if len(v) != dimensions {
		return fmt.Errorf("output dimension mismatch: %d != %d", len(v), dimensions)
	}
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	if math.IsNaN(norm) || math.IsInf(norm, 0) || math.Abs(norm-1) > .0001 {
		return fmt.Errorf("invalid embedding norm: %g", norm)
	}
	return nil
}

func mem() memory {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	v := memory{HeapBytes: m.HeapAlloc, HeapObjects: m.HeapObjects}
	b, _ := os.ReadFile("/proc/self/status")
	for _, line := range strings.Split(string(b), "\n") {
		p := strings.Fields(line)
		if len(p) < 2 {
			continue
		}
		n, _ := strconv.ParseUint(p[1], 10, 64)
		switch p[0] {
		case "VmRSS:":
			v.RSSBytes = n * 1024
		case "VmHWM:":
			v.PeakRSSBytes = n * 1024
		}
	}
	return v
}
func digest(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), n, err
}
func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
func distribution(samples []float64, bytes, objects uint64) timing {
	x := append([]float64(nil), samples...)
	sort.Float64s(x)
	return timing{SamplesMS: samples, MedianMS: x[len(x)/2], MinMS: x[0], MaxMS: x[len(x)-1], P95MS: x[int(math.Ceil(.95*float64(len(x))))-1], AllocBytesPerOp: float64(bytes) / float64(len(x)), AllocsPerOp: float64(objects) / float64(len(x))}
}
func cosine(a, b []float32, dim int) float64 {
	var dot, aa, bb float64
	for i := 0; i < dim; i++ {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		aa += x * x
		bb += y * y
	}
	return dot / math.Sqrt(aa*bb)
}
func evaluate(c corpus, docs, qs [][]float32, dim int) retrievalResult {
	r := retrievalResult{Dimensions: dim, Queries: len(c.Queries)}
	for i, q := range c.Queries {
		hits := make([]hit, len(docs))
		for j, d := range docs {
			hits[j] = hit{ID: c.Documents[j].ID, Score: cosine(qs[i], d, dim)}
		}
		sort.SliceStable(hits, func(a, b int) bool { return hits[a].Score > hits[b].Score })
		position := 0
		for j, h := range hits {
			if h.ID == q.Relevant {
				position = j + 1
			}
		}
		if position == 0 {
			panic("missing relevance label")
		}
		if position == 1 {
			r.Recall1++
		}
		if position <= 3 {
			r.Recall3++
		}
		if position <= 10 {
			r.Recall10++
			r.NDCG10 += 1 / math.Log2(float64(position)+1)
		}
		r.MRR += 1 / float64(position)
		if q.Language == "en" {
			r.EnglishQueries++
			if position == 1 {
				r.EnglishRecall1++
			}
		} else {
			r.MultilingualQueries++
			if position == 1 {
				r.MultilingualRecall1++
			}
		}
		r.Ranks = append(r.Ranks, rank{Query: q.ID, Language: q.Language, Relevant: q.Relevant, Category: q.Category, Rank: position, Top: hits[:min(10, len(hits))]})
	}
	r.Recall1 /= float64(r.Queries)
	r.Recall3 /= float64(r.Queries)
	r.Recall10 /= float64(r.Queries)
	r.NDCG10 /= float64(r.Queries)
	r.MRR /= float64(r.Queries)
	if r.EnglishQueries != 0 {
		r.EnglishRecall1 /= float64(r.EnglishQueries)
	}
	if r.MultilingualQueries != 0 {
		r.MultilingualRecall1 /= float64(r.MultilingualQueries)
	}
	return r
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	engine := flag.String("engine", "", "gte or gemma")
	modelPath := flag.String("model", "", "checkpoint path")
	corpusPath := flag.String("corpus", "tools/embedding-compare/corpus.json", "fixed corpus")
	profileDir := flag.String("profiles", "", "optional project-owned CPU/heap scratch directory")
	revision := flag.String("revision", "", "source revision")
	mode := flag.String("mode", "all", "all, performance or retrieval")
	// Flags for an explicit shared CPU budget.
	threads := flag.Int("threads", 1, "CPU scheduler budget")
	workers := flag.Int("gemma-workers", 1, "Gemma projection workers")
	flag.Parse()
	if *engine != "gte" && *engine != "gemma" {
		return fmt.Errorf("invalid engine")
	}
	if *mode != "all" && *mode != "performance" && *mode != "retrieval" {
		return fmt.Errorf("invalid mode")
	}
	if *threads < 1 || *threads > 64 || *workers < 1 || *workers > 8 {
		return fmt.Errorf("invalid CPU worker budget")
	}
	runtime.GOMAXPROCS(*threads)
	raw, err := os.ReadFile(*corpusPath)
	if err != nil {
		return err
	}
	var c corpus
	if err = json.Unmarshal(raw, &c); err != nil {
		return err
	}
	if err = validateCorpus(c); err != nil {
		return err
	}
	ch := sha256.Sum256(raw)
	hash, size, err := digest(*modelPath)
	if err != nil {
		return err
	}
	r := report{GemmaWorkers: *workers, Engine: *engine, Revision: *revision, Toolchain: runtime.Version(), GOMAXPROCS: runtime.GOMAXPROCS(0), Backend: "auto/avx2", CorpusSHA256: hex.EncodeToString(ch[:]), ModelSHA256: hash, ModelBytes: size}
	r.Description = c.Description
	_, _, r.CPUAccountingAvailable = processCPU()
	if status, e := os.ReadFile("/proc/self/status"); e == nil {
		for _, line := range strings.Split(string(status), "\n") {
			if strings.HasPrefix(line, "Cpus_allowed_list:") {
				r.AllowedCPUs = strings.TrimSpace(strings.TrimPrefix(line, "Cpus_allowed_list:"))
			}
		}
	}
	var embed func(string, string, string) ([]float32, error)
	var tokenize func(string, string, string) ([]int, error)
	dimensions := 384
	ctx := context.Background()
	start := time.Now()
	if *engine == "gte" {
		m, e := gte.Load(*modelPath)
		if e != nil {
			return e
		}
		if err = m.SetSIMD("auto"); err != nil {
			return err
		}
		r.Config = m.Config()
		r.Backend = string(m.SIMDBackend())
		r.LoadMS = ms(time.Since(start))
		text := func(s, title string) string {
			if title != "" {
				return title + "\n" + s
			}
			return s
		}
		embed = func(s, role, title string) ([]float32, error) { return m.Embed(text(s, title)) }
		tokenize = func(s, role, title string) ([]int, error) { return m.Tokenize(text(s, title)) }
	} else {
		m, e := embeddinggemma.Load(*modelPath, "auto")
		if e != nil {
			return e
		}
		defer m.Close()
		r.LoadMS = ms(time.Since(start))
		r.Config = map[string]int{"layers": 24, "hidden": 512, "dimensions": 768, "max_tokens": 8192}
		dimensions = 768
		w := &embeddinggemma.Workspace{Workers: *workers}
		embed = func(s, role, title string) ([]float32, error) {
			return m.Embed(ctx, s, embeddinggemma.Options{Role: embeddinggemma.Role(role), Title: title}, w)
		}
		tokenize = func(s, role, title string) ([]int, error) {
			return m.Tokenize(ctx, s, embeddinggemma.Options{Role: embeddinggemma.Role(role), Title: title})
		}
	}
	r.LoadedMemory = mem()
	runtime.GC()
	r.AfterGCMemory = mem()
	start = time.Now()
	v, err := embed(c.Workloads[0].Text, c.Workloads[0].Role, "")
	if err != nil {
		return err
	}
	if err = validVector(v, dimensions); err != nil {
		return err
	}
	r.FirstEmbedMS = ms(time.Since(start))
	if *profileDir != "" {
		root := os.Getenv("PROJECT_TMP_ROOT")
		rel, e := filepath.Rel(root, *profileDir)
		if root == "" || e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("profiles must be beneath PROJECT_TMP_ROOT")
		}
		if err = os.MkdirAll(*profileDir, 0755); err != nil {
			return err
		}
		f, e := os.Create(filepath.Join(*profileDir, *engine+".cpu"))
		if e != nil {
			return e
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return err
		}
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
			heap, e := os.Create(filepath.Join(*profileDir, *engine+".heap"))
			if e == nil {
				pprof.WriteHeapProfile(heap)
				heap.Close()
			}
		}()
	}
	for _, w := range c.Workloads {
		if *mode == "retrieval" {
			break
		}
		ids, e := tokenize(w.Text, w.Role, "")
		if e != nil {
			return e
		}
		samples := make([]float64, w.Repeats)
		runtime.GC()
		if _, e = embed(w.Text, w.Role, ""); e != nil {
			return e
		}
		var a, b runtime.MemStats
		runtime.ReadMemStats(&a)
		userBefore, systemBefore, _ := processCPU()
		for i := range samples {
			start = time.Now()
			v, e = embed(w.Text, w.Role, "")
			samples[i] = ms(time.Since(start))
			if e != nil {
				return e
			}
			if err = validVector(v, dimensions); err != nil {
				return err
			}
		}
		userAfter, systemAfter, _ := processCPU()
		runtime.ReadMemStats(&b)
		warm := distribution(samples, b.TotalAlloc-a.TotalAlloc, b.Mallocs-a.Mallocs)
		warm.CPUUserMSPerOp = ms(userAfter-userBefore) / float64(w.Repeats)
		warm.CPUSystemMSPerOp = ms(systemAfter-systemBefore) / float64(w.Repeats)
		r.Workloads = append(r.Workloads, workResult{Name: w.Name, Characters: len([]rune(w.Text)), Tokens: len(ids), Dimensions: dimensions, Warm: warm, Memory: mem()})
		fmt.Fprintf(os.Stderr, "%s %s tokens=%d median_ms=%.3f\n", *engine, w.Name, len(ids), r.Workloads[len(r.Workloads)-1].Warm.MedianMS)
	}
	if *mode != "performance" {
		docs := make([][]float32, len(c.Documents))
		qs := make([][]float32, len(c.Queries))
		start = time.Now()
		userBefore, systemBefore, _ := processCPU()
		for i, d := range c.Documents {
			docs[i], err = embed(d.Text, "document", d.Title)
			if err != nil {
				return err
			}
			if err = validVector(docs[i], dimensions); err != nil {
				return fmt.Errorf("document %s: %w", d.ID, err)
			}
		}
		for i, q := range c.Queries {
			qs[i], err = embed(q.Text, "query", "")
			if err != nil {
				return err
			}
			if err = validVector(qs[i], dimensions); err != nil {
				return fmt.Errorf("query %s: %w", q.ID, err)
			}
		}
		userAfter, systemAfter, _ := processCPU()
		r.RetrievalCPUUserMS = ms(userAfter - userBefore)
		r.RetrievalCPUSystemMS = ms(systemAfter - systemBefore)
		r.RetrievalEmbedMS = ms(time.Since(start))
		r.EmbeddedTexts = len(docs) + len(qs)
		r.Retrieval = append(r.Retrieval, evaluate(c, docs, qs, dimensions))
		if dimensions == 768 {
			for _, d := range []int{512, 384, 256, 128} {
				r.Retrieval = append(r.Retrieval, evaluate(c, docs, qs, d))
			}
		}
	}
	return json.NewEncoder(os.Stdout).Encode(r)
}
