package graphdebug

import (
	"context"
	"github.com/rcarmo/memento/internal/access"
)

type ClusterExpansion struct {
	SchemaVersion  int          `json:"schema_version"`
	Revisions      Revisions    `json:"revisions"`
	ClusterID      string       `json:"cluster_id"`
	ParentPosition Position     `json:"parent_position"`
	Nodes          []Node       `json:"nodes"`
	Edges          []Edge       `json:"edges"`
	NextCursor     *string      `json:"next_cursor"`
	Truncated      bool         `json:"truncated,omitempty"`
	Diagnostics    []Diagnostic `json:"diagnostics"`
}
type ClusterOptions struct {
	Cursor, RefreshMaxPaths, EdgeLimit, ExpansionNodeLimit, ClusterLimit int
	Semantic                                                             SemanticConfig
	IncludeTrash                                                         bool
}

func (s *SnapshotService) ExpandCluster(ctx context.Context, clusterID string, policy *access.EffectivePolicy, o ClusterOptions) (ClusterExpansion, error) {
	empty := ClusterExpansion{}
	if o.Cursor < 0 {
		return empty, &SnapshotError{"cluster cursor must be non-negative"}
	}
	revisions, err := s.Revisions(ctx)
	if err != nil {
		return empty, err
	}
	nodes, err := s.Nodes(ctx, nil, o.RefreshMaxPaths+1, policy, o.IncludeTrash)
	if err != nil {
		return empty, err
	}
	nodeTruncated := len(nodes) > o.RefreshMaxPaths
	if nodeTruncated {
		nodes = nodes[:o.RefreshMaxPaths]
	}
	ids := make([]string, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	explicit, err := s.completeExplicitEdges(ctx, ids, nil, nil, policy)
	if err != nil {
		return empty, err
	}
	nodes = ScopedNodes(nodes, explicit)
	displayExplicit, edgeTruncated := truncateExplicitEdges(explicit, o.EdgeLimit)
	semantic, err := s.SemanticEdges(ctx, nodes, revisions, o.Semantic, o.EdgeLimit-len(displayExplicit))
	if err != nil {
		return empty, err
	}
	layoutEdges := append(append([]Edge{}, explicit...), semantic...)
	layoutEdges = append(layoutEdges, OverlayEdges(nodes, revisions.Repository, o.EdgeLimit-len(displayExplicit)-len(semantic))...)
	layout := AggregateLayout(nodes, layoutEdges, revisions.Repository, o.ClusterLimit)
	members := map[string]bool{}
	for _, pair := range layout.Memberships {
		if pair[1] == clusterID {
			members[pair[0].(string)] = true
		}
	}
	var cluster *AggregateNode
	for i := range layout.Clusters {
		if layout.Clusters[i].ID == clusterID {
			cluster = &layout.Clusters[i]
			break
		}
	}
	if cluster == nil {
		return empty, &SnapshotError{"unknown cluster"}
	}
	ordered := []Node{}
	for _, node := range nodes {
		if members[node.ID] {
			ordered = append(ordered, node)
		}
	}
	start := min(o.Cursor, len(ordered))
	end := min(start+o.ExpansionNodeLimit, len(ordered))
	page := ordered[start:end]
	var next *string
	if end < len(ordered) {
		value := integerText(int64(end))
		next = &value
	}
	visible := map[string]bool{}
	for _, node := range page {
		visible[node.ID] = true
	}
	pageEdges := []Edge{}
	for _, edge := range displayExplicit {
		if edge.Target != nil && visible[edge.Source] && visible[*edge.Target] {
			pageEdges = append(pageEdges, edge)
		}
	}
	for _, edge := range semantic {
		if edge.Target != nil && visible[edge.Source] && visible[*edge.Target] {
			pageEdges = append(pageEdges, edge)
		}
	}
	diagnostics, err := s.scopedDiagnostics(ctx, nodes, explicit, revisions, page)
	if err != nil {
		return empty, err
	}
	return ClusterExpansion{1, revisions, clusterID, cluster.CoarsePosition, page, pageEdges, next, next != nil || edgeTruncated || nodeTruncated, diagnostics}, nil
}
