package graphdebug

import (
	"encoding/json"
	"math"
	"testing"
)

func FuzzAggregateLayout(f *testing.F) {
	for _, seed := range [][3]string{{"a,b", "a>b", "rev"}, {"", "", ""}, {"a,a,b", "b>a,a>x", "é"}} {
		f.Add(seed[0], seed[1], seed[2], uint8(4))
	}
	f.Fuzz(func(t *testing.T, idsRaw, edgesRaw, revision string, limitRaw uint8) {
		if len(idsRaw)+len(edgesRaw)+len(revision) > 1<<16 {
			t.Skip()
		}
		ids := splitBounded(idsRaw, 16)
		nodes := make([]Node, 0, len(ids))
		seen := map[string]bool{}
		for _, id := range ids {
			if len(id) > 256 {
				id = id[:256]
			}
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			nodes = append(nodes, Node{ID: id, Path: "/" + id + ".md", Title: id, Type: "concept", Status: "active", Namespace: "/", Tags: []string{}, ProvenanceKeys: []string{}, AnomalyIDs: []string{}})
		}
		edges := []Edge{}
		for i, spec := range splitBounded(edgesRaw, 32) {
			parts := splitArrow(spec)
			if len(parts) != 2 {
				continue
			}
			target := parts[1]
			edges = append(edges, Edge{ID: integerText(int64(i)), Source: parts[0], Target: &target, Kind: "explicit", Canonical: true})
		}
		limit := max(1, int(limitRaw%17))
		first := AggregateLayout(nodes, edges, revision, limit)
		second := AggregateLayout(nodes, edges, revision, limit)
		a, errA := json.Marshal(first)
		b, errB := json.Marshal(second)
		if errA != nil || errB != nil || string(a) != string(b) {
			t.Fatalf("layout is not deterministic: %v %v", errA, errB)
		}
		for _, cluster := range first.Clusters {
			p := cluster.CoarsePosition
			if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsNaN(p.Z) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) || math.IsInf(p.Z, 0) {
				t.Fatalf("non-finite position: %#v", p)
			}
		}
	})
}

func splitBounded(raw string, limit int) []string {
	out := []string{}
	start := 0
	for i := 0; i <= len(raw) && len(out) < limit; i++ {
		if i == len(raw) || raw[i] == ',' {
			out = append(out, raw[start:i])
			start = i + 1
		}
	}
	return out
}
func splitArrow(raw string) []string {
	for i := 0; i < len(raw); i++ {
		if raw[i] == '>' {
			return []string{raw[:i], raw[i+1:]}
		}
	}
	return nil
}
