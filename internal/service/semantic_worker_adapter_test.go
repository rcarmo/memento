package service

import (
	"context"
	"github.com/rcarmo/memento/internal/derived"
	"testing"
)

type adapterIndex struct{}

func (adapterIndex) PendingEmbeddingPaths(context.Context, int) ([]string, error) { return nil, nil }
func (adapterIndex) RefreshEmbeddingPaths(context.Context, string, []string, derived.SemanticRefreshConfig, derived.SemanticClient) error {
	return nil
}
func TestSemanticRefreshAdapter(t *testing.T) {
	empty := SemanticRefreshAdapter{}
	if empty.Enqueue("", "", nil, false) || empty.State().Alive {
		t.Fatal("nil")
	}
	worker := derived.NewSemanticWorker(adapterIndex{}, adapterClient{}, derived.SemanticRefreshConfig{})
	adapter := SemanticRefreshAdapter{worker}
	if !adapter.Enqueue("root", "r", nil, true) {
		t.Fatal("enqueue")
	}
	state := adapter.State()
	if !state.Alive {
		t.Fatal(state)
	}
	worker.Close()
	if adapter.State().Alive {
		t.Fatal("closed")
	}
}

type adapterClient struct{}

func (adapterClient) ModelInfo() derived.SemanticModelInfo { return derived.SemanticModelInfo{} }
func (adapterClient) Embed(string) ([]float32, error)      { return []float32{1}, nil }
