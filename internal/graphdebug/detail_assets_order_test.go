package graphdebug

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAssetsSemanticOrderByKindAndLimit(t *testing.T) {
	root := t.TempDir()
	write := func(kind, name string, payload map[string]any) {
		t.Helper()
		dir := filepath.Join(root, ".assets", "id", kind)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		payload["asset_kind"] = kind
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("docs", "1.0.0", map[string]any{"version": "1.0.0"})
	write("docs", "1.0.2", map[string]any{"version": "1.0.2"})
	write("docs", "1.0.1", map[string]any{"version": "1.0.1"})
	write("docs", "alpha", map[string]any{"version": "alpha"})
	write("docs", "beta", map[string]any{"version": "beta"})
	write("images", "1.0.1", map[string]any{"version": "1.0.1"})
	write("media", "a", map[string]any{"version": "zeta"})
	write("media", "b", map[string]any{"version": "1.0.0", "source_proposal_id": "first"})
	write("media", "c", map[string]any{"version": "1.0.0", "source_proposal_id": "second"})

	got := NewSnapshotService(root, "", "").Assets("id", 10)
	want := []struct {
		kind, version string
	}{
		{"docs", "1.0.2"},
		{"docs", "1.0.1"},
		{"docs", "1.0.0"},
		{"docs", "beta"},
		{"docs", "alpha"},
		{"images", "1.0.1"},
		{"media", "1.0.0"},
		{"media", "1.0.0"},
		{"media", "zeta"},
	}
	if len(got) != len(want) {
		t.Fatal(got)
	}
	for i, item := range want {
		if got[i].AssetKind != item.kind || got[i].Version != item.version {
			t.Fatal(got)
		}
	}
	if got[6].SourceProposalID == nil || *got[6].SourceProposalID != "first" || got[7].SourceProposalID == nil || *got[7].SourceProposalID != "second" {
		t.Fatal(got)
	}

	limited := NewSnapshotService(root, "", "").Assets("id", 2)
	if len(limited) != 2 || limited[0].Version != "1.0.2" || limited[1].Version != "1.0.1" {
		t.Fatal(limited)
	}
}
