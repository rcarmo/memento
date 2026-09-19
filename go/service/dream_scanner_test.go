package service

import (
	"github.com/rcarmo/memento/go/repository"
	"reflect"
	"testing"
	"time"
)

func dreamEntry(path, id, title, status, body string, updated time.Time) repository.BundleEntry {
	return repository.BundleEntry{BundlePath: path, Document: repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{ID: id, Title: title, Status: status, UpdatedAt: updated}, Body: body}}
}
func TestDetectDreamSignals(t *testing.T) {
	stamp := time.Unix(1000, 0).UTC()
	bundle := repository.RepositoryBundle{Entries: []repository.BundleEntry{dreamEntry("/a.md", "a", "A", "active", "[b](/b.md) [missing](/missing.md) [web](https://x)\n# One\n界界界", stamp), dreamEntry("/b.md", "b", "B", "active", "# B\n", stamp), dreamEntry("/c.md", "c", "C", "archived", "# One\n# Two\n", stamp)}}
	config := DreamScannerConfig{OversizedBodyChars: 10, OversizedTopLevelSections: 2, MaxOversizedCandidates: 2, DuplicateSimilarityThreshold: .86}
	signals := DetectDreamSignals(bundle, "r2", "r1", map[string]bool{"/b.md": true}, config)
	keys := []string{}
	for _, signal := range signals {
		keys = append(keys, signal.DedupeKey)
	}
	want := []string{"broken_link|/a.md|/missing.md", "orphan|a", "oversized_concept|a", "oversized_concept|c", "recent_activity|b|r2"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatal(keys)
	}
	for _, signal := range signals {
		if signal.DedupeKey == "oversized_concept|a" && signal.Evidence["body_chars"] != 60 {
			t.Fatal(signal.Evidence)
		}
	}
	without := DetectDreamSignals(bundle, "r1", "r1", map[string]bool{"/b.md": true}, config)
	for _, signal := range without {
		if signal.SignalType == "recent_activity" {
			t.Fatal(signal)
		}
	}
}
func TestDreamDuplicates(t *testing.T) {
	description := "same description"
	stamp := time.Unix(1, 0)
	entry := func(path, id, title string, tags []string) repository.BundleEntry {
		item := dreamEntry(path, id, title, "active", "body", stamp)
		item.Document.Frontmatter.Description = &description
		item.Document.Frontmatter.Tags = tags
		return item
	}
	bundle := repository.RepositoryBundle{Entries: []repository.BundleEntry{entry("/z", "z", "Straße", []string{"x"}), entry("/a", "a", "STRASSE", []string{"x"}), entry("/b", "b", "Project Alpha", []string{"one", "two"}), entry("/c", "c", "Project Alfa", []string{"one", "two"}), entry("/d", "d", "Unrelated", nil)}}
	signals := detectDreamDuplicates(bundle, .75)
	keys := []string{}
	for _, signal := range signals {
		keys = append(keys, signal.DedupeKey)
		if signal.DedupeKey == "likely_duplicate|a|z" {
			if signal.Severity != "medium" || signal.Evidence["score"] != 1.0 || !reflect.DeepEqual(signal.Evidence["paths"], []string{"/a", "/z"}) {
				t.Fatal(signal)
			}
		}
	}
	if !reflect.DeepEqual(keys, []string{"likely_duplicate|a|z", "likely_duplicate|b|c"}) {
		t.Fatal(keys)
	}
	if got := detectDreamDuplicates(bundle, 1); len(got) != 1 {
		t.Fatal(got)
	}
}
func TestDreamOversizedTie(t *testing.T) {
	stamp := time.Unix(1, 0)
	bundle := repository.RepositoryBundle{Entries: []repository.BundleEntry{dreamEntry("/b", "b", "B", "active", "12345", stamp), dreamEntry("/a", "a", "A", "active", "12345", stamp)}}
	signals := DetectDreamSignals(bundle, "r", "", nil, DreamScannerConfig{OversizedBodyChars: 5, OversizedTopLevelSections: 9, MaxOversizedCandidates: 1, DuplicateSimilarityThreshold: .86})
	found := ""
	for _, signal := range signals {
		if signal.SignalType == "oversized_concept" {
			found = signal.DedupeKey
		}
	}
	if found != "oversized_concept|a" {
		t.Fatal(found)
	}
}
func TestDreamQuietUntil(t *testing.T) {
	stamp := time.Unix(1000, 0).UTC()
	empty := repository.RepositoryBundle{}
	if DreamQuietUntil(empty, 1001, 300) != nil || DreamQuietUntil(repository.RepositoryBundle{Entries: []repository.BundleEntry{dreamEntry("/a", "a", "A", "active", "", stamp)}}, 1001, 0) != nil {
		t.Fatal("disabled")
	}
	bundle := repository.RepositoryBundle{Entries: []repository.BundleEntry{dreamEntry("/a", "a", "A", "active", "", stamp), dreamEntry("/b", "b", "B", "active", "", stamp.Add(time.Second))}}
	value := DreamQuietUntil(bundle, 1100, 300)
	if value == nil || *value != "1970-01-01T00:21:41Z" {
		t.Fatal(value)
	}
	if DreamQuietUntil(bundle, 1301, 300) != nil {
		t.Fatal("elapsed")
	}
}
