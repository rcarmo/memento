package graphdebug

import (
	"context"
	"github.com/rcarmo/memento/internal/access"
	"sort"
)

func (s *SnapshotService) ExportSelection(ctx context.Context, conceptIDs []string, limit, edgeLimit int, policy *access.EffectivePolicy) ([]Node, []Edge, Revisions, error) {
	uniqueMap := map[string]bool{}
	for _, id := range conceptIDs {
		uniqueMap[id] = true
	}
	unique := make([]string, 0, len(uniqueMap))
	for id := range uniqueMap {
		unique = append(unique, id)
	}
	sort.Strings(unique)
	if len(unique) == 0 || len(unique) > limit {
		return nil, nil, Revisions{}, &SnapshotError{"export selection must contain a bounded non-empty concept set"}
	}
	nodes, err := s.Nodes(ctx, unique, limit, policy, false)
	if err != nil {
		return nil, nil, Revisions{}, err
	}
	if len(nodes) != len(unique) {
		return nil, nil, Revisions{}, &SnapshotError{"export selection includes unknown memories"}
	}
	ids := make([]string, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	edges, err := s.ExplicitEdges(ctx, ids, nil, nil, edgeLimit, policy)
	if err != nil {
		return nil, nil, Revisions{}, err
	}
	nodes = ScopedNodes(nodes, edges)
	revisions, err := s.Revisions(ctx)
	if err != nil {
		return nil, nil, Revisions{}, err
	}
	return nodes, edges, revisions, nil
}
