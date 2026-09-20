package service

import (
	"context"
	"errors"
	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
	"testing"
)

type capabilityIndex struct {
	embedding string
	err       error
}

func (c capabilityIndex) Status(context.Context, access.EffectivePolicy) (derived.StatusSnapshot, error) {
	return derived.StatusSnapshot{State: derived.IndexState{RepoRevision: "r", IndexRevision: "r", Status: "ready"}, VisibleConcepts: 1}, nil
}
func (c capabilityIndex) SearchLexical(context.Context, access.EffectivePolicy, derived.SearchOptions) (derived.SearchPage, error) {
	return derived.SearchPage{}, nil
}
func (c capabilityIndex) Graph(context.Context, access.EffectivePolicy, string, derived.GraphOptions) (derived.GraphNeighborhood, error) {
	return derived.GraphNeighborhood{}, nil
}
func (c capabilityIndex) EmbeddingRevision(context.Context) (string, error) {
	return c.embedding, c.err
}
func TestRuntimeCapabilitiesStatus(t *testing.T) {
	controls, _, _ := realApplyTest(t)
	controls.Metadata, _ = NewModelsOffMetadata("standard")
	revision, _ := repository.GetMainRevision(controls.Queue.Paths)
	controls.Index = capabilityIndex{embedding: revision}
	controls.RuntimeCapabilities = RuntimeCapabilities{SemanticEnabled: true, SemanticLoaded: true, SemanticModelID: "m", SemanticDimensions: 2, NeedleEnabled: true, NeedleLoaded: true, NeedleModelPath: "/needle", NeedleRuntime: "go-mmap-subprocess"}
	data, _, err := controls.Status(context.Background(), ProposalActor{Policy: access.EffectivePolicy{Principal: "actor", ReadPrefixes: []string{"/"}}})
	if err != nil {
		t.Fatal(err)
	}
	features := data["features"].(map[string]any)
	readiness := data["readiness"].(map[string]any)
	semantic := readiness["semantic_search"].(map[string]any)
	needle := readiness["needle_router"].(map[string]any)
	if features["semantic_search"] != true || features["needle_router"] != true || semantic["ready"] != true || semantic["model_id"] != "m" || needle["runtime"] != "go-mmap-subprocess" || needle["model_path"] != "/needle" {
		t.Fatal(data)
	}
	controls.Index = capabilityIndex{embedding: "partial"}
	data, _, err = controls.Status(context.Background(), ProposalActor{Policy: access.EffectivePolicy{ReadPrefixes: []string{"/"}}})
	if err != nil || data["readiness"].(map[string]any)["semantic_search"].(map[string]any)["ready"] != false {
		t.Fatal(data, err)
	}
	controls.Index = capabilityIndex{err: errors.New("state")}
	if _, _, err = controls.Status(context.Background(), ProposalActor{Policy: access.EffectivePolicy{ReadPrefixes: []string{"/"}}}); err == nil {
		t.Fatal("state")
	}
}
