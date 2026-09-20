package graphdebug

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"sort"
	"strings"
)

type SemanticConfig struct {
	Neighbours           int
	MinSimilarity        float64
	NodeLimit, EdgeLimit int
}
type semanticVector struct {
	values          []float64
	norm            float64
	model, revision string
}
type scoredTarget struct {
	score  float64
	target string
}

func selectSemantic(vectors map[string]semanticVector, revisions Revisions, config SemanticConfig, limit int) []Edge {
	if limit <= 0 || len(vectors) < 2 || len(vectors) > config.NodeLimit || revisions.Embedding == nil || *revisions.Embedding != revisions.Repository {
		return []Edge{}
	}
	ids := make([]string, 0, len(vectors))
	for id := range vectors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	directed := map[string][]scoredTarget{}
	sets := map[string]map[string]bool{}
	for _, source := range ids {
		a := vectors[source]
		candidates := []scoredTarget{}
		for _, target := range ids {
			if target == source {
				continue
			}
			b := vectors[target]
			if a.model != b.model || a.revision != b.revision || len(a.values) != len(b.values) {
				continue
			}
			dot := 0.0
			for i := range a.values {
				dot += a.values[i] * b.values[i]
			}
			score := dot / (a.norm * b.norm)
			if score >= config.MinSimilarity {
				candidates = append(candidates, scoredTarget{score, target})
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].score != candidates[j].score {
				return candidates[i].score > candidates[j].score
			}
			return candidates[i].target < candidates[j].target
		})
		if len(candidates) > config.Neighbours {
			candidates = candidates[:config.Neighbours]
		}
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
		v := vectors[item.source]
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
func (s *SnapshotService) SemanticEdges(ctx context.Context, nodes []Node, revisions Revisions, config SemanticConfig, limit int) ([]Edge, error) {
	if limit <= 0 || len(nodes) < 2 || len(nodes) > config.NodeLimit || revisions.Embedding == nil || *revisions.Embedding != revisions.Repository {
		return []Edge{}, nil
	}
	db, err := s.open(ctx, s.DerivedDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ids := make([]string, len(nodes))
	for i, node := range nodes {
		ids[i] = node.ID
	}
	sort.Strings(ids)
	query := "SELECT concept_id,embedding_blob,embedding_norm,model_id,embedding_revision FROM concept_embeddings WHERE status='ready' AND concept_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ") ORDER BY concept_id"
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vectors := map[string]semanticVector{}
	for rows.Next() {
		var id, model, revision string
		var blob []byte
		var stored sql.NullFloat64
		if err = rows.Scan(&id, &blob, &stored, &model, &revision); err != nil {
			return nil, err
		}
		if len(blob) == 0 || len(blob)%4 != 0 {
			continue
		}
		values := make([]float64, len(blob)/4)
		sum := 0.0
		for i := range values {
			values[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:])))
			sum += values[i] * values[i]
		}
		norm := math.Sqrt(sum)
		if stored.Valid && stored.Float64 != 0 {
			norm = stored.Float64
		}
		if norm <= 0 {
			continue
		}
		vectors[id] = semanticVector{values, norm, model, revision}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return selectSemantic(vectors, revisions, config, limit), nil
}
