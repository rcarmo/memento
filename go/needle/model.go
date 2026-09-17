// Package needle ports the NDL1 model and Needle router to pure Go. Model parsing
// is implemented first; SentencePiece and inference remain separate parity gates.
package needle

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"unicode/utf8"
)

// Config contains the checkpoint's architecture settings, preserving field names.
type Config struct {
	Activation     string  `json:"activation"`
	ContrastiveDim uint32  `json:"contrastive_dim"`
	DFF            uint32  `json:"d_ff"`
	DModel         uint32  `json:"d_model"`
	DropoutRate    float32 `json:"dropout_rate"`
	DType          string  `json:"dtype"`
	MaxSequence    uint32  `json:"max_seq_len"`
	NoFeedforward  bool    `json:"no_feedforward"`
	EncoderLayers  uint32  `json:"num_encoder_layers"`
	DecoderLayers  uint32  `json:"num_decoder_layers"`
	Heads          uint32  `json:"num_heads"`
	KVHeads        uint32  `json:"num_kv_heads"`
	MemorySlots    uint32  `json:"num_memory_slots"`
	PadToken       uint32  `json:"pad_token_id"`
	RopeTheta      float32 `json:"rope_theta"`
	VocabSize      uint32  `json:"vocab_size"`
}

// Piece is the NDL1 copy of tokenizer vocabulary, not the SentencePiece model.
type Piece struct {
	Text  string  `json:"piece"`
	Type  uint8   `json:"piece_type"`
	Score float32 `json:"score"`
}

// Tensor is immutable outside the package; accessors return owned copies.
type Tensor struct {
	name  string
	shape []uint32
	data  []byte
	hash  [32]byte
}

// Name returns the tensor's checkpoint name.
func (t Tensor) Name() string { return t.name }

// Shape returns an isolated dimension slice (rank zero represents a scalar).
func (t Tensor) Shape() []uint32 { return append([]uint32{}, t.shape...) }

// RawBF16 returns owned little-endian BF16 bytes.
func (t Tensor) RawBF16() []byte { return append([]byte{}, t.data...) }

// Float32 converts BF16 by exact bit expansion without numerical rounding.
func (t Tensor) Float32() []float32 {
	out := make([]float32, len(t.data)/2)
	for i := range out {
		out[i] = math.Float32frombits(uint32(binary.LittleEndian.Uint16(t.data[i*2:])) << 16)
	}
	return out
}

// SHA256 is the descriptor digest. The reference validates section/metadata
// hashes, not individual descriptor hashes; do not silently change that contract.
func (t Tensor) SHA256() [32]byte { return t.hash }

// Model owns its validated sections; no native mapping or runtime is required.
type Model struct {
	config   Config
	metadata json.RawMessage
	pieces   []Piece
	tensors  map[string]Tensor
}

// Config returns a copy of architecture settings.
func (m *Model) Config() Config { return m.config }

// Metadata returns the validated JSON metadata, detached from internal storage.
func (m *Model) Metadata() json.RawMessage { return append(json.RawMessage(nil), m.metadata...) }

// Pieces returns an isolated vocabulary copy; encoding needs the separate SP model.
func (m *Model) Pieces() []Piece { return append([]Piece{}, m.pieces...) }

// TensorNames matches the Rust BTreeMap's lexicographic name order.
func (m *Model) TensorNames() []string {
	names := make([]string, 0, len(m.tensors))
	for name := range m.tensors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Tensor returns an immutable view; accessor outputs remain caller-owned.
func (m *Model) Tensor(name string) (Tensor, bool) { t, ok := m.tensors[name]; return t, ok }

// TensorFloat32 validates a caller's expected shape before allocating output.
func (m *Model) TensorFloat32(name string, shape []uint32) ([]float32, error) {
	t, ok := m.tensors[name]
	if !ok {
		return nil, fmt.Errorf("missing tensor: %s", name)
	}
	if !equalShape(t.shape, shape) {
		return nil, fmt.Errorf("invalid tensor shape for %s: expected %v, got %v", name, shape, t.shape)
	}
	return t.Float32(), nil
}
func equalShape(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Load reads and validates an NDL1 file.
func Load(path string) (*Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return FromBytes(data)
}

func invalid(format string, args ...any) error { return fmt.Errorf("invalid model: "+format, args...) }

// FromBytes preserves NDL1 section/checksum, metadata and tensor ordering rules.
// Unknown/duplicate sections use the source's last-wins map semantics. Bounds are
// checked before allocation; malformed inputs cannot demand huge buffers.
func FromBytes(data []byte) (*Model, error) {
	if len(data) < 12 || string(data[:4]) != "NDL1" {
		return nil, fmt.Errorf("invalid model magic")
	}
	version := binary.LittleEndian.Uint16(data[4:])
	if version != 1 {
		return nil, fmt.Errorf("unsupported model version: %d", version)
	}
	count := int(binary.LittleEndian.Uint16(data[6:]))
	if count > (len(data)-12)/52 {
		return nil, invalid("section descriptor table extends past file")
	}
	sections := make(map[string][]byte, count)
	for i := 0; i < count; i++ {
		d := data[12+i*52 : 12+(i+1)*52]
		kind := string(bytes.ToValidUTF8(d[:4], []byte("�")))
		offset, length := binary.LittleEndian.Uint64(d[4:]), binary.LittleEndian.Uint64(d[12:])
		if length > math.MaxUint64-offset {
			return nil, invalid("section %s length overflows", kind)
		}
		if offset+length > uint64(len(data)) {
			return nil, fmt.Errorf("section %s is out of bounds: offset %d, len %d, file len %d", kind, offset, length, len(data))
		}
		payload := data[int(offset):int(offset+length)]
		digest := sha256.Sum256(payload)
		if !bytes.Equal(digest[:], d[20:52]) {
			return nil, fmt.Errorf("checksum mismatch for section %s", kind)
		}
		sections[string(d[:4])] = payload
	}
	for _, name := range []string{"CONF", "TOKN", "META", "TDIR", "DATA"} {
		if _, ok := sections[name]; !ok {
			return nil, fmt.Errorf("missing section: %s", name)
		}
	}
	var config Config
	if err := decodeRequired(sections["CONF"], &config, []string{"activation", "contrastive_dim", "d_ff", "d_model", "dropout_rate", "dtype", "max_seq_len", "no_feedforward", "num_decoder_layers", "num_encoder_layers", "num_heads", "num_kv_heads", "num_memory_slots", "pad_token_id", "rope_theta", "vocab_size"}); err != nil {
		return nil, invalid("invalid config json: %v", err)
	}
	metadata, err := readMetadata(sections["META"])
	if err != nil {
		return nil, invalid("invalid metadata json: %v", err)
	}
	pieces, err := parsePieces(sections["TOKN"])
	if err != nil {
		return nil, err
	}
	tensors, err := parseTensors(sections["TDIR"], sections["DATA"])
	if err != nil {
		return nil, err
	}
	if uint64(metadata.TensorCount) != uint64(len(tensors)) {
		return nil, invalid("metadata tensor_count %d != parsed %d", metadata.TensorCount, len(tensors))
	}
	if uint64(metadata.PieceCount) != uint64(len(pieces)) {
		return nil, invalid("metadata tokenizer_piece_count %d != parsed %d", metadata.PieceCount, len(pieces))
	}
	for _, c := range []struct{ label, kind, hash string }{{"config", "CONF", metadata.Hashes.Config}, {"tokenizer", "TOKN", metadata.Hashes.Tokenizer}, {"tensor_directory", "TDIR", metadata.Hashes.Directory}, {"tensor_data", "DATA", metadata.Hashes.Data}} {
		h := sha256.Sum256(sections[c.kind])
		actual := hex.EncodeToString(h[:])
		if actual != c.hash {
			return nil, invalid("metadata hash mismatch for %s: %s != %s", c.label, c.hash, actual)
		}
	}
	return &Model{config: config, metadata: append(json.RawMessage(nil), sections["META"]...), pieces: pieces, tensors: tensors}, nil
}

func decodeRequired(data []byte, out any, fields []string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok {
			return fmt.Errorf("missing field `%s`", field)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("null field `%s`", field)
		}
	}
	return json.Unmarshal(data, out)
}

type metadataHashes struct {
	Config    string `json:"config_sha256"`
	Tokenizer string `json:"tokenizer_sha256"`
	Directory string `json:"tensor_directory_sha256"`
	Data      string `json:"tensor_data_sha256"`
}
type metadataValues struct {
	Hashes      metadataHashes `json:"section_hashes"`
	TensorCount uint32         `json:"tensor_count"`
	PieceCount  uint32         `json:"tokenizer_piece_count"`
}

func readMetadata(data []byte) (metadataValues, error) {
	var values metadataValues
	if err := decodeRequired(data, &values, []string{"converter", "section_hashes", "source", "tensor_count", "tokenizer_piece_count"}); err != nil {
		return values, err
	}
	var object map[string]json.RawMessage
	_ = json.Unmarshal(data, &object)
	if err := decodeRequired(object["section_hashes"], &values.Hashes, []string{"config_sha256", "tensor_data_sha256", "tensor_directory_sha256", "tokenizer_sha256"}); err != nil {
		return values, err
	}
	var converter struct {
		Format  string `json:"format"`
		Tool    string `json:"tool"`
		Version uint16 `json:"version"`
	}
	if err := decodeRequired(object["converter"], &converter, []string{"format", "tool", "version"}); err != nil {
		return values, err
	}
	var source map[string]json.RawMessage
	if err := decodeRequired(object["source"], &source, []string{"checkpoint", "tokenizer_model"}); err != nil {
		return values, err
	}
	for _, key := range []string{"checkpoint", "tokenizer_model"} {
		var entry struct {
			Name string `json:"name"`
			Hash string `json:"sha256"`
		}
		if err := decodeRequired(source[key], &entry, []string{"name", "sha256"}); err != nil {
			return values, err
		}
	}
	return values, nil
}

type reader struct {
	data []byte
	at   int
	err  error
}

func (r *reader) take(n int, label string) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.data)-r.at {
		r.err = invalid("%s", label)
		return nil
	}
	out := r.data[r.at : r.at+n]
	r.at += n
	return out
}
func (r *reader) u32() uint32 {
	b := r.take(4, "u32 out of bounds")
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}
func (r *reader) u64() uint64 {
	b := r.take(8, "u64 out of bounds")
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}
func (r *reader) byte(label string) byte {
	b := r.take(1, label)
	if b == nil {
		return 0
	}
	return b[0]
}

func parsePieces(data []byte) ([]Piece, error) {
	r := reader{data: data}
	count := r.u32()
	pieces := []Piece{}
	for i := uint32(0); i < count; i++ {
		length := r.u32()
		text := r.take(int(length), "tokenizer piece out of bounds")
		kind := r.byte("tokenizer piece type missing")
		score := math.Float32frombits(r.u32())
		if r.err != nil {
			return nil, r.err
		}
		if !utf8.Valid(text) {
			return nil, invalid("tokenizer piece utf8: invalid UTF-8")
		}
		pieces = append(pieces, Piece{string(text), kind, score})
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.at != len(data) {
		return nil, invalid("tokenizer section has trailing bytes")
	}
	return pieces, nil
}

func parseTensors(data, weights []byte) (map[string]Tensor, error) {
	r := reader{data: data}
	count := r.u32()
	tensors := make(map[string]Tensor)
	for i := uint32(0); i < count; i++ {
		length := r.u32()
		name := r.take(int(length), "tensor name out of bounds")
		dtype := r.byte("tensor dtype missing")
		rank := r.byte("tensor rank missing")
		shape := make([]uint32, rank)
		for j := range shape {
			shape[j] = r.u32()
		}
		offset, size := r.u64(), r.u64()
		digest := r.take(32, "32-byte field out of bounds")
		if r.err != nil {
			return nil, r.err
		}
		if !utf8.Valid(name) {
			return nil, invalid("tensor name utf8: invalid UTF-8")
		}
		text := string(name)
		if dtype != 1 {
			return nil, invalid("tensor %s has unsupported dtype tag %d", text, dtype)
		}
		elements := uint64(1)
		for _, dim := range shape {
			if dim != 0 && elements > math.MaxUint64/uint64(dim) {
				elements = math.MaxUint64
			} else {
				elements *= uint64(dim)
			}
		}
		if elements > math.MaxUint64/2 {
			return nil, invalid("tensor %s size overflows", text)
		}
		if elements*2 != size {
			return nil, invalid("tensor %s byte_len %d != expected bf16 bytes %d", text, size, elements*2)
		}
		if offset > uint64(len(weights)) || size > uint64(len(weights))-offset {
			return nil, invalid("tensor %s range exceeds tensor data section", text)
		}
		var hash [32]byte
		copy(hash[:], digest)
		tensors[text] = Tensor{text, shape, append([]byte(nil), weights[int(offset):int(offset+size)]...), hash}
	}
	if r.err != nil {
		return nil, r.err
	}
	if r.at != len(data) {
		return nil, invalid("tensor directory has trailing bytes")
	}
	return tensors, nil
}
