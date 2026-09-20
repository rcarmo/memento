package derived

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/rcarmo/memento/internal/access"
)

type ConceptNotFoundError struct{ ConceptID string }

func (e *ConceptNotFoundError) Error() string { return e.ConceptID }

type GraphEdge struct {
	ConceptID       string `json:"concept_id"`
	Path            string `json:"path"`
	Title           string `json:"title"`
	Depth           int    `json:"depth"`
	Direction       string `json:"direction"`
	BrokenLinkCount int    `json:"broken_link_count"`
	Orphan          bool   `json:"orphan_flag"`
}
type GraphNeighborhood struct {
	CenterID      string      `json:"center_id"`
	Outbound      []GraphEdge `json:"outbound"`
	Inbound       []GraphEdge `json:"inbound"`
	BrokenTargets []string    `json:"broken_targets"`
	RepoRevision  string      `json:"repo_revision"`
	IndexRevision string      `json:"index_revision"`
}
type GraphMetrics struct {
	ConceptID       string `json:"concept_id"`
	InboundDegree   int    `json:"inbound_degree"`
	OutboundDegree  int    `json:"outbound_degree"`
	BrokenLinkCount int    `json:"broken_link_count"`
	Orphan          bool   `json:"orphan_flag"`
}
type GraphOptions struct {
	Depth   int
	Strict  bool
	Timeout time.Duration
}

func (s ContentStore) Graph(ctx context.Context, policy access.EffectivePolicy, id string, options GraphOptions) (GraphNeighborhood, error) {
	empty := GraphNeighborhood{}
	if options.Strict {
		if _, err := s.WaitForFreshness(ctx, options.Timeout); err != nil {
			return empty, err
		}
	}
	state, err := s.State(ctx)
	if err != nil {
		return empty, err
	}
	if state.Status == "quarantined" {
		return empty, &CorruptionError{"derived index is quarantined"}
	}
	if err = authorizeConcept(ctx, s.DB, policy, id); err != nil {
		return empty, err
	}
	depth := max(0, min(options.Depth, 2))
	outbound, err := collectNeighbors(ctx, s.DB, policy, id, "outbound", depth)
	if err != nil {
		return empty, err
	}
	inbound, err := collectNeighbors(ctx, s.DB, policy, id, "inbound", depth)
	if err != nil {
		return empty, err
	}
	broken, err := visibleBroken(ctx, s.DB, policy, id, true)
	if err != nil {
		return empty, err
	}
	return GraphNeighborhood{id, outbound, inbound, broken, state.RepoRevision, state.IndexRevision}, nil
}

// Metrics is unscoped, exactly as in Python. Caller must authorise before
// exposing this helper; Graph uses distinct, policy-scoped broken/orphan counts.
func (s ContentStore) Metrics(ctx context.Context, id string) (GraphMetrics, error) {
	var m GraphMetrics
	var orphan int
	err := s.DB.QueryRowContext(ctx, "SELECT concept_id,inbound_degree,outbound_degree,broken_link_count,orphan_flag FROM graph_metrics WHERE concept_id=?", id).Scan(&m.ConceptID, &m.InboundDegree, &m.OutboundDegree, &m.BrokenLinkCount, &orphan)
	if errors.Is(err, sql.ErrNoRows) {
		return m, &ConceptNotFoundError{id}
	}
	if err != nil {
		return m, err
	}
	m.Orphan = orphan != 0
	return m, nil
}
func authorizeConcept(ctx context.Context, db executor, policy access.EffectivePolicy, id string) error {
	var path string
	err := db.QueryRowContext(ctx, "SELECT path FROM concepts WHERE id=?", id).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return &ConceptNotFoundError{id}
	}
	if err != nil {
		return err
	}
	if _, err = access.AuthorizePath(policy, path, "read"); err != nil {
		return &ConceptNotFoundError{id}
	}
	return nil
}
func neighborQuery(direction string, policy access.EffectivePolicy) (string, []any) {
	scope, args := authorizedPrefixes(policy, "c")
	join, filter := "l.source_id", "l.target_id"
	if direction == "outbound" {
		join, filter = "l.target_id", "l.source_id"
	}
	return "SELECT c.id,c.path,c.title FROM links l JOIN concepts c ON c.id=" + join + " WHERE " + filter + "=? AND " + scope, args
}
func collectNeighbors(ctx context.Context, db executor, policy access.EffectivePolicy, id, direction string, depth int) ([]GraphEdge, error) {
	edges := []GraphEdge{}
	if depth == 0 {
		return edges, nil
	}
	seen := map[string]bool{id: true}
	type node struct {
		id    string
		depth int
	}
	frontier := []node{{id, 0}}
	for len(frontier) > 0 {
		current := frontier[0]
		frontier = frontier[1:]
		if current.depth >= depth {
			continue
		}
		query, args := neighborQuery(direction, policy)
		args = append([]any{current.id}, args...)
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		neighbors, err := neighborRows(rows)
		if err != nil {
			return nil, err
		}
		for _, neighbor := range neighbors {
			if seen[neighbor.ConceptID] {
				continue
			}
			seen[neighbor.ConceptID] = true
			neighbor.Depth = current.depth + 1
			neighbor.Direction = direction
			frontier = append(frontier, node{neighbor.ConceptID, neighbor.Depth})
			broken, err := visibleBroken(ctx, db, policy, neighbor.ConceptID, false)
			if err != nil {
				return nil, err
			}
			neighbor.BrokenLinkCount = len(broken)
			outgoing, err := hasNeighbors(ctx, db, policy, neighbor.ConceptID, "outbound")
			if err != nil {
				return nil, err
			}
			incoming, err := hasNeighbors(ctx, db, policy, neighbor.ConceptID, "inbound")
			if err != nil {
				return nil, err
			}
			neighbor.Orphan = !outgoing && !incoming
			edges = append(edges, neighbor)
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.Depth != b.Depth {
			return a.Depth < b.Depth
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.ConceptID < b.ConceptID
	})
	return edges, nil
}
func neighborRows(rows rows) ([]GraphEdge, error) {
	defer rows.Close()
	result := []GraphEdge{}
	for rows.Next() {
		var edge GraphEdge
		if err := rows.Scan(&edge.ConceptID, &edge.Path, &edge.Title); err != nil {
			return nil, err
		}
		result = append(result, edge)
	}
	return result, rows.Err()
}
func hasNeighbors(ctx context.Context, db executor, policy access.EffectivePolicy, id, direction string) (bool, error) {
	query, args := neighborQuery(direction, policy)
	args = append([]any{id}, args...)
	var one int
	err := db.QueryRowContext(ctx, "SELECT 1 FROM ("+query+") LIMIT 1", args...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
func visibleBroken(ctx context.Context, db executor, policy access.EffectivePolicy, id string, ordered bool) ([]string, error) {
	query := "SELECT DISTINCT raw_target,target_path FROM links WHERE source_id=? AND resolution_state='broken'"
	if ordered {
		query += " ORDER BY raw_target"
	}
	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	return brokenRows(rows, policy)
}
func brokenRows(rows rows, policy access.EffectivePolicy) ([]string, error) {
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var raw string
		var path sql.NullString
		if err := rows.Scan(&raw, &path); err != nil {
			return nil, err
		}
		target := raw
		if path.Valid && path.String != "" {
			target = path.String
		}
		if _, err := access.AuthorizePath(policy, target, "read"); err == nil {
			result = append(result, raw)
		}
	}
	return result, rows.Err()
}
