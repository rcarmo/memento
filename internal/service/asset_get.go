package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"slices"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/repository"
)

type AssetGetOptions struct {
	IDOrPath, AssetKind, View         string
	Version, FilePath, ExpectedSHA256 *string
	Offset                            int64
	Limit                             *int64
}

// AssetGet holds the repository lock through version selection, byte verification
// and revision capture, preventing checkout replacement or pruning mid-read.
// Role and range checks happen before waiting for a writer, as in Python.
func (c *ProposalControls) AssetGet(ctx context.Context, actor ProposalActor, o AssetGetOptions) (map[string]any, SuccessOptions, error) {
	return c.assetGet(ctx, actor, o, os.Stat, func(path string) (assetSource, error) { return os.Open(path) }, defaultProposalRepository())
}

type assetSource interface {
	io.Reader
	io.ReaderAt
	io.Closer
}

func (c *ProposalControls) assetGet(ctx context.Context, actor ProposalActor, o AssetGetOptions, stat func(string) (fs.FileInfo, error), open func(string) (assetSource, error), repo proposalRepository) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if err := access.RequireRole(actor.Policy, "reader"); err != nil {
		return nil, options, err
	}
	if o.View != "archive" && o.View != "manifest" && o.View != "file" {
		return nil, options, &Error{"validation_error", "asset view must be archive, manifest or file"}
	}
	limit := int64(assets.DefaultChunkBytes)
	if o.Limit != nil {
		limit = *o.Limit
	}
	if err := assets.ValidateRange(o.Offset, limit, nil); err != nil {
		return nil, options, err
	}
	if o.View == "file" && o.FilePath == nil {
		return nil, options, &Error{"validation_error", "file view requires file_path"}
	}
	if o.View != "file" && o.FilePath != nil {
		return nil, options, &Error{"validation_error", "file_path requires file view"}
	}
	if o.View == "manifest" && (o.Offset != 0 || o.Limit != nil) {
		return nil, options, &Error{"validation_error", "manifest view does not accept ranges"}
	}
	if o.Offset > 0 && (o.Version == nil || o.ExpectedSHA256 == nil) {
		return nil, options, &Error{"validation_error", "resuming requires explicit version and expected_sha256"}
	}
	var data map[string]any
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		data, err = c.readAcceptedAsset(actor, o, limit, stat, open)
		if err != nil {
			return err
		}
		revision, err := repo.main(c.Queue.Paths)
		if err != nil {
			return err
		}
		options.RepoRevision = &revision
		return nil
	})
	var missing *assets.AssetVersionNotFoundError
	if errors.As(err, &missing) || errors.Is(err, fs.ErrNotExist) {
		err = &Error{"not_found", err.Error()}
	}
	return data, options, err
}
func (c *ProposalControls) readAcceptedAsset(actor ProposalActor, o AssetGetOptions, limit int64, stat func(string) (fs.FileInfo, error), open func(string) (assetSource, error)) (map[string]any, error) {
	root := c.Queue.Paths.CurrentDir
	path, err := c.resolveReadPath(actor.Policy, o.IDOrPath)
	if err != nil {
		return nil, err
	}
	if _, err = access.AuthorizePath(actor.Policy, path, "read"); err != nil {
		return nil, err
	}
	entry, err := repository.ReadBundleEntry(root, path)
	if err != nil {
		return nil, err
	}
	id := entry.Document.Frontmatter.ID
	version, err := assets.ResolveAssetVersion(root, id, o.AssetKind, o.Version)
	if err != nil {
		return nil, err
	}
	metadata, err := assets.LoadAssetMetadata(root, id, o.AssetKind, version)
	if err != nil {
		return nil, err
	}
	if metadata["concept_id"] != id || metadata["asset_kind"] != o.AssetKind || metadata["version"] != version {
		return nil, &assets.ReadError{Message: "asset metadata identity mismatch"}
	}
	// Python str() can also turn an arbitrary-size JSON integer into a digest.
	// All other JSON types render characters outside the lowercase hex grammar.
	digest, _ := metadata["zip_sha256"].(string)
	if number, ok := metadata["zip_sha256"].(json.Number); ok {
		digest = number.String()
	}
	if err = assets.CheckExpectedDigest(&digest, digest); err != nil {
		return nil, &assets.ReadError{Message: "invalid ZIP digest in asset metadata"}
	}
	raw, _ := json.Marshal(metadata["manifest"]) // parsed JSON has no unsupported values
	manifest, err := assets.ParseManifest(string(raw))
	if err != nil {
		return nil, &assets.ReadError{Message: err.Error()}
	}
	if err = assets.CheckedManifest(manifest, digest); err != nil {
		return nil, err
	}
	if err = assets.CheckExpectedDigest(o.ExpectedSHA256, digest); err != nil {
		return nil, err
	}
	_, zipPath, _ := assets.AssetVersionPaths(id, o.AssetKind, version) // validated by resolution/metadata
	archive, err := repository.ValidateRepositoryReadPath(root, zipPath)
	if err != nil {
		return nil, err
	}
	info, err := stat(archive.AbsolutePath)
	if err != nil {
		return nil, err
	}
	total := info.Size()
	if err = assets.ValidateArchiveSize(total); err != nil {
		return nil, err
	}
	versions, err := assets.ListAssetVersions(root, id, o.AssetKind)
	if err != nil {
		return nil, err
	}
	slices.Reverse(versions)
	payload := map[string]any{"concept_id": id, "concept_path": path, "asset_kind": o.AssetKind, "version": version, "view": o.View, "media_type": "application/zip", "versions": versions, "zip_sha256": digest, "zip_bytes": total, "manifest_sha256": assets.ManifestDigest(manifest), "file_count": manifest.FileCount, "total_uncompressed_bytes": manifest.TotalUncompressedBytes}
	if o.View == "manifest" {
		payload["manifest"] = manifest
		return payload, nil
	}
	source, err := open(archive.AbsolutePath)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	if o.View == "file" {
		if _, err = assets.VerifiedSlice(source, total, digest, 0, 1); err != nil {
			return nil, err
		}
		file, err := assets.ReadPackFile(source, total, manifest, *o.FilePath, o.Offset, limit)
		if err != nil {
			return nil, err
		}
		payload["file"] = file
	} else {
		inline := o.Limit == nil && o.Offset == 0 && total <= assets.MaxInlineArchiveBytes
		if inline {
			limit = total
		}
		content, err := assets.VerifiedSlice(source, total, digest, o.Offset, limit)
		if err != nil {
			return nil, err
		}
		for key, value := range assets.RangePayload(content, o.Offset, total) {
			payload[key] = value
		}
		payload["zip_base64"] = base64.StdEncoding.EncodeToString(content)
		payload["encoding"] = "base64"
		if inline {
			payload["manifest"] = manifest
		}
	}
	return payload, nil
}
