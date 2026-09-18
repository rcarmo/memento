package service

import (
	"context"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/graphdebug"
)

type GraphSnapshotAdapter struct {
	Service                    *graphdebug.SnapshotService
	DirectNodeLimit, EdgeLimit int
}

func (a GraphSnapshotAdapter) Overview(ctx context.Context, policy access.EffectivePolicy) (AuditOverview, error) {
	if a.Service == nil {
		return AuditOverview{}, &GraphSnapshotError{"graph snapshot unavailable"}
	}
	limit := a.DirectNodeLimit
	if limit <= 0 {
		limit = 2000
	}
	edges := a.EdgeLimit
	if edges <= 0 {
		edges = 12000
	}
	overview, err := a.Service.Overview(ctx, &policy, graphdebug.OverviewOptions{DirectNodeLimit: limit, EdgeLimit: edges})
	if err != nil {
		return AuditOverview{}, err
	}
	if overview.Mode != "direct" {
		return AuditOverview{}, &GraphSnapshotError{"graph audit requires a direct snapshot"}
	}
	nodes := make([]AuditGraphNode, len(overview.Nodes))
	for i, node := range overview.Nodes {
		nodes[i] = AuditGraphNode{ID: node.ID, Path: node.Path}
	}
	diagnostics := make([]AuditDiagnostic, len(overview.Diagnostics))
	for i, item := range overview.Diagnostics {
		diagnostics[i] = AuditDiagnostic{ID: item.ID, Rule: item.Rule, Severity: item.Severity, ConceptIDs: append([]string{}, item.ConceptIDs...), Message: item.Message, Measured: item.Measured, Threshold: item.Threshold, Derived: item.Derived}
	}
	return AuditOverview{IndexRevision: overview.Revisions.Index, Stale: overview.Revisions.Stale, Nodes: nodes, Diagnostics: diagnostics}, nil
}
