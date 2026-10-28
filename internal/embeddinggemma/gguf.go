// Package embeddinggemma implements the CPU-only EmbeddingGemma 2 foundation.
// Quantized matrices stay in their original GGUF layout. No native runtime is loaded.
package embeddinggemma

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"sync"
)

const (
	maxFileBytes          = 1 << 30
	maxHeaderBytes        = 64 << 20
	maxStringBytes        = 16 << 20
	maxArrayElements      = 1_000_000
	maxTotalArrayElements = 2_000_000
)

// Type identifies a supported GGML tensor encoding.
type Type uint32

const (
	F32  Type = 0
	F16  Type = 1
	Q8_0 Type = 8
	Q4_K Type = 12
	Q5_K Type = 13
	Q6_K Type = 14
	BF16 Type = 30
)

func (t Type) String() string {
	switch t {
	case F32:
		return "F32"
	case F16:
		return "F16"
	case Q8_0:
		return "Q8_0"
	case Q4_K:
		return "Q4_K"
	case Q5_K:
		return "Q5_K"
	case Q6_K:
		return "Q6_K"
	case BF16:
		return "BF16"
	}
	return fmt.Sprintf("unknown(%d)", t)
}
func (t Type) layout() (elements, bytes int, err error) {
	switch t {
	case F32:
		return 1, 4, nil
	case F16, BF16:
		return 1, 2, nil
	case Q8_0:
		return 32, 34, nil
	case Q4_K:
		return 256, 144, nil
	case Q5_K:
		return 256, 176, nil
	case Q6_K:
		return 256, 210, nil
	}
	return 0, 0, fmt.Errorf("unsupported tensor type %d", t)
}

// Tensor describes GGUF dimensions in innermost-first order. Returned shapes are copies.
type Tensor struct {
	Name   string   `json:"name"`
	Shape  []uint64 `json:"shape"`
	Type   Type     `json:"type"`
	Offset uint64   `json:"offset"`
	Bytes  uint64   `json:"bytes"`
}

// File owns a read-only mapping. Matrix operations hold its read lock; Close waits
// for active projections and invalidates subsequent operations. Do not copy File.
type File struct {
	mu         sync.RWMutex
	data       []byte
	metadata   map[string]any
	tensors    map[string]Tensor
	order      []string
	dataOffset uint64
	path       string
}

// Open validates the complete header/directory before exposing any matrix.
// It supports little-endian GGUF v2/v3 and rejects unsupported tensor encodings.
func Open(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() < 24 || st.Size() > maxFileBytes {
		return nil, errors.New("embeddinggemma: model must be a regular GGUF file between 24 bytes and 1GiB")
	}
	p := parser{r: bufio.NewReaderSize(io.NewSectionReader(f, 0, min(st.Size(), maxHeaderBytes)), 64<<10), remaining: min(st.Size(), maxHeaderBytes)}
	meta, ts, offset, err := p.header(uint64(st.Size()))
	if err != nil {
		return nil, fmt.Errorf("embeddinggemma: %w", err)
	}
	if meta["general.architecture"] != "gemma-embedding2" {
		return nil, errors.New("embeddinggemma: expected gemma-embedding2 architecture")
	}
	data, err := mapReadOnly(f, int(st.Size()))
	if err != nil {
		return nil, err
	}
	result := &File{data: data, metadata: meta, tensors: make(map[string]Tensor, len(ts)), dataOffset: offset, path: path}
	for _, t := range ts {
		result.tensors[t.Name] = t
		result.order = append(result.order, t.Name)
	}
	return result, nil
}
func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.data == nil {
		return nil
	}
	err := unmap(f.data)
	if err == nil {
		f.data = nil
	}
	return err
}
func (f *File) Tensors() []Tensor {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]Tensor, 0, len(f.order))
	for _, name := range f.order {
		t := f.tensors[name]
		t.Shape = append([]uint64(nil), t.Shape...)
		out = append(out, t)
	}
	return out
}

// Metadata returns scalar metadata only. Large tokenizer arrays remain private;
// future tokenizer support must not expose mutable model-owned storage.
func (f *File) Metadata() map[string]any {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make(map[string]any)
	for k, v := range f.metadata {
		switch v.(type) {
		case string, bool, uint64, int64, float64:
			out[k] = v
		}
	}
	return out
}
func (f *File) Matrix(name string) (*Matrix, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.data == nil {
		return nil, errors.New("embeddinggemma: model closed")
	}
	t, ok := f.tensors[name]
	if !ok {
		return nil, fmt.Errorf("embeddinggemma: missing tensor %q", name)
	}
	if len(t.Shape) != 2 {
		return nil, fmt.Errorf("embeddinggemma: %q is not a matrix", name)
	}
	return &Matrix{owner: f, tensor: t, columns: int(t.Shape[0]), rows: int(t.Shape[1])}, nil
}

type parser struct {
	r             *bufio.Reader
	remaining     int64
	pos           uint64
	arrayElements uint64
	word          [8]byte // fixed storage prevents one escaping allocation per scalar
}

func (p *parser) read(b []byte) error {
	if int64(len(b)) > p.remaining {
		return io.ErrUnexpectedEOF
	}
	n, err := io.ReadFull(p.r, b)
	p.remaining -= int64(n)
	p.pos += uint64(n)
	return err
}
func (p *parser) uint(n int) (uint64, error) {
	b := p.word[:]
	if err := p.read(b[:n]); err != nil {
		return 0, err
	}
	switch n {
	case 1:
		return uint64(b[0]), nil
	case 2:
		return uint64(binary.LittleEndian.Uint16(b[:])), nil
	case 4:
		return uint64(binary.LittleEndian.Uint32(b[:])), nil
	case 8:
		return binary.LittleEndian.Uint64(b[:]), nil
	}
	return 0, errors.New("invalid integer width")
}
func (p *parser) text() (string, error) {
	n, err := p.uint(8)
	if err != nil {
		return "", err
	}
	if n > maxStringBytes || n > uint64(p.remaining) {
		return "", errors.New("string length exceeds header bounds")
	}
	// Avoid an intermediate allocated byte slice for ordinary vocab strings.
	if n <= uint64(p.r.Size()) {
		b, e := p.r.Peek(int(n))
		if e != nil {
			return "", e
		}
		s := string(b)
		if _, e = p.r.Discard(int(n)); e != nil {
			return "", e
		}
		p.remaining -= int64(n)
		p.pos += n
		return s, nil
	}
	b := make([]byte, int(n))
	if err = p.read(b); err != nil {
		return "", err
	}
	return string(b), nil
}
func (p *parser) value(t uint64, array bool) (any, error) {
	switch t {
	case 0, 1, 2, 3, 4, 5, 6, 7, 10, 11, 12:
		widths := [...]int{1, 1, 2, 2, 4, 4, 4, 1, 0, 0, 8, 8, 8}
		n, err := p.uint(widths[t])
		if err != nil {
			return nil, err
		}
		switch t {
		case 1:
			return int64(int8(n)), nil
		case 3:
			return int64(int16(n)), nil
		case 5:
			return int64(int32(n)), nil
		case 11:
			return int64(n), nil
		case 6:
			return float64(math.Float32frombits(uint32(n))), nil
		case 12:
			return math.Float64frombits(n), nil
		case 7:
			if n > 1 {
				return nil, errors.New("invalid boolean")
			}
			return n == 1, nil
		}
		return n, nil
	case 8:
		return p.text()
	case 9:
		if array {
			return nil, errors.New("nested metadata array")
		}
		et, err := p.uint(4)
		if err != nil {
			return nil, err
		}
		n, err := p.uint(8)
		if err != nil {
			return nil, err
		}
		if n > maxArrayElements || n > maxTotalArrayElements-p.arrayElements || n > uint64(p.remaining) {
			return nil, errors.New("aggregate metadata arrays exceed header bounds")
		}
		p.arrayElements += n
		if et == 8 {
			a := make([]string, int(n))
			for i := range a {
				a[i], err = p.text()
				if err != nil {
					return nil, err
				}
			}
			return a, nil
		}
		a := make([]any, int(n))
		for i := range a {
			a[i], err = p.value(et, true)
			if err != nil {
				return nil, err
			}
		}
		return a, nil
	default:
		return nil, fmt.Errorf("unsupported metadata type %d", t)
	}
}
func (p *parser) header(size uint64) (map[string]any, []Tensor, uint64, error) {
	fail := func(err error) (map[string]any, []Tensor, uint64, error) { return nil, nil, 0, err }
	var magic [4]byte
	if err := p.read(magic[:]); err != nil {
		return fail(err)
	}
	if string(magic[:]) != "GGUF" {
		return fail(errors.New("invalid GGUF magic"))
	}
	v, err := p.uint(4)
	if err != nil {
		return fail(err)
	}
	if v != 2 && v != 3 {
		return fail(errors.New("unsupported GGUF version"))
	}
	nt, err := p.uint(8)
	if err != nil {
		return fail(err)
	}
	nk, err := p.uint(8)
	if err != nil {
		return fail(err)
	}
	if nt == 0 || nt > 4096 || nk > 4096 {
		return fail(errors.New("excessive or empty GGUF directory"))
	}
	meta := make(map[string]any, int(nk))
	for i := uint64(0); i < nk; i++ {
		key, e := p.text()
		if e != nil {
			return fail(e)
		}
		if _, ok := meta[key]; ok {
			return fail(errors.New("duplicate metadata key"))
		}
		t, e := p.uint(4)
		if e != nil {
			return fail(e)
		}
		meta[key], e = p.value(t, false)
		if e != nil {
			return fail(e)
		}
	}
	ts := make([]Tensor, int(nt))
	seen := make(map[string]bool, int(nt))
	for i := range ts {
		t := &ts[i]
		t.Name, err = p.text()
		if err != nil {
			return fail(err)
		}
		if t.Name == "" || seen[t.Name] {
			return fail(errors.New("empty or duplicate tensor name"))
		}
		seen[t.Name] = true
		rank, e := p.uint(4)
		if e != nil {
			return fail(e)
		}
		if rank == 0 || rank > 4 {
			return fail(errors.New("unsupported tensor rank"))
		}
		t.Shape = make([]uint64, int(rank))
		elements := uint64(1)
		for j := range t.Shape {
			d, e := p.uint(8)
			if e != nil {
				return fail(e)
			}
			if d == 0 || d > maxFileBytes || elements > maxFileBytes/d {
				return fail(errors.New("invalid tensor shape"))
			}
			t.Shape[j] = d
			elements *= d
		}
		qt, e := p.uint(4)
		if e != nil {
			return fail(e)
		}
		t.Type = Type(qt)
		t.Offset, e = p.uint(8)
		if e != nil {
			return fail(e)
		}
		block, bytes, e := t.Type.layout()
		if e != nil {
			return fail(e)
		}
		if t.Shape[0]%uint64(block) != 0 {
			return fail(errors.New("tensor row misaligned to quantization block"))
		}
		t.Bytes = elements / uint64(block) * uint64(bytes)
	}
	alignment := uint64(32)
	if a, ok := meta["general.alignment"]; ok {
		var valid bool
		alignment, valid = a.(uint64)
		if !valid {
			return fail(errors.New("invalid alignment type"))
		}
	}
	if alignment == 0 || alignment > 4096 || alignment&(alignment-1) != 0 {
		return fail(errors.New("invalid GGUF alignment"))
	}
	offset := (p.pos + alignment - 1) &^ (alignment - 1)
	if offset > size {
		return fail(io.ErrUnexpectedEOF)
	}
	sorted := append([]Tensor(nil), ts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Offset < sorted[j].Offset })
	end := uint64(0)
	for _, t := range sorted {
		if t.Offset%alignment != 0 || t.Offset < end || t.Offset > size-offset || t.Bytes > size-offset-t.Offset {
			return fail(errors.New("tensor overlap, alignment or file bounds violation"))
		}
		end = t.Offset + t.Bytes
	}
	return meta, ts, offset, nil
}
