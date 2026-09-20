package sentencepiece

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func varint(v uint64) []byte {
	var b [10]byte
	n := binary.PutUvarint(b[:], v)
	return append([]byte{}, b[:n]...)
}
func field(n int, v []byte) []byte {
	b := varint(uint64(n<<3 | 2))
	b = append(b, varint(uint64(len(v)))...)
	return append(b, v...)
}
func number(n int, v uint64) []byte { return append(varint(uint64(n<<3)), varint(v)...) }
func pieceData(p Piece) []byte {
	b := field(1, []byte(p.Text))
	b = append(b, byte(2<<3|5))
	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(p.Score))
	return append(b, number(3, uint64(p.Kind))...)
}
func protoModel(pieces []Piece, kind int, fallback bool) []byte {
	var b []byte
	for _, p := range pieces {
		b = append(b, field(1, pieceData(p))...)
	}
	trainer := number(3, uint64(kind))
	if fallback {
		trainer = append(trainer, number(35, 1)...)
	}
	b = append(b, field(2, trainer)...)
	return b
}
func basicPieces() []Piece {
	return []Piece{{"<unk>", 0, Unknown}, {"<s>", 0, Control}, {"</s>", 0, Control}, {"▁", -1, Normal}, {"a", -1, Normal}, {"b", -2, Normal}, {"ab", 1, Normal}, {"▁ab", 2, Normal}, {"<0xFF>", 0, Byte}, {"<0xC3>", 0, Byte}, {"<0xA9>", 0, Byte}, {"<0xC2>", 0, Byte}, {"<0x80>", 0, Byte}, {"<0x??>", 0, Byte}, {"unused", 1, Unused}, {"custom", 2, UserDefined}}
}
func processor(t *testing.T, kind int, fallback bool) *Processor {
	t.Helper()
	p, err := FromBytes(protoModel(basicPieces(), kind, fallback))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProcessorEncodeDecode(t *testing.T) {
	for _, kind := range []int{BPE, Unigram} {
		p := processor(t, kind, false)
		ids, err := p.Encode(" ab ")
		if err != nil {
			t.Fatal(err)
		}
		text, err := p.Decode(ids)
		if err != nil || text != "ab" {
			t.Fatal(kind, ids, text, err)
		}
		ids, err = p.Encode("zz")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, []int{3, 0}) {
			t.Fatal(ids)
		}
		ids, err = p.Encode("")
		if err != nil || len(ids) != 0 {
			t.Fatal(ids, err)
		}
		if _, err = p.Encode("\xff"); err == nil {
			t.Fatal("invalid UTF-8 accepted")
		}
		if _, err = p.Decode([]int{-1}); err == nil {
			t.Fatal("negative id")
		}
		if _, err = p.Decode([]int{99}); err == nil {
			t.Fatal("large id")
		}
		if text, err = p.Decode([]int{1, 3, 4, 5, 0, 2}); err != nil || text != "ab ⁇ " {
			t.Fatal(text, err)
		}
		pieces := p.Pieces()
		pieces[0].Text = "modified"
		if p.Pieces()[0].Text == "modified" {
			t.Fatal("alias")
		}
		a, b, c, d := p.IDs()
		if a != 0 || b != 1 || c != 2 || d != -1 {
			t.Fatal(a, b, c, d)
		}
	}
	p := processor(t, BPE, true)
	ids, err := p.Encode("é")
	if err != nil || !reflect.DeepEqual(ids, []int{3, 9, 10}) {
		t.Fatal(ids, err)
	}
	text, err := p.Decode(ids)
	if err != nil || text != "é" {
		t.Fatal(text, err)
	}
	ids, err = p.Encode("z")
	if err != nil || !reflect.DeepEqual(ids, []int{3, 0}) {
		t.Fatal(ids, err)
	}
	for _, c := range []struct {
		IDs  []int
		Text string
	}{{[]int{8}, "�"}, {[]int{9}, "�"}, {[]int{9, 4}, "�a"}, {[]int{9, 10, 3, 4}, "é a"}, {[]int{13}, ""}, {[]int{11, 12}, "\u0080"}, {[]int{3, 3, 4}, "a"}} {
		text, err = p.Decode(c.IDs)
		if err != nil || text != c.Text {
			t.Fatal(c, text, err)
		}
	}
	p.model.normalizer.removeExtra = false
	text, err = p.Decode([]int{3, 3, 4})
	if err != nil || text != " a" {
		t.Fatal(text, err)
	}
	p.model.normalizer.dummy = false
	p.model.normalizer.removeExtra = false
	text, err = p.Decode([]int{3, 4})
	if err != nil || text != " a" {
		t.Fatal(text, err)
	}
	for _, kind := range []int{Word, Char} {
		p = processor(t, kind, false)
		if _, err = p.Encode("a"); err == nil {
			t.Fatal("unsupported model accepted")
		}
	}
	if _, err := FromBytes(nil); err == nil {
		t.Fatal("empty model")
	}
	path := filepath.Join(t.TempDir(), "sp.model")
	if err = os.WriteFile(path, protoModel(basicPieces(), BPE, false), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path + "none"); err == nil {
		t.Fatal("missing file")
	}
}

func TestVocabularyEdgesAndSegmentation(t *testing.T) {
	// First duplicate wins for BPE lookup; Unigram trie insertions are last-wins.
	pieces := append(basicPieces(), Piece{"a", 9, Normal})
	p, err := FromBytes(protoModel(pieces, BPE, false))
	if err != nil {
		t.Fatal(err)
	}
	if p.ids["a"] != 4 {
		t.Fatal("duplicate mismatch")
	}
	if len(p.bpe(nil)) != 0 || len(p.unigram(nil)) != 0 {
		t.Fatal("empty segmentation")
	}
	// High-order pairs and ties exercise stale merges and neighbours.
	pieces = append(pieces, Piece{"aa", 4, Normal}, Piece{"aaa", 3, Normal}, Piece{"ba", 4, Normal}, Piece{"aba", 8, Normal}, Piece{"ababa", 9, Normal})
	p, err = FromBytes(protoModel(pieces, BPE, false))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"aaaabaaababa", "abababababa", "aaaaaaa", "abbaabab"} {
		if _, err = p.Encode(text); err != nil {
			t.Fatal(err)
		}
	}
	p, err = FromBytes(protoModel(pieces, Unigram, false))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"custom", "aaaabaaababa", "xx界", "ab"} {
		if _, err = p.Encode(text); err != nil {
			t.Fatal(err)
		}
	}
	p, err = FromBytes(protoModel([]Piece{{"<unk>", 0, Unknown}, {"unused", 1, Unused}}, Unigram, false))
	if err != nil || p.minScore != 0 {
		t.Fatal(p, err)
	}
	if _, err = p.Encode("anything"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"<0xGG>", "<0x001>", "X", "<0x01]"} {
		if _, ok := pieceByte(s); ok {
			t.Fatal(s)
		}
	}
	if _, ok := pieceByte("<0x3A>"); !ok {
		t.Fatal("valid byte")
	}
	a := agenda{{score: 0, left: 2}, {score: 0, left: 1}, {score: -1, left: 0}}
	if a.Less(0, 1) || !a.Less(1, 0) || !a.Less(0, 2) {
		t.Fatal("heap order")
	}
	if floatOrder(float32(math.Copysign(0, -1))) >= floatOrder(0) {
		t.Fatal("signed zero ordering")
	}
}

func TestLossyUTF8(t *testing.T) {
	for _, c := range []struct {
		B []byte
		S string
	}{{[]byte{0xe2, 0x82}, "�"}, {[]byte{0xff, 0xff}, "��"}, {[]byte{0xed, 0xa0, 0x80}, "���"}, {[]byte{0xf0, 0x9f, 0x92}, "�"}, {[]byte{0xe0, 0x80}, "��"}, {[]byte{0xf4, 0x90, 0x80}, "���"}, {[]byte{0xc2, 'x'}, "�x"}, {[]byte("☃"), "☃"}} {
		if got := lossy(c.B); got != c.S {
			t.Fatal(c, got)
		}
	}
}

func TestProtoMalformedAndDefaults(t *testing.T) {
	for _, data := range [][]byte{{0x80}, {0x0b}, {0x0a, 0x7f}, {0x09, 1}, {0x0d, 1}, bytes.Repeat([]byte{0x80}, 12), {8, 0x80}, {10, 0x80}} {
		if _, err := parseModel(data); err == nil {
			t.Fatalf("accepted %x", data)
		}
	}
	for _, n := range []int{1, 2, 3} {
		data := field(n, []byte{0x80})
		if _, err := parseModel(data); err == nil {
			t.Fatal(n)
		}
	}
	data := protoModel(basicPieces(), BPE, false)
	data = append(data, number(1, 5)...)
	data = append(data, varint(99<<3|1)...)
	data = append(data, make([]byte, 8)...)
	// Valid fixed64 unknown field, and unknown top-level fields.
	if _, err := parseModel(data); err != nil {
		t.Fatal(err)
	}
	weirdPiece := append(number(1, 1), field(2, []byte("bad"))...)
	weirdPiece = append(weirdPiece, number(3, 99)...)
	weirdPiece = append(weirdPiece, number(90, 1)...)
	p, err := parsePiece(weirdPiece)
	if err != nil || p.Text != "" || p.Score != 0 || p.Kind != Normal {
		t.Fatal(p, err)
	}
	trainer := number(3, 99)
	for _, n := range []int{24, 35, 40, 41, 42, 43, 44, 99} {
		trainer = append(trainer, field(n, []byte("x"))...)
	}
	m, err := parseModel(append(field(1, pieceData(Piece{"x", 0, Normal})), field(2, trainer)...))
	if err != nil || m.kind != Unigram {
		t.Fatal(m, err)
	}
	trainer = append(number(24, 1), number(35, 1)...)
	for _, n := range []int{40, 41, 42, 43} {
		trainer = append(trainer, number(n, uint64(n))...)
	}
	trainer = append(trainer, field(44, []byte("?"))...)
	if err = parseTrainer(trainer, &m); err != nil || !m.suffix || !m.byteFallback || m.unk != 40 || m.unkSurface != "?" {
		t.Fatal(m, err)
	}
	spec, err := parseNormalizer(append(append(append(number(2, 1), field(3, []byte("x"))...), number(4, 0)...), number(5, 0)...))
	if err != nil || !spec.dummy || spec.removeExtra || spec.escape {
		t.Fatal(spec, err)
	}
	spec, err = parseNormalizer(append(field(2, []byte{1, 2, 3}), number(99, 0)...))
	if err != nil || len(spec.charsmap) != 3 {
		t.Fatal(spec, err)
	}
	m, err = parseModel(append(protoModel(basicPieces(), BPE, false), field(3, nil)...))
	if err != nil || !m.normalizer.dummy {
		t.Fatal(m, err)
	}
	value := protoValue{wire: 2, data: []byte{0xff}}
	if value.text() != "�" || value.integer(9) != 9 || value.boolean(false) {
		t.Fatal(value)
	}
}

func TestNormalizerFlagsAndCharsmap(t *testing.T) {
	m := model{normalizer: normalDefaults()}
	if got := m.normalize("  a  b  "); got != "▁a▁b" {
		t.Fatal(got)
	}
	m.suffix = true
	if got := m.normalize(" a "); got != "a▁" {
		t.Fatal(got)
	}
	m.normalizer = normalizerSpec{}
	m.suffix = false
	if got := m.normalize("  a  "); got != "  a  " {
		t.Fatal(got)
	}
	for _, b := range [][]byte{nil, {0, 0, 0, 0}, {9, 0, 0, 0, 0, 0}, {1, 0, 0, 0, 0, 0}} {
		if decodeCharsmap(b) != nil {
			t.Fatal("invalid charsmap")
		}
	}
	// Tiny Darts trie: root offset=1, 'A' transition at 64, leaf at 2.
	units := make([]uint32, 65)
	units[0] = 1 << 10
	units[64] = uint32('A') | (1 << 8) | (66 << 10)
	units[2] = 0
	blob := binary.LittleEndian.AppendUint32(nil, uint32(len(units)*4))
	for _, u := range units {
		blob = binary.LittleEndian.AppendUint32(blob, u)
	}
	blob = append(blob, 'a', 0)
	m.normalizer = normalDefaults()
	m.normalizer.charsmap = blob
	m.normalizerMap = decodeCharsmap(blob)
	if got := m.normalize("AA?"); got != "▁aa?" {
		t.Fatal(got)
	}
	cm := decodeCharsmap(blob)
	cm.replacement = []byte{'x'}
	r, n := cm.prefix([]byte("A"))
	if string(r) != "x" || n != 1 {
		t.Fatal(r, n)
	}
	cm.units[2] = 99
	r, n = cm.prefix([]byte("A"))
	if string(r) != "A" || n != 1 {
		t.Fatal(r, n)
	}
	cm.units[2] = 0
	cm.replacement = []byte{0xff}
	m.normalizer.charsmap = append(blob[:len(blob)-2], 0xff, 0)
	m.normalizerMap = decodeCharsmap(m.normalizer.charsmap)
	if m.normalize("A") != "" {
		t.Fatal("invalid replacement should return empty")
	}
	cm = &charsmap{replacement: []byte{0}}
	r, n = cm.prefix([]byte("☃"))
	if string(r) != "☃" || n != 3 {
		t.Fatal(r, n)
	}
	cm = &charsmap{units: []uint32{999 << 10}}
	_, _ = cm.prefix([]byte("a"))
	m.normalizer = normalizerSpec{dummy: true, removeExtra: false, escape: false}
	m.suffix = false
	if got := m.normalize(" a "); got != "  a " {
		t.Fatal(got)
	}
}

func FuzzProcessor(f *testing.F) {
	f.Add(protoModel(basicPieces(), BPE, true), "ab")
	f.Add([]byte{}, "hello")
	f.Fuzz(func(t *testing.T, data []byte, text string) {
		p, err := FromBytes(data)
		if err != nil {
			return
		}
		ids, err := p.Encode(text)
		if err != nil {
			return
		}
		_, _ = p.Decode(ids)
	})
}

func TestTypedBPEAgenda(t *testing.T) {
	self := agenda{{score: 2, left: 0}}
	if self.Less(0, 0) {
		t.Fatal("self ordering")
	}
	ordered := agenda{{score: 1}, {score: 2}}
	if ordered.Less(0, 1) {
		t.Fatal("lower score sorted first")
	}
	queue := agenda{}
	for _, value := range []pair{{score: 1, left: 4}, {score: 2, left: 5}, {score: 2, left: 1}, {score: -1, left: 0}} {
		queue.push(value)
	}
	for _, want := range []pair{{score: 2, left: 1}, {score: 2, left: 5}, {score: 1, left: 4}, {score: -1, left: 0}} {
		if got := queue.pop(); got.score != want.score || got.left != want.left {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
	if len(queue) != 0 {
		t.Fatal(queue)
	}
}

func TestMinScoreNonfinite(t *testing.T) {
	p, err := FromBytes(protoModel([]Piece{{"x", float32(math.Inf(-1)), Normal}}, BPE, false))
	if err != nil || p.minScore != 0 {
		t.Fatal(p, err)
	}
	if got := strings.TrimSpace(p.model.unkSurface); got != "⁇" {
		t.Fatal(got)
	}
}

func TestRustSegmentationParity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parity/sentencepiece.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Bytes   []byte `json:"model_bytes"`
		Text    string
		IDs     []int
		Decoded string
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		p, err := FromBytes(c.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		ids, err := p.Encode(c.Text)
		if err != nil || !reflect.DeepEqual(ids, c.IDs) {
			t.Errorf("%q kind %d: %v != %v (%v)", c.Text, p.model.kind, ids, c.IDs, err)
		}
		text, err := p.Decode(c.IDs)
		if err != nil || text != c.Decoded {
			t.Errorf("decode %v: %q != %q (%v)", c.IDs, text, c.Decoded, err)
		}
	}
}
