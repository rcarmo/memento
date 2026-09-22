package graphdebug

import (
	"context"
	"math"
	"sort"
	"strings"
)

type SemanticConfig struct {
	Neighbours           int
	MinSimilarity        float64
	NodeLimit, EdgeLimit int
}
type semanticScore struct {
	source, target  string
	score           float64
	model, revision string
}
type scoredTarget struct {
	score  float64
	target string
}

func selectSemanticScores(scores []semanticScore, revisions Revisions, config SemanticConfig, limit int) []Edge {
	if limit <= 0 {
		return []Edge{}
	}
	directed := map[string][]scoredTarget{}
	metadata := map[string]semanticScore{}
	for _, pair := range scores {
		if pair.score < config.MinSimilarity {
			continue
		}
		directed[pair.source] = append(directed[pair.source], scoredTarget{pair.score, pair.target})
		directed[pair.target] = append(directed[pair.target], scoredTarget{pair.score, pair.source})
		metadata[pair.source+"\x00"+pair.target] = pair
	}
	sets := map[string]map[string]bool{}
	for source, candidates := range directed {
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].score != candidates[j].score {
				return candidates[i].score > candidates[j].score
			}
			return candidates[i].target < candidates[j].target
		})
		candidates = candidates[:min(len(candidates), max(0, config.Neighbours))]
		directed[source] = candidates
		sets[source] = map[string]bool{}
		for _, item := range candidates {
			sets[source][item.target] = true
		}
	}
	type pair struct {
		source, target string
		score          float64
		mutual         bool
	}
	pairs := map[string]pair{}
	for source, targets := range directed {
		for _, item := range targets {
			left, right := source, item.target
			if left > right {
				left, right = right, left
			}
			key := left + "\x00" + right
			candidate := pair{left, right, item.score, sets[item.target][source]}
			previous, ok := pairs[key]
			if !ok || candidate.score > previous.score || candidate.mutual && !previous.mutual {
				pairs[key] = candidate
			}
		}
	}
	ordered := make([]pair, 0, len(pairs))
	for _, item := range pairs {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].mutual != ordered[j].mutual {
			return ordered[i].mutual
		}
		if ordered[i].score != ordered[j].score {
			return ordered[i].score > ordered[j].score
		}
		if ordered[i].source != ordered[j].source {
			return ordered[i].source < ordered[j].source
		}
		return ordered[i].target < ordered[j].target
	})
	maximum := min(limit, config.EdgeLimit)
	if len(ordered) > maximum {
		ordered = ordered[:maximum]
	}
	out := make([]Edge, 0, len(ordered))
	for _, item := range ordered {
		v := metadata[item.source+"\x00"+item.target]
		target := item.target
		score := math.Round(item.score*1e6) / 1e6
		out = append(out, Edge{ID: "semantic:" + item.source + ":" + item.target, Source: item.source, Target: &target, RawTarget: "cosine:" + float4(item.score), Kind: "semantic_similarity", Canonical: false, Resolution: "derived", FirstSeenRevision: revisions.Repository, LastCheckedRevision: revisions.Repository, Similarity: &score, ModelID: &v.model, EmbeddingRevision: &v.revision})
	}
	return out
}

func float4(value float64) string {
	value = math.Round(value*1e4) / 1e4
	text := fmtFloat(value, 4)
	return text
}
func fmtFloat(value float64, places int) string {
	whole := int64(value)
	fraction := int64(math.Round(math.Abs(value-float64(whole)) * math.Pow10(places)))
	sign := ""
	if value < 0 && whole == 0 {
		sign = "-"
	}
	return sign + integerText(whole) + "." + strings.Repeat("0", places-len(integerText(fraction))) + integerText(fraction)
}

// SemanticEdges reads immutable item-relative scores. No vector blobs or dot
// products are loaded/computed on the HTTP path; permissions are applied before
// neighbour selection so hidden nodes cannot influence visible ranking.
func (s *SnapshotService) SemanticEdges(ctx context.Context, nodes []Node, revisions Revisions, config SemanticConfig, limit int) ([]Edge, error) {
	if limit <= 0 || len(nodes) < 2 || len(nodes) > config.NodeLimit {
		return []Edge{}, nil
	}
	db, err := s.open(ctx, s.DerivedDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var tables int
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('semantic_graph_items','semantic_graph_pairs')").Scan(&tables); err != nil {
		return nil, err
	}
	if tables != 2 {
		return []Edge{}, nil
	}
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	sort.Strings(ids)
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := []any{s.EmbeddingPolicy, s.EmbeddingPolicy}
	for _, id := range ids {
		args = append(args, id)
	}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.QueryContext(ctx, `SELECT pair.source_id,pair.target_id,pair.similarity,a.model_id,ea.embedding_revision
 FROM semantic_graph_pairs pair
 JOIN semantic_graph_items a ON a.concept_id=pair.source_id
 JOIN semantic_graph_items b ON b.concept_id=pair.target_id
 JOIN concept_embeddings ea ON ea.concept_id=a.concept_id
 JOIN concept_embeddings eb ON eb.concept_id=b.concept_id
 JOIN concept_embedding_policy pa ON pa.concept_id=a.concept_id
 JOIN concept_embedding_policy pb ON pb.concept_id=b.concept_id
 WHERE ea.status='ready' AND eb.status='ready'
 AND a.algorithm='mean-normalized-chunks-v1' AND b.algorithm=a.algorithm
 AND a.document_hash=ea.embedding_text_hash AND b.document_hash=eb.embedding_text_hash
 AND a.model_id=ea.model_id AND b.model_id=eb.model_id AND a.model_id=b.model_id
 AND a.model_revision=ea.model_revision AND b.model_revision=eb.model_revision AND a.model_revision=b.model_revision
 AND a.dimensions=ea.dimensions AND b.dimensions=eb.dimensions AND a.dimensions=b.dimensions
 AND a.policy=pa.policy AND b.policy=pb.policy AND a.policy=b.policy
 AND (?='' OR a.policy=?) AND a.concept_id IN (`+marks+`) AND b.concept_id IN (`+marks+`) ORDER BY pair.source_id,pair.target_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scores := []semanticScore{}
	for rows.Next() {
		var row semanticScore
		if err = rows.Scan(&row.source, &row.target, &row.score, &row.model, &row.revision); err != nil {
			return nil, err
		}
		if math.IsNaN(row.score) || math.IsInf(row.score, 0) {
			continue
		}
		scores = append(scores, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return selectSemanticScores(scores, revisions, config, limit), nil
}
