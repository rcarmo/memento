package gte

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestRustInferenceParity(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/gte-inference.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Layers      int
		Texts       []string
		Outputs     [][]float32
		Checkpoints []string
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		m, err := FromBytes(syntheticModel(c.Layers))
		if err != nil {
			t.Fatal(err)
		}
		labels := []string{}
		out, err := m.EmbedBatch(c.Texts, BatchOptions{}, func(label string) error { labels = append(labels, label); return nil })
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(labels, c.Checkpoints) {
			t.Fatal(labels, c.Checkpoints)
		}
		for i := range out {
			for d, v := range out[i] {
				if math.Abs(float64(v-c.Outputs[i][d])) > 1e-6 {
					t.Errorf("layers %d text %q dim %d got %.9f want %.9f", c.Layers, c.Texts[i], d, v, c.Outputs[i][d])
				}
			}
			single, err := m.Embed(c.Texts[i])
			if err != nil {
				t.Fatal(err)
			}
			for d := range single {
				if math.Abs(float64(single[d]-out[i][d])) > 1e-6 {
					t.Fatal("single/batch mismatch")
				}
			}
		}
	}
}

func TestInferenceErrorsAndCancellation(t *testing.T) {
	m, err := FromBytes(syntheticModel(2))
	if err != nil {
		t.Fatal(err)
	}
	if err = m.EmbedTo("hello", make([]float32, 1), nil); err == nil {
		t.Fatal("output size accepted")
	}
	if _, err = m.Embed("\xff"); err == nil {
		t.Fatal("invalid input accepted")
	}
	if _, err = m.EmbedBatch([]string{"\xff"}, BatchOptions{}, nil); err == nil {
		t.Fatal("invalid batch input accepted")
	}
	empty, err := m.EmbedBatch(nil, BatchOptions{}, nil)
	if err != nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
	limit := 1
	if _, err = m.EmbedBatch([]string{"a", "b"}, BatchOptions{MaxBatch: &limit}, nil); err == nil || err.Error() != "batch too large: 2" {
		t.Fatal(err)
	}
	if _, err = m.EmbedBatch([]string{"界"}, BatchOptions{MaxInputBytes: &limit}, nil); err == nil || err.Error() != "input too large at index 0: 3 chars > 1" {
		t.Fatal(err)
	}
	boom := errors.New("cancel")
	for _, point := range []string{"batch_item_start", "tokenized", "embeddings_ready", "layer_done", "batch_item_done"} {
		_, err = m.EmbedBatch([]string{"hello"}, BatchOptions{}, func(label string) error {
			if point == label {
				return boom
			}
			return nil
		})
		if !errors.Is(err, boom) {
			t.Fatal(point, err)
		}
	}
	for _, point := range []string{"tokenized", "embeddings_ready", "layer_done"} {
		out := []float32{42, 42}
		err = m.EmbedTo("hello", out, func(label string) error {
			if point == label {
				return boom
			}
			return nil
		})
		if !errors.Is(err, boom) || out[0] != 42 || out[1] != 42 {
			t.Fatal(point, out, err)
		}
	}
}

func TestScalarKernelEdges(t *testing.T) {
	got := linear([]float32{1, 2}, []float32{3, 4}, nil, 1, 2, 1)
	if !reflect.DeepEqual(got, []float32{11}) {
		t.Fatal(got)
	}
	zero := []float32{0, 0}
	normalize(zero)
	if !reflect.DeepEqual(zero, []float32{0, 0}) {
		t.Fatal(zero)
	}
	values := []float32{-1, 1, 0}
	gelu(values)
	if !(values[0] < 0 && values[1] > 0 && values[2] == 0) {
		t.Fatal(values)
	}
}
