package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

const maxProposalSummaryChanges = 200
const maxProposalSummaryAssets = 200
const maxProposalSummaryText = 2000

// Summary exposes proposal metadata without building a preview. The caller must
// authorise visibility before calling it; this is also used by review responses.
func (q ProposalQueue) Summary(ctx context.Context, record control.ProposalRecord, includeConflicts bool) (map[string]any, error) {
	assets, err := q.Proposals.ListAssets(ctx, control.ProposalAssetQuery{ProposalID: &record.ProposalID})
	if err != nil {
		return nil, err
	}
	return q.summary(ctx, record, assets, includeConflicts, defaultProposalRepository())
}
func (q ProposalQueue) summary(ctx context.Context, record control.ProposalRecord, allAssets []control.ProposalAssetRecord, includeConflicts bool, repo proposalRepository) (map[string]any, error) {
	patch, err := record.Patch()
	if err != nil {
		return nil, err
	}
	changes := []any{}
	if value, exists := patch["changes"]; exists {
		var ok bool
		changes, ok = value.([]any)
		if !ok {
			return nil, &ChangeValidationError{"proposal changes must be a list"}
		}
	}
	summaries := []any{}
	for i, value := range changes[:min(len(changes), maxProposalSummaryChanges)] {
		raw, ok := value.(map[string]any)
		if !ok {
			continue
		}
		item := map[string]any{"index": i, "kind": raw["kind"], "path": raw["path"]}
		if raw["kind"] == "rename" {
			item["new_path"] = raw["new_path"]
		}
		summaries = append(summaries, item)
	}
	assets := []any{}
	for _, asset := range allAssets[:min(len(allAssets), maxProposalSummaryAssets)] {
		manifest, err := asset.Manifest()
		if err != nil {
			return nil, err
		}
		matching := []string{}
		var bodyMatches any
		body, err := q.resultingBody(changes, asset.ConceptPath, repo)
		if err == nil {
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(repository.NormalizeConceptBody(body))))
			entries := []any{}
			if value, exists := manifest["entries"]; exists {
				var ok bool
				entries, ok = value.([]any)
				if !ok {
					return nil, errors.New("manifest entries must be a list")
				}
			}
			for _, entry := range entries {
				item, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				path, ok := item["path"].(string)
				if ok && item["sha256"] == digest {
					matching = append(matching, path)
				}
			}
			sort.Strings(matching)
			bodyMatches = len(matching) > 0
		} else if !ignorableBundleError(err) {
			return nil, err
		}
		assets = append(assets, map[string]any{"asset_id": asset.AssetID, "concept_path": asset.ConceptPath, "asset_kind": asset.AssetKind, "version": asset.Version, "file_count": manifest["file_count"], "total_uncompressed_bytes": manifest["total_uncompressed_bytes"], "zip_sha256": asset.SHA256, "concept_body_matching_entries": matching, "concept_body_matches_asset": bodyMatches})
	}
	revision, err := repo.main(q.Paths)
	if err != nil {
		return nil, err
	}
	conflicts := []ProposalConflict{}
	if includeConflicts {
		limit := maxProposalSummaryChanges
		conflicts, err = q.conflicts(ctx, record, "", &limit, nil, repo)
		if err != nil {
			return nil, err
		}
	}
	intent := []rune(record.Intent)
	return map[string]any{
		"proposal_id": record.ProposalID, "author_principal": record.AuthorPrincipal, "intent": string(intent[:min(len(intent), maxProposalSummaryText)]), "intent_truncated": len(intent) > maxProposalSummaryText, "status": string(record.Status), "base_revision": record.BaseRevision, "current_revision": revision, "reviewed_by": nullableText(record.ReviewedBy), "applied_operation_id": nullableText(record.AppliedOperationID), "applied_revision": nullableText(record.AppliedRevision), "created_at": record.CreatedAt, "updated_at": record.UpdatedAt, "expires_at": nullableText(record.ExpiresAt), "change_count": len(changes), "changes_truncated": len(changes) > len(summaries), "changes": summaries, "conflicts_included": includeConflicts, "conflicts": conflictDetails(conflicts), "asset_count": len(allAssets), "assets_truncated": len(allAssets) > len(assets), "assets": assets,
	}, nil
}

func (q ProposalQueue) resultingBody(changes []any, path string, repo proposalRepository) (string, error) {
	for _, value := range changes {
		raw, ok := value.(map[string]any)
		if !ok {
			return "", &ChangeValidationError{"proposal change must be an object"}
		}
		if raw["path"] == path && (raw["kind"] == "create" || raw["kind"] == "patch") {
			if body, ok := raw["body"].(string); ok {
				return body, nil
			}
		}
	}
	entry, err := repo.read(q.Paths.CurrentDir, path)
	if err != nil {
		return "", err
	}
	return entry.Document.Body, nil
}
func ignorableBundleError(err error) bool {
	var bundle *repository.BundleError
	var frontmatter *repository.FrontmatterError
	var safety *repository.PathSafetyError
	return errors.Is(err, fs.ErrNotExist) || errors.As(err, &bundle) || errors.As(err, &frontmatter) || errors.As(err, &safety)
}
func conflictDetails(conflicts []ProposalConflict) []any {
	details := []any{}
	for _, c := range conflicts {
		item := map[string]any{"index": c.Index, "status": c.Status, "conflicting_paths": c.ConflictingPaths}
		if c.Reason != "" {
			item["reason"] = c.Reason
		}
		details = append(details, item)
	}
	return details
}
