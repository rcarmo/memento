package needle

import (
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func routerFixture(vocab int) *Model {
	c := Config{DModel: 4, Heads: 2, KVHeads: 1, EncoderLayers: 1, DecoderLayers: 1, MaxSequence: 8, VocabSize: uint32(vocab), RopeTheta: 10000, NoFeedforward: true}
	m := &Model{config: c, tensors: make(map[string]Tensor)}
	add := func(name string, shape ...uint32) {
		count := 1
		for _, d := range shape {
			count *= int(d)
		}
		m.tensors[name] = Tensor{name: name, shape: shape, data: make([]byte, count*2)}
	}
	add("embedding.embedding", c.VocabSize, 4)
	add("encoder.final_norm.scale", 4)
	add("decoder.ZCRMSNorm_0.scale", 4)
	add("encoder.layers.EncoderBlock_0.ZCRMSNorm_0.scale", 1, 4)
	add("encoder.layers.EncoderBlock_0.attn_gate", 1)
	for _, name := range []string{"ZCRMSNorm_0.scale", "ZCRMSNorm_1.scale"} {
		add("decoder.layers.DecoderBlock_0."+name, 1, 4)
	}
	add("decoder.layers.DecoderBlock_0.self_attn_gate", 1)
	add("decoder.layers.DecoderBlock_0.cross_attn_gate", 1)
	for _, prefix := range []string{"encoder.layers.EncoderBlock_0.self_attn.", "decoder.layers.DecoderBlock_0.self_attn.", "decoder.layers.DecoderBlock_0.cross_attn."} {
		for _, name := range []string{"q_proj.kernel", "out_proj.kernel"} {
			add(prefix+name, 1, 4, 4)
		}
		for _, name := range []string{"k_proj.kernel", "v_proj.kernel"} {
			add(prefix+name, 1, 4, 2)
		}
		for _, name := range []string{"q_norm.scale", "k_norm.scale"} {
			add(prefix+name, 1, 2)
		}
	}
	embedding := m.tensors["embedding.embedding"]
	for i := 4; i < 8; i++ {
		binary.LittleEndian.PutUint16(embedding.data[i*2:], 0x3f80)
	}
	return m
}

func TestRouterConstructionAndMath(t *testing.T) {
	t.Setenv("MEMENTO_SIMD", "bad")
	if _, err := NewRouter(routerFixture(13)); err == nil {
		t.Fatal("invalid SIMD")
	}
	t.Setenv("MEMENTO_SIMD", "")
	m := routerFixture(13)
	r, err := NewRouter(m)
	if err != nil {
		t.Fatal(err)
	}
	tokens := []int{1, 6}
	encoded, err := r.encode(tokens, nil)
	if err != nil || len(encoded) != 8 {
		t.Fatal(encoded, err)
	}
	state, err := r.decoderState(encoded, nil)
	if err != nil {
		t.Fatal(err)
	}
	for pos := 0; pos < 2; pos++ {
		hidden, err := r.decodeStep(1, pos, state, nil)
		if err != nil || len(hidden) != 4 {
			t.Fatal(hidden, err)
		}
	}
	for _, token := range []int{-1, 13} {
		if _, err = r.encode([]int{token}, nil); err == nil {
			t.Fatal("encode bad ID")
		}
		if _, err = r.decodeStep(token, 0, state, nil); err == nil {
			t.Fatal("decode bad ID")
		}
	}
	for _, pos := range []int{-1, 8} {
		if _, err = r.decodeStep(1, pos, state, nil); err == nil {
			t.Fatal("bad position")
		}
	}
	for _, c := range []Config{{}, {DModel: 4, Heads: 3, KVHeads: 1, MaxSequence: 2}, {DModel: 4, Heads: 2, KVHeads: 3, MaxSequence: 2}} {
		bad := routerFixture(13)
		bad.config = c
		if _, err = NewRouter(bad); err == nil {
			t.Fatal("bad geometry")
		}
	}
	for _, name := range m.TensorNames() {
		bad := routerFixture(13)
		delete(bad.tensors, name)
		if _, err = NewRouter(bad); err == nil {
			t.Fatal("missing tensor", name)
		}
	}
	zero := attendWithEngine([]float32{1, 1}, nil, nil, 1, 1, 2, nil)
	if !reflect.DeepEqual(zero, []float32{0, 0}) {
		t.Fatal(zero)
	}
	if got := argmaxWithEngine([]float32{1}, []float32{-1, 2, 1}, []int{0, 2}, nil); got != 2 {
		t.Fatal(got)
	}
	if got := argmaxWithEngine([]float32{1}, []float32{-1, 2, 1}, nil, nil); got != 1 {
		t.Fatal(got)
	}
}

func TestGenerateBoundsAndCancellation(t *testing.T) {
	r, err := NewRouter(routerFixture(13))
	if err != nil {
		t.Fatal(err)
	}
	tokenizer, err := TokenizerFromBytes(syntheticTokenizer(true))
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultGenerationOptions()
	opts.MaxGenerated = 4
	opts.MaxEncoded = 4
	output, err := r.Generate(tokenizer, "a", "[]", opts, nil)
	if err != nil || output != "" {
		t.Fatal(output, err)
	}
	opts.Constrained = false
	if output, err = r.Generate(tokenizer, "a", "[]", opts, nil); err != nil || output != "" {
		t.Fatal(output, err)
	}
	for _, bad := range []GenerationOptions{{MaxEncoded: -1}, {MaxGenerated: -1}} {
		if _, err = r.Generate(tokenizer, "a", "[]", bad, nil); err == nil {
			t.Fatal("negative bounds")
		}
	}
	for _, c := range [][2]string{{"\xff", "[]"}, {"a", "\xff"}} {
		if _, err = r.Generate(tokenizer, c[0], c[1], opts, nil); err == nil {
			t.Fatal("bad tokenizer input")
		}
	}
	for _, max := range []int{0, 1, 2} {
		if _, err := encoderInput(tokenizer, "a a a", "a a a", max); err != nil {
			t.Fatal(err)
		}
	}
	boom := errors.New("cancel")
	for _, point := range []string{"tokenized", "encoder_layer", "encoded", "decoder_cross_prep", "decoder_layer"} {
		_, err = r.Generate(tokenizer, "a", "[]", opts, func(label string) error {
			if label == point {
				return boom
			}
			return nil
		})
		if !errors.Is(err, boom) {
			t.Fatal(point, err)
		}
	}
	// With a non-EOS argmax, bounded generation must error and never emit a partial call.
	for i := 3 * 4; i < 4*4; i++ {
		r.embedding[i] = 2
	}
	if _, err = r.Generate(tokenizer, "a", "[]", opts, nil); err == nil || err.Error() != "generation exceeded max length 4" {
		t.Fatal(err)
	}
	opts.MaxGenerated = 9
	if _, err = r.Generate(tokenizer, "a", "[]", opts, nil); err == nil || !strings.Contains(err.Error(), "position") {
		t.Fatal(err)
	}
	opts.MaxGenerated = 0
	if _, err = r.Generate(tokenizer, "a", "[]", opts, nil); err == nil {
		t.Fatal("zero gen")
	}
	small, _ := NewRouter(routerFixture(3))
	opts.MaxGenerated = 2
	if _, err = small.Generate(tokenizer, "a", "[]", opts, nil); err == nil {
		t.Fatal("vocab mismatch not rejected")
	}
	// Force an out-of-tokenizer ID on the first step, then EOS on the next.
	wide, _ := NewRouter(routerFixture(14))
	for i := 13 * 4; i < 14*4; i++ {
		wide.embedding[i] = 2
	}
	steps := 0
	opts.MaxGenerated = 4
	_, err = wide.Generate(tokenizer, "a", "[]", opts, func(label string) error {
		if label == "decoder_layer" {
			steps++
			if steps == 2 {
				for i := 13 * 4; i < 14*4; i++ {
					wide.embedding[i] = 0
				}
			}
		}
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatal(err)
	}
}

func TestScalarNumerics(t *testing.T) {
	if got := project([]float32{1, 2}, 2, 2, []float32{1, 2, 3, 4}); !reflect.DeepEqual(got, []float32{7, 10}) {
		t.Fatal(got)
	}
	if got := dot([]float32{1, 2}, []float32{3, 4}); got != 11 {
		t.Fatal(got)
	}
	if math.Abs(float64(sigmoid(0)-.5)) > 1e-6 {
		t.Fatal("sigmoid")
	}
}
