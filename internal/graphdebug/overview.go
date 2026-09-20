package graphdebug

import (
	"context"

	"github.com/rcarmo/memento/internal/access"
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
	SchemaVersion int             `json:"schema_version"`
	Mode          string          `json:"mode"`
	Revisions     Revisions       `json:"revisions"`
	Metrics       Metrics         `json:"metrics"`
	Nodes         []Node          `json:"nodes"`
	Edges         []Edge          `json:"edges"`
	Clusters      []AggregateNode `json:"clusters"`
	ClusterEdges  []AggregateEdge `json:"cluster_edges"`
	Memberships   []CountPair     `json:"memberships"`
	LayoutSeed    string          `json:"layout_seed"`
	LayoutVersion string          `json:"layout_version"`
	Diagnostics   []Diagnostic    `json:"diagnostics"`
	Truncated     bool            `json:"truncated"`
}

func sparseOverview(nodes []Node, edges []Edge) bool {
	if len(nodes) < 12 {
		return false
	}
	resolved := 0
	for _, edge := range edges {
		if edge.Kind == "explicit" && edge.Target != nil {
			resolved++
		}
	}
	if resolved >= max(1, len(nodes)/4) {
		return false
	}
	orphans := 0
	for _, node := range nodes {
		if node.Orphan {
			orphans++
		}
	}
	return float64(orphans)/float64(len(nodes)) >= .5
}

type OverviewOptions struct {
	DirectNodeLimit int
	EdgeLimit       int
	IncludeTrash    bool
	Semantic        SemanticConfig
	RefreshMaxPaths int
	ClusterLimit    int
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
		nodes = nodes[:options.DirectNodeLimit]
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
	hashes, err := s.ContentHashes(ctx, ids)
	if err != nil {
		return empty, err
	}
	diagnostics := DiagnoseGraph(nodes, edges, revisions, hashes)
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
	metricNodes, metricEdges := nodes, edges
	var layout *Layout
	if truncated || sparseOverview(nodes, edges) {
		maximum := options.RefreshMaxPaths
		if maximum <= 0 {
			maximum = 2000
		}
		allNodes, loadErr := s.Nodes(ctx, nil, maximum, policy, options.IncludeTrash)
		if loadErr != nil {
			return empty, loadErr
		}
		allIDs := make([]string, len(allNodes))
		for i, node := range allNodes {
			allIDs[i] = node.ID
		}
		allExplicit, loadErr := s.ExplicitEdges(ctx, allIDs, nil, nil, options.EdgeLimit, policy)
		if loadErr != nil {
			return empty, loadErr
		}
		allNodes = ScopedNodes(allNodes, allExplicit)
		allSemantic := []Edge{}
		if options.Semantic.NodeLimit > 0 {
			allSemantic, loadErr = s.SemanticEdges(ctx, allNodes, revisions, options.Semantic, options.EdgeLimit-len(allExplicit))
			if loadErr != nil {
				return empty, loadErr
			}
		}
		allEdges := append(append([]Edge{}, allExplicit...), allSemantic...)
		allEdges = append(allEdges, OverlayEdges(allNodes, revisions.Repository, options.EdgeLimit-len(allEdges))...)
		allHashes, loadErr := s.ContentHashes(ctx, allIDs)
		if loadErr != nil {
			return empty, loadErr
		}
		diagnostics = DiagnoseGraph(allNodes, allExplicit, revisions, allHashes)
		clusterLimit := options.ClusterLimit
		if clusterLimit <= 0 {
			clusterLimit = 500
		}
		value := AggregateLayout(allNodes, allEdges, revisions.Repository, clusterLimit)
		layout = &value
		metricNodes, metricEdges = allNodes, allExplicit
	}
	metrics := Metrics{MemoryCount: len(metricNodes), ExplicitEdges: len(metricEdges)}
	for _, node := range metricNodes {
		metrics.MarkdownBytes += node.MarkdownBytes
		metrics.AssetBytes += node.AssetBytes
		if node.Orphan {
			metrics.OrphanCount++
		}
	}
	for _, edge := range metricEdges {
		if !externalLink(edge.RawTarget) && (edge.Target == nil || edge.Resolution != "resolved") {
			metrics.BrokenEdges++
		}
	}
	result := Overview{SchemaVersion: 1, Mode: "direct", Revisions: revisions, Metrics: metrics, Nodes: nodes, Edges: allEdges, Clusters: []AggregateNode{}, ClusterEdges: []AggregateEdge{}, Memberships: []CountPair{}, LayoutSeed: revisions.Repository, LayoutVersion: "v1", Diagnostics: diagnostics, Truncated: truncated}
	if layout != nil {
		result.Mode = "aggregated"
		result.Nodes = []Node{}
		result.Edges = []Edge{}
		result.Clusters = layout.Clusters
		result.ClusterEdges = layout.Edges
		result.Memberships = layout.Memberships
		result.LayoutVersion = layout.Version
	}
	return result, nil
}
