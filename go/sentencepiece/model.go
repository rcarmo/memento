// Package sentencepiece implements the deterministic SentencePiece inference
// used by Needle. Derived from sentencepiece-rust 0.1.1 (Apache-2.0); see NOTICE.
package sentencepiece

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

const (
	Normal      = 1
	Unknown     = 2
	Control     = 3
	UserDefined = 4
	Unused      = 5
	Byte        = 6
	Unigram     = 1
	BPE         = 2
	Word        = 3
	Char        = 4
)

// Piece is an immutable vocabulary entry with its original float32 score.
type Piece struct {
	Text  string
	Score float32
	Kind  int
}
type normalizerSpec struct {
	charsmap                   []byte
	dummy, removeExtra, escape bool
}
type model struct {
	pieces               []Piece
	kind                 int
	byteFallback, suffix bool
	unk, bos, eos, pad   int
	unkSurface           string
	normalizer           normalizerSpec
	normalizerMap        *charsmap
}

func normalDefaults() normalizerSpec {
	return normalizerSpec{dummy: true, removeExtra: true, escape: true}
}

type protoValue struct {
	wire uint64
	n    uint64
	data []byte
}
type protoReader struct {
	data []byte
	pos  int
}

func (r *protoReader) varint() (uint64, error) {
	var value uint64
	var shift uint
	for {
		if r.pos >= len(r.data) {
			return 0, fmt.Errorf("protobuf decode error: truncated varint")
		}
		if shift >= 64 {
			return 0, fmt.Errorf("protobuf decode error: varint overflow")
		}
		b := r.data[r.pos]
		r.pos++
		value |= uint64(b&127) << shift
		if b&128 == 0 {
			return value, nil
		}
		shift += 7
	}
}
func (r *protoReader) take(n uint64) ([]byte, error) {
	if n > uint64(len(r.data)-r.pos) {
		return nil, fmt.Errorf("protobuf decode error: length-delimited field out of bounds")
	}
	v := r.data[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return v, nil
}
func (r *protoReader) next() (uint32, protoValue, error) {
	k, err := r.varint()
	if err != nil {
		return 0, protoValue{}, err
	}
	v := protoValue{wire: k & 7}
	switch v.wire {
	case 0:
		v.n, err = r.varint()
	case 1:
		v.data, err = r.take(8)
	case 2:
		var n uint64
		n, err = r.varint()
		if err == nil {
			v.data, err = r.take(n)
		}
	case 5:
		v.data, err = r.take(4)
	default:
		err = fmt.Errorf("protobuf decode error: unsupported wire type %d", v.wire)
	}
	return uint32(k >> 3), v, err
}
func (v protoValue) integer(fallback int) int {
	if v.wire != 0 {
		return fallback
	}
	return int(int32(v.n))
}
func (v protoValue) boolean(fallback bool) bool {
	if v.wire != 0 {
		return fallback
	}
	return v.n != 0
}
func (v protoValue) text() string {
	if v.wire != 2 {
		return ""
	}
	return lossy(v.data)
}

// Lossy UTF-8 uses maximal valid prefixes like Rust String::from_utf8_lossy.
func lossy(data []byte) string {
	var out strings.Builder
	for len(data) > 0 {
		r, n := utf8.DecodeRune(data)
		if r != utf8.RuneError || n > 1 {
			out.Write(data[:n])
			data = data[n:]
			continue
		}
		n = 1
		first := data[0]
		length := utf8Length(first)
		for n < length && n < len(data) {
			b := data[n]
			if b < 128 || b > 191 {
				break
			}
			if n == 1 && ((first == 0xe0 && b < 0xa0) || (first == 0xed && b > 0x9f) || (first == 0xf0 && b < 0x90) || (first == 0xf4 && b > 0x8f)) {
				break
			}
			n++
		}
		out.WriteRune(utf8.RuneError)
		data = data[n:]
	}
	return out.String()
}
func utf8Length(b byte) int {
	switch {
	case b <= 0x7f:
		return 1
	case b >= 0xc2 && b <= 0xdf:
		return 2
	case b >= 0xe0 && b <= 0xef:
		return 3
	case b >= 0xf0 && b <= 0xf4:
		return 4
	default:
		return 1
	}
}

func parseModel(data []byte) (model, error) {
	m := model{kind: Unigram, unk: 0, bos: 1, eos: 2, pad: -1, unkSurface: " ⁇ ", normalizer: normalDefaults()}
	r := protoReader{data: data}
	for r.pos < len(data) {
		number, v, err := r.next()
		if err != nil {
			return m, err
		}
		if v.wire != 2 {
			continue
		}
		switch number {
		case 1:
			p, err := parsePiece(v.data)
			if err != nil {
				return m, err
			}
			m.pieces = append(m.pieces, p)
		case 2:
			if err := parseTrainer(v.data, &m); err != nil {
				return m, err
			}
		case 3:
			n, err := parseNormalizer(v.data)
			if err != nil {
				return m, err
			}
			m.normalizer = n
		}
	}
	if len(m.pieces) == 0 {
		return m, fmt.Errorf("invalid model: model contains no pieces")
	}
	m.normalizerMap = decodeCharsmap(m.normalizer.charsmap)
	return m, nil
}
func parsePiece(data []byte) (Piece, error) {
	p := Piece{Kind: Normal}
	r := protoReader{data: data}
	for r.pos < len(data) {
		n, v, err := r.next()
		if err != nil {
			return p, err
		}
		switch n {
		case 1:
			p.Text = v.text()
		case 2:
			p.Score = 0
			if v.wire == 5 {
				p.Score = math.Float32frombits(binary.LittleEndian.Uint32(v.data))
			}
		case 3:
			p.Kind = v.integer(Normal)
			if p.Kind < Normal || p.Kind > Byte {
				p.Kind = Normal
			}
		}
	}
	return p, nil
}
func parseNormalizer(data []byte) (normalizerSpec, error) {
	n := normalDefaults()
	r := protoReader{data: data}
	for r.pos < len(data) {
		field, v, err := r.next()
		if err != nil {
			return n, err
		}
		switch field {
		case 2:
			n.charsmap = nil
			if v.wire == 2 {
				n.charsmap = append([]byte(nil), v.data...)
			}
		case 3:
			n.dummy = v.boolean(true)
		case 4:
			n.removeExtra = v.boolean(true)
		case 5:
			n.escape = v.boolean(true)
		}
	}
	return n, nil
}
func parseTrainer(data []byte, m *model) error {
	r := protoReader{data: data}
	for r.pos < len(data) {
		field, v, err := r.next()
		if err != nil {
			return err
		}
		switch field {
		case 3:
			m.kind = v.integer(Unigram)
			if m.kind < Unigram || m.kind > Char {
				m.kind = Unigram
			}
		case 24:
			m.suffix = v.boolean(false)
		case 35:
			m.byteFallback = v.boolean(false)
		case 40:
			m.unk = v.integer(0)
		case 41:
			m.bos = v.integer(1)
		case 42:
			m.eos = v.integer(2)
		case 43:
			m.pad = v.integer(-1)
		case 44:
			if v.wire == 2 {
				m.unkSurface = v.text()
			}
		}
	}
	return nil
}
