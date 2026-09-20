package service

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/pyjson"
)

// ApplyChanges mirrors the source mutation callback, including a sorted set of
// paths. It does not validate the overall archival batch or publish the worktree.
func (m WorktreeMutator) ApplyChanges(ctx context.Context, root string, changes []ProposalChange, actor string, policy access.EffectivePolicy, proposalID *string) ([]string, error) {
	changed := map[string]bool{}
	for _, change := range changes {
		var paths []string
		var err error
		switch change["kind"] {
		case "create":
			err = m.Create(root, change, actor)
			paths = []string{change["path"].(string)}
		case "patch":
			err = m.Patch(root, change, actor)
			paths = []string{change["path"].(string)}
		case "rename":
			paths, err = m.Rename(root, change, actor, policy)
		case "trash":
			paths, err = m.Trash(root, change, policy)
		case "attach_asset_pack":
			if proposalID == nil {
				return nil, &Error{"validation_error", "asset changes require a proposal"}
			}
			paths, err = m.ApplyAssetPack(ctx, root, change, actor, *proposalID)
		default:
			return nil, &ChangeValidationError{"unsupported normalised change"}
		}
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			changed[path] = true
		}
	}
	result := make([]string, 0, len(changed))
	for path := range changed {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

// ApplyAssetPack preserves the source boundary: it compares stored digests and
// validates manifest shape but does not re-hash/re-validate ZIP bytes. Those are
// validated when preparing/staging proposals. Re-validating here changes parity.
func (m WorktreeMutator) ApplyAssetPack(ctx context.Context, root string, change ProposalChange, actor, proposalID string) ([]string, error) {
	return m.applyAssetPack(ctx, root, change, actor, proposalID, defaultMutationIO())
}
func (m WorktreeMutator) applyAssetPack(ctx context.Context, root string, change ProposalChange, actor, proposalID string, ops mutationIO) ([]string, error) {
	entry, err := ops.read(root, change["path"].(string))
	if err != nil {
		return nil, err
	}
	asset, err := m.Proposals.GetAsset(ctx, proposalID, change["asset_id"].(string))
	if err != nil {
		return nil, err
	}
	if asset.SHA256 != change["zip_sha256"] {
		return nil, &Error{"conflict", "proposal asset digest does not match proposal metadata"}
	}
	raw, err := pyjson.Dumps(change["manifest"])
	if err != nil {
		return nil, err
	}
	manifest, err := assets.ParseManifest(raw)
	if err != nil {
		return nil, err
	}
	if manifest.SHA256 != asset.SHA256 {
		return nil, &Error{"conflict", "proposal asset manifest digest does not match stored bytes"}
	}
	now := m.now()
	return assets.WriteAssetVersion(root, assets.AcceptedVersion{ConceptID: entry.Document.Frontmatter.ID, ConceptPath: change["path"].(string), AssetKind: change["asset_kind"].(string), Version: change["version"].(string), ZIPBytes: asset.BlobBytes, Manifest: manifest, AcceptedBy: actor, SourceProposalID: proposalID, CreatedAt: &now})
}

// AdaptExistingAssetConcepts changes a create into a patch only when that same
// proposal attaches an asset and the current concept path exists. Metadata/body
// fields retain their normalised values, including empty tags and aliases.
func AdaptExistingAssetConcepts(root string, changes []ProposalChange) []ProposalChange {
	return adaptExistingAssetConcepts(root, changes, defaultMutationIO())
}
func adaptExistingAssetConcepts(root string, changes []ProposalChange, ops mutationIO) []ProposalChange {
	attached := map[string]bool{}
	for _, change := range changes {
		if change["kind"] == "attach_asset_pack" {
			attached[change["path"].(string)] = true
		}
	}
	result := make([]ProposalChange, 0, len(changes))
	for _, change := range changes {
		path := change["path"].(string)
		if change["kind"] == "create" && attached[path] && targetExists(filepath.Join(root, strings.TrimPrefix(path, "/")), ops) {
			result = append(result, ProposalChange{"kind": "patch", "path": path, "title": change["title"], "description": change["description"], "body": change["body"], "tags": change["tags"], "aliases": change["aliases"], "status": nil})
		} else {
			result = append(result, change)
		}
	}
	return result
}
