package graphdebug

import "sort"

func OverlayEdges(nodes []Node, revision string, limit int) []Edge {
	if limit <= 0 {
		return []Edge{}
	}
	kinds := []string{"shared_tag", "shared_namespace", "shared_type", "shared_provenance"}
	groups := map[string]map[string][]Node{}
	for _, kind := range kinds {
		groups[kind] = map[string][]Node{}
	}
	ordered := append([]Node{}, nodes...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	for _, node := range ordered {
		seen := map[string]bool{}
		for _, tag := range node.Tags {
			if !seen[tag] {
				groups["shared_tag"][tag] = append(groups["shared_tag"][tag], node)
				seen[tag] = true
			}
		}
		groups["shared_namespace"][node.Namespace] = append(groups["shared_namespace"][node.Namespace], node)
		groups["shared_type"][node.Type] = append(groups["shared_type"][node.Type], node)
		seen = map[string]bool{}
		for _, key := range node.ProvenanceKeys {
			if !seen[key] {
				groups["shared_provenance"][key] = append(groups["shared_provenance"][key], node)
				seen[key] = true
			}
		}
	}
	streams := [][]Edge{}
	for _, kind := range kinds {
		keys := []string{}
		for key := range groups[kind] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		stream := []Edge{}
		seen := map[string]bool{}
		for _, key := range keys {
			members := groups[kind][key]
			for i := 0; i+1 < len(members); i++ {
				left, right := members[i], members[i+1]
				pair := left.ID + "\x00" + right.ID
				if seen[pair] {
					continue
				}
				seen[pair] = true
				raw := key
				if kind == "shared_provenance" {
					raw = kind
				}
				target := right.ID
				stream = append(stream, Edge{ID: kind + ":" + left.ID + ":" + right.ID, Source: left.ID, Target: &target, RawTarget: raw, Kind: kind, Canonical: false, Resolution: "derived", FirstSeenRevision: revision, LastCheckedRevision: revision})
			}
		}
		streams = append(streams, stream)
	}
	result := []Edge{}
	indexes := make([]int, len(streams))
	for len(result) < limit {
		remaining := false
		for i, stream := range streams {
			if indexes[i] < len(stream) {
				result = append(result, stream[indexes[i]])
				indexes[i]++
				remaining = true
				if len(result) == limit {
					break
				}
			}
		}
		if !remaining {
			break
		}
	}
	return result
}
