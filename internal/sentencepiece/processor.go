package sentencepiece

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Processor owns the immutable model; calls use independent scalar work buffers.
type Processor struct {
	model    model
	ids      map[string]int
	bytes    [256]int
	minScore float32
	trie     []trieNode
}

// Load reads a SentencePiece model using the Go runtime only.
func Load(path string) (*Processor, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return FromBytes(b)
}

// FromBytes parses only the model fields used by the reference inference runtime.
func FromBytes(data []byte) (*Processor, error) {
	m, err := parseModel(data)
	if err != nil {
		return nil, err
	}
	p := &Processor{model: m, ids: make(map[string]int), minScore: float32(math.Inf(1))}
	for i := range p.bytes {
		p.bytes[i] = -1
	}
	for i, piece := range m.pieces {
		if _, exists := p.ids[piece.Text]; !exists {
			p.ids[piece.Text] = i
		} // Source code is first-wins, despite its stale comment.
		if piece.Kind == Byte {
			if value, ok := pieceByte(piece.Text); ok {
				p.bytes[value] = i
			}
		}
		if piece.Kind == Normal && piece.Score < p.minScore {
			p.minScore = piece.Score
		}
	}
	if math.IsInf(float64(p.minScore), 0) || math.IsNaN(float64(p.minScore)) {
		p.minScore = 0
	}
	p.buildTrie()
	return p, nil
}

// Pieces returns an owned vocabulary copy.
func (p *Processor) Pieces() []Piece { return append([]Piece{}, p.model.pieces...) }

// VocabSize returns the immutable vocabulary size without copying it.
func (p *Processor) VocabSize() int { return len(p.model.pieces) }

// TokenID returns the first matching vocabulary ID without copying the vocab.
func (p *Processor) TokenID(text string) (int, bool) {
	id, ok := p.ids[text]
	return id, ok
}

// IDs returns configured unknown/BOS/EOS/PAD IDs in that order.
func (p *Processor) IDs() (int, int, int, int) {
	return p.model.unk, p.model.bos, p.model.eos, p.model.pad
}

// Piece returns a token's text/category, or false if its ID is outside vocabulary.
func (p *Processor) Piece(id int) (Piece, bool) {
	if id < 0 || id >= len(p.model.pieces) {
		return Piece{}, false
	}
	return p.model.pieces[id], true
}

func pieceByte(piece string) (byte, bool) {
	if len(piece) != 6 || piece[:3] != "<0x" || piece[5] != '>' {
		return 0, false
	}
	v, err := strconv.ParseUint(piece[3:5], 16, 8)
	return byte(v), err == nil
}

type span struct{ start, length, id int }

// Encode returns source-equivalent deterministic token IDs, including byte fallback.
func (p *Processor) Encode(text string) ([]int, error) {
	if !utf8.ValidString(text) {
		return nil, fmt.Errorf("input is not UTF-8")
	}
	normalized := p.model.normalize(text)
	if normalized == "" {
		return []int{}, nil
	}
	var spans []span
	switch p.model.kind {
	case BPE:
		spans = p.bpe([]byte(normalized))
	case Unigram:
		spans = p.unigram([]byte(normalized))
	default:
		return nil, fmt.Errorf("unsupported: Word/Char models")
	}
	ids := make([]int, 0, len(spans))
	previous := false
	for _, s := range spans {
		unknown := s.id == p.model.unk
		if unknown && p.model.byteFallback {
			for _, b := range []byte(normalized[s.start : s.start+s.length]) {
				id := p.bytes[b]
				if id < 0 {
					id = p.model.unk
				}
				ids = append(ids, id)
			}
		} else if !unknown || !previous {
			ids = append(ids, s.id)
		}
		previous = unknown
	}
	return ids, nil
}

// Decode reassembles byte runs, removes control symbols and handles the dummy
// whitespace prefix exactly as the reference SentencePiece processor.
func (p *Processor) Decode(ids []int) (string, error) {
	var out strings.Builder
	var run []byte
	begin, seen := true, false
	flush := func() {
		if len(run) > 0 {
			out.WriteString(lossy(run))
			run = run[:0]
		}
	}
	for _, id := range ids {
		piece, ok := p.Piece(id)
		if !ok {
			return "", fmt.Errorf("invalid model: id %d out of range", id)
		}
		if piece.Kind == Byte {
			if b, ok := pieceByte(piece.Text); ok {
				run = append(run, b)
			}
			continue
		}
		flush()
		if seen || out.Len() > 0 {
			begin = false
		}
		seen = false
		switch piece.Kind {
		case Control:
			continue
		case Unknown:
			out.WriteString(p.model.unkSurface)
			continue
		}
		text := piece.Text
		if begin && (p.model.normalizer.dummy || p.model.normalizer.removeExtra) {
			if strings.HasPrefix(text, spaceSymbol) {
				text = strings.TrimPrefix(text, spaceSymbol)
				seen = true
			}
			if p.model.normalizer.removeExtra {
				seen = false
			}
		}
		out.WriteString(strings.ReplaceAll(text, spaceSymbol, " "))
	}
	flush()
	return out.String(), nil
}
