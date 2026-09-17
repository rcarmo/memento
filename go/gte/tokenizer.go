// Package gte restores the original Go GTE algorithms with Memento's checked
// model/tokenizer behaviour. Scalar correctness precedes SIMD optimisation.
package gte

import (
	"errors"
	"unicode/utf8"
)

const (
	TokenPAD  = 0
	TokenUNK  = 100
	TokenCLS  = 101
	TokenSEP  = 102
	TokenMASK = 103
)

// Tokenizer is immutable and safe for concurrent use. Token outputs are owned
// by the caller. Derived from rcarmo/go-gte tokenizer.go at d2ffa3a, with Rust
// Memento's Unicode character-boundary fix, no unsafe strings or SIMD helpers.
type Tokenizer struct {
	vocab       map[string]int
	maxSequence int
}

// NewTokenizer validates the same reserved-token and sequence limits as GTE1.
func NewTokenizer(vocab []string, maxSequence int) (*Tokenizer, error) {
	if len(vocab) <= TokenMASK {
		return nil, errors.New("invalid model: vocab_size must include reserved token id 103")
	}
	if maxSequence < 2 {
		return nil, errors.New("invalid model: max_seq_len must be at least 2")
	}
	mapped := make(map[string]int, len(vocab))
	for i, word := range vocab {
		if !utf8.ValidString(word) {
			return nil, errors.New("invalid model: vocabulary is not UTF-8")
		}
		mapped[word] = i
	}
	return &Tokenizer{vocab: mapped, maxSequence: maxSequence}, nil
}

func punctuation(b byte) bool {
	return b >= 33 && b <= 47 || b >= 58 && b <= 64 || b >= 91 && b <= 96 || b >= 123 && b <= 126
}
func whitespace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

func basicTokens(text string) []string {
	var tokens []string
	for i := 0; i < len(text); {
		for i < len(text) && whitespace(text[i]) {
			i++
		}
		if i >= len(text) {
			break
		}
		start := i
		if punctuation(text[i]) {
			i++
		} else {
			for i < len(text) && !whitespace(text[i]) && !punctuation(text[i]) {
				i++
			}
		}
		word := []byte(text[start:i])
		for j, c := range word {
			if c >= 'A' && c <= 'Z' {
				word[j] = c + 32
			}
		}
		tokens = append(tokens, string(word))
	}
	return tokens
}

func (t *Tokenizer) wordpieces(word string, out []int) []int {
	for start := 0; start < len(word); {
		found := -1
		end := len(word)
		for end > start {
			candidate := word[start:end]
			if start > 0 {
				candidate = "##" + candidate
			}
			if id, ok := t.vocab[candidate]; ok {
				found = id
				break
			}
			_, size := utf8.DecodeLastRuneInString(word[start:end])
			end -= size
		}
		if found < 0 {
			out = append(out, TokenUNK)
			_, size := utf8.DecodeRuneInString(word[start:])
			start += size
		} else {
			out = append(out, found)
			start = end
		}
	}
	return out
}

// Tokenize performs ASCII lowercasing and Unicode-safe greedy WordPiece. It
// excludes a whole word if its pieces exceed the reserved SEP position.
func (t *Tokenizer) Tokenize(text string) ([]int, error) {
	if !utf8.ValidString(text) {
		return nil, errors.New("input is not UTF-8")
	}
	tokens := make([]int, 1, min(t.maxSequence, len(text)+2))
	tokens[0] = TokenCLS
	for _, word := range basicTokens(text) {
		if len(tokens) >= t.maxSequence-1 {
			break
		}
		previous := len(tokens)
		tokens = t.wordpieces(word, tokens)
		if len(tokens) > t.maxSequence-1 {
			tokens = tokens[:previous]
			break
		}
	}
	// maxSequence>=2 and the loop always reserves one final token.
	tokens = append(tokens, TokenSEP)
	return tokens, nil
}
