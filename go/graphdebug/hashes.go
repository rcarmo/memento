package graphdebug

import (
	"context"
	"sort"
	"strings"
)

func (s *SnapshotService) ContentHashes(ctx context.Context, ids []string) (map[string]string, error) {
	if ids != nil && len(ids) == 0 {
		return map[string]string{}, nil
	}
	db, err := s.open(ctx, s.DerivedDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	query := "SELECT id,content_hash FROM concepts"
	args := []any{}
	if ids != nil {
		ordered := append([]string{}, ids...)
		sort.Strings(ordered)
		query += " WHERE id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ordered)), ",") + ") ORDER BY id"
		for _, id := range ordered {
			args = append(args, id)
		}
	} else {
		query += " ORDER BY id"
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, hash string
		if err = rows.Scan(&id, &hash); err != nil {
			return nil, err
		}
		out[id] = hash
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
