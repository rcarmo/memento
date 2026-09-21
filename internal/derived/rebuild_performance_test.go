package derived

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRebuildThreeHundredConceptStartupBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("startup budget")
	}
	root := t.TempDir()
	for index := 0; index < 300; index++ {
		text := fmt.Sprintf("---\nschema_version: 1\nid: 00000000-0000-4000-8000-%012d\ntype: concept\ntitle: Startup budget %d\nstatus: active\ndescription: null\naliases: []\ntags: [budget]\nsource_refs: []\nsupersedes: []\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: budget\n---\nstartup budget body %d\n", index, index, index)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("item-%03d.md", index)), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	index := &Index{Path: filepath.Join(t.TempDir(), "derived.sqlite")}
	started := time.Now()
	if err := index.Rebuild(context.Background(), root, "budget"); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	if elapsed > 10*time.Second {
		t.Fatalf("300-concept rebuild took %s; startup must remain below 10s", elapsed)
	}
	state, err := index.State(context.Background())
	if err != nil || state.Status != "ready" || state.IndexRevision != "budget" {
		t.Fatal(state, err)
	}
}
