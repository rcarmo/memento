package graphdebug

import (
	"sort"
	"strings"
)

func median(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]int64{}, values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return float64(ordered[middle])
	}
	return (float64(ordered[middle-1]) + float64(ordered[middle])) / 2
}

// DiagnoseGraph adds the remaining Python graph diagnostics to the verified
// structural/lifecycle foundation, then reapplies the canonical final order.
func DiagnoseGraph(nodes []Node, edges []Edge, revisions Revisions, contentHashes map[string]string) []Diagnostic {
	out := DiagnoseFoundation(nodes, edges, revisions)
	byNamespace := map[string][]Node{}
	for _, node := range nodes {
		byNamespace[node.Namespace] = append(byNamespace[node.Namespace], node)
	}
	namespaces := make([]string, 0, len(byNamespace))
	for namespace := range byNamespace {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	for _, namespace := range namespaces {
		members := byNamespace[namespace]
		if len(members) < 4 {
			continue
		}
		values := make([]int64, len(members))
		for i, node := range members {
			values[i] = node.CombinedBytes
		}
		centre := median(values)
		deviations := make([]int64, len(values))
		for i, value := range values {
			deviations[i] = int64(absFloat(float64(value) - centre))
		}
		mad := median(deviations)
		threshold := centre + maxFloat(1024, 6*mad)
		for _, node := range members {
			if float64(node.CombinedBytes) > threshold {
				out = append(out, diagnostic("size_outlier", "warning", []string{node.ID}, "Combined size is an outlier in namespace "+namespace+".", map[string]any{"combined_bytes": node.CombinedBytes, "namespace_median": centre}, map[string]any{"threshold_bytes": threshold, "mad": mad}, false))
			}
		}
		counts := map[string]int{}
		for _, node := range members {
			for _, tag := range node.Tags {
				counts[tag]++
			}
		}
		common := map[string]bool{}
		for tag, count := range counts {
			if float64(count)/float64(len(members)) >= .75 {
				common[tag] = true
			}
		}
		for _, node := range members {
			have := map[string]bool{}
			for _, tag := range node.Tags {
				have[tag] = true
			}
			missing := []string{}
			for tag := range common {
				if !have[tag] {
					missing = append(missing, tag)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				out = append(out, diagnostic("tag_drift", "info", []string{node.ID}, "Memory lacks common namespace tag(s): "+strings.Join(missing, ", ")+".", map[string]any{"namespace": namespace, "missing_tags": strings.Join(missing, ",")}, map[string]any{"common_tag_fraction": .75}, false))
			}
		}
	}
	byID := map[string]Node{}
	neighbours := map[string][]string{}
	for _, node := range nodes {
		byID[node.ID] = node
	}
	for _, edge := range edges {
		if edge.Target == nil {
			continue
		}
		_, sourceOK := byID[edge.Source]
		_, targetOK := byID[*edge.Target]
		if sourceOK && targetOK {
			neighbours[edge.Source] = append(neighbours[edge.Source], *edge.Target)
			neighbours[*edge.Target] = append(neighbours[*edge.Target], edge.Source)
		}
	}
	for _, node := range nodes {
		linked := neighbours[node.ID]
		if len(linked) < 3 {
			continue
		}
		other := 0
		for _, id := range linked {
			if byID[id].Namespace != node.Namespace {
				other++
			}
		}
		ratio := float64(other) / float64(len(linked))
		if ratio >= .8 {
			out = append(out, diagnostic("namespace_outlier", "info", []string{node.ID}, "Most explicit neighbours are outside the memory namespace.", map[string]any{"external_neighbour_fraction": ratio, "explicit_neighbours": len(linked)}, map[string]any{"threshold": .8}, false))
		}
	}
	groups := map[string][]string{}
	for id, hash := range contentHashes {
		groups[hash] = append(groups[hash], id)
	}
	hashes := make([]string, 0, len(groups))
	for hash := range groups {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)
	for _, hash := range hashes {
		ids := groups[hash]
		if len(ids) > 1 {
			sort.Strings(ids)
			out = append(out, diagnostic("exact_duplicate", "warning", ids, "Memories have identical canonical content hashes.", map[string]any{"content_hash": hash, "count": len(ids)}, map[string]any{}, false))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return out[i].ID < out[j].ID
	})
	return out
}
func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}
