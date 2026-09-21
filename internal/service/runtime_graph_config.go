package service

import (
	"errors"
	"strings"

	"github.com/rcarmo/memento/internal/graphdebug"
)

type GraphExplorerConfig struct {
	Enabled               bool    `json:"enabled"`
	RoutePrefix           string  `json:"route_prefix"`
	DirectNodeLimit       int     `json:"direct_node_limit"`
	OverviewClusterLimit  int     `json:"overview_cluster_limit"`
	ExpansionNodeLimit    int     `json:"expansion_node_limit"`
	EdgeLimit             int     `json:"edge_limit"`
	PreviewChars          int     `json:"preview_chars"`
	SemanticNeighbours    int     `json:"semantic_neighbours"`
	SemanticMinSimilarity float64 `json:"semantic_min_similarity"`
	SemanticEdgeNodeLimit int     `json:"semantic_edge_node_limit"`
	SemanticEdgeLimit     int     `json:"semantic_edge_limit"`
	ExportNodeLimit       int     `json:"export_node_limit"`
	RefreshMaxPaths       int     `json:"refresh_max_paths"`
}
type ObservabilityConfig struct {
	GraphExplorer GraphExplorerConfig `json:"graph_explorer"`
}

func DefaultGraphExplorerConfig() GraphExplorerConfig {
	return GraphExplorerConfig{RoutePrefix: "/graph", DirectNodeLimit: 2000, OverviewClusterLimit: 500, ExpansionNodeLimit: 2000, EdgeLimit: 12000, PreviewChars: 4000, SemanticNeighbours: 12, SemanticMinSimilarity: .75, SemanticEdgeNodeLimit: 300, SemanticEdgeLimit: 1500, ExportNodeLimit: 2000, RefreshMaxPaths: 2000}
}
func (c GraphExplorerConfig) Validate() error {
	prefix := strings.TrimSpace(c.RoutePrefix)
	if !strings.HasPrefix(prefix, "/") || prefix == "/" || strings.HasSuffix(prefix, "/") || strings.ContainsAny(prefix, "?#") {
		return errors.New("graph route_prefix must be an absolute non-root path without a trailing slash")
	}
	for _, part := range strings.Split(prefix, "/") {
		if part == ".." {
			return errors.New("graph route_prefix must be an absolute non-root path without a trailing slash")
		}
	}
	checks := []struct{ value, min, max int }{{c.DirectNodeLimit, 1, 2000}, {c.OverviewClusterLimit, 1, 1000}, {c.ExpansionNodeLimit, 1, 2000}, {c.EdgeLimit, 1, 20000}, {c.PreviewChars, 0, 16000}, {c.SemanticNeighbours, 1, 100}, {c.SemanticEdgeNodeLimit, 1, 2000}, {c.SemanticEdgeLimit, 1, 12000}, {c.ExportNodeLimit, 1, 2000}, {c.RefreshMaxPaths, 1, 10000}}
	for _, check := range checks {
		if check.value < check.min || check.value > check.max {
			return errors.New("graph explorer setting out of range")
		}
	}
	if c.SemanticMinSimilarity < -1 || c.SemanticMinSimilarity > 1 {
		return errors.New("graph explorer setting out of range")
	}
	return nil
}
func (c GraphExplorerConfig) HTTPConfig() GraphHTTPConfig {
	semantic := graphdebug.SemanticConfig{Neighbours: c.SemanticNeighbours, MinSimilarity: c.SemanticMinSimilarity, NodeLimit: c.SemanticEdgeNodeLimit, EdgeLimit: c.SemanticEdgeLimit}
	return GraphHTTPConfig{Enabled: c.Enabled, RoutePrefix: strings.TrimSpace(c.RoutePrefix), ExportNodeLimit: c.ExportNodeLimit, Overview: graphdebug.OverviewOptions{DirectNodeLimit: c.DirectNodeLimit, EdgeLimit: c.EdgeLimit, ClusterLimit: c.OverviewClusterLimit, Semantic: semantic, RefreshMaxPaths: c.RefreshMaxPaths}, Neighbourhood: graphdebug.NeighbourhoodOptions{Depth: 1, ExpansionNodeLimit: c.ExpansionNodeLimit, EdgeLimit: c.EdgeLimit, Semantic: semantic, SemanticNodeLimit: c.RefreshMaxPaths, SemanticEdgeLimit: c.SemanticEdgeLimit}, Cluster: graphdebug.ClusterOptions{RefreshMaxPaths: c.RefreshMaxPaths, EdgeLimit: c.EdgeLimit, ExpansionNodeLimit: c.ExpansionNodeLimit, ClusterLimit: c.OverviewClusterLimit, Semantic: semantic}, PreviewChars: c.PreviewChars, SummaryLimit: c.OverviewClusterLimit}
}
