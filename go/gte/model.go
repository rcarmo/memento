package gte

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"unicode/utf8"
)

// Config matches the six little-endian GTE1 header fields.
type Config struct {
	VocabSize    int `json:"vocab_size"`
	HiddenSize   int `json:"hidden_size"`
	NumLayers    int `json:"num_layers"`
	NumHeads     int `json:"num_heads"`
	Intermediate int `json:"intermediate"`
	MaxSequence  int `json:"max_seq_len"`
}

type layer struct {
	query, queryBias, key, keyBias, value, valueBias                               []float32
	attention, attentionBias, attentionNorm, attentionNormBias                     []float32
	intermediate, intermediateBias, output, outputBias, outputNorm, outputNormBias []float32
}

// Model owns immutable FP32 weights in the original Go GTE1 tensor order.
// The initial correctness loader copies weights; mmap is a later lifecycle task.
type Model struct {
	config                                               Config
	tokenizer                                            *Tokenizer
	token, position, tokenType, embedNorm, embedNormBias []float32
	layers                                               []layer
	pooler, poolerBias                                   []float32 // Present in the format, unused by mean pooling.
}

// Config returns a copy, preventing callers from invalidating tensor dimensions.
func (m *Model) Config() Config { return m.config }

// Dim is the embedding width.
func (m *Model) Dim() int { return m.config.HiddenSize }

// Tokenize uses the same immutable vocabulary and bounds as inference.
func (m *Model) Tokenize(text string) ([]int, error) { return m.tokenizer.Tokenize(text) }

// Load reads a GTE1 file without depending on a native runtime.
func Load(path string) (*Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return FromBytes(data)
}

type modelReader struct {
	data []byte
	at   int
	err  error
}

var errModelEOF = errors.New("io error: unexpected end of file")

func (r *modelReader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.data)-r.at {
		r.err = errModelEOF
		return nil
	}
	out := r.data[r.at : r.at+n]
	r.at += n
	return out
}
func (r *modelReader) u32() int {
	b := r.take(4)
	if b == nil {
		return 0
	}
	return int(binary.LittleEndian.Uint32(b))
}
func (r *modelReader) word() string {
	b := r.take(2)
	if b == nil {
		return ""
	}
	b = r.take(int(binary.LittleEndian.Uint16(b)))
	if !utf8.Valid(b) {
		r.err = errors.New("invalid model: vocabulary is not UTF-8")
		return ""
	}
	return string(b)
}
func (r *modelReader) weights(count int) []float32 {
	if r.err != nil {
		return nil
	}
	if count < 0 || count > (len(r.data)-r.at)/4 {
		r.err = errModelEOF
		return nil
	}
	b := r.take(count * 4)
	out := make([]float32, count)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}

// FromBytes validates before allocation, including truncation/overflow guards.
// As in the reference, unused trailing bytes and nonfinite raw weights are
// not rejected at this layer; inference/output consumers validate their results.
func FromBytes(data []byte) (*Model, error) {
	r := modelReader{data: data}
	magic := r.take(4)
	if r.err != nil {
		return nil, r.err
	}
	if string(magic) != "GTE1" {
		return nil, errors.New("invalid model magic")
	}
	c := Config{r.u32(), r.u32(), r.u32(), r.u32(), r.u32(), r.u32()}
	if r.err != nil {
		return nil, r.err
	}
	if c.VocabSize <= TokenMASK {
		return nil, errors.New("invalid model: vocab_size must include reserved token id 103")
	}
	if c.HiddenSize == 0 || c.Intermediate == 0 {
		return nil, errors.New("invalid model: hidden_size and intermediate must be positive")
	}
	if c.NumHeads == 0 || c.HiddenSize%c.NumHeads != 0 {
		return nil, fmt.Errorf("invalid model: num_heads must be positive and divide hidden_size: %d heads, %d hidden", c.NumHeads, c.HiddenSize)
	}
	if c.MaxSequence < 2 {
		return nil, errors.New("invalid model: max_seq_len must be at least 2")
	}
	// Each vocabulary entry requires a length prefix. Never allocate from an
	// unchecked header claiming billions of entries in a tiny file.
	if c.VocabSize > (len(data)-r.at)/2 {
		return nil, errModelEOF
	}
	vocab := make([]string, c.VocabSize)
	for i := range vocab {
		vocab[i] = r.word()
	}
	if r.err != nil {
		return nil, r.err
	}
	h := c.HiddenSize
	// Every dimension occurs in stored tensors. These bounds make the products
	// below safe and prevent allocating an impossible layer/header count.
	available := (len(data) - r.at) / 4
	if h > available || c.Intermediate > available || c.NumLayers > available/h {
		return nil, errModelEOF
	}
	product := func(a, b int) int {
		if a > available/b {
			return -1
		}
		return a * b
	}
	m := &Model{config: c}
	m.token = r.weights(product(c.VocabSize, h))
	m.position = r.weights(product(c.MaxSequence, h))
	m.tokenType = r.weights(product(2, h))
	m.embedNorm = r.weights(h)
	m.embedNormBias = r.weights(h)
	if r.err != nil {
		return nil, r.err
	}
	m.layers = make([]layer, 0, c.NumLayers)
	for range c.NumLayers {
		m.layers = append(m.layers, layer{
			r.weights(product(h, h)), r.weights(h), r.weights(product(h, h)), r.weights(h), r.weights(product(h, h)), r.weights(h),
			r.weights(product(h, h)), r.weights(h), r.weights(h), r.weights(h),
			r.weights(product(c.Intermediate, h)), r.weights(c.Intermediate), r.weights(product(h, c.Intermediate)), r.weights(h), r.weights(h), r.weights(h),
		})
		if r.err != nil {
			return nil, r.err
		}
	}
	m.pooler = r.weights(product(h, h))
	m.poolerBias = r.weights(h)
	if r.err != nil {
		return nil, r.err
	}
	// Header/vocabulary checks above already enforce constructor invariants.
	m.tokenizer, _ = NewTokenizer(vocab, c.MaxSequence)
	return m, nil
}
