package needle

import (
	"fmt"
	"sync"

	msimd "github.com/rcarmo/memento/go/internal/simd"
)

// Checkpoint is a cooperative cancellation callback at the reference boundaries.
type Checkpoint func(string) error

func poll(cp Checkpoint, label string) error {
	if cp != nil {
		return cp(label)
	}
	return nil
}

type attentionWeights struct{ q, k, v, out, qNorm, kNorm []float32 }
type encoderLayer struct {
	norm, gate []float32
	self       attentionWeights
}
type decoderLayer struct {
	norm0, norm1, selfGate, crossGate []float32
	self, cross                       attentionWeights
}

// Router holds immutable expanded model weights; each generation owns caches.
type Router struct {
	config                                Config
	embedding, encoderFinal, decoderFinal []float32
	encoder                               []encoderLayer
	decoder                               []decoderLayer
	simd                                  *msimd.Engine
	constraintTemplates                   sync.Map
}

// NewRouter checks all model tensor names/shapes before inference. Invalid head
// geometry fails cleanly rather than reproducing the source's division panic.
func NewRouter(model *Model) (*Router, error) {
	return newRouter(model.Config(), model.TensorFloat32)
}
func newRouter(c Config, tensor func(string, []uint32) ([]float32, error)) (*Router, error) {
	if c.DModel == 0 || c.Heads == 0 || c.KVHeads == 0 || c.DModel%c.Heads != 0 || c.Heads%c.KVHeads != 0 || c.MaxSequence == 0 {
		return nil, invalid("invalid attention geometry")
	}
	dm, heads, kv := int(c.DModel), int(c.Heads), int(c.KVHeads)
	hd := dm / heads
	kvd := kv * hd
	r := &Router{config: c}
	engine, engineErr := msimd.NewFromEnvironment()
	if engineErr != nil {
		return nil, engineErr
	}
	if engine.Backend() != msimd.Scalar {
		r.simd = &engine
	}
	var loadErr error
	get := func(name string, shape ...uint32) []float32 {
		if loadErr != nil {
			return nil
		}
		v, err := tensor(name, shape)
		if err != nil {
			loadErr = err
		}
		return v
	}
	r.embedding = get("embedding.embedding", c.VocabSize, c.DModel)
	r.encoderFinal = get("encoder.final_norm.scale", c.DModel)
	r.decoderFinal = get("decoder.ZCRMSNorm_0.scale", c.DModel)
	block := func(prefix string, layers uint32) []attentionWeights {
		q := get(prefix+"q_proj.kernel", layers, c.DModel, c.DModel)
		k := get(prefix+"k_proj.kernel", layers, c.DModel, uint32(kvd))
		v := get(prefix+"v_proj.kernel", layers, c.DModel, uint32(kvd))
		out := get(prefix+"out_proj.kernel", layers, c.DModel, c.DModel)
		qn := get(prefix+"q_norm.scale", layers, uint32(hd))
		kn := get(prefix+"k_norm.scale", layers, uint32(hd))
		if loadErr != nil {
			return nil
		}
		result := make([]attentionWeights, layers)
		for i := range result {
			result[i] = attentionWeights{sliceLayer(q, i, dm*dm), sliceLayer(k, i, dm*kvd), sliceLayer(v, i, dm*kvd), sliceLayer(out, i, dm*dm), sliceLayer(qn, i, hd), sliceLayer(kn, i, hd)}
		}
		return result
	}
	en := get("encoder.layers.EncoderBlock_0.ZCRMSNorm_0.scale", c.EncoderLayers, c.DModel)
	eg := get("encoder.layers.EncoderBlock_0.attn_gate", c.EncoderLayers)
	ea := block("encoder.layers.EncoderBlock_0.self_attn.", c.EncoderLayers)
	n0 := get("decoder.layers.DecoderBlock_0.ZCRMSNorm_0.scale", c.DecoderLayers, c.DModel)
	n1 := get("decoder.layers.DecoderBlock_0.ZCRMSNorm_1.scale", c.DecoderLayers, c.DModel)
	sg := get("decoder.layers.DecoderBlock_0.self_attn_gate", c.DecoderLayers)
	cg := get("decoder.layers.DecoderBlock_0.cross_attn_gate", c.DecoderLayers)
	sa := block("decoder.layers.DecoderBlock_0.self_attn.", c.DecoderLayers)
	ca := block("decoder.layers.DecoderBlock_0.cross_attn.", c.DecoderLayers)
	if loadErr != nil {
		return nil, loadErr
	}
	r.encoder = make([]encoderLayer, c.EncoderLayers)
	for i := range r.encoder {
		r.encoder[i] = encoderLayer{sliceLayer(en, i, dm), sliceLayer(eg, i, 1), ea[i]}
	}
	r.decoder = make([]decoderLayer, c.DecoderLayers)
	for i := range r.decoder {
		r.decoder[i] = decoderLayer{sliceLayer(n0, i, dm), sliceLayer(n1, i, dm), sliceLayer(sg, i, 1), sliceLayer(cg, i, 1), sa[i], ca[i]}
	}
	return r, nil
}
func (r *Router) SetSIMD(value string) error {
	engine, err := msimd.New(value)
	if err != nil {
		return err
	}
	if engine.Backend() == msimd.Scalar {
		r.simd = nil
	} else {
		r.simd = &engine
	}
	return nil
}
func (r *Router) SIMDBackend() msimd.Backend {
	if r.simd == nil {
		return msimd.Scalar
	}
	return r.simd.Backend()
}
func sliceLayer(values []float32, layer, width int) []float32 {
	return values[layer*width : (layer+1)*width]
}

type constraintCache struct {
	normalized string
	names      map[string]string
	template   *constraintTemplate
}
type crossCache struct{ k, v []float32 }
type decoderState struct {
	k, v  [][]float32
	cross []crossCache
	rope  rope

	x, norm0, norm1, qSelf, qCross, kStep, vStep, context, output, scores []float32
	maxGenerated                                                          int
}

func (r *Router) encode(tokens []int, cp Checkpoint) ([]float32, error) {
	dm := int(r.config.DModel)
	heads, kv := int(r.config.Heads), int(r.config.KVHeads)
	hd := dm / heads
	x := make([]float32, len(tokens)*dm)
	scale := sqrt32(float32(dm))
	for pos, id := range tokens {
		if id < 0 || id >= int(r.config.VocabSize) {
			return nil, invalid("token id %d out of range", id)
		}
		for i := 0; i < dm; i++ {
			x[pos*dm+i] = float32(r.embedding[id*dm+i] * scale)
		}
	}
	rope := precomputeRope(hd, len(tokens), r.config.RopeTheta)
	rows, kvRows := len(tokens)*dm, len(tokens)*kv*hd
	normalized := make([]float32, rows)
	q := make([]float32, rows)
	k := make([]float32, kvRows)
	v := make([]float32, kvRows)
	contexts := make([]float32, rows)
	output := make([]float32, rows)
	scores := make([]float32, len(tokens))
	for _, l := range r.encoder {
		if err := poll(cp, "encoder_layer"); err != nil {
			return nil, err
		}
		normalized = normRowsInto(normalized, x, dm, l.norm)
		q = projectInto(q, normalized, dm, dm, l.self.q, r.simd)
		k = projectInto(k, normalized, dm, kv*hd, l.self.k, r.simd)
		v = projectInto(v, normalized, dm, kv*hd, l.self.v, r.simd)
		headNorm(q, len(tokens)*heads, hd, l.self.qNorm)
		headNorm(k, len(tokens)*kv, hd, l.self.kNorm)
		ropeRows(q, heads, hd, rope)
		ropeRows(k, kv, hd, rope)
		for pos := range tokens {
			start := pos * dm
			attendInto(contexts[start:start+dm], scores, q[start:start+dm], k, v, heads, kv, hd, r.simd)
		}
		output = projectInto(output, contexts, dm, dm, l.self.out, r.simd)
		gatedResidual(x, output, l.gate[0])
	}
	return normRowsInto(normalized, x, dm, r.encoderFinal), nil
}
func (r *Router) decoderState(encoded []float32, cp Checkpoint) (*decoderState, error) {
	return r.decoderStateFor(encoded, 0, cp)
}
func (r *Router) decoderStateFor(encoded []float32, maxGenerated int, cp Checkpoint) (*decoderState, error) {
	dm := int(r.config.DModel)
	hd := dm / int(r.config.Heads)
	kv := int(r.config.KVHeads)
	s := &decoderState{
		k: make([][]float32, len(r.decoder)), v: make([][]float32, len(r.decoder)), cross: make([]crossCache, len(r.decoder)),
		rope: precomputeRope(hd, int(r.config.MaxSequence), r.config.RopeTheta),
		x:    make([]float32, dm), norm0: make([]float32, dm), norm1: make([]float32, dm),
		qSelf: make([]float32, dm), qCross: make([]float32, dm), kStep: make([]float32, kv*hd), vStep: make([]float32, kv*hd),
		context: make([]float32, dm), output: make([]float32, dm), scores: make([]float32, max(int(r.config.MaxSequence), len(encoded)/dm)),
		maxGenerated: min(maxGenerated, int(r.config.MaxSequence), 32),
	}
	cacheCapacity := s.maxGenerated * kv * hd
	for i, l := range r.decoder {
		if cacheCapacity > 0 {
			s.k[i] = make([]float32, 0, cacheCapacity)
			s.v[i] = make([]float32, 0, cacheCapacity)
		}
		if err := poll(cp, "decoder_cross_prep"); err != nil {
			return nil, err
		}
		k := projectWithEngine(encoded, dm, kv*hd, l.cross.k, r.simd)
		v := projectWithEngine(encoded, dm, kv*hd, l.cross.v, r.simd)
		headNorm(k, len(encoded)/dm*kv, hd, l.cross.kNorm)
		s.cross[i] = crossCache{k, v}
	}
	return s, nil
}
func (r *Router) decodeStep(token, pos int, state *decoderState, cp Checkpoint) ([]float32, error) {
	dm := int(r.config.DModel)
	heads, kv := int(r.config.Heads), int(r.config.KVHeads)
	hd := dm / heads
	if token < 0 || token >= int(r.config.VocabSize) {
		return nil, invalid("token id %d out of range", token)
	}
	if pos < 0 || pos >= int(r.config.MaxSequence) {
		return nil, fmt.Errorf("generation position %d exceeds model sequence length %d", pos, r.config.MaxSequence)
	}
	x := state.x
	scale := sqrt32(float32(dm))
	for i := range x {
		x[i] = float32(r.embedding[token*dm+i] * scale)
	}
	for i, l := range r.decoder {
		if err := poll(cp, "decoder_layer"); err != nil {
			return nil, err
		}
		normalized := normVectorInto(state.norm0, x, l.norm0)
		q := projectInto(state.qSelf, normalized, dm, dm, l.self.q, r.simd)
		k := projectInto(state.kStep, normalized, dm, kv*hd, l.self.k, r.simd)
		v := projectInto(state.vStep, normalized, dm, kv*hd, l.self.v, r.simd)
		headNorm(q, heads, hd, l.self.qNorm)
		headNorm(k, kv, hd, l.self.kNorm)
		applyRope(q, heads, hd, state.rope, pos)
		applyRope(k, kv, hd, state.rope, pos)
		state.k[i] = append(state.k[i], k...)
		state.v[i] = append(state.v[i], v...)
		context := attendInto(state.context, state.scores[:pos+1], q, state.k[i], state.v[i], heads, kv, hd, r.simd)
		output := projectInto(state.output, context, dm, dm, l.self.out, r.simd)
		gatedResidual(x, output, l.selfGate[0])
		normalized = normVectorInto(state.norm1, x, l.norm1)
		q = projectInto(state.qCross, normalized, dm, dm, l.cross.q, r.simd)
		headNorm(q, heads, hd, l.cross.qNorm)
		crossTokens := len(state.cross[i].k) / (kv * hd)
		context = attendInto(state.context, state.scores[:crossTokens], q, state.cross[i].k, state.cross[i].v, heads, kv, hd, r.simd)
		output = projectInto(state.output, context, dm, dm, l.cross.out, r.simd)
		gatedResidual(x, output, l.crossGate[0])
	}
	return normVectorInto(state.norm0, x, r.decoderFinal), nil
}
