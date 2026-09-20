package service

import (
	"context"
	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"testing"
)

type serviceSemanticIndex struct {
	lexical, semantic int
	options           derived.SemanticSearchOptions
}

func (s *serviceSemanticIndex) SearchLexical(context.Context, access.EffectivePolicy, derived.SearchOptions) (derived.SearchPage, error) {
	s.lexical++
	return derived.SearchPage{Results: []derived.SearchResult{}, RepoRevision: "r", IndexRevision: "i", Warnings: []string{}}, nil
}
func (s *serviceSemanticIndex) SearchSemantic(_ context.Context, _ access.EffectivePolicy, options derived.SemanticSearchOptions, _ derived.SemanticClient) (derived.SearchPage, error) {
	s.semantic++
	s.options = options
	return derived.SearchPage{Results: []derived.SearchResult{{ConceptID: "a", Path: "/a", Title: "A", Tags: []string{}}}, RepoRevision: "r", IndexRevision: "i", Warnings: []string{}}, nil
}
func (s *serviceSemanticIndex) Graph(context.Context, access.EffectivePolicy, string, derived.GraphOptions) (derived.GraphNeighborhood, error) {
	return derived.GraphNeighborhood{}, nil
}
func TestServiceSemanticSearchSelection(t *testing.T) {
	index := &serviceSemanticIndex{}
	controls := ProposalControls{Index: index, DefaultSearchMode: "semantic", SemanticClient: runtimeSemanticClient{}, SemanticMaxCandidates: 7}
	data, options, err := controls.Search(context.Background(), ProposalActor{Policy: access.EffectivePolicy{ReadPrefixes: []string{"/"}}}, "q", nil, 3, nil, nil, "plain")
	if err != nil || index.semantic != 1 || index.lexical != 0 || index.options.MaxCandidates != 7 || index.options.Hybrid || data["search_mode"] != "semantic" || options.RepoRevision == nil {
		t.Fatal(data, options, index, err)
	}
	mode := "hybrid"
	_, _, err = controls.Search(context.Background(), ProposalActor{}, "q", nil, 3, nil, &mode, "plain")
	if err != nil || !index.options.Hybrid {
		t.Fatal(index.options, err)
	}
	controls.SemanticClient = nil
	data, options, err = controls.Search(context.Background(), ProposalActor{}, "q", nil, 3, nil, &mode, "plain")
	if err != nil || index.lexical != 1 || len(options.Warnings) != 1 || data["search_mode"] != "hybrid" {
		t.Fatal(data, options, index, err)
	}
}
