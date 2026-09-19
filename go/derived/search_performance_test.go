package derived

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/access"
)

func lexicalBenchmarkIndex(b *testing.B) *Index {
	b.Helper()
	root := b.TempDir()
	for index := 0; index < 100; index++ {
		text := fmt.Sprintf("---\nschema_version: 1\nid: 00000000-0000-4000-8000-%012d\ntype: concept\ntitle: Shared benchmark %d\nstatus: active\ndescription: null\naliases: []\ntags: [benchmark]\nsource_refs: []\nsupersedes: []\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: benchmark\n---\nshared benchmark body %d\n", index, index, index)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("item-%03d.md", index)), []byte(text), 0600); err != nil {
			b.Fatal(err)
		}
	}
	path := filepath.Join(b.TempDir(), "derived.sqlite")
	result := &Index{Path: path}
	if err := result.Rebuild(context.Background(), root, "benchmark"); err != nil {
		b.Fatal(err)
	}
	return result
}

func BenchmarkSearchLexical100(b *testing.B) {
	index := lexicalBenchmarkIndex(b)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	options := SearchOptions{Query: "shared benchmark", Syntax: "plain", Limit: 20}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		page, err := index.SearchLexical(context.Background(), policy, options)
		if err != nil || len(page.Results) != 20 {
			b.Fatal(page, err)
		}
	}
}
