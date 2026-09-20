package service

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

//go:embed metadata_tools.json
var metadataToolDefinitions []byte

func (j *Jobs) RegisterMetadataTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callStagingOrProposalTool, notify, metadataToolDefinitions)
}

type AssetMetadataOptions struct {
	IDOrPath, PathPrefix, AssetKind, Version, Cursor *string
	Limit, VersionLimit, FileLimit                   int
	IncludeFiles                                     any
}
type metadataIO struct {
	stat, lstat func(string) (fs.FileInfo, error)
	timestamp   func(context.Context, repository.GitRepositoryPaths, string, string) (time.Time, error)
}

func defaultMetadataIO() metadataIO {
	return metadataIO{os.Stat, os.Lstat, repository.PathCommitTimestamp}
}

const metadataKindsError = "asset metadata exceeds 50 asset kinds; narrow path_prefix or asset_kind"
const metadataVersionsError = "asset metadata exceeds 50 version records; narrow the request"
const metadataFilesError = "asset metadata exceeds 500 file records; lower file_limit or narrow the request"

func (c *ProposalControls) AssetMetadata(ctx context.Context, actor ProposalActor, o AssetMetadataOptions) (map[string]any, SuccessOptions, error) {
	return c.assetMetadata(ctx, actor, o, defaultProposalRepository(), defaultMetadataIO())
}
func (c *ProposalControls) assetMetadata(ctx context.Context, actor ProposalActor, o AssetMetadataOptions, repo proposalRepository, ops metadataIO) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if err := access.RequireRole(actor.Policy, "reader"); err != nil {
		return nil, options, err
	}
	if o.IDOrPath != nil && o.PathPrefix != nil {
		return nil, options, manifestInvalid("asset metadata accepts id_or_path or path_prefix, not both")
	}
	if o.IDOrPath != nil && o.Cursor != nil {
		return nil, options, manifestInvalid("asset metadata cursor requires path_prefix scope")
	}
	if o.Version != nil && o.AssetKind == nil {
		return nil, options, manifestInvalid("asset metadata version requires asset_kind")
	}
	if o.Limit < 1 || o.Limit > 20 {
		return nil, options, manifestInvalid("asset metadata limit must be between 1 and 20")
	}
	if o.VersionLimit < 1 || o.VersionLimit > 5 {
		return nil, options, manifestInvalid("asset metadata version_limit must be between 1 and 5")
	}
	if o.FileLimit < 1 || o.FileLimit > 100 {
		return nil, options, manifestInvalid("asset metadata file_limit must be between 1 and 100")
	}
	if o.AssetKind != nil {
		if err := assets.ValidateKind(*o.AssetKind); err != nil {
			return nil, options, err
		}
	}
	if o.Version != nil {
		if _, err := assets.ParseStableSemver(*o.Version); err != nil {
			return nil, options, err
		}
	}
	var prefix, next any
	selected := []string{}
	if o.IDOrPath != nil {
		path, err := c.resolveReadPath(actor.Policy, *o.IDOrPath)
		if err != nil {
			return nil, options, err
		}
		selected = append(selected, path)
	} else {
		p := "/"
		if o.PathPrefix != nil && *o.PathPrefix != "" {
			p = *o.PathPrefix
		}
		if err := validateInventoryPrefix(p); err != nil {
			return nil, options, err
		}
		if !inventoryPrefixReadable(actor.Policy, p) {
			return nil, options, &Error{"forbidden", "principal " + actor.Policy.Principal + " cannot read asset metadata under " + p}
		}
		if o.Cursor != nil && (!strings.HasPrefix(*o.Cursor, p) || !strings.HasSuffix(*o.Cursor, ".md")) {
			return nil, options, manifestInvalid("asset metadata cursor must be a concept path under path_prefix")
		}
		paths, err := repository.ListBundlePaths(c.Queue.Paths.CurrentDir, repository.BundleFilter{IncludePath: func(path string) bool { return strings.HasPrefix(path, p) && readable(actor.Policy, path) }, IncludeDirectory: func(dir string) bool {
			return (strings.HasPrefix(p, dir) || strings.HasPrefix(dir, p)) && inventoryPrefixReadable(actor.Policy, dir)
		}})
		if err != nil {
			return nil, options, err
		}
		candidates := []string{}
		for _, path := range paths {
			if o.Cursor == nil || path > *o.Cursor {
				candidates = append(candidates, path)
			}
		}
		selected = candidates[:min(o.Limit, len(candidates))]
		if len(candidates) > len(selected) && len(selected) > 0 {
			next = selected[len(selected)-1]
		}
		prefix = p
	}
	revision, err := repo.main(c.Queue.Paths)
	if err != nil {
		return nil, options, err
	}
	kinds, versions, files := 0, 0, 0
	entries := []any{}
	for _, path := range selected {
		if _, err = access.AuthorizePath(actor.Policy, path, "read"); err != nil {
			return nil, options, err
		}
		entry, err := repository.ReadBundleEntry(c.Queue.Paths.CurrentDir, path)
		if err != nil {
			return nil, options, err
		}
		body := []byte(entry.Document.Body)
		digest := fmt.Sprintf("%x", sha256.Sum256(body))
		packs, v, f, err := c.metadataAssets(ctx, entry.Document.Frontmatter.ID, digest, revision, o, 50-kinds, 50-versions, 500-files, ops)
		if err != nil {
			return nil, options, err
		}
		kinds += len(packs)
		versions += v
		files += f
		// The helper enforces remaining budgets before adding records, making the
		// source's repeated post-add checks unreachable here.
		entries = append(entries, map[string]any{"path": path, "id": entry.Document.Frontmatter.ID, "current_concept_body_sha256": digest, "current_concept_body_bytes": len(body), "asset_present": len(packs) > 0, "assets": packs})
	}
	options.RepoRevision = &revision
	return map[string]any{"id_or_path": nullableText(o.IDOrPath), "path_prefix": prefix, "asset_kind": nullableText(o.AssetKind), "version": nullableText(o.Version), "include_files": o.IncludeFiles, "entries": entries, "next_cursor": next}, options, nil
}
func (c *ProposalControls) metadataAssets(ctx context.Context, concept, bodySHA, revision string, o AssetMetadataOptions, kindBudget, versionBudget, fileBudget int, ops metadataIO) ([]any, int, int, error) {
	root := c.Queue.Paths.CurrentDir
	directory := filepath.Join(root, ".assets", concept)
	for _, path := range []string{filepath.Join(root, ".assets"), directory} {
		if info, err := ops.lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, 0, 0, manifestInvalid("asset metadata directories must not be symbolic links")
		}
	}
	if info, err := ops.stat(directory); err == nil && !info.IsDir() {
		return nil, 0, 0, manifestInvalid("concept asset metadata path must be a directory")
	}
	kinds, err := assets.ListAssetKinds(root, concept)
	if err != nil {
		return nil, 0, 0, err
	}
	selected := []string{}
	for _, kind := range kinds {
		if o.AssetKind == nil || *o.AssetKind == kind {
			selected = append(selected, kind)
		}
	}
	if len(selected) > kindBudget {
		return nil, 0, 0, manifestInvalid(metadataKindsError)
	}
	result := []any{}
	versionCount, fileCount := 0, 0
	for _, kind := range selected {
		info, err := ops.lstat(filepath.Join(directory, kind))
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, 0, 0, manifestInvalid("asset kind metadata path must be a regular directory")
		}
		versions, err := assets.ListAssetVersions(root, concept, kind)
		if err != nil {
			return nil, 0, 0, err
		}
		if len(versions) == 0 {
			continue
		}
		reported := append([]string{}, versions[max(0, len(versions)-o.VersionLimit):]...)
		detailed := []string{}
		present := false
		if o.Version != nil {
			reported = []string{}
			for _, v := range versions {
				if v == *o.Version {
					present = true
					reported = append(reported, v)
				}
			}
			detailed = append(detailed, reported...)
		} else {
			for i := len(reported) - 1; i >= 0; i-- {
				detailed = append(detailed, reported[i])
			}
		}
		if len(detailed) > versionBudget-versionCount {
			return nil, 0, 0, manifestInvalid(metadataVersionsError)
		}
		details := []any{}
		for _, version := range detailed {
			detail, n, err := c.assetVersionMetadata(ctx, concept, kind, version, bodySHA, revision, manifestTruthy(o.IncludeFiles), o.FileLimit, ops)
			if err != nil {
				return nil, 0, 0, err
			}
			details = append(details, detail)
			fileCount += n
			if fileCount > fileBudget {
				return nil, 0, 0, manifestInvalid(metadataFilesError)
			}
		}
		latest, err := assets.LoadAssetMetadata(root, concept, kind, versions[len(versions)-1])
		if err != nil {
			return nil, 0, 0, err
		}
		digest, ok := latest["zip_sha256"].(string)
		if !ok || !lowerMetadataSHA(digest) {
			return nil, 0, 0, manifestInvalid("asset metadata has an invalid zip_sha256")
		}
		versionCount += len(details)
		payload := map[string]any{"kind": kind, "versions": reported, "version_count": len(versions), "versions_truncated": len(reported) < len(versions), "latest_version": versions[len(versions)-1], "latest_sha256": digest, "version_metadata": details, "version_metadata_truncated": o.Version == nil && len(versions) > len(detailed)}
		if o.Version != nil {
			payload["requested_version_present"] = present
		}
		result = append(result, payload)
	}
	return result, versionCount, fileCount, nil
}
func lowerMetadataSHA(value string) bool {
	return manifestSHA.MatchString(value) && strings.ToLower(value) == value
}
func (c *ProposalControls) assetVersionMetadata(ctx context.Context, concept, kind, version, bodySHA, revision string, include bool, limit int, ops metadataIO) (map[string]any, int, error) {
	metaPath, zipPath, err := assets.AssetVersionPaths(concept, kind, version)
	if err != nil {
		return nil, 0, err
	}
	metadata, err := assets.LoadAssetMetadata(c.Queue.Paths.CurrentDir, concept, kind, version)
	if err != nil {
		return nil, 0, err
	}
	n, number := metadata["schema_version"].(json.Number)
	schemaOK := metadata["schema_version"] == true
	if number {
		r, ok := new(big.Rat).SetString(string(n))
		schemaOK = ok && r.Cmp(big.NewRat(1, 1)) == 0
	}
	if !schemaOK {
		return nil, 0, manifestInvalid("asset metadata has an invalid schema_version")
	}
	// LoadAssetMetadata already enforces kind=asset_pack_version.
	for _, field := range []struct{ name, value string }{{"concept_id", concept}, {"asset_kind", kind}, {"version", version}} {
		if metadata[field.name] != field.value {
			return nil, 0, manifestInvalid("asset metadata has an invalid " + field.name)
		}
	}
	path, ok := metadata["concept_path"].(string)
	if !ok || !strings.HasPrefix(path, "/") {
		return nil, 0, manifestInvalid("asset metadata has an invalid concept_path")
	}
	digest, ok := metadata["zip_sha256"].(string)
	if !ok || !lowerMetadataSHA(digest) {
		return nil, 0, manifestInvalid("asset metadata has an invalid zip_sha256")
	}
	actor, ok := metadata["accepted_by"].(string)
	if !ok || actor == "" {
		return nil, 0, manifestInvalid("asset metadata has an invalid accepted_by")
	}
	proposal, ok := metadata["source_proposal_id"].(string)
	if !ok || proposal == "" {
		return nil, 0, manifestInvalid("asset metadata has an invalid source_proposal_id")
	}
	raw, _ := json.Marshal(metadata["manifest"])
	manifest, err := assets.ParseManifest(string(raw))
	if err != nil {
		return nil, 0, &ChangeValidationError{Message: err.Error()}
	}
	if manifest.SHA256 != digest {
		return nil, 0, manifestInvalid("asset metadata manifest and ZIP digests differ")
	}
	stamp := metadata["created_at"]
	if stamp == nil {
		stamp, err = ops.timestamp(ctx, c.Queue.Paths, revision, metaPath)
		if err != nil {
			return nil, 0, err
		}
	}
	created, err := manifestTimestamp(stamp)
	if err != nil {
		return nil, 0, err
	}
	zip := filepath.Join(c.Queue.Paths.CurrentDir, zipPath[1:])
	info, err := ops.lstat(zip)
	if err != nil || !info.Mode().IsRegular() {
		return nil, 0, manifestInvalid("asset ZIP must be a regular file")
	}
	info, err = ops.stat(zip)
	if err != nil {
		return nil, 0, err
	}
	ordered := append([]assets.ManifestEntry{}, manifest.Entries...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	result := map[string]any{"version": version, "created_at": modelTimestamp(created), "created_by": actor, "source_proposal_id": proposal, "zip_sha256": digest, "zip_bytes": info.Size(), "manifest_sha256": assets.ManifestDigest(manifest), "file_count": manifest.FileCount, "total_uncompressed_bytes": manifest.TotalUncompressedBytes}
	count := 0
	if include {
		selected := ordered[:min(limit, len(ordered))]
		files := []any{}
		for _, file := range selected {
			files = append(files, map[string]any{"path": file.Path, "sha256": file.SHA256, "bytes": file.Size, "media_type": file.MediaType})
		}
		result["files"] = files
		result["files_truncated"] = len(selected) < len(ordered)
		count = len(selected)
	}
	if kind == "skill" {
		var skill *assets.ManifestEntry
		for i := range manifest.Entries {
			if manifest.Entries[i].Path == "SKILL.md" {
				skill = &manifest.Entries[i]
				break
			}
		}
		if skill == nil {
			return nil, 0, manifestInvalid("skill asset metadata is missing root SKILL.md")
		}
		result["concept_body_sha256_at_publish"] = skill.SHA256
		result["kind_invariants"] = map[string]any{"skill_root_matches_current_concept_body": skill.SHA256 == bodySHA}
	}
	return result, count, nil
}
