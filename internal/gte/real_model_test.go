package gte

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"sync"
	"testing"
)

func BenchmarkRealGTEEmbed(b *testing.B) {
	path := os.Getenv("GTE_MODEL_PATH")
	if path == "" {
		b.Skip("set GTE_MODEL_PATH")
	}
	model, err := Load(path)
	if err != nil {
		b.Fatal(err)
	}
	if err = model.SetSIMD("auto"); err != nil {
		b.Fatal(err)
	}
	output := make([]float32, model.Dim())
	// This gate measures steady-state reuse. Charging the first workspace to
	// b.N made B/op depend on host speed; cold allocation is measured separately.
	if err = model.EmbedTo("Memento semantic search allocation profile", output, nil); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err = model.EmbedTo("Memento semantic search allocation profile", output, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRealGTEColdWorkspace measures first-call workspace allocation without
// model loading. Keep it separate from the steady-state allocation budget.
func BenchmarkRealGTEColdWorkspace(b *testing.B) {
	path := os.Getenv("GTE_MODEL_PATH")
	if path == "" {
		b.Skip("set GTE_MODEL_PATH")
	}
	model, err := Load(path)
	if err != nil {
		b.Fatal(err)
	}
	if err = model.SetSIMD("auto"); err != nil {
		b.Fatal(err)
	}
	output := make([]float32, model.Dim())
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		// Empty pools retain their New factories but discard every reusable buffer.
		model.workspaces = sync.Pool{New: model.workspaces.New}
		model.tokenBuffers = sync.Pool{New: model.tokenBuffers.New}
		b.StartTimer()
		if err = model.EmbedTo("Memento semantic search allocation profile", output, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// TestRealGTEModel is explicitly gated on a public, digest-pinned model artefact.
// It supplements, rather than replaces, the always-on synthetic parity suite.
func TestRealGTESIMD(t *testing.T) {
	path := os.Getenv("GTE_MODEL_PATH")
	if path == "" {
		t.Skip("set GTE_MODEL_PATH")
	}
	model, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	scalar, err := model.EmbedBatch([]string{"hello world", "Unicode café 日本語"}, BatchOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = model.SetSIMD("auto"); err != nil {
		t.Fatal(err)
	}
	fast, err := model.EmbedBatch([]string{"hello world", "Unicode café 日本語"}, BatchOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range scalar {
		var maxError float64
		for j := range scalar[i] {
			maxError = math.Max(maxError, math.Abs(float64(scalar[i][j]-fast[i][j])))
		}
		t.Logf("%s max_abs=%g", model.SIMDBackend(), maxError)
		if maxError > 1e-5 {
			t.Fatal(i, maxError)
		}
	}
}
func TestRealGTEModel(t *testing.T) {
	path := os.Getenv("GTE_MODEL_PATH")
	if path == "" {
		t.Skip("set GTE_MODEL_PATH for the required model-parity CI job")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	raw, err := os.ReadFile("../../testdata/parity/gte-real.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ModelSHA string `json:"model_sha256"`
		Texts    []string
		Tokens   [][]int
		Outputs  [][]float32
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if hex.EncodeToString(hash[:]) != c.ModelSHA {
			t.Fatal("reference model digest mismatch")
		}
		m, err := FromBytes(data)
		if err != nil {
			t.Fatal(err)
		}
		if m.Dim() != 384 {
			t.Fatal(m.Dim())
		}
		out, err := m.EmbedBatch(c.Texts, BatchOptions{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		for i, text := range c.Texts {
			tokens, err := m.Tokenize(text)
			if err != nil || !reflect.DeepEqual(tokens, c.Tokens[i]) {
				t.Fatalf("token mismatch %q: %v %v", text, tokens, err)
			}
			var maxError, dot, normA, normB float64
			for d, v := range out[i] {
				reference := c.Outputs[i][d]
				maxError = math.Max(maxError, math.Abs(float64(v-reference)))
				dot += float64(v) * float64(reference)
				normA += float64(v) * float64(v)
				normB += float64(reference) * float64(reference)
			}
			cosine := dot / math.Sqrt(normA*normB)
			t.Logf("text %q: max_abs=%g cosine=%.12f norm=%.9f", text, maxError, cosine, math.Sqrt(normA))
			if maxError > 1e-5 || cosine < 0.999999 || math.Abs(math.Sqrt(normA)-1) > 1e-5 {
				t.Errorf("real model parity exceeds explicit tolerance")
			}
		}
	}
}
