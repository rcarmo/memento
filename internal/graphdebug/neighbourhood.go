package graphdebug

import (
	"context"
	"github.com/rcarmo/memento/internal/access"
	"sort"
)

type Neighbourhood struct {
	SchemaVersion int          `json:"schema_version"`
	Revisions     Revisions    `json:"revisions"`
	CenterID      string       `json:"center_id"`
	Nodes         []Node       `json:"nodes"`
	Edges         []Edge       `json:"edges"`
	Depth         int          `json:"depth"`
	Truncated     bool         `json:"truncated,omitempty"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
}
type NeighbourhoodOptions struct {
	Depth, EdgeLimit, SemanticNodeLimit, SemanticEdgeLimit, ExpansionNodeLimit int
	IncludeTrash                                                               bool
	Semantic                                                                   SemanticConfig
}

func (s *SnapshotService) Neighbourhood(ctx context.Context, conceptID string, policy *access.EffectivePolicy, o NeighbourhoodOptions) (Neighbourhood, error) {
	empty := Neighbourhood{}
	if o.Depth != 1 {
		return empty, &SnapshotError{"MVP neighbourhood depth must be 1"}
	}
	center, err := s.Nodes(ctx, []string{conceptID}, 1, policy, o.IncludeTrash)
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
	visible, err := s.Nodes(ctx, nil, o.SemanticNodeLimit+1, policy, o.IncludeTrash)
	if err != nil {
		return empty, err
	}
	scopeTruncated := len(visible) > o.SemanticNodeLimit
	if scopeTruncated {
		visible = visible[:o.SemanticNodeLimit]
	}
	foundCenter := false
	for _, node := range visible {
		if node.ID == conceptID {
			foundCenter = true
		}
	}
	if !foundCenter {
		visible = append(visible, center[0])
	}
	visibleIDs := make([]string, len(visible))
	for i, node := range visible {
		visibleIDs[i] = node.ID
	}
	explicit, err := s.completeExplicitEdges(ctx, visibleIDs, nil, nil, policy)
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
		if id != conceptID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	ids = append([]string{conceptID}, ids...)
	nodeTruncated := len(ids) > o.ExpansionNodeLimit
	if nodeTruncated {
		ids = ids[:o.ExpansionNodeLimit]
	}
	nodes, err := s.Nodes(ctx, ids, o.ExpansionNodeLimit, policy, o.IncludeTrash)
	if err != nil {
		return empty, err
	}
	included := make([]string, len(nodes))
	include := map[string]bool{}
	for i, node := range nodes {
		included[i] = node.ID
		include[node.ID] = true
	}
	scopedExplicit := filterExplicitEdges(explicit, include)
	displayExplicit, edgeTruncated := truncateExplicitEdges(scopedExplicit, o.EdgeLimit)
	edges := append([]Edge{}, displayExplicit...)
	for _, edge := range semantic {
		if edge.Target != nil && include[edge.Source] && include[*edge.Target] {
			edges = append(edges, edge)
		}
	}
	nodes = ScopedNodes(nodes, explicit)
	diagnostics, err := s.scopedDiagnostics(ctx, visible, explicit, revisions, nodes)
	if err != nil {
		return empty, err
	}
	return Neighbourhood{1, revisions, conceptID, nodes, edges, 1, nodeTruncated || edgeTruncated || scopeTruncated, diagnostics}, nil
}
