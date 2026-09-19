package service

import (
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func DetectDreamSignals(bundle repository.RepositoryBundle, revision, previous string, changed map[string]bool, config DreamScannerConfig) []control.DetectedSignal {
	entries := map[string]repository.BundleEntry{}
	inbound := map[string]int{}
	for _, entry := range bundle.Entries {
		entries[entry.BundlePath] = entry
		inbound[entry.BundlePath] = 0
	}
	out := []control.DetectedSignal{}
	for _, entry := range bundle.Entries {
		for _, link := range repository.ExtractStructuralLinks(entry.Document.Body) {
			if !strings.HasPrefix(link.Href, "/") {
				continue
			}
			target, _, _ := strings.Cut(link.Href, "#")
			if _, ok := entries[target]; ok {
				inbound[target]++
				continue
			}
			out = append(out, control.DetectedSignal{SignalType: "broken_link", EntityRefs: []string{entry.BundlePath, target}, Severity: "medium", DedupeKey: "broken_link|" + entry.BundlePath + "|" + target, Evidence: map[string]any{"source_path": entry.BundlePath, "target_path": target}})
		}
	}
	type oversized struct {
		weight int
		signal control.DetectedSignal
	}
	large := []oversized{}
	for _, entry := range bundle.Entries {
		m := entry.Document.Frontmatter
		if m.Status == "active" && inbound[entry.BundlePath] == 0 {
			out = append(out, control.DetectedSignal{SignalType: "orphan", EntityRefs: []string{m.ID, entry.BundlePath}, Severity: "medium", DedupeKey: "orphan|" + m.ID, Evidence: map[string]any{"path": entry.BundlePath, "title": m.Title}})
		}
		bodyChars := utf8.RuneCountInString(entry.Document.Body)
		sections := 0
		for _, line := range strings.Split(entry.Document.Body, "\n") {
			if strings.HasPrefix(line, "# ") {
				sections++
			}
		}
		if bodyChars >= config.OversizedBodyChars || sections >= config.OversizedTopLevelSections {
			weight := bodyChars
			if sections*1000 > weight {
				weight = sections * 1000
			}
			large = append(large, oversized{weight, control.DetectedSignal{SignalType: "oversized_concept", EntityRefs: []string{m.ID, entry.BundlePath}, Severity: "medium", DedupeKey: "oversized_concept|" + m.ID, Evidence: map[string]any{"path": entry.BundlePath, "body_chars": bodyChars, "top_level_sections": sections}}})
		}
		if previous != "" && previous != revision && changed[entry.BundlePath] {
			out = append(out, control.DetectedSignal{SignalType: "recent_activity", EntityRefs: []string{m.ID, entry.BundlePath}, Severity: "low", DedupeKey: "recent_activity|" + m.ID + "|" + revision, Evidence: map[string]any{"path": entry.BundlePath, "since_revision": previous, "repo_revision": revision}})
		}
	}
	sort.Slice(large, func(i, j int) bool {
		if large[i].weight != large[j].weight {
			return large[i].weight > large[j].weight
		}
		return large[i].signal.DedupeKey < large[j].signal.DedupeKey
	})
	limit := min(config.MaxOversizedCandidates, len(large))
	for _, item := range large[:limit] {
		out = append(out, item.signal)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SignalType != out[j].SignalType {
			return out[i].SignalType < out[j].SignalType
		}
		return out[i].DedupeKey < out[j].DedupeKey
	})
	return out
}
func DreamQuietUntil(bundle repository.RepositoryBundle, nowUnix int64, seconds int) *string {
	if seconds <= 0 || len(bundle.Entries) == 0 {
		return nil
	}
	latest := bundle.Entries[0].Document.Frontmatter.UpdatedAt.Unix()
	for _, entry := range bundle.Entries[1:] {
		if value := entry.Document.Frontmatter.UpdatedAt.Unix(); value > latest {
			latest = value
		}
	}
	if nowUnix-latest >= int64(seconds) {
		return nil
	}
	value := time.Unix(latest+int64(seconds), 0).UTC().Format(time.RFC3339)
	return &value
}
