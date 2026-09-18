package graphdebug

import (
	"context"
	"errors"

	"github.com/rcarmo/memento/go/access"
)

type Metrics struct {
	MemoryCount   int   `json:"memory_count"`
	MarkdownBytes int64 `json:"markdown_bytes"`
	AssetBytes    int64 `json:"asset_bytes"`
	ExplicitEdges int   `json:"explicit_edges"`
	BrokenEdges   int   `json:"broken_edges"`
	OrphanCount   int   `json:"orphan_count"`
}

type Overview struct {
	SchemaVersion int          `json:"schema_version"`
	Mode          string       `json:"mode"`
	Revisions     Revisions    `json:"revisions"`
	Metrics       Metrics      `json:"metrics"`
	Nodes         []Node       `json:"nodes"`
	Edges         []Edge       `json:"edges"`
	Clusters      []any        `json:"clusters"`
	ClusterEdges  []any        `json:"cluster_edges"`
	Memberships   []any        `json:"memberships"`
	LayoutSeed    string       `json:"layout_seed"`
	LayoutVersion string       `json:"layout_version"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Truncated     bool         `json:"truncated"`
}

type OverviewOptions struct {
	DirectNodeLimit int
	EdgeLimit       int
	IncludeTrash    bool
	Semantic        SemanticConfig
}

func (s *SnapshotService) Overview(ctx context.Context, policy *access.EffectivePolicy, options OverviewOptions) (Overview, error) {
	empty := Overview{}
	revisions, err := s.Revisions(ctx)
	if err != nil {
		return empty, err
	}
	nodes, err := s.Nodes(ctx, nil, options.DirectNodeLimit+1, policy, options.IncludeTrash)
	if err != nil {
		return empty, err
	}
	truncated := len(nodes) > options.DirectNodeLimit
	if truncated {
		return empty, errors.New("graph overview aggregation is required")
	}
	ids := make([]string, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	edges, err := s.ExplicitEdges(ctx, ids, nil, nil, options.EdgeLimit, policy)
	if err != nil {
		return empty, err
	}
	nodes = ScopedNodes(nodes, edges)
	diagnostics := DiagnoseFoundation(nodes, edges, revisions)
	nodes = ApplyDiagnosticIDs(nodes, diagnostics)
	semantic := []Edge{}
	if options.Semantic.NodeLimit > 0 {
		semantic, err = s.SemanticEdges(ctx, nodes, revisions, options.Semantic, options.EdgeLimit-len(edges))
		if err != nil {
			return empty, err
		}
	}
	overlays := OverlayEdges(nodes, revisions.Repository, options.EdgeLimit-len(edges)-len(semantic))
	allEdges := append(append(append([]Edge{}, edges...), semantic...), overlays...)
	metrics := Metrics{MemoryCount: len(nodes), ExplicitEdges: len(edges)}
	for _, node := range nodes {
		metrics.MarkdownBytes += node.MarkdownBytes
		metrics.AssetBytes += node.AssetBytes
		if node.Orphan {
			metrics.OrphanCount++
		}
	}
	for _, edge := range edges {
		if !externalLink(edge.RawTarget) && (edge.Target == nil || edge.Resolution != "resolved") {
			metrics.BrokenEdges++
		}
	}
	return Overview{SchemaVersion: 1, Mode: "direct", Revisions: revisions, Metrics: metrics, Nodes: nodes, Edges: allEdges, Clusters: []any{}, ClusterEdges: []any{}, Memberships: []any{}, LayoutSeed: revisions.Repository, LayoutVersion: "v1", Diagnostics: diagnostics, Truncated: truncated}, nil
}
