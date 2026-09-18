package graphdebug

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"
	"strings"
)

type CountPair [2]any
type AggregateNode struct {
	ID                string      `json:"id"`
	Label             string      `json:"label"`
	Namespace         string      `json:"namespace"`
	MemberCount       int         `json:"member_count"`
	MarkdownBytes     int64       `json:"markdown_bytes"`
	AssetBytes        int64       `json:"asset_bytes"`
	CombinedBytes     int64       `json:"combined_bytes"`
	ExplicitInDegree  int         `json:"explicit_in_degree"`
	ExplicitOutDegree int         `json:"explicit_out_degree"`
	BrokenLinkCount   int         `json:"broken_link_count"`
	OrphanCount       int         `json:"orphan_count"`
	TypeCounts        []CountPair `json:"type_counts"`
	StatusCounts      []CountPair `json:"status_counts"`
	CoarsePosition    Position    `json:"coarse_position"`
}
type AggregateEdge struct {
	ID                string   `json:"id"`
	Source            string   `json:"source"`
	Target            string   `json:"target"`
	ExplicitEdgeCount int      `json:"explicit_edge_count"`
	Kind              string   `json:"kind"`
	Canonical         bool     `json:"canonical"`
	Similarity        *float64 `json:"similarity"`
}
type Layout struct {
	Seed        string          `json:"seed"`
	Version     string          `json:"version"`
	Clusters    []AggregateNode `json:"clusters"`
	Edges       []AggregateEdge `json:"edges"`
	Memberships []CountPair     `json:"memberships"`
}
type nodeGroup struct {
	id    string
	nodes []Node
}

func explicitComponents(nodes []Node, edges []Edge) [][]Node {
	ids := map[string]bool{}
	byID := map[string]Node{}
	neighbors := map[string]map[string]bool{}
	for _, n := range nodes {
		ids[n.ID] = true
		byID[n.ID] = n
		neighbors[n.ID] = map[string]bool{}
	}
	for _, e := range edges {
		if e.Kind == "explicit" && e.Target != nil && ids[e.Source] && ids[*e.Target] {
			neighbors[e.Source][*e.Target] = true
			neighbors[*e.Target][e.Source] = true
		}
	}
	unseen := map[string]bool{}
	for id := range ids {
		unseen[id] = true
	}
	out := [][]Node{}
	for len(unseen) > 0 {
		start := ""
		for id := range unseen {
			if start == "" || id < start {
				start = id
			}
		}
		delete(unseen, start)
		stack := []string{start}
		component := []Node{}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			component = append(component, byID[current])
			next := []string{}
			for id := range neighbors[current] {
				if unseen[id] {
					next = append(next, id)
				}
			}
			sort.Sort(sort.Reverse(sort.StringSlice(next)))
			for _, id := range next {
				delete(unseen, id)
				stack = append(stack, id)
			}
		}
		sort.Slice(component, func(i, j int) bool { return component[i].ID < component[j].ID })
		out = append(out, component)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0].ID < out[j][0].ID })
	return out
}
func sparseNamespace(nodes []Node, components [][]Node) bool {
	if len(nodes) <= 1 || len(components) <= 1 {
		return false
	}
	single := 0
	for _, c := range components {
		if len(c) == 1 {
			single++
		}
	}
	return float64(single)/float64(len(components)) >= .5
}
func clusterID(namespace string, index int, nodes []Node) string {
	input := namespace + "\x00"
	for i, n := range nodes {
		if i > 0 {
			input += "\x00"
		}
		input += n.ID
	}
	sum := sha256.Sum256([]byte(input))
	return "cluster:" + strings.Trim(namespace, "/") + ":" + integerText(int64(index)) + ":" + hex.EncodeToString(sum[:6])
}
func mergeGroups(groups []nodeGroup, limit int) []nodeGroup {
	sort.Slice(groups, func(i, j int) bool {
		if len(groups[i].nodes) != len(groups[j].nodes) {
			return len(groups[i].nodes) > len(groups[j].nodes)
		}
		return groups[i].id < groups[j].id
	})
	retained := append([]nodeGroup{}, groups[:max(0, limit-1)]...)
	keep := map[string]bool{}
	for _, g := range retained {
		keep[g.id] = true
	}
	overflow := []Node{}
	for _, g := range groups {
		if !keep[g.id] {
			overflow = append(overflow, g.nodes...)
		}
	}
	sort.Slice(overflow, func(i, j int) bool { return overflow[i].ID < overflow[j].ID })
	retained = append(retained, nodeGroup{"cluster:overflow", overflow})
	sort.Slice(retained, func(i, j int) bool { return retained[i].id < retained[j].id })
	return retained
}
func countPairs(values map[string]int) []CountPair {
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]CountPair, len(keys))
	for i, key := range keys {
		out[i] = CountPair{key, values[key]}
	}
	return out
}
func clusterPosition(id, revision string, ordinal, total int) Position {
	sum := sha256.Sum256([]byte("v1\x00" + revision + "\x00" + id))
	jitter := float64(binary.BigEndian.Uint32(sum[:4]))/math.Exp2(32) - .5
	angle := 2 * math.Pi * (float64(ordinal) + .5 + jitter*.2) / float64(max(1, total))
	radius := 3 + math.Sqrt(float64(max(1, total)))*.6
	z := (float64(binary.BigEndian.Uint32(sum[4:8]))/math.Exp2(32) - .5) * 2
	return Position{math.Cos(angle) * radius, math.Sin(angle) * radius, z}
}
func aggregateNode(group nodeGroup, revision string, ordinal, total int) AggregateNode {
	namespace := "/"
	if len(group.nodes) > 0 {
		namespace = group.nodes[0].Namespace
	}
	label := namespace
	if namespace == "/trash/" {
		label = "Trash"
	} else if len(group.nodes) == 1 {
		label = group.nodes[0].Title
	}
	out := AggregateNode{ID: group.id, Label: label, Namespace: namespace, MemberCount: len(group.nodes), CoarsePosition: clusterPosition(group.id, revision, ordinal, total)}
	types, statuses := map[string]int{}, map[string]int{}
	for _, n := range group.nodes {
		out.MarkdownBytes += n.MarkdownBytes
		out.AssetBytes += n.AssetBytes
		out.CombinedBytes += n.CombinedBytes
		out.ExplicitInDegree += n.ExplicitInDegree
		out.ExplicitOutDegree += n.ExplicitOutDegree
		out.BrokenLinkCount += n.BrokenLinkCount
		if n.Orphan {
			out.OrphanCount++
		}
		types[n.Type]++
		statuses[n.Status]++
	}
	out.TypeCounts = countPairs(types)
	out.StatusCounts = countPairs(statuses)
	return out
}
func AggregateLayout(nodes []Node, edges []Edge, revision string, clusterLimit int) Layout {
	nodes = append([]Node{}, nodes...)
	edges = append([]Edge{}, edges...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	byNamespace := map[string][]Node{}
	for _, n := range nodes {
		byNamespace[n.Namespace] = append(byNamespace[n.Namespace], n)
	}
	namespaces := []string{}
	for n := range byNamespace {
		namespaces = append(namespaces, n)
	}
	sort.Strings(namespaces)
	groups := []nodeGroup{}
	for _, namespace := range namespaces {
		members := byNamespace[namespace]
		components := explicitComponents(members, edges)
		if namespace == "/trash/" || sparseNamespace(members, components) {
			id := clusterID(namespace, 0, members)
			if namespace == "/trash/" {
				id = "cluster:trash"
			}
			groups = append(groups, nodeGroup{id, members})
		} else {
			for i, component := range components {
				groups = append(groups, nodeGroup{clusterID(namespace, i, component), component})
			}
		}
	}
	if len(groups) > clusterLimit {
		trash := []nodeGroup{}
		others := []nodeGroup{}
		for _, g := range groups {
			if g.id == "cluster:trash" {
				trash = append(trash, g)
			} else {
				others = append(others, g)
			}
		}
		if len(trash) > 0 && clusterLimit > 1 {
			groups = append(trash, mergeGroups(others, clusterLimit-1)...)
		} else {
			groups = mergeGroups(groups, clusterLimit)
		}
	}
	memberships := map[string]string{}
	clusters := make([]AggregateNode, len(groups))
	for i, g := range groups {
		clusters[i] = aggregateNode(g, revision, i, len(groups))
		for _, n := range g.nodes {
			memberships[n.ID] = g.id
		}
	}
	type key struct{ kind, source, target string }
	counts := map[key]int{}
	sums := map[key]float64{}
	for _, e := range edges {
		if e.Target == nil {
			continue
		}
		source, target := memberships[e.Source], memberships[*e.Target]
		if source == "" || target == "" || source == target {
			continue
		}
		k := key{e.Kind, source, target}
		counts[k]++
		if e.Kind == "semantic_similarity" && e.Similarity != nil {
			sums[k] += *e.Similarity
		}
	}
	keys := []key{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].kind != keys[j].kind {
			return keys[i].kind < keys[j].kind
		}
		if keys[i].source != keys[j].source {
			return keys[i].source < keys[j].source
		}
		return keys[i].target < keys[j].target
	})
	aggregateEdges := []AggregateEdge{}
	for _, k := range keys {
		count := counts[k]
		edge := AggregateEdge{ID: "cluster-" + k.kind + ":" + k.source + ":" + k.target, Source: k.source, Target: k.target, Kind: k.kind, Canonical: k.kind == "explicit"}
		if k.kind == "explicit" {
			edge.ExplicitEdgeCount = count
		}
		if k.kind == "semantic_similarity" {
			value := sums[k] / float64(count)
			edge.Similarity = &value
		}
		aggregateEdges = append(aggregateEdges, edge)
	}
	pairs := []CountPair{}
	ids := []string{}
	for id := range memberships {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		pairs = append(pairs, CountPair{id, memberships[id]})
	}
	return Layout{revision, "v1", clusters, aggregateEdges, pairs}
}
