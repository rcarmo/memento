package derived

import (
	"errors"
	"reflect"
	"testing"
)

type semanticBatchStub struct {
	semanticClientStub
	batches      [][]string
	batchVectors [][]float32
	batchErr     error
	singles      []string
}

func (s *semanticBatchStub) EmbedBatch(texts []string) ([][]float32, error) {
	s.batches = append(s.batches, append([]string{}, texts...))
	return s.batchVectors, s.batchErr
}
func (s *semanticBatchStub) Embed(text string) ([]float32, error) {
	s.singles = append(s.singles, text)
	return s.semanticClientStub.Embed(text)
}
func TestEmbedSemanticBatch(t *testing.T) {
	s := &semanticBatchStub{batchVectors: [][]float32{{1}, {2}}}
	got := EmbedSemanticBatch(s, []string{"a", "b"}, 2)
	if len(got) != 2 || got[0].Vector[0] != 1 || len(s.singles) != 0 {
		t.Fatal(got, s)
	}
	s = &semanticBatchStub{batchVectors: [][]float32{{1}}, semanticClientStub: semanticClientStub{vector: []float32{9}}}
	got = EmbedSemanticBatch(s, []string{"a", "b"}, 2)
	if len(got) != 2 || len(s.singles) != 2 || got[1].Vector[0] != 9 {
		t.Fatal(got, s.singles)
	}
	boom := errors.New("boom")
	s = &semanticBatchStub{batchErr: boom, semanticClientStub: semanticClientStub{vector: []float32{3}}}
	got = EmbedSemanticBatch(s, []string{"a", "b", "c"}, 2)
	if len(got) != 3 || len(s.batches) != 2 || len(s.singles) != 2 || !errors.Is(got[2].Err, boom) {
		t.Fatal(got, s.batches, s.singles)
	}
	plain := &semanticClientStub{vector: []float32{4}}
	got = EmbedSemanticBatch(plain, []string{"a", "b"}, 0)
	if !reflect.DeepEqual(got[0].Vector, []float32{4}) || plain.text != "b" {
		t.Fatal(got, plain.text)
	}
	if got := EmbedSemanticBatch(plain, nil, 1); len(got) != 0 {
		t.Fatal(got)
	}
}
