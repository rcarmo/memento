package gte

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Chunk returns overlapping text windows measured with this model's WordPiece
// vocabulary, without passing the document through the truncating Tokenize API.
// maxTokens excludes CLS/SEP; maxChars is a per-window Unicode character guard.
// Paragraph/heading boundaries are preferred in the latter half of a window.
// Overlap is up to overlap tokens, rounded down to a whole basic-token boundary.
func (t *Tokenizer) Chunk(text string, maxTokens, overlap, maxChars int) ([]string, error) {
	if !utf8.ValidString(text) {
		return nil, errors.New("input is not UTF-8")
	}
	if maxTokens < 1 || maxTokens > t.maxSequence-2 || overlap < 0 || overlap >= maxTokens || maxChars < 1 {
		return nil, errors.New("invalid embedding chunk limits")
	}
	type atom struct{ start, end, tokens, chars int }
	atoms := []atom{}
	chars := 0
	for at := 0; at < len(text); {
		if whitespace(text[at]) {
			at++
			chars++
			continue
		}
		start, before := at, chars
		if punctuation(text[at]) {
			at++
			chars++
		} else {
			for at < len(text) && !whitespace(text[at]) && !punctuation(text[at]) {
				_, size := utf8.DecodeRuneInString(text[at:])
				at += size
				chars++
			}
		}
		word := text[start:at]
		// Keep ordinary words intact. A single pathological word must not be
		// silently dropped by Tokenize's whole-word rollback rule.
		if chars-before <= maxChars && len(t.wordpieces(asciiLower(word), nil)) <= maxTokens {
			atoms = append(atoms, atom{start, at, len(t.wordpieces(asciiLower(word), nil)), chars})
			continue
		}
		// WordPiece emits at most one token per rune. Splitting overlong basic
		// tokens at this bound guarantees progress even for an all-UNK input.
		for pos, cumulative := start, before; pos < at; {
			end := pos
			for n := 0; n < min(maxTokens, maxChars) && end < at; n++ {
				_, size := utf8.DecodeRuneInString(text[end:at])
				end += size
				cumulative++
			}
			atoms = append(atoms, atom{pos, end, utf8.RuneCountInString(text[pos:end]), cumulative})
			pos = end
		}
	}
	result := []string{}
	for first := 0; first < len(atoms); {
		end, tokens, preferred := first, 0, 0
		baseChars := atoms[first].chars - utf8.RuneCountInString(text[atoms[first].start:atoms[first].end])
		for end < len(atoms) && tokens+atoms[end].tokens <= maxTokens && atoms[end].chars-baseChars <= maxChars {
			tokens += atoms[end].tokens
			end++
			if end < len(atoms) {
				gap := text[atoms[end-1].end:atoms[end].start]
				heading := strings.Contains(gap, "\n") && text[atoms[end].start] == '#'
				if tokens > overlap && heading || tokens >= maxTokens/2 && strings.Contains(gap, "\n\n") {
					preferred = end
				}
			}
		}
		if preferred > first {
			end = preferred
		}
		result = append(result, text[atoms[first].start:atoms[end-1].end])
		if end == len(atoms) {
			break
		}
		next, shared := end, 0
		// Do not drag the previous section into a short new heading section.
		// Ordinary windows/paragraphs retain up to overlap shared tokens.
		heading := text[atoms[end].start] == '#' && strings.Contains(text[atoms[end-1].end:atoms[end].start], "\n")
		for !heading && next > first+1 && shared+atoms[next-1].tokens <= overlap {
			next--
			shared += atoms[next].tokens
		}
		first = next
	}
	return result, nil
}

func asciiLower(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, text)
}
