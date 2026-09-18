package service

import (
	"bytes"
	"context"
	"errors"
	"os"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

// GetProposal returns the models-off service payload. Unknown views follow the
// source's detailed branch; MCP Literal validation belongs at the tool boundary.
func (c *ProposalControls) GetProposal(ctx context.Context, actor ProposalActor, id, view string) (map[string]any, error) {
	var result map[string]any
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		result, err = c.getProposal(ctx, actor, id, view, defaultProposalRepository())
		return err
	})
	return result, err
}
func (c *ProposalControls) getProposal(ctx context.Context, actor ProposalActor, id, view string, repo proposalRepository) (map[string]any, error) {
	if err := access.RequireRole(actor.Policy, "proposer"); err != nil {
		return nil, err
	}
	record, err := c.Queue.VisibleProposal(ctx, actor.Policy, id)
	if err != nil {
		return nil, err
	}
	record, err = c.Queue.refresh(ctx, record, "", nil, repo)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if view == "summary" {
		allAssets, loadErr := c.Queue.Proposals.ListAssets(ctx, control.ProposalAssetQuery{ProposalID: &id})
		if loadErr != nil {
			return nil, loadErr
		}
		payload, err = c.Queue.summary(ctx, record, allAssets, true, repo)
	} else {
		changes, changeErr := proposalChanges(record)
		if changeErr != nil {
			return nil, changeErr
		}
		preview, previewErr := (WorktreeMutator{MaxConceptBytes: c.MaxConceptBytes}).PreviewChanges(c.Queue.Paths.CurrentDir, changes)
		if previewErr != nil {
			var safety *repository.PathSafetyError
			if errors.Is(previewErr, os.ErrNotExist) || errors.As(previewErr, &safety) {
				preview = "Preview unavailable at current revision; inspect stored changes and conflicts."
			} else {
				return nil, previewErr
			}
		}
		payload, err = c.Queue.payload(ctx, record, preview, repo)
	}
	if err != nil {
		return nil, err
	}
	visible, err := c.Queue.visibleArchivalImpact(ctx, record, actor.Policy, c.DerivedIndexPath, repo)
	if err != nil {
		return nil, err
	}
	payload["archival_impact"] = visible
	return map[string]any{"proposal": payload}, nil
}

// ProposalAssetGet requires proposal visibility AND read access to the stored
// asset concept. Metadata-only reads deliberately do not validate stored bytes.
func (c *ProposalControls) ProposalAssetGet(ctx context.Context, actor ProposalActor, id, assetID string, filePath *string, offset, limit int64) (map[string]any, error) {
	if err := access.RequireRole(actor.Policy, "proposer"); err != nil {
		return nil, err
	}
	if _, err := c.Queue.VisibleProposal(ctx, actor.Policy, id); err != nil {
		return nil, err
	}
	if offset < 0 {
		return nil, &Error{"validation_error", "asset file offset must not be negative"}
	}
	if limit < 1 || limit > assets.MaxChunkBytes {
		return nil, &Error{"validation_error", "asset file limit must be between 1 and 262144"}
	}
	asset, err := c.Queue.Proposals.GetAsset(ctx, id, assetID)
	if err != nil {
		return nil, err
	}
	if _, err = access.AuthorizePath(actor.Policy, asset.ConceptPath, "read"); err != nil {
		return nil, err
	}
	manifest, err := asset.Manifest()
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"proposal_id": id, "asset_id": asset.AssetID, "concept_path": asset.ConceptPath, "asset_kind": asset.AssetKind, "version": asset.Version, "media_type": asset.MediaType, "zip_sha256": asset.SHA256, "manifest": manifest}
	if filePath == nil {
		return payload, nil
	}
	parsed, err := assets.ParseManifest(asset.ManifestJSON)
	if err != nil {
		return nil, err
	}
	if err = assets.CheckedManifest(parsed, asset.SHA256); err != nil {
		return nil, err
	}
	if err = assets.ValidateArchiveSize(int64(len(asset.BlobBytes))); err != nil {
		return nil, err
	}
	if _, err = assets.VerifiedSlice(bytes.NewReader(asset.BlobBytes), int64(len(asset.BlobBytes)), asset.SHA256, 0, 1); err != nil {
		return nil, err
	}
	file, err := assets.ReadPackFile(bytes.NewReader(asset.BlobBytes), int64(len(asset.BlobBytes)), parsed, *filePath, offset, limit)
	if err != nil {
		return nil, err
	}
	payload["file"] = file
	return payload, nil
}
