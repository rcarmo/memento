package graphdebug

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

const (
	alphaNodeID = "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d"
	betaNodeID  = "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e"
)

func nodeByID(t *testing.T, nodes []Node, id string) Node {
	t.Helper()
	for _, node := range nodes {
		if node.ID == id {
			return node
		}
	}
	t.Fatalf("missing node %s", id)
	return Node{}
}

func TestDetailEdgeLimitKeepsScopedMetrics(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := s.Detail(context.Background(), alphaNodeID, nil, 100, 1, 10, DetailScope{NodeLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Outbound) != 1 || got.Outbound[0].Resolution != "broken" || got.Outbound[0].RawTarget != "/private/missing.md" {
		t.Fatal(got.Outbound)
	}
	if got.Node.ExplicitOutDegree != 1 || got.Node.BrokenLinkCount != 1 || got.Node.Orphan {
		t.Fatal(got.Node)
	}
}

func TestOverviewEdgeLimitKeepsScopedMetricsAndMarksTruncated(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := s.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || len(got.Edges) != 1 || got.Edges[0].Resolution != "broken" || got.Edges[0].RawTarget != "/private/missing.md" {
		t.Fatal(got)
	}
	alpha := nodeByID(t, got.Nodes, alphaNodeID)
	beta := nodeByID(t, got.Nodes, betaNodeID)
	if alpha.ExplicitOutDegree != 1 || alpha.BrokenLinkCount != 1 || alpha.Orphan {
		t.Fatal(alpha)
	}
	if beta.ExplicitInDegree != 1 || beta.BrokenLinkCount != 1 || beta.Orphan {
		t.Fatal(beta)
	}
	if got.Metrics.ExplicitEdges != 3 || got.Metrics.BrokenEdges != 2 {
		t.Fatal(got.Metrics)
	}
}

func TestOverviewAcceptedAssetDoesNotCountBroken(t *testing.T) {
	root, path := nodeDB(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("DELETE FROM links; INSERT INTO links VALUES(?,NULL,'references/guide.md','/a.md/references/guide.md',NULL,'internal','asset','r1','r2')", alphaNodeID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	s := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := s.Overview(context.Background(), nil, OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got.Metrics.BrokenEdges != 0 {
		t.Fatal(got.Metrics)
	}
	alpha := nodeByID(t, got.Nodes, alphaNodeID)
	if alpha.BrokenLinkCount != 0 || alpha.ExplicitOutDegree != 0 || !alpha.Orphan {
		t.Fatal(alpha)
	}
}

func TestNeighbourhoodEdgeLimitKeepsScopedMetrics(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	o := NeighbourhoodOptions{Depth: 1, EdgeLimit: 1, SemanticNodeLimit: 10, ExpansionNodeLimit: 10}
	got, err := s.Neighbourhood(context.Background(), betaNodeID, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	alpha := nodeByID(t, got.Nodes, alphaNodeID)
	beta := nodeByID(t, got.Nodes, betaNodeID)
	if alpha.ExplicitOutDegree != 1 || alpha.BrokenLinkCount != 1 || alpha.Orphan {
		t.Fatal(alpha)
	}
	if beta.ExplicitInDegree != 1 || beta.Orphan {
		t.Fatal(beta)
	}
	if len(got.Edges) != 1 || got.Edges[0].Resolution != "broken" || got.Edges[0].RawTarget != "/private/missing.md" {
		t.Fatal(got.Edges)
	}
}

func TestClusterExpansionEdgeLimitKeepsScopedMetrics(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := s.ExpandCluster(context.Background(), "cluster:overflow", nil, ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 1, ExpansionNodeLimit: 1, ClusterLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 1 || got.Nodes[0].ID != alphaNodeID {
		t.Fatal(got)
	}
	if got.Nodes[0].ExplicitOutDegree != 1 || got.Nodes[0].BrokenLinkCount != 1 || got.Nodes[0].Orphan {
		t.Fatal(got.Nodes[0])
	}
}

func TestDiagnosticDisplayBoundsAndCentre(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	ctx := t.Context()
	if edges, truncated := truncateExplicitEdges([]Edge{{ID: "e"}}, 0); len(edges) != 0 || !truncated {
		t.Fatal(edges, truncated)
	}
	target := "outside"
	if edges := filterExplicitEdges([]Edge{{Source: "inside", Target: &target}}, map[string]bool{"inside": true}); len(edges) != 0 {
		t.Fatal(edges)
	}
	// A cap smaller than the visible scope must remain visible to the client.
	overview, err := s.Overview(ctx, nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 100, RefreshMaxPaths: 1, ClusterLimit: 10})
	if err != nil || !overview.Truncated {
		t.Fatal(overview, err)
	}
	full, err := s.Overview(ctx, nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 100, RefreshMaxPaths: 10, ClusterLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExpandCluster(ctx, full.Clusters[0].ID, nil, ClusterOptions{RefreshMaxPaths: 1, EdgeLimit: 100, ExpansionNodeLimit: 1, ClusterLimit: 1}); err == nil {
		t.Fatal("a different capped cluster universe cannot reuse another cluster ID")
	}
	nodes, err := s.Nodes(ctx, nil, 10, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	center := nodes[len(nodes)-1].ID
	hood, err := s.Neighbourhood(ctx, center, nil, NeighbourhoodOptions{Depth: 1, SemanticNodeLimit: 1, ExpansionNodeLimit: 1, EdgeLimit: 0})
	if err != nil || len(hood.Nodes) != 1 || hood.Nodes[0].ID != center || !hood.Truncated {
		t.Fatal(hood, err)
	}
}
func TestDetailSelfAndInboundLimits(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	ctx := t.Context()
	nodes, err := s.Nodes(ctx, nil, 10, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, node := range nodes {
		if _, err = db.Exec(`INSERT INTO links(source_id,target_id,raw_target,target_path,anchor,link_kind,resolution_state,first_seen_revision,last_checked_revision) VALUES(?,?,?,?,'','markdown','resolved','main','main')`, node.ID, node.ID, "#self", node.Path); err != nil {
			t.Fatal(err)
		}
		detail, err := s.Detail(ctx, node.ID, nil, 50, 100, 1, DetailScope{NodeLimit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if detail.Node.Orphan {
			t.Fatal("self link is explicit by established contract")
		}
		zero, err := s.Detail(ctx, node.ID, nil, 50, 0, 1, DetailScope{NodeLimit: 100})
		if err != nil || !zero.Truncated || len(zero.Inbound)+len(zero.Outbound) != 0 {
			t.Fatal(zero, err)
		}
	}
}
func TestClusterSemanticEdgesAndTrashScope(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	ctx := t.Context()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`UPDATE concept_embeddings SET status='ready',embedding_revision='main',model_id='model',embedding_blob=?,embedding_norm=1,dimensions=2; UPDATE index_state SET value='main' WHERE key='semantic_embedding_revision'`, blob(1, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO concept_embeddings(concept_id,status,model_id,dimensions,embedding_revision,model_revision,updated_at,embedding_blob,embedding_norm) SELECT id,'ready','model',2,'main','v1','now',?,1 FROM concepts WHERE id NOT IN (SELECT concept_id FROM concept_embeddings)`, blob(1, 0)); err != nil {
		t.Fatal(err)
	}
	// Exercise nonempty semantic edges while explicit edges also exist.
	seedGraphChunks(t, db)
	options := ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 100, ExpansionNodeLimit: 10, ClusterLimit: 1, Semantic: SemanticConfig{Neighbours: 12, MinSimilarity: -1, NodeLimit: 10, EdgeLimit: 100}}
	overview, err := s.Overview(ctx, nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 100, RefreshMaxPaths: 10, ClusterLimit: 1, Semantic: options.Semantic})
	if err != nil {
		t.Fatal(err)
	}
	expansion, err := s.ExpandCluster(ctx, overview.Clusters[0].ID, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, edge := range expansion.Edges {
		if edge.Kind == "semantic_similarity" {
			found = true
		}
	}
	if !found {
		t.Fatal("semantic edges lost", expansion.Edges)
	}
}

func TestFreshSelectionDiagnostics(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	ctx := t.Context()
	nodes, err := s.Nodes(ctx, nil, 10, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.Detail(ctx, nodes[0].ID, nil, 50, 1, 1, DetailScope{NodeLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range d.Diagnostics {
		for _, id := range finding.ConceptIDs {
			if id != d.Node.ID {
				t.Fatal("unrelated diagnostic target", finding)
			}
		}
	}
	before := 0
	for _, finding := range d.Diagnostics {
		if finding.Rule == "broken_links" {
			before++
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("DELETE FROM links WHERE resolution_state='broken'"); err != nil {
		t.Fatal(err)
	}
	d, err = s.Detail(ctx, nodes[0].ID, nil, 50, 1, 1, DetailScope{NodeLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if before == 0 {
		t.Fatal("fixture lacked broken link")
	}
	for _, finding := range d.Diagnostics {
		if finding.Rule == "broken_links" {
			t.Fatal("stale diagnostic retained")
		}
	}
}
func TestScopedDiagnosticQueryFailure(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	s.open = func(context.Context, string) (*sql.DB, error) { return nil, errors.New("unavailable") }
	if _, err := s.scopedDiagnostics(t.Context(), []Node{{ID: "x"}}, nil, Revisions{}, []Node{{ID: "x"}}); err == nil {
		t.Fatal("query error")
	}
}

func TestClusterIncludeTrashMatchesView(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	ctx := t.Context()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("UPDATE concepts SET path='/trash/private/b.md' WHERE id=?", betaNodeID); err != nil {
		t.Fatal(err)
	}
	options := ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 100, ExpansionNodeLimit: 10, ClusterLimit: 1}
	for _, include := range []bool{false, true} {
		all, err := s.Overview(ctx, nil, OverviewOptions{DirectNodeLimit: 1, EdgeLimit: 100, RefreshMaxPaths: 10, ClusterLimit: 1, IncludeTrash: include})
		if err != nil {
			t.Fatal(err)
		}
		if !include {
			for _, node := range all.Nodes {
				if node.ID == betaNodeID {
					t.Fatal("trash in overview")
				}
			}
			continue
		}
		options.IncludeTrash = true
		expansion, err := s.ExpandCluster(ctx, all.Clusters[0].ID, nil, options)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, node := range expansion.Nodes {
			if node.ID == betaNodeID {
				found = true
			}
		}
		if !found {
			t.Fatal("included trash missing")
		}
		options.IncludeTrash = false
		expansion, err = s.ExpandCluster(ctx, all.Clusters[0].ID, nil, options)
		if err == nil {
			for _, node := range expansion.Nodes {
				if node.ID == betaNodeID {
					t.Fatal("trash in hidden expansion")
				}
			}
		}
	}
}

func TestDetailScopeCapRetainsCenter(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	got, err := s.Detail(t.Context(), betaNodeID, nil, 50, 10, 10, DetailScope{NodeLimit: 0})
	if err != nil {
		t.Fatal(err)
	}
	if got.Node.ID != betaNodeID || !got.Truncated {
		t.Fatal(got)
	}
	for _, edge := range append(got.Inbound, got.Outbound...) {
		if edge.Source == alphaNodeID || edge.Target != nil && *edge.Target == alphaNodeID {
			t.Fatal("out-of-scope alpha included", edge)
		}
	}
}

func TestDrilldownIncludeTrashMatchesView(t *testing.T) {
	root, path := nodeDB(t)
	s := NewSnapshotService(root, path, emptyControlDB(t))
	ctx := t.Context()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("UPDATE concepts SET path='/trash/b.md' WHERE id=?", betaNodeID); err != nil {
		t.Fatal(err)
	}
	for _, include := range []bool{false, true} {
		detail, err := s.Detail(ctx, alphaNodeID, nil, 50, 100, 10, DetailScope{NodeLimit: 100, IncludeTrash: include})
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if include {
			want = 1
		}
		if detail.Node.ExplicitOutDegree != want {
			t.Fatal(include, detail.Node)
		}
		hood, err := s.Neighbourhood(ctx, alphaNodeID, nil, NeighbourhoodOptions{Depth: 1, EdgeLimit: 100, SemanticNodeLimit: 100, ExpansionNodeLimit: 100, IncludeTrash: include})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, node := range hood.Nodes {
			if node.ID == betaNodeID {
				found = true
			}
		}
		if found != include {
			t.Fatal(include, hood)
		}
	}
}
