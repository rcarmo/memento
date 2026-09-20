package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/internal/repository"
)

// PreparedProposal holds only store-ready changes; upload handles and inline
// bytes are replaced by independently validated manifest/digest references.
type PreparedProposal struct {
	Changes   []any
	Assets    []control.ProposalAssetInput
	StagedIDs []string
}

func (c *ProposalControls) prepareAssets(ctx context.Context, changes []any, principal string, repo proposalRepository) (PreparedProposal, error) {
	result := PreparedProposal{Changes: []any{}, Assets: []control.ProposalAssetInput{}, StagedIDs: []string{}}
	renamePaths := map[string]bool{}
	for _, value := range changes {
		raw, ok := value.(map[string]any)
		if !ok {
			return result, &ChangeValidationError{"proposal change must be an object"}
		}
		if raw["kind"] == "rename" {
			for _, key := range []string{"path", "new_path"} {
				p, err := proposalPathValue(raw, key)
				if err != nil {
					return result, err
				}
				renamePaths[p] = true
			}
		}
	}
	for _, value := range changes {
		raw := value.(map[string]any)
		item := map[string]any{}
		for key, value := range raw {
			item[key] = value
		}
		if item["kind"] != "attach_asset_pack" {
			result.Changes = append(result.Changes, item)
			continue
		}
		p, err := proposalPathValue(item, "path")
		if err != nil {
			return result, err
		}
		if renamePaths[p] {
			return result, &Error{"validation_error", "rename and asset attachment must use separate proposals"}
		}
		kind, err := proposalPathValue(item, "asset_kind")
		if err != nil {
			return result, err
		}
		if err = assets.ValidateKind(kind); err != nil {
			return result, err
		}
		version, err := proposalPathValue(item, "version")
		if err != nil {
			return result, err
		}
		encoded, inline := item["zip_base64"].(string)
		stagedID, staged := item["staged_asset_id"].(string)
		if inline == staged {
			return result, &Error{"validation_error", "attach_asset_pack requires exactly one of zip_base64 or staged_asset_id"}
		}
		var blob []byte
		if staged {
			if c.Staging == nil {
				return result, &Error{"validation_error", "asset staging is unavailable"}
			}
			upload, err := c.Staging.Get(ctx, principal, stagedID, true)
			if err != nil {
				var staging *assets.StagedAssetError
				if errors.As(err, &staging) {
					return result, &Error{"validation_error", err.Error()}
				}
				return result, err
			}
			if upload.AssetKind != kind || upload.Version != version {
				return result, &Error{"validation_error", "staged asset kind/version does not match proposal"}
			}
			blob = upload.BlobBytes
			result.StagedIDs = append(result.StagedIDs, stagedID)
		} else {
			blob, err = decodeProposalZIP(encoded)
			if err != nil {
				return result, err
			}
		}
		body, err := c.Queue.resultingBody(changes, p, repo)
		if err != nil {
			return result, err
		}
		var pack assets.ValidatedPack
		if kind == "skill" {
			canonical := repository.NormalizeConceptBody(body)
			if body != canonical {
				return result, &Error{"validation_error", "skill concept body must be canonical UTF-8 text with LF endings, no trailing whitespace and no final newline"}
			}
			name := path.Base(strings.TrimRight(p, "/"))
			if i := strings.LastIndexByte(name, '.'); i > 0 && i < len(name)-1 {
				name = name[:i]
			}
			pack, err = assets.ValidateSkillPack(name, version, canonical, blob)
		} else {
			pack, err = assets.ValidateAssetPack(kind, version, blob, "", "")
		}
		if err != nil {
			return result, err
		}
		existing, err := c.Queue.Proposals.ListAssets(ctx, control.ProposalAssetQuery{ConceptPath: &p, AssetKind: &kind})
		if err != nil {
			return result, err
		}
		for _, asset := range existing {
			proposal, err := c.Queue.Proposals.Get(ctx, asset.ProposalID)
			if err != nil {
				return result, err
			}
			proposal, err = c.Queue.refresh(ctx, proposal, "", nil, repo)
			if err != nil {
				return result, err
			}
			if asset.Version == version && (proposal.Status == control.Submitted || proposal.Status == control.Approved) {
				return result, &Error{"conflict", "active asset proposal already exists: " + p + " " + kind + " " + version}
			}
		}
		id, err := c.newOperationID()
		if err != nil {
			return result, err
		}
		// A concrete manifest contains no dynamic values; Encode to bytes.Buffer
		// cannot fail. Field order matches Pydantic model_dump_json, without ASCII
		// escaping or spaces; the enclosing patch uses pyjson's sorted ASCII form.
		var out bytes.Buffer
		encoder := json.NewEncoder(&out)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(pack.Manifest)
		manifestJSON := strings.TrimSuffix(out.String(), "\n")
		manifest, _ := pyjson.Parse(manifestJSON)
		result.Changes = append(result.Changes, map[string]any{"kind": "attach_asset_pack", "path": p, "asset_kind": kind, "version": version, "asset_id": id, "zip_sha256": pack.Manifest.SHA256, "manifest": manifest})
		result.Assets = append(result.Assets, control.ProposalAssetInput{AssetID: id, ConceptPath: p, AssetKind: kind, Version: version, MediaType: "application/zip", SHA256: pack.Manifest.SHA256, BlobBytes: blob, ManifestJSON: manifestJSON})
	}
	return result, nil
}
func decodeProposalZIP(encoded string) ([]byte, error) {
	if utf8.RuneCountInString(encoded) > ((assets.MaxZIPBytes+2)/3)*4 {
		return nil, &Error{"validation_error", "zip_base64 exceeds maximum encoded size"}
	}
	invalid := func() ([]byte, error) { return nil, &Error{"validation_error", "zip_base64 must be valid base64"} }
	// The pinned Python validate=True rejects redundant padding and line breaks
	// but accepts nonzero unused pad bits (so Go's Strict decoder is not used).
	if strings.ContainsAny(encoded, "\r\n") {
		return invalid()
	}
	blob, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return invalid()
	}
	return blob, nil
}
