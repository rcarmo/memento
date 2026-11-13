package embeddinggemma

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
)

func syntheticGemmaTokenizer(t *testing.T) *Tokenizer {
	t.Helper()
	words := []string{
		"<pad>", "<eos>", "<bos>", "<unk>", "▁", "h", "e", "l", "o",
		"▁h", "he", "ll", "llo", "▁he", "▁hello", "\n", "<0xC3>", "<0xA9>",
		"<x>", "<x|>",
	}
	types := make([]any, len(words))
	for i := range types {
		types[i] = uint64(1)
	}
	types[0], types[1], types[2], types[3], types[18], types[19] = uint64(3), uint64(3), uint64(3), uint64(3), uint64(3), uint64(4)
	f := &File{metadata: map[string]any{
		"tokenizer.ggml.model":      "gemma4",
		"tokenizer.ggml.tokens":     toAny(words),
		"tokenizer.ggml.token_type": types,
		"tokenizer.ggml.merges": toAny([]string{
			"▁ h", "h e", "l l", "ll o", "▁h e", "▁he llo",
		}),
	}}
	tok, err := NewTokenizer(f)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func toAny(values []string) []any {
	out := make([]any, len(values))
	for i := range values {
		out[i] = values[i]
	}
	return out
}

func TestTokenizerGemma4BPEAndAddedTokens(t *testing.T) {
	tok := syntheticGemmaTokenizer(t)
	for _, tc := range []struct {
		text string
		want []int
	}{
		{"", []int{2, 1}},
		{"hello", []int{2, 10, 12, 1}},
		{"hello hello", []int{2, 10, 12, 14, 1}},
		{"hello\nhello", []int{2, 10, 12, 15, 10, 12, 1}},
		{"é", []int{2, 16, 17, 1}},
		// The longer user-defined token must win over its control-token prefix.
		{"hello<x|> hello", []int{2, 10, 12, 19, 14, 1}},
	} {
		got, err := tok.Encode(context.Background(), tc.text, 20)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Encode(%q) = %v, %v; want %v", tc.text, got, err, tc.want)
		}
	}
}

func TestTokenizerValidationLimitsAndCancellation(t *testing.T) {
	tok := syntheticGemmaTokenizer(t)
	for _, tc := range []struct {
		name string
		ctx  context.Context
		text string
		max  int
	}{
		{"invalid UTF-8", context.Background(), "\xff", 10},
		{"too few token slots", context.Background(), "", 1},
		{"does not truncate", context.Background(), "hello hello", 3},
		{"input bound", context.Background(), string(make([]byte, maxTokenizerInputBytes+1)), maxTokenizerInputBytes + 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tok.Encode(tc.ctx, tc.text, tc.max); err == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tok.Encode(ctx, "hello", 10); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	//lint:ignore SA1012 deliberately verify rejection of a nil caller context
	if _, err := tok.Encode(nil, "hello", 10); err == nil {
		t.Fatal("accepted nil context")
	}
}

func TestTokenizerRejectsMalformedMetadata(t *testing.T) {
	valid := syntheticGemmaTokenizer(t)
	if valid == nil {
		t.Fatal("missing tokenizer")
	}
	base := map[string]any{
		"tokenizer.ggml.model":      "gemma4",
		"tokenizer.ggml.tokens":     []string{"<pad>", "<eos>", "<bos>"},
		"tokenizer.ggml.token_type": []any{uint64(3), uint64(3), uint64(3)},
		"tokenizer.ggml.merges":     []string{"a b"},
	}
	for name, mutate := range map[string]func(map[string]any){
		"model":       func(m map[string]any) { m["tokenizer.ggml.model"] = "bpe" },
		"merge":       func(m map[string]any) { m["tokenizer.ggml.merges"] = []string{"bad"} },
		"type length": func(m map[string]any) { m["tokenizer.ggml.token_type"] = []any{uint64(3)} },
		"duplicate token": func(m map[string]any) {
			m["tokenizer.ggml.tokens"] = []string{"<pad>", "<eos>", "<bos>", "<bos>"}
			m["tokenizer.ggml.token_type"] = []any{uint64(3), uint64(3), uint64(3), uint64(1)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := make(map[string]any, len(base))
			for k, v := range base {
				m[k] = v
			}
			mutate(m)
			if _, err := NewTokenizer(&File{metadata: m}); err == nil {
				t.Fatal("accepted malformed metadata")
			}
		})
	}
}

func TestRealGemmaTokenizer(t *testing.T) {
	path := os.Getenv("EMBEDDINGGEMMA_MODEL")
	if path == "" {
		t.Skip("set EMBEDDINGGEMMA_MODEL for the pinned EmbeddingGemma checkpoint")
	}
	f, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tok, err := NewTokenizer(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		text string
		want []int
	}{
		{"hello world", []int{2, 23391, 1902, 1}},
		{"hello\nworld", []int{2, 23391, 107, 12392, 1}},
		{"café", []int{2, 123125, 236859, 1}},
		{"<|image|>", []int{2, 258880, 1}},
	} {
		got, err := tok.Encode(t.Context(), tc.text, 32)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %v, %v; want %v", tc.text, got, err, tc.want)
		}
	}
}
