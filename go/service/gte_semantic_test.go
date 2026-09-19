package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/gte"
)

type gteSemanticStub struct {
	dim    int
	vector []float32
	batch  [][]float32
	err    error
}

func (s gteSemanticStub) Dim() int                        { return s.dim }
func (s gteSemanticStub) Embed(string) ([]float32, error) { return s.vector, s.err }
func (s gteSemanticStub) EmbedBatch([]string, gte.BatchOptions, gte.Checkpoint) ([][]float32, error) {
	return s.batch, s.err
}
func TestGTESemanticClientMethods(t *testing.T) {
	client := &GTESemanticClient{Model: gteSemanticStub{dim: 2, vector: []float32{1, 2}, batch: [][]float32{{1, 2}}}, Info: derived.SemanticModelInfo{ModelID: "m"}, MaxBatch: 2, MaxInputBytes: 10}
	if client.ModelInfo().ModelID != "m" {
		t.Fatal(client.ModelInfo())
	}
	if vector, err := client.Embed("x"); err != nil || len(vector) != 2 {
		t.Fatal(vector, err)
	}
	if batch, err := client.EmbedBatch([]string{"x"}); err != nil || len(batch) != 1 {
		t.Fatal(batch, err)
	}
}
func TestGTESemanticClientErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model")
	if _, err := LoadGTESemanticClient(path, "m", 2, 1, 10); err == nil {
		t.Fatal("missing")
	}
	_ = os.WriteFile(path, []byte("bad"), 0600)
	if _, err := LoadGTESemanticClient(path, "m", 2, 1, 10); err == nil {
		t.Fatal("bad")
	}
	var client *GTESemanticClient
	if _, err := client.Embed("x"); err == nil {
		t.Fatal("nil")
	}
	if _, err := client.EmbedBatch(nil); err == nil {
		t.Fatal("nil batch")
	}
	client = &GTESemanticClient{Model: gteSemanticStub{dim: 2}, MaxInputBytes: 1}
	if _, err := client.Embed("xx"); err == nil {
		t.Fatal("limit")
	}
	boom := errors.New("boom")
	if _, err := loadGTESemanticClient("x", "m", 0, 1, 1, func(string) ([]byte, error) { return nil, boom }, func(raw []byte) (gteSemanticModel, error) { return gte.FromBytes(raw) }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := loadGTESemanticClient("x", "m", 1, 1, 1, func(string) ([]byte, error) { return []byte("x"), nil }, func([]byte) (gteSemanticModel, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := loadGTESemanticClient("x", "m", 1, 1, 1, func(string) ([]byte, error) { return []byte("x"), nil }, func([]byte) (gteSemanticModel, error) { return gteSemanticStub{dim: 0}, nil }); err == nil {
		t.Fatal("dimension")
	}
	client, err := loadGTESemanticClient("x", "m", 2, 1, 1, func(string) ([]byte, error) { return []byte("x"), nil }, func([]byte) (gteSemanticModel, error) { return gteSemanticStub{dim: 2}, nil })
	if err != nil || client.Info.Revision == "" {
		t.Fatal(client, err)
	}
}
