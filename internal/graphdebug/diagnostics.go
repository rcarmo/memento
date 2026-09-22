package graphdebug

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

type Diagnostic struct {
	ID           string         `json:"id"`
	Rule         string         `json:"rule"`
	Severity     string         `json:"severity"`
	ConceptIDs   []string       `json:"concept_ids"`
	Message      string         `json:"message"`
	Measured     map[string]any `json:"measured"`
	Threshold    map[string]any `json:"threshold"`
	Derived      bool           `json:"derived"`
	ScopeLimited bool           `json:"scope_limited,omitempty"`
}

func diagnostic(rule, severity string, ids []string, message string, measured, threshold map[string]any, derived bool) Diagnostic {
	ids = append([]string{}, ids...)
	sort.Strings(ids)
	input := rule + "\x00"
	for i, id := range ids {
		if i > 0 {
			input += "\x00"
		}
		input += id
	}
	sum := sha256.Sum256([]byte(input))
	return Diagnostic{ID: "diagnostic:" + rule + ":" + hex.EncodeToString(sum[:8]), Rule: rule, Severity: severity, ConceptIDs: ids, Message: message, Measured: measured, Threshold: threshold, Derived: derived}
}
func percentile(values []int, fraction float64) int {
	if len(values) == 0 {
		return 0
	}
	values = append([]int{}, values...)
	sort.Ints(values)
	return values[int(float64(len(values)-1)*fraction+.5)]
}
func ApplyDiagnosticIDs(nodes []Node, diagnostics []Diagnostic) []Node {
	byID := map[string][]string{}
	for _, diagnostic := range diagnostics {
		for _, id := range diagnostic.ConceptIDs {
			byID[id] = append(byID[id], diagnostic.ID)
		}
	}
	result := make([]Node, len(nodes))
	for i, node := range nodes {
		node.AnomalyIDs = append([]string{}, byID[node.ID]...)
		sort.Strings(node.AnomalyIDs)
		result[i] = node
	}
	return result
}

func DiagnoseFoundation(nodes []Node, edges []Edge, revisions Revisions) []Diagnostic {
	nodes = append([]Node{}, nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	namespaces := map[string]bool{}
	links := map[string]bool{}
	byID := map[string]Node{}
	degrees := []int{}
	for _, node := range nodes {
		namespaces[node.Namespace] = true
		byID[node.ID] = node
		degrees = append(degrees, node.ExplicitInDegree+node.ExplicitOutDegree)
	}
	for _, edge := range edges {
		if edge.Target != nil {
			source, sok := byID[edge.Source]
			target, tok := byID[*edge.Target]
			if sok && tok && source.Namespace != target.Namespace {
				links[source.Namespace+"\x00"+target.Namespace] = true
				links[target.Namespace+"\x00"+source.Namespace] = true
			}
		}
	}
	threshold := 20
	if value := percentile(degrees, .95); value > threshold {
		threshold = value
	}
	out := []Diagnostic{}
	for _, node := range nodes {
		degree := node.ExplicitInDegree + node.ExplicitOutDegree
		if node.Orphan || degree == 0 {
			out = append(out, diagnostic("orphan", "warning", []string{node.ID}, "No explicit inbound or outbound links.", map[string]any{"explicit_degree": degree}, map[string]any{"maximum_orphan_degree": 0}, false))
		}
		if node.BrokenLinkCount > 0 {
			out = append(out, diagnostic("broken_links", "error", []string{node.ID}, fmt.Sprintf("%d explicit link target(s) do not resolve.", node.BrokenLinkCount), map[string]any{"broken_link_count": node.BrokenLinkCount}, map[string]any{"maximum": 0}, false))
		}
		if degree > threshold {
			out = append(out, diagnostic("high_degree", "info", []string{node.ID}, "Explicit degree exceeds the graph high-degree threshold.", map[string]any{"explicit_degree": degree}, map[string]any{"threshold": threshold}, false))
		}
	}
	namespaceKeys := []string{}
	for namespace := range namespaces {
		namespaceKeys = append(namespaceKeys, namespace)
	}
	sort.Strings(namespaceKeys)
	if len(namespaceKeys) > 1 {
		for _, namespace := range namespaceKeys {
			linked := false
			for key := range links {
				if len(key) >= len(namespace) && (key[:len(namespace)] == namespace || key[len(key)-len(namespace):] == namespace) {
					linked = true
				}
			}
			if !linked {
				ids := []string{}
				for _, node := range nodes {
					if node.Namespace == namespace {
						ids = append(ids, node.ID)
					}
				}
				out = append(out, diagnostic("isolated_cluster", "info", ids, "Namespace "+namespace+" has no explicit links to another namespace.", map[string]any{"namespace": namespace, "member_count": len(ids)}, map[string]any{"minimum_external_edges": 1}, false))
			}
		}
	}
	if revisions.Stale {
		ids := []string{}
		for _, node := range nodes {
			ids = append(ids, node.ID)
		}
		out = append(out, diagnostic("index_stale", "error", ids, "Derived index revision does not match the repository revision.", map[string]any{"repository_revision": revisions.Repository, "index_revision": revisions.Index}, map[string]any{}, false))
	}
	for _, node := range nodes {
		e := node.Embedding
		if e.Status == "error" {
			out = append(out, diagnostic("embedding_failed", "warning", []string{node.ID}, "Derived embedding generation failed.", map[string]any{"status": e.Status, "error": e.Error}, map[string]any{}, true))
		} else if e.Status == "legacy" {
			out = append(out, diagnostic("embedding_legacy", "info", []string{node.ID}, "Legacy single-vector embedding needs item-level chunk regeneration; explicit relationships are unaffected.", map[string]any{"status": e.Status}, map[string]any{}, true))
		} else if e.Status == "stale" {
			out = append(out, diagnostic("embedding_stale", "warning", []string{node.ID}, "Embedding input or model configuration changed for this item; explicit relationships are unaffected.", map[string]any{"status": e.Status}, map[string]any{}, true))
		} else if e.Status != "ready" {
			out = append(out, diagnostic("embedding_missing", "info", []string{node.ID}, "No current ready embedding is available; explicit relationships are unaffected.", map[string]any{"status": e.Status}, map[string]any{}, true))
		}
		if node.PendingProposalCount > 0 {
			out = append(out, diagnostic("pending_proposals", "info", []string{node.ID}, "Memory has pending proposal or review state.", map[string]any{"pending_proposal_count": node.PendingProposalCount}, map[string]any{"maximum": 0}, false))
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
