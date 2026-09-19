package service

import (
	"github.com/rcarmo/memento/go/graphdebug"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGraphExplorerDefaultsAndMapping(t *testing.T) {
	c := DefaultGraphExplorerConfig()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	got := c.HTTPConfig()
	if got.Enabled || got.RoutePrefix != "/graph" || got.ExportNodeLimit != 2000 || got.PreviewChars != 4000 || got.Overview.DirectNodeLimit != 2000 || got.Overview.ClusterLimit != 500 || got.Overview.EdgeLimit != 12000 || got.Neighbourhood.Depth != 1 || got.Neighbourhood.ExpansionNodeLimit != 2000 || got.Cluster.RefreshMaxPaths != 2000 || got.SummaryLimit != 500 {
		t.Fatal(got)
	}
	semantic := graphdebug.SemanticConfig{Neighbours: 12, MinSimilarity: .75, NodeLimit: 300, EdgeLimit: 1500}
	if !reflect.DeepEqual(got.Overview.Semantic, semantic) || !reflect.DeepEqual(got.Cluster.Semantic, semantic) || !reflect.DeepEqual(got.Neighbourhood.Semantic, semantic) {
		t.Fatal(got)
	}
}
func TestGraphExplorerValidation(t *testing.T) {
	base := DefaultGraphExplorerConfig()
	for _, prefix := range []string{"graph", "/", "/graph/", "/graph?x", "/graph#x", "/graph/../x"} {
		c := base
		c.RoutePrefix = prefix
		if c.Validate() == nil {
			t.Fatal(prefix)
		}
	}
	mutations := []func(*GraphExplorerConfig){func(c *GraphExplorerConfig) { c.DirectNodeLimit = 0 }, func(c *GraphExplorerConfig) { c.OverviewClusterLimit = 1001 }, func(c *GraphExplorerConfig) { c.ExpansionNodeLimit = 0 }, func(c *GraphExplorerConfig) { c.EdgeLimit = 20001 }, func(c *GraphExplorerConfig) { c.PreviewChars = -1 }, func(c *GraphExplorerConfig) { c.SemanticNeighbours = 101 }, func(c *GraphExplorerConfig) { c.SemanticEdgeNodeLimit = 0 }, func(c *GraphExplorerConfig) { c.SemanticEdgeLimit = 12001 }, func(c *GraphExplorerConfig) { c.ExportNodeLimit = 0 }, func(c *GraphExplorerConfig) { c.RefreshMaxPaths = 10001 }, func(c *GraphExplorerConfig) { c.SemanticMinSimilarity = 2 }}
	for index, mutate := range mutations {
		c := base
		mutate(&c)
		if c.Validate() == nil {
			t.Fatal(index)
		}
	}
}
func TestRuntimeConfigGraphExplorer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"repository":{"root_path":"/tmp/runtime"},"authorization":{"principals":{},"protected_read_prefixes":[]},"observability":{"graph_explorer":{"enabled":true,"route_prefix":" /debug ","direct_node_limit":10}}}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadRuntimeConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	graph := config.Observability.GraphExplorer
	if !graph.Enabled || graph.RoutePrefix != " /debug " || graph.DirectNodeLimit != 10 || graph.EdgeLimit != 12000 {
		t.Fatal(graph)
	}
	if graph.HTTPConfig().RoutePrefix != "/debug" {
		t.Fatal(graph.HTTPConfig())
	}
	if err = os.WriteFile(path, []byte(`{"repository":{"root_path":"/tmp/runtime"},"authorization":{},"observability":{"graph_explorer":{"route_prefix":"bad"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadRuntimeConfig(path); err == nil {
		t.Fatal("invalid graph config")
	}
}
