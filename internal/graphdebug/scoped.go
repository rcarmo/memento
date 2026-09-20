package graphdebug

import (
	"strings"

	"github.com/rcarmo/memento/internal/repository"
)

func externalLink(target string) bool {
	return repository.IsExternalLink(strings.TrimSpace(target))
}
func ScopedNodes(nodes []Node, edges []Edge) []Node {
	inbound, outbound, broken := map[string]int{}, map[string]int{}, map[string]int{}
	for _, edge := range edges {
		if edge.Kind != "explicit" || externalLink(edge.RawTarget) {
			continue
		}
		if edge.Resolution == "asset" {
			continue
		}
		if edge.Target == nil || edge.Resolution != "resolved" {
			broken[edge.Source]++
			continue
		}
		outbound[edge.Source]++
		inbound[*edge.Target]++
	}
	result := make([]Node, len(nodes))
	for i, node := range nodes {
		node.ExplicitInDegree = inbound[node.ID]
		node.ExplicitOutDegree = outbound[node.ID]
		node.BrokenLinkCount = broken[node.ID]
		node.Orphan = node.ExplicitInDegree == 0 && node.ExplicitOutDegree == 0
		result[i] = node
	}
	return result
}
