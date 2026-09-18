package graphdebug

import (
	"context"
	"encoding/json"
	"github.com/rcarmo/memento/go/access"
	"sort"
)

type ProposalCount struct{ Total, Pending int }

func proposalPaths(raw string) []string {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return []string{}
	}
	found := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, item := range value {
				if key == "path" || key == "new_path" || key == "concept_path" {
					if path, ok := item.(string); ok {
						found[path] = true
					}
				}
				visit(item)
			}
		case []any:
			for _, item := range value {
				visit(item)
			}
		}
	}
	visit(value)
	paths := make([]string, 0, len(found))
	for path := range found {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
func proposalVisible(author string, paths []string, policy *access.EffectivePolicy) bool {
	if policy == nil {
		return true
	}
	if author != policy.Principal && !hasRole(policy.Roles, "curator") {
		return false
	}
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		if _, err := access.AuthorizePath(*policy, path, "read"); err != nil {
			return false
		}
	}
	return true
}
func (s *SnapshotService) ProposalCounts(ctx context.Context, policy *access.EffectivePolicy) (map[string]ProposalCount, error) {
	db, err := s.open(ctx, s.ControlDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT status,patch_json,author_principal FROM proposals ORDER BY proposal_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]ProposalCount{}
	for rows.Next() {
		var status, patch, author string
		if err = rows.Scan(&status, &patch, &author); err != nil {
			return nil, err
		}
		paths := proposalPaths(patch)
		if !proposalVisible(author, paths, policy) {
			continue
		}
		for _, path := range paths {
			count := counts[path]
			count.Total++
			if status == "draft" || status == "submitted" || status == "approved" {
				count.Pending++
			}
			counts[path] = count
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}
