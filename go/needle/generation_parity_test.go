package needle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestRealNeedleGeneration(t *testing.T) {
	path, tokenizerPath := os.Getenv("NEEDLE_MODEL_PATH"), os.Getenv("NEEDLE_TOKENIZER_PATH")
	if path == "" || tokenizerPath == "" {
		t.Skip("set pinned model/tokenizer paths for required inference CI")
	}
	raw, err := os.ReadFile("../testdata/parity/needle-generation.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		ModelSHA string `json:"model_sha256"`
		Tools    string `json:"tools_json"`
		Cases    []struct {
			Query, Output string
			Checkpoints   []string
		}
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	if hex.EncodeToString(h[:]) != ref.ModelSHA {
		t.Fatal("model mismatch")
	}
	m, err := FromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRouter(m)
	if err != nil {
		t.Fatal(err)
	}
	tokenizer, err := LoadTokenizer(tokenizerPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range ref.Cases {
		t.Run(c.Query, func(t *testing.T) {
			labels := []string{}
			got, err := r.Generate(tokenizer, c.Query, ref.Tools, DefaultGenerationOptions(), func(label string) error { labels = append(labels, label); return nil })
			if err != nil {
				t.Fatal(err)
			}
			if got != c.Output {
				t.Errorf("got %s\nwant %s", got, c.Output)
			}
			if !reflect.DeepEqual(labels, c.Checkpoints) {
				t.Errorf("checkpoint sequence differs: %d != %d", len(labels), len(c.Checkpoints))
			}
			t.Log(got)
		})
	}
}

func TestRealNeedleCorpus(t *testing.T) {
	if os.Getenv("NEEDLE_FULL_CORPUS") != "1" {
		t.Skip("set NEEDLE_FULL_CORPUS=1 for mandatory full release/parity gate")
	}
	path, tokenizerPath := os.Getenv("NEEDLE_MODEL_PATH"), os.Getenv("NEEDLE_TOKENIZER_PATH")
	if path == "" || tokenizerPath == "" {
		t.Fatal("corpus gate requires model paths")
	}
	raw, err := os.ReadFile("../testdata/parity/needle-corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		ModelSHA string `json:"model_sha256"`
		Tools    string `json:"tools_json"`
		Cases    []struct {
			Query  string
			Output *string
			Error  *string
		}
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	if len(ref.Cases) != 360 {
		t.Fatal("incomplete held-out corpus")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != ref.ModelSHA {
		t.Fatal("wrong model")
	}
	model, err := FromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(model)
	if err != nil {
		t.Fatal(err)
	}
	tokenizer, err := LoadTokenizer(tokenizerPath)
	if err != nil {
		t.Fatal(err)
	}
	options := DefaultGenerationOptions()
	options.MaxGenerated = 128
	matched := 0
	for i, c := range ref.Cases {
		got, err := router.Generate(tokenizer, c.Query, ref.Tools, options, nil)
		if c.Error != nil {
			if err == nil || err.Error() != *c.Error {
				t.Errorf("case %d %q: got error %v want %q", i, c.Query, err, *c.Error)
			} else {
				matched++
			}
		} else if err != nil || got != *c.Output {
			t.Errorf("case %d %q\ngot %q (%v)\nwant %q", i, c.Query, got, err, *c.Output)
		} else {
			matched++
		}
		if (i+1)%30 == 0 {
			t.Logf("completed %d/360; exact matches %d", i+1, matched)
		}
	}
	t.Logf("Exact Rust output/error matches: %d/%d", matched, len(ref.Cases))
}
