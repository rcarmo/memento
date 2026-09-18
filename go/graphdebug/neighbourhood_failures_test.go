package graphdebug

import (
	"context"
	"database/sql"
	"testing"
)

func TestNeighbourhoodFailureStages(t *testing.T) {
	for _, failure := range []int{1, 3, 5, 6, 7, 9, 10} {
		s, policy := freshFixtureService(t)
		base := s.open
		calls := 0
		s.open = func(ctx context.Context, path string) (*sql.DB, error) {
			calls++
			if calls == failure {
				return nil, context.Canceled
			}
			return base(ctx, path)
		}
		o := NeighbourhoodOptions{Depth: 1, EdgeLimit: 10, SemanticNodeLimit: 10, SemanticEdgeLimit: 10, ExpansionNodeLimit: 10, Semantic: SemanticConfig{Neighbours: 2, MinSimilarity: .5, NodeLimit: 10, EdgeLimit: 10}}
		if _, err := s.Neighbourhood(context.Background(), "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", policy, o); err == nil {
			t.Fatal(failure, calls)
		}
	}
}
