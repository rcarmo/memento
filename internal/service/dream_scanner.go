package service

import (
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
	"golang.org/x/text/cases"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func acceptedBundleAssets(bundle repository.RepositoryBundle) (map[string]map[string]bool, error) {
	assetPaths := map[string]map[string]bool{}
	for _, entry := range bundle.Entries {
		paths, err := assets.AcceptedAssetPaths(bundle.Root, entry.Document.Frontmatter.ID)
		if err != nil {
			return nil, err
		}
		assetPaths[entry.Document.Frontmatter.ID] = paths
	}
	return assetPaths, nil
}

func DetectDreamSignals(bundle repository.RepositoryBundle, revision, previous string, changed map[string]bool, config DreamScannerConfig) []control.DetectedSignal {
	return detectDreamSignals(bundle, revision, previous, changed, config, nil)
}
func detectDreamSignals(bundle repository.RepositoryBundle, revision, previous string, changed map[string]bool, config DreamScannerConfig, assetPaths map[string]map[string]bool) []control.DetectedSignal {
	entries := map[string]repository.BundleEntry{}
	inbound := map[string]int{}
	for _, entry := range bundle.Entries {
		entries[entry.BundlePath] = entry
		inbound[entry.BundlePath] = 0
	}
	out := []control.DetectedSignal{}
	for _, entry := range bundle.Entries {
		for _, link := range repository.ExtractStructuralLinks(entry.Document.Body) {
			target, internal := repository.ResolveLinkPath(entry.BundlePath, link.Href)
			if !internal {
				continue
			}
			if _, ok := entries[target]; ok {
				inbound[target]++
				continue
			}
			assetPath, candidate := repository.ResolveAssetLinkPath(link.Href)
			if candidate && assetPaths[entry.Document.Frontmatter.ID][assetPath] {
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
	out = append(out, detectDreamDuplicates(bundle, config.DuplicateSimilarityThreshold)...)
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
func detectDreamDuplicates(bundle repository.RepositoryBundle, threshold float64) []control.DetectedSignal {
	active := []repository.BundleEntry{}
	for _, entry := range bundle.Entries {
		if entry.Document.Frontmatter.Status == "active" {
			active = append(active, entry)
		}
	}
	fold := cases.Fold()
	out := []control.DetectedSignal{}
	for i, left := range active {
		for _, right := range active[i+1:] {
			lm, rm := left.Document.Frontmatter, right.Document.Frontmatter
			title := sequenceRatio(fold.String(lm.Title), fold.String(rm.Title))
			ld, rd := "", ""
			if lm.Description != nil {
				ld = *lm.Description
			}
			if rm.Description != nil {
				rd = *rm.Description
			}
			description := sequenceRatio(fold.String(ld), fold.String(rd))
			leftTags, rightTags := map[string]bool{}, map[string]bool{}
			for _, tag := range lm.Tags {
				leftTags[tag] = true
			}
			for _, tag := range rm.Tags {
				rightTags[tag] = true
			}
			intersection := 0
			for tag := range leftTags {
				if rightTags[tag] {
					intersection++
				}
			}
			union := len(leftTags)
			for tag := range rightTags {
				if !leftTags[tag] {
					union++
				}
			}
			tagRatio := 0.0
			if union > 0 {
				tagRatio = float64(intersection) / float64(union)
			}
			score := math.Max(title, title*.6+description*.2+tagRatio*.2)
			if title < 1 && score < threshold {
				continue
			}
			ids := []string{lm.ID, rm.ID}
			paths := []string{left.BundlePath, right.BundlePath}
			sort.Strings(ids)
			sort.Strings(paths)
			severity := "medium"
			if title < 1 {
				severity = "low"
			}
			out = append(out, control.DetectedSignal{SignalType: "likely_duplicate", EntityRefs: ids, Severity: severity, DedupeKey: "likely_duplicate|" + ids[0] + "|" + ids[1], Evidence: map[string]any{"paths": paths, "score": math.Round(score*1000) / 1000}})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DedupeKey < out[j].DedupeKey })
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
