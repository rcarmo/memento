package needle

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRealNeedleTokenizer(t *testing.T) {
	path := os.Getenv("NEEDLE_TOKENIZER_PATH")
	if path == "" {
		t.Skip("set NEEDLE_TOKENIZER_PATH for required model CI")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../testdata/parity/needle-tokenizer.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		ModelSHA string `json:"model_sha256"`
		Cases    []struct {
			Text    string
			IDs     []int
			Decoded string
		}
		DecodedTokens []string `json:"decoded_tokens"`
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != ref.ModelSHA {
		t.Fatal("tokenizer hash mismatch")
	}
	tokenizer, err := TokenizerFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range ref.Cases {
		ids, err := tokenizer.Encode(c.Text)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(ids, c.IDs) {
			t.Errorf("%q IDs\ngot %v\nwant %v", c.Text, ids, c.IDs)
		}
		decoded, err := tokenizer.Decode(c.IDs)
		if err != nil || decoded != c.Decoded {
			t.Errorf("decode %q: %q != %q (%v)", c.Text, decoded, c.Decoded, err)
		}
	}
	for id, want := range ref.DecodedTokens {
		got, err := tokenizer.Decode([]int{id})
		if err != nil || got != want {
			t.Errorf("token %d: %q != %q (%v)", id, got, want, err)
		}
	}
	t.Logf("Matched %d full encode/decode cases and %d individual-token decodes", len(ref.Cases), len(ref.DecodedTokens))
}

func spVar(v uint64) []byte {
	var out [10]byte
	n := binary.PutUvarint(out[:], v)
	return append([]byte{}, out[:n]...)
}
func spField(n int, data []byte) []byte {
	out := spVar(uint64(n<<3 | 2))
	out = append(out, spVar(uint64(len(data)))...)
	return append(out, data...)
}
func spNumber(n int, value uint64) []byte { return append(spVar(uint64(n<<3)), spVar(value)...) }
func syntheticTokenizer(space bool) []byte {
	pieces := []struct {
		Text string
		Kind uint64
	}{{"<pad>", 3}, {"</s>", 3}, {"<s>", 3}, {"<unk>", 2}, {"<tool_call>", 4}, {"<tools>", 4}, {"a", 1}, {"▁a", 1}, {"<0xFF>", 6}, {"<0x??>", 6}, {"notbyte", 6}, {"▁b", 1}}
	if space {
		pieces = append(pieces, struct {
			Text string
			Kind uint64
		}{"▁", 1})
	}
	var data []byte
	for _, piece := range pieces {
		p := spField(1, []byte(piece.Text))
		p = append(p, byte(2<<3|5), 0, 0, 0x80, 0xbf)
		p = append(p, spNumber(3, piece.Kind)...)
		data = append(data, spField(1, p)...)
	}
	trainer := spNumber(3, 2)
	trainer = append(trainer, spNumber(40, 3)...)
	data = append(data, spField(2, trainer)...)
	return data
}

func TestNeedleTokenizerSynthetic(t *testing.T) {
	data := syntheticTokenizer(true)
	tok, err := TokenizerFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if tok.VocabSize() != 13 {
		t.Fatal(tok.VocabSize())
	}
	for _, c := range []struct {
		Text string
		IDs  []int
	}{{"a", []int{7}}, {"<tools>a", []int{12, 5, 6}}, {"<tool_call>a", []int{12, 4, 6}}, {"a<tools>a", []int{7, 5, 6}}, {"<tools><tool_call>", []int{12, 5, 4}}, {"<tools> ", []int{12, 5}}, {"<tools>x", []int{12, 5, 3}}, {"<tools>b", []int{12, 5, 11}}} {
		got, err := tok.Encode(c.Text)
		if err != nil || !reflect.DeepEqual(got, c.IDs) {
			t.Errorf("%q: %v != %v %v", c.Text, got, c.IDs, err)
		}
	}
	if _, err = tok.Encode("\xff"); err == nil {
		t.Fatal("bad UTF8")
	}
	if _, err = tok.Encode("\xff<tools>"); err == nil {
		t.Fatal("bad prefix UTF8")
	}
	if _, err = tok.Encode("<tools>\xff"); err == nil {
		t.Fatal("bad suffix UTF8")
	}
	if _, err = tok.Decode([]int{999}); err == nil {
		t.Fatal("bad ID")
	}
	if text, err := tok.Decode([]int{7}); err != nil || text != "a" {
		t.Fatal(text, err)
	}
	for _, c := range []struct {
		ID   int
		Text string
	}{{-1, ""}, {999, ""}, {0, ""}, {3, ""}, {7, " a"}, {8, "ÿ"}, {9, ""}, {10, ""}} {
		if text := tok.TokenString(c.ID); text != c.Text {
			t.Fatal(c, text)
		}
	}
	if id, ok := tok.TokenToID("a"); !ok || id != 6 {
		t.Fatal(id, ok)
	}
	if _, ok := tok.TokenToID("missing"); ok {
		t.Fatal("unknown found")
	}
	noSpace, err := TokenizerFromBytes(syntheticTokenizer(false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = noSpace.Encode("<tools>a"); err != nil {
		t.Fatal(err)
	}
	if _, err = TokenizerFromBytes(nil); err == nil {
		t.Fatal("empty accepted")
	}
	path := filepath.Join(t.TempDir(), "needle.model")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadTokenizer(path); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadTokenizer(path + "none"); err == nil {
		t.Fatal("missing accepted")
	}
}
