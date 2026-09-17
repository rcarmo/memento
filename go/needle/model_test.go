package needle

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func put32(b *bytes.Buffer, n uint32) { _ = binary.Write(b, binary.LittleEndian, n) }
func put64(b *bytes.Buffer, n uint64) { _ = binary.Write(b, binary.LittleEndian, n) }
func hashString(data []byte) string   { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func encode(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
func baseSections() map[string][]byte {
	cfg := Config{Activation: "gelu", DModel: 2, DFF: 3, Heads: 1, KVHeads: 1, EncoderLayers: 1, DecoderLayers: 1, VocabSize: 2, MaxSequence: 8, DType: "bfloat16"}
	tok := new(bytes.Buffer)
	put32(tok, 2)
	for _, p := range []Piece{{"<pad>", 3, 0}, {"▁test", 1, -1}} {
		put32(tok, uint32(len(p.Text)))
		tok.WriteString(p.Text)
		tok.WriteByte(p.Type)
		put32(tok, math.Float32bits(p.Score))
	}
	weights := []byte{0x80, 0x3f, 0x00, 0xc0}
	dir := tensorDirectory("a", 1, []uint32{2}, 0, 4)
	return map[string][]byte{"CONF": encode(cfg), "TOKN": tok.Bytes(), "TDIR": dir, "DATA": weights}
}
func tensorDirectory(name string, dtype byte, shape []uint32, offset, size uint64) []byte {
	b := new(bytes.Buffer)
	put32(b, 1)
	put32(b, uint32(len(name)))
	b.WriteString(name)
	b.WriteByte(dtype)
	b.WriteByte(byte(len(shape)))
	for _, d := range shape {
		put32(b, d)
	}
	put64(b, offset)
	put64(b, size)
	b.Write(make([]byte, 32))
	return b.Bytes()
}
func withMetadata(sections map[string][]byte) {
	sections["META"] = encode(map[string]any{"converter": map[string]any{"format": "NDL1", "tool": "test", "version": 1}, "section_hashes": map[string]any{"config_sha256": hashString(sections["CONF"]), "tensor_data_sha256": hashString(sections["DATA"]), "tensor_directory_sha256": hashString(sections["TDIR"]), "tokenizer_sha256": hashString(sections["TOKN"])}, "source": map[string]any{"checkpoint": map[string]any{"name": "synthetic", "sha256": "fixture"}, "tokenizer_model": map[string]any{"name": "synthetic", "sha256": "fixture"}}, "tensor_count": 1, "tokenizer_piece_count": 2})
}
func packed(sections map[string][]byte) []byte {
	names := []string{}
	for _, name := range []string{"CONF", "TOKN", "META", "TDIR", "DATA"} {
		if _, ok := sections[name]; ok {
			names = append(names, name)
		}
	}
	header := make([]byte, 12+len(names)*52)
	copy(header, "NDL1")
	binary.LittleEndian.PutUint16(header[4:], 1)
	binary.LittleEndian.PutUint16(header[6:], uint16(len(names)))
	data := append([]byte{}, header...)
	for i, name := range names {
		pos := 12 + i*52
		p := sections[name]
		copy(data[pos:], name)
		binary.LittleEndian.PutUint64(data[pos+4:], uint64(len(data)))
		binary.LittleEndian.PutUint64(data[pos+12:], uint64(len(p)))
		h := sha256.Sum256(p)
		copy(data[pos+20:], h[:])
		data = append(data, p...)
	}
	return data
}
func goodModel() []byte { s := baseSections(); withMetadata(s); return packed(s) }

func TestModelRoundtripAndIsolation(t *testing.T) {
	data := goodModel()
	m, err := FromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.Config().DModel != 2 || len(m.Pieces()) != 2 || !reflect.DeepEqual(m.TensorNames(), []string{"a"}) {
		t.Fatal(m.Config())
	}
	meta := m.Metadata()
	meta[0] = '?'
	if m.Metadata()[0] != '{' {
		t.Fatal("metadata alias")
	}
	pieces := m.Pieces()
	pieces[0].Text = "changed"
	if m.Pieces()[0].Text == "changed" {
		t.Fatal("piece alias")
	}
	tensor, ok := m.Tensor("a")
	if !ok || tensor.Name() != "a" {
		t.Fatal(tensor, ok)
	}
	shape := tensor.Shape()
	shape[0] = 99
	if tensor.Shape()[0] != 2 {
		t.Fatal("shape alias")
	}
	raw := tensor.RawBF16()
	raw[0] = 0
	if tensor.RawBF16()[0] != 0x80 {
		t.Fatal("byte alias")
	}
	if tensor.SHA256() != ([32]byte{}) {
		t.Fatal("descriptor hash altered")
	}
	out, err := m.TensorFloat32("a", []uint32{2})
	if err != nil || !reflect.DeepEqual(out, []float32{1, -2}) {
		t.Fatal(out, err)
	}
	if _, err = m.TensorFloat32("absent", nil); err == nil {
		t.Fatal("missing tensor accepted")
	}
	if _, err = m.TensorFloat32("a", nil); err == nil {
		t.Fatal("rank mismatch accepted")
	}
	if _, err = m.TensorFloat32("a", []uint32{1}); err == nil {
		t.Fatal("shape mismatch accepted")
	}
	for i := range data {
		data[i] = 0
	}
	if !reflect.DeepEqual(tensor.Float32(), []float32{1, -2}) {
		t.Fatal("source bytes alias")
	}
	path := filepath.Join(t.TempDir(), "model.ndl")
	if err = os.WriteFile(path, goodModel(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path + "missing"); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestNDL1Errors(t *testing.T) {
	good := goodModel()
	for n := 0; n < len(good); n++ {
		if _, err := FromBytes(good[:n]); err == nil {
			t.Fatalf("truncation %d accepted", n)
		}
	}
	bad := append([]byte{}, good...)
	bad[0] = 'X'
	if _, err := FromBytes(bad); err == nil || err.Error() != "invalid model magic" {
		t.Fatal(err)
	}
	bad = append([]byte{}, good...)
	bad[4] = 2
	if _, err := FromBytes(bad); err == nil || err.Error() != "unsupported model version: 2" {
		t.Fatal(err)
	}
	bad = append([]byte{}, good...)
	binary.LittleEndian.PutUint64(bad[16:], math.MaxUint64)
	if _, err := FromBytes(bad); err == nil || !strings.Contains(err.Error(), "length overflows") {
		t.Fatal(err)
	}
	bad = append([]byte{}, good...)
	bad[len(bad)-1] ^= 1
	if _, err := FromBytes(bad); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatal(err)
	}
	for _, name := range []string{"CONF", "TOKN", "META", "TDIR", "DATA"} {
		s := baseSections()
		withMetadata(s)
		delete(s, name)
		if _, err := FromBytes(packed(s)); err == nil || err.Error() != "missing section: "+name {
			t.Fatal(name, err)
		}
	}
	for _, name := range []string{"CONF", "META", "TOKN", "TDIR"} {
		s := baseSections()
		withMetadata(s)
		s[name] = []byte("bad")
		if _, err := FromBytes(packed(s)); err == nil {
			t.Fatal(name)
		}
	}
	for _, field := range []string{"tensor_count", "tokenizer_piece_count"} {
		s := baseSections()
		withMetadata(s)
		var meta map[string]any
		_ = json.Unmarshal(s["META"], &meta)
		meta[field] = 9
		s["META"] = encode(meta)
		if _, err := FromBytes(packed(s)); err == nil || !strings.Contains(err.Error(), "!= parsed") {
			t.Fatal(field, err)
		}
	}
	for _, field := range []string{"config_sha256", "tokenizer_sha256", "tensor_directory_sha256", "tensor_data_sha256"} {
		s := baseSections()
		withMetadata(s)
		var meta map[string]any
		_ = json.Unmarshal(s["META"], &meta)
		meta["section_hashes"].(map[string]any)[field] = "wrong"
		s["META"] = encode(meta)
		if _, err := FromBytes(packed(s)); err == nil || !strings.Contains(err.Error(), "metadata hash mismatch") {
			t.Fatal(field, err)
		}
	}
}

func TestMetadataErrors(t *testing.T) {
	s := baseSections()
	withMetadata(s)
	for _, field := range []string{"converter", "section_hashes", "source"} {
		var value map[string]any
		_ = json.Unmarshal(s["META"], &value)
		value[field] = map[string]any{}
		if _, err := readMetadata(encode(value)); err == nil {
			t.Fatal(field)
		}
	}
	for _, field := range []string{"checkpoint", "tokenizer_model"} {
		var value map[string]any
		_ = json.Unmarshal(s["META"], &value)
		value["source"].(map[string]any)[field] = map[string]any{}
		if _, err := readMetadata(encode(value)); err == nil {
			t.Fatal(field)
		}
	}
	for _, data := range []string{"[]", "{", `{"x":null}`, `{}`, `{"x":"wrong"}`} {
		var out struct {
			X int `json:"x"`
		}
		if err := decodeRequired([]byte(data), &out, []string{"x"}); err == nil {
			t.Fatal(data)
		}
	}
}

func TestPieceParserErrors(t *testing.T) {
	good := baseSections()["TOKN"]
	for n := 0; n < len(good); n++ {
		if _, err := parsePieces(good[:n]); err == nil {
			t.Fatal(n)
		}
	}
	if _, err := parsePieces(append(good, 0)); err == nil {
		t.Fatal("trailing accepted")
	}
	bad := append([]byte{}, good...)
	bad[8] = 0xff
	if _, err := parsePieces(bad); err == nil {
		t.Fatal("invalid utf8 accepted")
	}
	if pieces, err := parsePieces([]byte{0, 0, 0, 0}); err != nil || len(pieces) != 0 {
		t.Fatal(pieces, err)
	}
}

func TestTensorParserErrors(t *testing.T) {
	good := tensorDirectory("a", 1, []uint32{2}, 0, 4)
	weights := make([]byte, 4)
	for n := 0; n < len(good); n++ {
		if _, err := parseTensors(good[:n], weights); err == nil {
			t.Fatal(n)
		}
	}
	for _, data := range [][]byte{append(append([]byte{}, good...), 0), tensorDirectory("\xff", 1, []uint32{2}, 0, 4), tensorDirectory("a", 2, []uint32{2}, 0, 4), tensorDirectory("a", 1, []uint32{2}, 0, 2), tensorDirectory("a", 1, []uint32{2}, 5, 4), tensorDirectory("a", 1, []uint32{2}, 2, 4), tensorDirectory("a", 1, []uint32{math.MaxUint32, math.MaxUint32, math.MaxUint32}, 0, 4)} {
		if _, err := parseTensors(data, weights); err == nil {
			t.Fatal("invalid descriptor accepted")
		}
	}
	for _, shape := range [][]uint32{nil, {0}, {2}} {
		size := uint64(4)
		if len(shape) == 0 {
			size = 2
		} else if shape[0] == 0 {
			size = 0
		}
		if _, err := parseTensors(tensorDirectory("a", 1, shape, 0, size), weights); err != nil {
			t.Fatal(err)
		}
	}
	if tensors, err := parseTensors([]byte{0, 0, 0, 0}, nil); err != nil || len(tensors) != 0 {
		t.Fatal(tensors, err)
	}
}

func FuzzModel(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("NDL1"))
	f.Add(goodModel())
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = FromBytes(data) })
}
