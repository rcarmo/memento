package graphdebug

import "context"

// scopedDiagnostics calculates findings against the permitted snapshot, then
// returns only findings and target IDs belonging to the requested selection.
// Messages/measurements retain their full-snapshot meaning.
func (s *SnapshotService) scopedDiagnostics(ctx context.Context, nodes []Node, edges []Edge, revisions Revisions, selected []Node) ([]Diagnostic, error) {
	ids := make([]string, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	hashes, err := s.ContentHashes(ctx, ids)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, node := range selected {
		wanted[node.ID] = true
	}
	result := []Diagnostic{}
	for _, d := range DiagnoseGraph(ScopedNodes(nodes, edges), edges, revisions, hashes) {
		matched := []string{}
		for _, id := range d.ConceptIDs {
			if wanted[id] {
				matched = append(matched, id)
			}
		}
		if len(matched) == 0 {
			continue
		}
		d.ScopeLimited = len(matched) != len(d.ConceptIDs)
		d.ConceptIDs = matched
		result = append(result, d)
	}
	return result, nil
}
