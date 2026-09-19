package derived

import "sort"

const semanticRRFK = 60
const hybridLexicalWeight = .5

type SemanticCandidate struct {
	ID, Path string
	Cosine   float64
}

func hybridCandidateLess(left, right SemanticCandidate, lexical, semantic map[string]int) bool {
	score := func(item SemanticCandidate) float64 {
		value := 1 / float64(semanticRRFK+semantic[item.ID])
		if rank := lexical[item.ID]; rank > 0 {
			value += hybridLexicalWeight / float64(semanticRRFK+rank)
		}
		return value
	}
	ls, rs := score(left), score(right)
	if ls != rs {
		return ls > rs
	}
	if left.Cosine != right.Cosine {
		return left.Cosine > right.Cosine
	}
	li, lj := lexical[left.ID], lexical[right.ID]
	if li == 0 {
		li = 1_000_000_000
	}
	if lj == 0 {
		lj = 1_000_000_000
	}
	if li != lj {
		return li < lj
	}
	if left.Path != right.Path {
		return left.Path < right.Path
	}
	return left.ID < right.ID
}
func RankSemanticCandidates(candidates []SemanticCandidate, lexicalIDs []string, hybrid bool, offset, limit int) []SemanticCandidate {
	items := append([]SemanticCandidate{}, candidates...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Cosine != items[j].Cosine {
			return items[i].Cosine > items[j].Cosine
		}
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		return items[i].ID < items[j].ID
	})
	if hybrid {
		lexical := map[string]int{}
		for index, id := range lexicalIDs {
			lexical[id] = index + 1
		}
		semantic := map[string]int{}
		for index, item := range items {
			semantic[item.ID] = index + 1
		}
		sort.SliceStable(items, func(i, j int) bool { return hybridCandidateLess(items[i], items[j], lexical, semantic) })
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) || limit < 0 {
		return []SemanticCandidate{}
	}
	end := len(items)
	if limit+1 < end-offset {
		end = offset + limit + 1
	}
	return items[offset:end]
}
