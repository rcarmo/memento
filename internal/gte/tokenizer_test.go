package gte

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func testVocab() []string {
	v := make([]string, 110)
	for i := range v {
		v[i] = fmt.Sprintf("t%d", i)
	}
	copy(v[104:], []string{"hello", "world", "!", "##s", "界", "##界"})
	return v
}

func TestRustTokenizerParity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parity/gte-tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Vocab       []string
		MaxSequence int `json:"max_sequence"`
		Text        string
		Tokens      []int
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		tokenizer, err := NewTokenizer(c.Vocab, c.MaxSequence)
		if err != nil {
			t.Fatal(err)
		}
		got, err := tokenizer.Tokenize(c.Text)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, c.Tokens) {
			t.Errorf("%q limit %d: %v != %v", c.Text, c.MaxSequence, got, c.Tokens)
		}
	}
}

func TestTokenizerValidationAndBoundaries(t *testing.T) {
	if _, err := NewTokenizer(nil, 4); err == nil {
		t.Fatal("accepted reserved-token absence")
	}
	if _, err := NewTokenizer(testVocab(), 1); err == nil {
		t.Fatal("accepted tiny sequence")
	}
	bad := testVocab()
	bad[0] = "\xff"
	if _, err := NewTokenizer(bad, 4); err == nil {
		t.Fatal("accepted invalid vocabulary")
	}
	tokenizer, err := NewTokenizer(testVocab(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tokenizer.Tokenize("\xff"); err == nil {
		t.Fatal("accepted invalid input")
	}
	if got := tokenizer.wordpieces("", []int{1}); !reflect.DeepEqual(got, []int{1}) {
		t.Fatal(got)
	}
	input := testVocab()
	tokenizer, _ = NewTokenizer(input, 5)
	input[104] = "changed"
	tokens, _ := tokenizer.Tokenize("hello")
	if !reflect.DeepEqual(tokens, []int{101, 104, 102}) {
		t.Fatal("mutable input vocabulary")
	}
}

func FuzzTokenizer(f *testing.F) {
	for _, s := range []string{"hello", "HELLO world!", "☃界", "\x00", "\xff", "  "} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		tokenizer, _ := NewTokenizer(testVocab(), 16)
		tokens, err := tokenizer.Tokenize(s)
		if err != nil {
			return
		}
		if len(tokens) < 2 || len(tokens) > 16 || tokens[0] != TokenCLS || tokens[len(tokens)-1] != TokenSEP {
			t.Fatal(tokens)
		}
		for _, id := range tokens {
			if id < 0 || id >= len(testVocab()) {
				t.Fatal(id)
			}
		}
	})
}
