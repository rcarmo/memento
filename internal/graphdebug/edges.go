package graphdebug

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/rcarmo/memento/internal/access"
)

type Edge struct {
	ID                  string   `json:"id"`
	Source              string   `json:"source"`
	Target              *string  `json:"target"`
	RawTarget           string   `json:"raw_target"`
	Kind                string   `json:"kind"`
	Canonical           bool     `json:"canonical"`
	Resolution          string   `json:"resolution"`
	Anchor              *string  `json:"anchor"`
	FirstSeenRevision   string   `json:"first_seen_revision"`
	LastCheckedRevision string   `json:"last_checked_revision"`
	Similarity          *float64 `json:"similarity"`
	ModelID             *string  `json:"model_id"`
	EmbeddingRevision   *string  `json:"embedding_revision"`
}

func (s *SnapshotService) ExplicitEdges(ctx context.Context, ids []string, sourceID, targetID *string, limit int, policy *access.EffectivePolicy) ([]Edge, error) {
	db, err := s.open(ctx, s.DerivedDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	clauses := []string{}
	args := []any{}
	if sourceID != nil {
		clauses = append(clauses, "source_id = ?")
		args = append(args, *sourceID)
	}
	if targetID != nil {
		clauses = append(clauses, "target_id = ?")
		args = append(args, *targetID)
	}
	idset := map[string]bool{}
	if ids != nil {
		if len(ids) == 0 {
			return []Edge{}, nil
		}
		for _, id := range ids {
			idset[id] = true
		}
		ordered := make([]string, 0, len(idset))
		for id := range idset {
			ordered = append(ordered, id)
		}
		sort.Strings(ordered)
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ordered)), ",")
		clauses = append(clauses, "source_id IN ("+placeholders+")", "(target_id IS NULL OR target_id IN ("+placeholders+"))")
		for range 2 {
			for _, id := range ordered {
				args = append(args, id)
			}
		}
	}
	if policy != nil {
		scope, scopeArgs := pathScopeSQL("target_path", policy)
		clauses = append(clauses, "(target_id IS NOT NULL OR "+scope+")")
		args = append(args, scopeArgs...)
	}
	query := "SELECT rowid,source_id,target_id,raw_target,resolution_state,anchor,first_seen_revision,last_checked_revision FROM links"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY source_id,raw_target,rowid LIMIT ?"
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Edge{}
	for rows.Next() {
		var rowid int64
		var source, raw, resolution, first, last string
		var target, anchor sql.NullString
		if err = rows.Scan(&rowid, &source, &target, &raw, &resolution, &anchor, &first, &last); err != nil {
			return nil, err
		}
		if ids != nil && (!idset[source] || target.Valid && !idset[target.String]) {
			continue
		}
		var targetPtr, anchorPtr *string
		if target.Valid {
			value := target.String
			targetPtr = &value
		}
		if anchor.Valid {
			value := anchor.String
			anchorPtr = &value
		}
		result = append(result, Edge{ID: "explicit:" + integerText(rowid), Source: source, Target: targetPtr, RawTarget: raw, Kind: "explicit", Canonical: true, Resolution: resolution, Anchor: anchorPtr, FirstSeenRevision: first, LastCheckedRevision: last})
		if len(result) >= limit {
			break
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
func integerText(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	buf := make([]byte, 0, 20)
	for value > 0 {
		buf = append(buf, byte('0'+value%10))
		value /= 10
	}
	if negative {
		buf = append(buf, '-')
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
