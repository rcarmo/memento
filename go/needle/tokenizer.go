package needle

import (
	"fmt"
	"strings"

	"github.com/rcarmo/memento/go/sentencepiece"
)

const (
	TokenPAD      = 0
	TokenEOS      = 1
	TokenBOS      = 2
	TokenUNK      = 3
	TokenToolCall = 4
	TokenTools    = 5
)

// Tokenizer wraps SentencePiece with Needle's explicitly handled control tokens.
type Tokenizer struct{ processor *sentencepiece.Processor }

// TokenizerFromBytes loads the separately checksum-pinned SentencePiece model.
func TokenizerFromBytes(data []byte) (*Tokenizer, error) {
	p, err := sentencepiece.FromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("sentencepiece error: %w", err)
	}
	return &Tokenizer{p}, nil
}

// LoadTokenizer loads Needle's external tokenizer asset.
func LoadTokenizer(path string) (*Tokenizer, error) {
	p, err := sentencepiece.Load(path)
	if err != nil {
		return nil, fmt.Errorf("sentencepiece error: %w", err)
	}
	return &Tokenizer{p}, nil
}

// VocabSize returns the number of SentencePiece pieces.
func (t *Tokenizer) VocabSize() int { return t.processor.VocabSize() }

// Decode preserves SentencePiece byte-run and control token handling.
func (t *Tokenizer) Decode(ids []int) (string, error) {
	text, err := t.processor.Decode(ids)
	if err != nil {
		return "", fmt.Errorf("sentencepiece error: %w", err)
	}
	return text, nil
}

// TokenToID performs the reference's first-matching-piece lookup.
func (t *Tokenizer) TokenToID(text string) (int, bool) { return t.processor.TokenID(text) }

// TokenString returns the string fragment used by Needle's constrained decoder.
func (t *Tokenizer) TokenString(id int) string {
	p, ok := t.processor.Piece(id)
	if !ok || p.Kind == sentencepiece.Control || p.Kind == sentencepiece.Unknown {
		return ""
	}
	if p.Kind == sentencepiece.Byte {
		if len(p.Text) == 6 && strings.HasPrefix(p.Text, "<0x") && strings.HasSuffix(p.Text, ">") {
			var value uint8
			if _, err := fmt.Sscanf(p.Text, "<0x%02x>", &value); err == nil {
				return string(rune(value))
			}
		}
		return ""
	}
	return strings.ReplaceAll(p.Text, "▁", " ")
}

// Encode intercepts <tool_call>/<tools>, with the source's dummy-prefix rules.
func (t *Tokenizer) Encode(text string) ([]int, error) {
	specials := []struct {
		text string
		id   int
	}{{"<tool_call>", TokenToolCall}, {"<tools>", TokenTools}}
	has := false
	for _, s := range specials {
		if strings.Contains(text, s.text) {
			has = true
		}
	}
	if !has {
		return t.fragment(text, false)
	}
	ids := []int{}
	cursor := 0
	afterSpecial := false
	if strings.HasPrefix(text, "<") {
		if id, ok := t.TokenToID("▁"); ok {
			ids = append(ids, id)
		}
	}
	for cursor < len(text) {
		position := -1
		which := 0
		for i, s := range specials {
			if at := strings.Index(text[cursor:], s.text); at >= 0 && (position < 0 || cursor+at < position) {
				position = cursor + at
				which = i
			}
		}
		if position < 0 {
			fragment, err := t.fragment(text[cursor:], afterSpecial)
			if err != nil {
				return nil, err
			}
			ids = append(ids, fragment...)
			break
		}
		if position > cursor {
			fragment, err := t.fragment(text[cursor:position], afterSpecial)
			if err != nil {
				return nil, err
			}
			ids = append(ids, fragment...)
		}
		ids = append(ids, specials[which].id)
		cursor = position + len(specials[which].text)
		afterSpecial = true
	}
	return ids, nil
}
func (t *Tokenizer) fragment(text string, suppress bool) ([]int, error) {
	ids, err := t.processor.Encode(text)
	if err != nil {
		return nil, fmt.Errorf("sentencepiece error: %w", err)
	}
	if suppress && len(ids) > 0 {
		first, ok := t.processor.Piece(ids[0])
		if ok && strings.HasPrefix(first.Text, "▁") {
			if id, exists := t.TokenToID(strings.TrimPrefix(first.Text, "▁")); exists {
				ids[0] = id
			} else if first.Text == "▁" {
				ids = ids[1:]
			}
		}
	}
	return ids, nil
}
