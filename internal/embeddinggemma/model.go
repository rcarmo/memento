package embeddinggemma

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/rcarmo/memento/internal/simd"
)

// MaxTokens is the published embedding context, not the GGUF positional capacity.
const MaxTokens = 8192
const width = 512

// Role chooses the model-card retrieval prompt. Raw leaves text unchanged.
type Role string

const (
	Raw      Role = "raw"
	Query    Role = "query"
	Document Role = "document"
)

type Options struct {
	Role       Role
	Title      string
	Dimensions int
}

type layer struct {
	q, k, v, o, gate, up, down, pleGate, pleOut, pleInput       *Matrix
	attnNorm, qNorm, kNorm, attnPost, ffnNorm, ffnPost, plePost []float32
	scalar                                                      float32
	head, kv                                                    int
}

// Model is immutable and safe for concurrent calls. Each Workspace belongs to
// one caller; a busy workspace is rejected rather than raced. Close waits for
// active calls. No production service or native runtime is involved.
type Model struct {
	file                *File
	tokenizer           *Tokenizer
	engine              simd.Engine
	token, output       *Matrix
	outputNorm, pleNorm []float32
	layers              [24]layer
}

func Load(path, backend string) (*Model, error) {
	f, err := Open(path)
	if err != nil {
		return nil, err
	}
	m, err := NewModel(f, backend)
	if err != nil {
		f.Close()
		return nil, err
	}
	// Load owns f exclusively: discard parsed tokenizer arrays now that compact
	// lookup tables own the required strings. Keep the durable weight mapping.
	for key, value := range f.metadata {
		if len(key) >= 10 && key[:10] == "tokenizer." {
			if _, ok := value.([]any); ok {
				delete(f.metadata, key)
			}
		}
	}
	return m, nil
}

// NewModel takes ownership of f on success; on error ownership stays with caller.
// This implementation deliberately admits only the pinned 0.3B architecture.
func NewModel(f *File, backend string) (*Model, error) {
	if f == nil {
		return nil, errors.New("embeddinggemma: nil model")
	}
	eng, err := simd.New(backend)
	if err != nil {
		return nil, err
	}
	tok, err := NewTokenizer(f)
	if err != nil {
		return nil, err
	}
	if len(tok.vocab) != 262144 {
		return nil, errors.New("embeddinggemma: unsupported vocabulary size")
	}
	m := &Model{file: f, tokenizer: tok, engine: eng}
	integer := func(key string, want uint64) error {
		v, ok := f.metadata["gemma-embedding2."+key]
		if !ok {
			return fmt.Errorf("embeddinggemma: missing %s", key)
		}
		var n uint64
		switch x := v.(type) {
		case uint32:
			n = uint64(x)
		case uint64:
			n = x
		default:
			return fmt.Errorf("embeddinggemma: invalid %s", key)
		}
		if n != want {
			return fmt.Errorf("embeddinggemma: unsupported %s=%d", key, n)
		}
		return nil
	}
	for key, want := range map[string]uint64{"block_count": 24, "embedding_length": 512, "embedding_length_out": 768, "embedding_length_per_layer_input": 512, "feed_forward_length": 2048, "attention.head_count": 4, "attention.key_length": 512, "attention.value_length": 512, "attention.key_length_swa": 256, "attention.value_length_swa": 256, "attention.sliding_window": 1024, "attention.shared_kv_layers": 0, "rope.dimension_count": 512, "rope.dimension_count_swa": 256, "context_length": 262144, "pooling_type": 1} {
		if err = integer(key, want); err != nil {
			return nil, err
		}
	}
	if f.metadata["gemma-embedding2.attention.causal"] != false {
		return nil, errors.New("embeddinggemma: causal attention unsupported")
	}
	for key, want := range map[string]float32{"attention.layer_norm_rms_epsilon": 1e-6, "rope.freq_base": 1e6, "rope.freq_base_swa": 1e4} {
		v, ok := f.metadata["gemma-embedding2."+key].(float64)
		if !ok || float32(v) != want {
			return nil, fmt.Errorf("embeddinggemma: unsupported %s", key)
		}
	}
	get := func(name string, rows, cols int) *Matrix {
		if err != nil {
			return nil
		}
		var p *Matrix
		p, err = f.Matrix(name)
		if err == nil && (p.rows != rows || p.columns != cols) {
			err = fmt.Errorf("embeddinggemma: %s shape mismatch", name)
		}
		return p
	}
	vector := func(name string, n int) []float32 {
		if err != nil {
			return nil
		}
		var t Tensor
		var ok bool
		f.mu.RLock()
		defer f.mu.RUnlock()
		t, ok = f.tensors[name]
		if !ok || len(t.Shape) != 1 || t.Shape[0] != uint64(n) || t.Type != F32 || f.data == nil {
			err = fmt.Errorf("embeddinggemma: invalid vector %s", name)
			return nil
		}
		v := make([]float32, n)
		start := f.dataOffset + t.Offset
		err = decodeRow(v, f.data[start:start+t.Bytes], t.Type)
		for _, x := range v {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				err = fmt.Errorf("embeddinggemma: nonfinite %s", name)
			}
		}
		return v
	}
	m.token = get("token_embd.weight", 262144, 512)
	m.output = get("output.weight", 768, 512)
	ple := get("per_layer_model_proj.weight", 24*512, 512)
	m.outputNorm = vector("output_norm.weight", 512)
	m.pleNorm = vector("per_layer_proj_norm.weight", 512)
	for i := range m.layers {
		l := &m.layers[i]
		l.head, l.kv = 256, 2
		if i%6 == 5 {
			l.head, l.kv = 512, 1
		}
		prefix := fmt.Sprintf("blk.%d.", i)
		l.q = get(prefix+"attn_q.weight", 4*l.head, 512)
		l.k = get(prefix+"attn_k.weight", l.kv*l.head, 512)
		l.v = get(prefix+"attn_v.weight", l.kv*l.head, 512)
		l.o = get(prefix+"attn_output.weight", 512, 4*l.head)
		l.gate = get(prefix+"ffn_gate.weight", 2048, 512)
		l.up = get(prefix+"ffn_up.weight", 2048, 512)
		l.down = get(prefix+"ffn_down.weight", 512, 2048)
		l.pleGate = get(prefix+"inp_gate.weight", 512, 512)
		l.pleOut = get(prefix+"proj.weight", 512, 512)
		l.attnNorm = vector(prefix+"attn_norm.weight", 512)
		l.qNorm = vector(prefix+"attn_q_norm.weight", l.head)
		l.kNorm = vector(prefix+"attn_k_norm.weight", l.head)
		l.attnPost = vector(prefix+"post_attention_norm.weight", 512)
		l.ffnNorm = vector(prefix+"ffn_norm.weight", 512)
		l.ffnPost = vector(prefix+"post_ffw_norm.weight", 512)
		l.plePost = vector(prefix+"post_norm.weight", 512)
		s := vector(prefix+"layer_output_scale.weight", 1)
		if err != nil {
			return nil, err
		}
		l.scalar = s[0]
		// A row-range view avoids retaining all 24 per-layer activations at once.
		p := *ple
		p.tensor.Offset += uint64(i*512) * p.tensor.Bytes / uint64(p.rows)
		p.rows = 512
		p.tensor.Bytes /= 24
		l.pleInput = &p
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Model) Close() error { return m.file.Close() }
func (m *Model) Tokenize(ctx context.Context, text string, opts Options) ([]int, error) {
	switch opts.Role {
	case "", Raw:
	case Query:
		text = "task: search result | query: " + text
	case Document:
		title := opts.Title
		if title == "" {
			title = "none"
		}
		text = "title: " + title + " | text: " + text
	default:
		return nil, errors.New("embeddinggemma: unknown role")
	}
	return m.tokenizer.Encode(ctx, text, MaxTokens)
}

// Workspace grows to a bounded high-water mark, is reusable, and contains no
// model weights. Release it when a caller no longer needs its reserved memory.
type Workspace struct {
	mu                                                                  sync.Mutex
	capacity                                                            int
	initial, hidden, q, k, v, attn                                      []float32
	scratch, tmp, scores                                                []float32
	tileNorm, tileOut, tileGate, tileUp, tilePLE, tilePLEGate, tileSums []float32
}

func (w *Workspace) prepare(n int) {
	if w.capacity < n {
		w.capacity = n
		w.initial = make([]float32, n*512)
		w.hidden = make([]float32, n*512)
		w.q = make([]float32, n*2048)
		w.k = make([]float32, n*512)
		w.v = make([]float32, n*512)
		w.attn = make([]float32, n*2048)
		w.scores = make([]float32, n)
	}
	if w.scratch == nil {
		w.scratch = make([]float32, 2048)
		w.tmp = make([]float32, 512)
		w.tileNorm = make([]float32, projectionTile*512)
		w.tileOut = make([]float32, projectionTile*512)
		w.tileGate = make([]float32, projectionTile*2048)
		w.tileUp = make([]float32, projectionTile*2048)
		w.tilePLE = make([]float32, projectionTile*512)
		w.tilePLEGate = make([]float32, projectionTile*512)
		w.tileSums = make([]float32, projectionTile)
	}
}

// Embed returns an owned L2-normalized vector. Dimensions may be 768 (default),
// 512, 256 or 128; Matryoshka truncation is followed by renormalization.
func (m *Model) Embed(ctx context.Context, text string, opts Options, w *Workspace) ([]float32, error) {
	ids, err := m.Tokenize(ctx, text, opts)
	if err != nil {
		return nil, err
	}
	return m.EmbedTokens(ctx, ids, opts.Dimensions, w)
}

// EmbedTokens includes every supplied token in mean pooling. IDs must include
// the desired BOS/EOS. There is no padding, silent truncation or cross-call KV.
func (m *Model) EmbedTokens(ctx context.Context, ids []int, dimensions int, w *Workspace) ([]float32, error) {
	if ctx == nil {
		return nil, errors.New("embeddinggemma: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if dimensions == 0 {
		dimensions = 768
	}
	if dimensions != 768 && dimensions != 512 && dimensions != 256 && dimensions != 128 {
		return nil, errors.New("embeddinggemma: invalid output dimensions")
	}
	n := len(ids)
	if n < 1 || n > MaxTokens {
		return nil, errors.New("embeddinggemma: invalid token count")
	}
	for _, id := range ids {
		if id < 0 || id >= 262144 {
			return nil, errors.New("embeddinggemma: invalid token ID")
		}
	}
	if w == nil {
		return nil, errors.New("embeddinggemma: nil workspace")
	}
	if !w.mu.TryLock() {
		return nil, errors.New("embeddinggemma: workspace busy")
	}
	defer w.mu.Unlock()
	m.file.mu.RLock()
	defer m.file.mu.RUnlock()
	if m.file.data == nil {
		return nil, errors.New("embeddinggemma: model closed")
	}
	w.prepare(n)
	initial, hidden := w.initial[:n*512], w.hidden[:n*512]
	const scale = float32(22.627416997969522)
	for t, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		x := initial[t*512 : (t+1)*512]
		if err := decodeRow(x, m.token.rawRow(id), m.token.tensor.Type); err != nil {
			return nil, err
		}
		for j := range x {
			x[j] *= scale
		}
		copy(hidden[t*512:], x)
	}
	project := func(p *Matrix, dst, x []float32) error { return p.projectLocked(ctx, dst, x, w.scratch, m.engine) }
	for li := range m.layers {
		l := &m.layers[li]
		head := l.head
		qwidth := head * 4
		kvwidth := head * l.kv
		q, k, v, attn := w.q[:n*qwidth], w.k[:n*kvwidth], w.v[:n*kvwidth], w.attn[:n*qwidth]
		for base := 0; base < n; base += projectionTile {
			count := min(projectionTile, n-base)
			norm := w.tileNorm[:count*512]
			for t := 0; t < count; t++ {
				rms(norm[t*512:(t+1)*512], hidden[(base+t)*512:(base+t+1)*512], l.attnNorm)
			}
			for _, p := range []struct {
				m   *Matrix
				dst []float32
			}{{l.q, q[base*qwidth : (base+count)*qwidth]}, {l.k, k[base*kvwidth : (base+count)*kvwidth]}, {l.v, v[base*kvwidth : (base+count)*kvwidth]}} {
				if err := p.m.projectTokensLocked(ctx, p.dst, norm, count, w.scratch, w.tileSums, m.engine); err != nil {
					return nil, err
				}
			}
			for local := 0; local < count; local++ {
				t := base + local
				qt, kt, vt := q[t*qwidth:(t+1)*qwidth], k[t*kvwidth:(t+1)*kvwidth], v[t*kvwidth:(t+1)*kvwidth]
				for h := 0; h < 4; h++ {
					z := qt[h*head : (h+1)*head]
					rms(z, z, l.qNorm)
					rope(z, t, li%6 == 5)
				}
				for h := 0; h < l.kv; h++ {
					z := kt[h*head : (h+1)*head]
					rms(z, z, l.kNorm)
					rope(z, t, li%6 == 5)
					z = vt[h*head : (h+1)*head]
					rms(z, z, nil)
				}
			}
		}
		if err := attention(ctx, attn, q, k, v, n, head, l.kv, li%6 != 5, w.scores, m.engine); err != nil {
			return nil, err
		}
		for base := 0; base < n; base += projectionTile {
			count := min(projectionTile, n-base)
			batch := func(p *Matrix, dst, input []float32) error {
				return p.projectTokensLocked(ctx, dst, input, count, w.scratch, w.tileSums, m.engine)
			}
			out := w.tileOut[:count*512]
			norm := w.tileNorm[:count*512]
			gate := w.tileGate[:count*2048]
			up := w.tileUp[:count*2048]
			ple := w.tilePLE[:count*512]
			pg := w.tilePLEGate[:count*512]
			if err := batch(l.o, out, attn[base*qwidth:(base+count)*qwidth]); err != nil {
				return nil, err
			}
			for t := 0; t < count; t++ {
				x := hidden[(base+t)*512 : (base+t+1)*512]
				y := out[t*512 : (t+1)*512]
				rms(y, y, l.attnPost)
				for j := range x {
					x[j] += y[j]
				}
				rms(norm[t*512:(t+1)*512], x, l.ffnNorm)
			}
			if err := batch(l.gate, gate, norm); err != nil {
				return nil, err
			}
			if err := batch(l.up, up, norm); err != nil {
				return nil, err
			}
			for j := range gate {
				gate[j] = gelu(gate[j]) * up[j]
			}
			if err := batch(l.down, out, gate); err != nil {
				return nil, err
			}
			for t := 0; t < count; t++ {
				x := hidden[(base+t)*512 : (base+t+1)*512]
				y := out[t*512 : (t+1)*512]
				rms(y, y, l.ffnPost)
				for j := range x {
					x[j] += y[j]
				}
			}
			if err := batch(l.pleInput, ple, initial[base*512:(base+count)*512]); err != nil {
				return nil, err
			}
			for t := 0; t < count; t++ {
				y := ple[t*512 : (t+1)*512]
				for j := range y {
					y[j] /= scale
				}
				rms(y, y, m.pleNorm)
			}
			if err := batch(l.pleGate, pg, hidden[base*512:(base+count)*512]); err != nil {
				return nil, err
			}
			for j := range pg {
				pg[j] = gelu(pg[j]) * ple[j]
			}
			if err := batch(l.pleOut, out, pg); err != nil {
				return nil, err
			}
			for t := 0; t < count; t++ {
				x := hidden[(base+t)*512 : (base+t+1)*512]
				y := out[t*512 : (t+1)*512]
				rms(y, y, l.plePost)
				for j := range x {
					x[j] = (x[j] + y[j]) * l.scalar
				}
			}
		}
	}
	clear(w.tmp)
	for t := 0; t < n; t++ {
		x := hidden[t*512 : (t+1)*512]
		rms(x, x, m.outputNorm)
		for j := range x {
			w.tmp[j] += x[j] / float32(n)
		}
	}
	full := make([]float32, 768)
	if err := project(m.output, full, w.tmp); err != nil {
		return nil, err
	}
	out := full[:dimensions:dimensions]
	var sum float64
	for _, x := range out {
		sum += float64(x) * float64(x)
	}
	if sum == 0 || math.IsNaN(sum) || math.IsInf(sum, 0) {
		return nil, errors.New("embeddinggemma: invalid embedding norm")
	}
	inv := float32(1 / math.Sqrt(sum))
	for j := range out {
		out[j] *= inv
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func rms(dst, x, weight []float32) {
	var sum float32
	for _, v := range x {
		sum += v * v
	}
	s := float32(1 / math.Sqrt(float64(sum/float32(len(x))+1e-6)))
	for i, v := range x {
		z := v * s
		if weight != nil {
			z *= weight[i]
		}
		dst[i] = z
	}
}
func gelu(x float32) float32 {
	return .5 * x * (1 + float32(math.Tanh(.7978845608028654*float64(x)*(1+.044715*float64(x)*float64(x)))))
}
func rope(x []float32, pos int, global bool) {
	base := 10000.0
	if global {
		base = 1e6
	}
	half := len(x) / 2
	for i := 0; i < half; i++ {
		angle := float64(pos) * math.Pow(base, -float64(2*i)/float64(len(x)))
		s, c := math.Sincos(angle)
		a, b := x[i], x[i+half]
		x[i] = a*float32(c) - b*float32(s)
		x[i+half] = b*float32(c) + a*float32(s)
	}
}

// Stream one softmax row: O(tokens*head) storage, not a quadratic score matrix.
func attention(ctx context.Context, dst, q, k, v []float32, n, head, kv int, sliding bool, scores []float32, engine simd.Engine) error {
	for t := 0; t < n; t++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lo, hi := 0, n
		if sliding {
			lo = max(0, t-512)
			hi = min(n, t+513)
		}
		for h := 0; h < 4; h++ {
			qh := q[t*4*head+h*head:][:head]
			kh := h / (4 / kv)
			mx := float32(-math.MaxFloat32)
			for j := lo; j < hi; j++ {
				s, _ := engine.Dot(qh, k[j*kv*head+kh*head:][:head])
				scores[j] = s
				if s > mx {
					mx = s
				}
			}
			var sum float32
			for j := lo; j < hi; j++ {
				scores[j] = float32(math.Exp(float64(scores[j] - mx)))
				sum += scores[j]
			}
			out := dst[t*4*head+h*head:][:head]
			clear(out)
			for j := lo; j < hi; j++ {
				a := scores[j] / sum
				if err := engine.AXPY(a, v[j*kv*head+kh*head:][:head], out); err != nil {
					return err
				}
			}
		}
	}
	return ctx.Err()
}
