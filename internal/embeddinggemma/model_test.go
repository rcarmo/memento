package embeddinggemma

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/simd"
)

func TestRMSAndRope(t *testing.T) {
	x := []float32{1, 2, -3, 4}
	out := make([]float32, 4)
	weight := []float32{2, 3, 4, 5}
	rms(out, x, weight)
	for i, v := range x {
		want := float64(v) * float64(weight[i]) / math.Sqrt(7.5+1e-6)
		if math.Abs(float64(out[i])-want) > 1e-6 {
			t.Fatal(i, out[i], want)
		}
	}
	x = []float32{1, 2, 3, 4}
	before := append([]float32(nil), x...)
	rope(x, 0, false)
	for i := range x {
		if x[i] != before[i] {
			t.Fatal(x)
		}
	}
	rope(x, 37, false)
	var a, b float64
	for i := range x {
		a += float64(x[i]) * float64(x[i])
		b += float64(before[i]) * float64(before[i])
	}
	if math.Abs(a-b) > 1e-5 {
		t.Fatal(a, b)
	}
	for _, x := range []float32{-8, -2, -.1, 0, .1, 2, 8} {
		want := .5 * float64(x) * (1 + math.Tanh(math.Sqrt(2/math.Pi)*(float64(x)+.044715*math.Pow(float64(x), 3))))
		if math.Abs(float64(gelu(x))-want) > 1e-6 {
			t.Fatal(x, gelu(x), want)
		}
	}
}

func TestAttentionOracle(t *testing.T) {
	eng, _ := simd.New("scalar")
	const n = 1027
	const head = 2
	const kv = 2
	q, k, v := make([]float32, n*4*head), make([]float32, n*kv*head), make([]float32, n*kv*head)
	for i := range q {
		q[i] = float32(i%17-8) / 17
	}
	for i := range k {
		k[i] = float32(i%13-6) / 13
		v[i] = float32(i%19-9) / 19
	}
	dst := make([]float32, len(q))
	scores := make([]float32, n)
	for _, sliding := range []bool{false, true} {
		if err := attention(context.Background(), dst, q, k, v, n, head, kv, sliding, scores, eng); err != nil {
			t.Fatal(err)
		}
		for _, p := range []int{0, 511, 512, 513, 1026} {
			for h := 0; h < 4; h++ {
				lo, hi := 0, n
				if sliding {
					lo = max(0, p-512)
					hi = min(n, p+513)
				}
				var denom float64
				want := make([]float64, head)
				for j := lo; j < hi; j++ {
					var dot float64
					for d := 0; d < head; d++ {
						dot += float64(q[p*4*head+h*head+d]) * float64(k[j*kv*head+(h/2)*head+d])
					}
					s := math.Exp(dot)
					denom += s
					for d := 0; d < head; d++ {
						want[d] += s * float64(v[j*kv*head+(h/2)*head+d])
					}
				}
				for d := 0; d < head; d++ {
					if math.Abs(float64(dst[p*4*head+h*head+d])-want[d]/denom) > 2e-6 {
						t.Fatal(sliding, p, h, d)
					}
				}
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := attention(ctx, dst, q, k, v, n, head, kv, true, scores, eng); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func realModel(t testing.TB, backend string) *Model {
	t.Helper()
	path := os.Getenv("EMBEDDINGGEMMA_MODEL")
	if path == "" {
		t.Skip("EMBEDDINGGEMMA_MODEL not set")
	}
	m, err := Load(path, backend)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func TestModelValidation(t *testing.T) {
	if _, err := NewModel(nil, "auto"); err == nil {
		t.Fatal("nil")
	}
	m := realModel(t, "auto")
	w := &Workspace{}
	for _, dims := range []int{-1, 1, 769} {
		if _, err := m.EmbedTokens(context.Background(), []int{2, 1}, dims, w); err == nil {
			t.Fatal(dims)
		}
	}
	for _, ids := range [][]int{nil, {-1}, {262144}, make([]int, MaxTokens+1)} {
		if _, err := m.EmbedTokens(context.Background(), ids, 768, w); err == nil {
			t.Fatal("invalid IDs accepted")
		}
	}
	if _, err := m.EmbedTokens(nil, []int{2}, 768, w); err == nil {
		t.Fatal("nil context")
	}
	if _, err := m.EmbedTokens(context.Background(), []int{2}, 768, nil); err == nil {
		t.Fatal("nil workspace")
	}
	w.mu.Lock()
	if _, err := m.EmbedTokens(context.Background(), []int{2}, 768, w); err == nil {
		t.Fatal("busy")
	}
	w.mu.Unlock()
	if _, err := m.Tokenize(context.Background(), "hi", Options{Role: "other"}); err == nil {
		t.Fatal("role")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Embed(ctx, "hi", Options{}, w); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// Cancellation is observed during inference, not just at admission.
	ctx, cancel = context.WithCancel(context.Background())
	timer := time.AfterFunc(5*time.Millisecond, cancel)
	defer timer.Stop()
	if _, err := m.Embed(ctx, "hello world", Options{}, w); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.EmbedTokens(context.Background(), []int{2}, 768, w); err == nil {
		t.Fatal("closed")
	}
}

type goldenFixture struct {
	Cases []struct {
		Name, Text, Role, Title string
		Dimensions              int
		Tokens                  []int
		Embedding               []float32
	} `json:"cases"`
}

func TestReferenceEmbeddings(t *testing.T) {
	m := realModel(t, "auto")
	data, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture goldenFixture
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	w := &Workspace{}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			opts := Options{Role: Role(c.Role), Title: c.Title, Dimensions: c.Dimensions}
			ids, err := m.Tokenize(context.Background(), c.Text, opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(ids) != len(c.Tokens) {
				t.Fatal(len(ids), len(c.Tokens))
			}
			for i := range ids {
				if ids[i] != c.Tokens[i] {
					t.Fatal(i, ids[i], c.Tokens[i])
				}
			}
			v, err := m.EmbedTokens(context.Background(), ids, c.Dimensions, w)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]float32(nil), c.Embedding[:len(v)]...)
			var norm float64
			for _, x := range want {
				norm += float64(x) * float64(x)
			}
			for i := range want {
				want[i] /= float32(math.Sqrt(norm))
			}
			compareEmbedding(t, v, want)
		})
	}
}
func compareEmbedding(t *testing.T, a, b []float32) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatal(len(a), len(b))
	}
	var dot, aa, bb, sq, mx float64
	for i, x := range a {
		y := float64(b[i])
		d := float64(x) - y
		dot += float64(x) * y
		aa += float64(x) * float64(x)
		bb += y * y
		sq += d * d
		mx = math.Max(mx, math.Abs(d))
	}
	cos := dot / math.Sqrt(aa*bb)
	rms := math.Sqrt(sq / float64(len(a)))
	t.Logf("cos=%.10f rms=%g max=%g", cos, rms, mx)
	if cos < 1-1e-9 || rms > 3e-7 || mx > 1e-6 {
		t.Fatal("reference mismatch")
	}
}

func TestModelAdmissionAndClose(t *testing.T) {
	m := realModel(t, "auto")
	f, err := Open(os.Getenv("EMBEDDINGGEMMA_MODEL"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, key := range []string{"gemma-embedding2.attention.causal", "gemma-embedding2.embedding_length", "gemma-embedding2.rope.freq_base", "gemma-embedding2.pooling_type"} {
		old := f.metadata[key]
		f.metadata[key] = nil
		if _, err := NewModel(f, "auto"); err == nil {
			t.Fatal(key)
		}
		f.metadata[key] = old
	}
	ids, _ := m.Tokenize(context.Background(), "hello", Options{})
	ctx, cancel := context.WithCancel(context.Background())
	w := &Workspace{}
	done := make(chan error, 1)
	go func() { _, err := m.EmbedTokens(ctx, ids, 768, w); done <- err }()
	deadline := time.Now().Add(time.Second)
	for {
		if !w.mu.TryLock() {
			break
		}
		w.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("inference did not start")
		}
		time.Sleep(time.Millisecond)
	}
	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close/inference deadlock")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close blocked")
	}
}

func TestModelConcurrentAndScalar(t *testing.T) {
	m := realModel(t, "auto")
	scalar := realModel(t, "scalar")
	ctx := context.Background()
	want, err := scalar.Embed(ctx, "hello", Options{}, &Workspace{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := m.Embed(ctx, "hello", Options{}, &Workspace{})
			if err != nil {
				t.Error(err)
				return
			}
			compareEmbedding(t, got, want)
		}()
	}
	wg.Wait()
}

func BenchmarkEmbedding(b *testing.B) {
	m := realModel(b, "auto")
	ids, err := m.Tokenize(context.Background(), "task: search result | query: Where is the project documentation?", Options{})
	if err != nil {
		b.Fatal(err)
	}
	w := &Workspace{}
	if _, err = m.EmbedTokens(context.Background(), ids, 768, w); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = m.EmbedTokens(context.Background(), ids, 768, w); err != nil {
			b.Fatal(err)
		}
	}
}
