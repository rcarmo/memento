package graphdebug

import (
	"math"
	"sort"
)

// Pure in-memory reference verifies persisted-score ranking matches vector ranking.
type semanticVector struct {
	values                         []float64
	norm                           float64
	model, revision, modelRevision string
	policy                         string
}

func selectSemantic(vectors map[string]semanticVector, revisions Revisions, config SemanticConfig, limit int) []Edge {
	if limit <= 0 || len(vectors) < 2 || len(vectors) > config.NodeLimit {
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
			if a.model != b.model || a.modelRevision != b.modelRevision || a.policy != b.policy || len(a.values) != len(b.values) {
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
