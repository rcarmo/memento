package gte

import (
	"fmt"
	"math"

	msimd "github.com/rcarmo/memento/internal/simd"
)

// Checkpoint preserves the reference's named cooperative cancellation points.
// Return an error to stop without exposing a partial output vector.
type Checkpoint func(string) error

// BatchOptions mirrors the reference limits. Nil means no explicit limit.
// MaxInputBytes preserves the Rust implementation's byte count, despite its
// legacy max_chars_per_input field name. The worker API imposes its own bounds.
type BatchOptions struct {
	MaxBatch      *int
	MaxInputBytes *int
}

// EmbedTo runs the scalar transformer into a caller-owned output buffer.
func (m *Model) EmbedTo(text string, out []float32, checkpoint Checkpoint) error {
	if len(out) != m.Dim() {
		return fmt.Errorf("output buffer len %d != hidden size %d", len(out), m.Dim())
	}
	var tokenBuffer []int
	if pooled, _ := m.tokenBuffers.Get().(*[]int); pooled != nil {
		tokenBuffer = *pooled
	}
	tokens, err := m.tokenizer.tokenizeInto(text, tokenBuffer)
	if err != nil {
		return err
	}
	tokenStorage := tokens[:0]
	defer m.tokenBuffers.Put(&tokenStorage)
	if err = check(checkpoint, "tokenized"); err != nil {
		return err
	}
	batch := [1][]int{tokens}
	outputs := [1][]float32{out}
	_, err = m.forwardInto(batch[:], checkpoint, outputs[:])
	return err
}

// Embed returns an owned, mean-pooled, L2-normalised float32 vector.
func (m *Model) Embed(text string) ([]float32, error) {
	out := make([]float32, m.Dim())
	if err := m.EmbedTo(text, out, nil); err != nil {
		return nil, err
	}
	return out, nil
}

// EmbedBatch preserves the source's padding, checkpoint ordering and limits.
func (m *Model) EmbedBatch(texts []string, options BatchOptions, checkpoint Checkpoint) ([][]float32, error) {
	if options.MaxBatch != nil && len(texts) > *options.MaxBatch {
		return nil, fmt.Errorf("batch too large: %d", len(texts))
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	batch := make([][]int, 0, len(texts))
	for i, text := range texts {
		if options.MaxInputBytes != nil && len(text) > *options.MaxInputBytes {
			return nil, fmt.Errorf("input too large at index %d: %d chars > %d", i, len(text), *options.MaxInputBytes)
		}
		if err := check(checkpoint, "batch_item_start"); err != nil {
			return nil, err
		}
		tokens, err := m.Tokenize(text)
		if err != nil {
			return nil, err
		}
		batch = append(batch, tokens)
		if err = check(checkpoint, "tokenized"); err != nil {
			return nil, err
		}
	}
	outputs, err := m.forward(batch, checkpoint)
	if err != nil {
		return nil, err
	}
	for range texts {
		if err = check(checkpoint, "batch_item_done"); err != nil {
			return nil, err
		}
	}
	return outputs, nil
}

func check(checkpoint Checkpoint, label string) error {
	if checkpoint != nil {
		return checkpoint(label)
	}
	return nil
}

// linear follows the original Go x*w^T layout with scalar accumulation. Bias
// begins the accumulation to preserve the Memento vector kernel's operation order.
func linear(x, w, b []float32, rows, in, out int) []float32 {
	return linearWithEngine(x, w, b, rows, in, out, nil)
}
func linearWithEngine(x, w, b []float32, rows, in, out int, engine *msimd.Engine) []float32 {
	return linearInto(x, w, b, rows, in, out, engine, nil)
}
func linearInto(x, w, b []float32, rows, in, out int, engine *msimd.Engine, target []float32) []float32 {
	size := rows * out
	if cap(target) < size {
		target = make([]float32, size)
	} else {
		target = target[:size]
		clear(target)
	}
	y := target
	for row := 0; row < rows; row++ {
		result := y[row*out : (row+1)*out]
		if engine != nil {
			_ = engine.DotRows(x[row*in:(row+1)*in], w, result)
			if b != nil {
				for col := range result {
					result[col] = float32(b[col] + result[col])
				}
			}
			continue
		}
		for col := 0; col < out; col++ {
			var sum float32
			if b != nil {
				sum = b[col]
			}
			for k := 0; k < in; k++ {
				sum = float32(sum + float32(x[row*in+k]*w[col*in+k]))
			}
			result[col] = sum
		}
	}
	return y
}

func layerNorm(x, gamma, beta []float32, hidden int) {
	for base := 0; base < len(x); base += hidden {
		var mean, variance float32
		for _, v := range x[base : base+hidden] {
			mean = float32(mean + v)
		}
		mean /= float32(hidden)
		for _, v := range x[base : base+hidden] {
			d := float32(v - mean)
			variance = float32(variance + float32(d*d))
		}
		variance /= float32(hidden)
		inverse := float32(1) / float32(math.Sqrt(float64(float32(variance+float32(1e-12)))))
		for i := 0; i < hidden; i++ {
			x[base+i] = float32(float32(float32(gamma[i]*float32(x[base+i]-mean))*inverse) + beta[i])
		}
	}
}

func gelu(x []float32) {
	const c = float32(0.7978846)
	for i, v := range x {
		cube := float32(float32(float32(float32(0.044715)*v)*v) * v)
		argument := float32(c * float32(v+cube))
		x[i] = float32(float32(float32(.5)*v) * float32(1+float32(math.Tanh(float64(argument)))))
	}
}

func softmax(x []float32) {
	maximum := float32(math.Inf(-1))
	for _, v := range x {
		if v > maximum {
			maximum = v
		}
	}
	var sum float32
	for i, v := range x {
		x[i] = float32(math.Exp(float64(float32(v - maximum))))
		sum = float32(sum + x[i])
	}
	for i := range x {
		x[i] /= sum
	}
}

func residualInto(x, other, target []float32) []float32 {
	if cap(target) < len(x) {
		target = make([]float32, len(x))
	} else {
		target = target[:len(x)]
	}
	for i := range target {
		target[i] = float32(x[i] + other[i])
	}
	return target
}
func normalize(x []float32) {
	var sum float32
	for _, v := range x {
		sum = float32(sum + float32(v*v))
	}
	norm := float32(math.Sqrt(float64(sum)))
	if norm > 0 {
		for i := range x {
			x[i] /= norm
		}
	}
}

type forwardWorkspace struct {
	state, q, k, v, attention, projected, after, inter, output []float32
	mask                                                       []bool
	scores                                                     []float32
}

func takeFloats(target []float32, size int) []float32 {
	if cap(target) < size {
		return make([]float32, size)
	}
	target = target[:size]
	clear(target)
	return target
}
func takeBools(target []bool, size int) []bool {
	if cap(target) < size {
		return make([]bool, size)
	}
	target = target[:size]
	clear(target)
	return target
}

// forward restores the original Go attention/FFN algorithms but uses Memento's
// batch padding and exact-math scalar path, not upstream assembly or fast-math.
func (m *Model) forward(batch [][]int, checkpoint Checkpoint) ([][]float32, error) {
	return m.forwardInto(batch, checkpoint, nil)
}
func (m *Model) forwardInto(batch [][]int, checkpoint Checkpoint, outputs [][]float32) (_ [][]float32, err error) {
	seq := 0
	for _, tokens := range batch {
		seq = max(seq, len(tokens))
	}
	h := m.Dim()
	rows := len(batch) * seq
	workspace, _ := m.workspaces.Get().(*forwardWorkspace)
	if workspace == nil {
		workspace = &forwardWorkspace{}
	}
	defer m.workspaces.Put(workspace)
	workspace.state = takeFloats(workspace.state, rows*h)
	workspace.mask = takeBools(workspace.mask, rows)
	state, mask := workspace.state, workspace.mask
	for item, tokens := range batch {
		for pos, token := range tokens {
			row := item*seq + pos
			mask[row] = true
			for d := 0; d < h; d++ {
				state[row*h+d] = float32(float32(m.token[token*h+d]+m.position[pos*h+d]) + m.tokenType[d])
			}
		}
	}
	layerNorm(state, m.embedNorm, m.embedNormBias, h)
	if err := check(checkpoint, "embeddings_ready"); err != nil {
		return nil, err
	}
	headDim := h / m.config.NumHeads
	scale := float32(1) / float32(math.Sqrt(float64(float32(headDim))))
	for _, l := range m.layers {
		workspace.q = linearInto(state, l.query, l.queryBias, rows, h, h, m.simd, workspace.q)
		workspace.k = linearInto(state, l.key, l.keyBias, rows, h, h, m.simd, workspace.k)
		workspace.v = linearInto(state, l.value, l.valueBias, rows, h, h, m.simd, workspace.v)
		workspace.attention = takeFloats(workspace.attention, rows*h)
		workspace.scores = takeFloats(workspace.scores, seq)
		q, k, v, attention, scores := workspace.q, workspace.k, workspace.v, workspace.attention, workspace.scores
		for item := range batch {
			base := item * seq
			for head := 0; head < m.config.NumHeads; head++ {
				offset := head * headDim
				for pos := 0; pos < seq; pos++ {
					row := base + pos
					if !mask[row] {
						continue
					}
					qb := row*h + offset
					for key := 0; key < seq; key++ {
						kr := base + key
						if !mask[kr] {
							scores[key] = -10000
							continue
						}
						var score float32
						for d := 0; d < headDim; d++ {
							score = float32(score + float32(q[qb+d]*k[kr*h+offset+d]))
						}
						scores[key] = float32(score * scale)
					}
					softmax(scores)
					for d := 0; d < headDim; d++ {
						var sum float32
						for key, weight := range scores {
							sum = float32(sum + float32(weight*v[(base+key)*h+offset+d]))
						}
						attention[row*h+offset+d] = sum
					}
				}
			}
		}
		workspace.projected = linearInto(attention, l.attention, l.attentionBias, rows, h, h, m.simd, workspace.projected)
		workspace.after = residualInto(workspace.projected, state, workspace.after)
		layerNorm(workspace.after, l.attentionNorm, l.attentionNormBias, h)
		workspace.inter = linearInto(workspace.after, l.intermediate, l.intermediateBias, rows, h, m.config.Intermediate, m.simd, workspace.inter)
		gelu(workspace.inter)
		workspace.output = linearInto(workspace.inter, l.output, l.outputBias, rows, m.config.Intermediate, h, m.simd, workspace.output)
		state = residualInto(workspace.output, workspace.after, state)
		layerNorm(state, l.outputNorm, l.outputNormBias, h)
		if err := check(checkpoint, "layer_done"); err != nil {
			return nil, err
		}
	}
	if len(outputs) != len(batch) {
		outputs = make([][]float32, len(batch))
	}
	for item, tokens := range batch {
		out := outputs[item]
		if len(out) != h {
			out = make([]float32, h)
		} else {
			clear(out)
		}
		for pos := range tokens {
			for d := range out {
				out[d] = float32(out[d] + state[(item*seq+pos)*h+d])
			}
		}
		inverse := float32(1) / float32(len(tokens))
		for d := range out {
			out[d] = float32(out[d] * inverse)
		}
		normalize(out)
		outputs[item] = out
	}
	return outputs, nil
}
