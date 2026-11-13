package embeddinggemma

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	gemmaBOS               = 2
	gemmaEOS               = 1
	maxTokenizerInputBytes = 1 << 20
)

// Tokenizer is the Gemma 4 BPE tokenizer contained in an EmbeddingGemma GGUF.
// It owns copies of all tokenizer metadata and remains usable after File.Close.
type Tokenizer struct {
	vocab    map[string]int
	merges   map[bpePair]int
	specials []specialToken
}

type bpePair struct{ left, right int32 }
type specialToken struct {
	text string
	id   int
}

// NewTokenizer loads and validates the Gemma 4 BPE vocabulary and merge ranks
// from f. It does not use tokenizer scores or any external tokenizer asset.
func NewTokenizer(f *File) (*Tokenizer, error) {
	if f == nil {
		return nil, errors.New("embeddinggemma: nil GGUF file")
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.metadata["tokenizer.ggml.model"] != "gemma4" {
		return nil, errors.New("embeddinggemma: expected gemma4 tokenizer")
	}

	words, err := stringArray(f.metadata["tokenizer.ggml.tokens"], "tokenizer.ggml.tokens")
	if err != nil {
		return nil, err
	}
	if len(words) <= gemmaBOS {
		return nil, errors.New("embeddinggemma: tokenizer vocabulary lacks BOS/EOS")
	}
	vocab := make(map[string]int, len(words))
	for id, word := range words {
		if !utf8.ValidString(word) || word == "" {
			return nil, fmt.Errorf("embeddinggemma: invalid vocabulary token %d", id)
		}
		if _, exists := vocab[word]; exists {
			return nil, fmt.Errorf("embeddinggemma: duplicate vocabulary token %q", word)
		}
		vocab[word] = id
	}
	if words[gemmaBOS] != "<bos>" || words[gemmaEOS] != "<eos>" {
		return nil, errors.New("embeddinggemma: Gemma BOS/EOS IDs are invalid")
	}

	mergeText, err := stringArray(f.metadata["tokenizer.ggml.merges"], "tokenizer.ggml.merges")
	if err != nil {
		return nil, err
	}
	merges := make(map[bpePair]int, len(mergeText))
	for rank, merge := range mergeText {
		// GGUF follows llama.cpp: the first literal space after byte zero is
		// the separator. Pieces themselves use ▁ rather than ASCII spaces.
		if len(merge) < 3 {
			return nil, fmt.Errorf("embeddinggemma: malformed merge %d", rank)
		}
		at := strings.IndexByte(merge[1:], ' ')
		if at < 0 {
			return nil, fmt.Errorf("embeddinggemma: malformed merge %d", rank)
		}
		at++
		left, lok := vocab[merge[:at]]
		right, rok := vocab[merge[at+1:]]
		if !lok || !rok {
			return nil, fmt.Errorf("embeddinggemma: unknown merge pieces %d", rank)
		}
		pair := bpePair{int32(left), int32(right)}
		// llama.cpp's emplace preserves the first rank for duplicate pairs.
		if _, exists := merges[pair]; !exists {
			merges[pair] = rank
		}
	}

	types, err := intArray(f.metadata["tokenizer.ggml.token_type"], "tokenizer.ggml.token_type")
	if err != nil {
		return nil, err
	}
	if len(types) != len(words) {
		return nil, errors.New("embeddinggemma: tokenizer token_type length differs from vocabulary")
	}
	specials := make([]specialToken, 0)
	for id, typ := range types {
		// GGUF's CONTROL and USER_DEFINED tokens are Hugging Face added
		// tokens. Both must bypass normalization and BPE.
		if typ == 3 || typ == 4 {
			specials = append(specials, specialToken{text: words[id], id: id})
		}
	}
	sort.Slice(specials, func(i, j int) bool {
		if len(specials[i].text) == len(specials[j].text) {
			return specials[i].id < specials[j].id
		}
		return len(specials[i].text) > len(specials[j].text)
	})
	return &Tokenizer{vocab: vocab, merges: merges, specials: specials}, nil
}

func stringArray(value any, name string) ([]string, error) {
	if a, ok := value.([]string); ok {
		return append([]string(nil), a...), nil
	}
	a, ok := value.([]any)
	if !ok || len(a) == 0 {
		return nil, fmt.Errorf("embeddinggemma: missing or invalid %s", name)
	}
	out := make([]string, len(a))
	for i, value := range a {
		var ok bool
		out[i], ok = value.(string)
		if !ok {
			return nil, fmt.Errorf("embeddinggemma: invalid %s entry %d", name, i)
		}
	}
	return out, nil
}

func intArray(value any, name string) ([]int, error) {
	a, ok := value.([]any)
	if !ok || len(a) == 0 {
		return nil, fmt.Errorf("embeddinggemma: missing or invalid %s", name)
	}
	out := make([]int, len(a))
	for i, value := range a {
		var n uint64
		switch value := value.(type) {
		case uint64:
			n = value
		case int64:
			if value < 0 {
				return nil, fmt.Errorf("embeddinggemma: invalid %s entry %d", name, i)
			}
			n = uint64(value)
		default:
			return nil, fmt.Errorf("embeddinggemma: invalid %s entry %d", name, i)
		}
		if n > 6 {
			return nil, fmt.Errorf("embeddinggemma: invalid %s entry %d", name, i)
		}
		out[i] = int(n)
	}
	return out, nil
}

// Encode applies the Hugging Face Gemma 4 normalizer (ASCII spaces become ▁),
// recognizes GGUF control/user-defined added tokens, then applies ranked BPE.
// BOS (2) and EOS (1) are always emitted. maxTokens includes both; a result
// exceeding it is rejected and never truncated.
func (t *Tokenizer) Encode(ctx context.Context, text string, maxTokens int) ([]int, error) {
	if t == nil {
		return nil, errors.New("embeddinggemma: nil tokenizer")
	}
	if ctx == nil {
		return nil, errors.New("embeddinggemma: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !utf8.ValidString(text) {
		return nil, errors.New("embeddinggemma: input is not valid UTF-8")
	}
	if len(text) > maxTokenizerInputBytes {
		return nil, fmt.Errorf("embeddinggemma: input exceeds %d bytes", maxTokenizerInputBytes)
	}
	if maxTokens < 2 {
		return nil, errors.New("embeddinggemma: maxTokens must allow BOS and EOS")
	}

	out := make([]int, 1, min(maxTokens, len(text)+2))
	out[0] = gemmaBOS
	appendToken := func(id int) error {
		if len(out) >= maxTokens-1 { // reserve EOS and reject, never truncate
			return fmt.Errorf("embeddinggemma: token count exceeds maxTokens %d", maxTokens)
		}
		out = append(out, id)
		return nil
	}

	for cursor := 0; cursor < len(text); {
		if cursor&4095 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		match, at := t.nextSpecial(text, cursor)
		if at < 0 {
			if err := t.encodeText(ctx, text[cursor:], appendToken); err != nil {
				return nil, err
			}
			break
		}
		if at > cursor {
			if err := t.encodeText(ctx, text[cursor:at], appendToken); err != nil {
				return nil, err
			}
		}
		if err := appendToken(match.id); err != nil {
			return nil, err
		}
		cursor = at + len(match.text)
	}
	out = append(out, gemmaEOS)
	return out, nil
}

func (t *Tokenizer) nextSpecial(text string, start int) (specialToken, int) {
	bestAt := -1
	var best specialToken
	for _, special := range t.specials {
		at := strings.Index(text[start:], special.text)
		if at < 0 {
			continue
		}
		at += start
		if bestAt < 0 || at < bestAt {
			bestAt, best = at, special
		}
	}
	return best, bestAt
}

func (t *Tokenizer) encodeText(ctx context.Context, text string, appendToken func(int) error) error {
	// The Gemma 4 normalizer changes only ASCII spaces. Newline runs are BPE
	// pieces of their own, matching llama.cpp's [^\n]+|[\n]+ pre-tokenizer.
	text = strings.ReplaceAll(text, " ", "▁")
	for start := 0; start < len(text); {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := start
		newline := text[start] == '\n'
		for end < len(text) && (text[end] == '\n') == newline {
			end++
		}
		if err := t.encodePiece(ctx, text[start:end], appendToken); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func (t *Tokenizer) encodePiece(ctx context.Context, text string, appendToken func(int) error) error {
	if text == "" {
		return nil
	}
	if id, ok := t.vocab[text]; ok && strings.Trim(text, "\n") == "" {
		return appendToken(id) // Gemma 4's dedicated newline-run workaround.
	}

	symbols := make([]bpeSymbol, 0, utf8.RuneCountInString(text))
	for offset := 0; offset < len(text); {
		_, n := utf8.DecodeRuneInString(text[offset:])
		symbols = append(symbols, bpeSymbol{start: offset, end: offset + n, prev: len(symbols) - 1, next: -1})
		if len(symbols) > 1 {
			symbols[len(symbols)-2].next = len(symbols) - 1
		}
		offset += n
	}
	q := bpeQueue{}
	for i := 1; i < len(symbols); i++ {
		t.addPair(&q, symbols, text, i-1, i)
	}
	heap.Init(&q)
	for steps := 0; q.Len() != 0; steps++ {
		if steps&4095 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		candidate := heap.Pop(&q).(bpeMerge)
		left, right := &symbols[candidate.left], &symbols[candidate.right]
		if left.end == left.start || right.end == right.start || left.end != right.start || text[left.start:right.end] != candidate.text {
			continue
		}
		left.end = right.end
		right.start, right.end = 0, 0
		left.next = right.next
		if right.next >= 0 {
			symbols[right.next].prev = candidate.left
		}
		t.addPair(&q, symbols, text, left.prev, candidate.left)
		t.addPair(&q, symbols, text, candidate.left, left.next)
	}
	for i := 0; i >= 0; i = symbols[i].next {
		if err := ctx.Err(); err != nil {
			return err
		}
		s := symbols[i]
		piece := text[s.start:s.end]
		if id, ok := t.vocab[piece]; ok {
			if err := appendToken(id); err != nil {
				return err
			}
			continue
		}
		for _, b := range []byte(piece) {
			id, ok := t.vocab[fmt.Sprintf("<0x%02X>", b)]
			if !ok {
				return fmt.Errorf("embeddinggemma: missing byte fallback %02X", b)
			}
			if err := appendToken(id); err != nil {
				return err
			}
		}
	}
	return nil
}

type bpeSymbol struct{ start, end, prev, next int }
type bpeMerge struct {
	left, right int
	text        string
	rank        int
}
type bpeQueue []bpeMerge

func (q bpeQueue) Len() int { return len(q) }
func (q bpeQueue) Less(i, j int) bool {
	if q[i].rank != q[j].rank {
		return q[i].rank < q[j].rank
	}
	return q[i].left < q[j].left
}
func (q bpeQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *bpeQueue) Push(v any)   { *q = append(*q, v.(bpeMerge)) }
func (q *bpeQueue) Pop() any     { old := *q; n := len(old); v := old[n-1]; *q = old[:n-1]; return v }
func (t *Tokenizer) addPair(q *bpeQueue, symbols []bpeSymbol, text string, left, right int) {
	if left < 0 || right < 0 || symbols[left].end == symbols[left].start || symbols[right].end == symbols[right].start {
		return
	}
	a, aok := t.vocab[text[symbols[left].start:symbols[left].end]]
	b, bok := t.vocab[text[symbols[right].start:symbols[right].end]]
	if !aok || !bok {
		return
	}
	if rank, ok := t.merges[bpePair{int32(a), int32(b)}]; ok {
		heap.Push(q, bpeMerge{left: left, right: right, text: text[symbols[left].start:symbols[right].end], rank: rank})
	}
}
