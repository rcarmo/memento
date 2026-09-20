package graphdebug

import (
	"context"
	"github.com/rcarmo/memento/internal/access"
	"sort"
)

type Neighbourhood struct {
	SchemaVersion int       `json:"schema_version"`
	Revisions     Revisions `json:"revisions"`
	CenterID      string    `json:"center_id"`
	Nodes         []Node    `json:"nodes"`
	Edges         []Edge    `json:"edges"`
	Depth         int       `json:"depth"`
}
type NeighbourhoodOptions struct {
	Depth, EdgeLimit, SemanticNodeLimit, SemanticEdgeLimit, ExpansionNodeLimit int
	Semantic                                                                   SemanticConfig
}

func (s *SnapshotService) Neighbourhood(ctx context.Context, conceptID string, policy *access.EffectivePolicy, o NeighbourhoodOptions) (Neighbourhood, error) {
	empty := Neighbourhood{}
	if o.Depth != 1 {
		return empty, &SnapshotError{"MVP neighbourhood depth must be 1"}
	}
	center, err := s.Nodes(ctx, []string{conceptID}, 1, policy, false)
	if err != nil {
		return empty, err
	}
	if len(center) == 0 {
		return empty, &SnapshotError{"unknown memory"}
	}
	revisions, err := s.Revisions(ctx)
	if err != nil {
		return empty, err
	}
	visible, err := s.Nodes(ctx, nil, o.SemanticNodeLimit, policy, false)
	if err != nil {
		return empty, err
	}
	visibleIDs := make([]string, len(visible))
	for i, node := range visible {
		visibleIDs[i] = node.ID
	}
	explicit, err := s.ExplicitEdges(ctx, visibleIDs, nil, nil, o.EdgeLimit, policy)
	if err != nil {
		return empty, err
	}
	semantic, err := s.SemanticEdges(ctx, visible, revisions, o.Semantic, o.SemanticEdgeLimit)
	if err != nil {
		return empty, err
	}
	neighbors := map[string]bool{conceptID: true}
	for _, edge := range append(append([]Edge{}, explicit...), semantic...) {
		if edge.Target == nil {
			continue
		}
		if edge.Source == conceptID {
			neighbors[*edge.Target] = true
		}
		if *edge.Target == conceptID {
			neighbors[edge.Source] = true
		}
	}
	ids := make([]string, 0, len(neighbors))
	for id := range neighbors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) > o.ExpansionNodeLimit {
		ids = ids[:o.ExpansionNodeLimit]
	}
	nodes, err := s.Nodes(ctx, ids, o.ExpansionNodeLimit, policy, false)
	if err != nil {
		return empty, err
	}
	included := make([]string, len(nodes))
	include := map[string]bool{}
	for i, node := range nodes {
		included[i] = node.ID
		include[node.ID] = true
	}
	edges, err := s.ExplicitEdges(ctx, included, nil, nil, o.EdgeLimit, policy)
	if err != nil {
		return empty, err
	}
	for _, edge := range semantic {
		if edge.Target != nil && include[edge.Source] && include[*edge.Target] {
			edges = append(edges, edge)
		}
	}
	nodes = ScopedNodes(nodes, edges)
	return Neighbourhood{1, revisions, conceptID, nodes, edges, 1}, nil
}
