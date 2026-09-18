package service

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/repository"
	"github.com/rcarmo/memento/go/umcp"
)

//go:embed inventory_tools.json
var inventoryToolDefinitions []byte

// RegisterInventoryTools adds namespace inventory to the 24-tool development
// subset; configured discovery surfaces remain separate work.
func (j *Jobs) RegisterInventoryTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callStagingOrProposalTool, notify, inventoryToolDefinitions)
}

var inventoryFields = []string{"path", "id", "title", "status", "type", "tags", "created_at", "updated_at", "updated_by", "body_sha256", "body_bytes", "assets"}

func validateInventoryPrefix(prefix string) error {
	if !strings.HasPrefix(prefix, "/") || !strings.HasSuffix(prefix, "/") {
		return &Error{"validation_error", "inventory path_prefix must be an absolute directory prefix"}
	}
	parts := strings.Split(strings.TrimPrefix(prefix, "/"), "/")
	for _, part := range parts {
		if part == "." || part == ".." {
			return &Error{"validation_error", "inventory path_prefix is unsafe"}
		}
	}
	if strings.ContainsAny(prefix, "\\\x00") {
		return &Error{"validation_error", "inventory path_prefix is unsafe"}
	}
	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			return &Error{"validation_error", "inventory path_prefix must not contain empty segments"}
		}
	}
	return nil
}
func inventoryPrefixReadable(policy access.EffectivePolicy, prefix string) bool {
	if strings.HasPrefix(prefix, "/trash/") {
		prefix = strings.TrimPrefix(prefix, "/trash")
	}
	for _, grant := range policy.ReadPrefixes {
		intersection := ""
		if strings.HasPrefix(prefix, grant) {
			intersection = prefix
		} else if strings.HasPrefix(grant, prefix) {
			intersection = grant
		} else {
			continue
		}
		if readable(policy, intersection) {
			return true
		}
	}
	return false
}

// Inventory prunes directory traversal before pagination and parses only the
// selected concepts. Cursor is a lexical path boundary, not an opaque token.
// Nil fields means all defaults; an explicit empty list must fail.
func (c *ProposalControls) Inventory(ctx context.Context, actor ProposalActor, prefix string, fields []string, limit int, cursor *string) (map[string]any, SuccessOptions, error) {
	return c.inventory(ctx, actor, prefix, fields, limit, cursor, defaultProposalRepository())
}
func (c *ProposalControls) inventory(ctx context.Context, actor ProposalActor, prefix string, fields []string, limit int, cursor *string, repo proposalRepository) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if err := access.RequireRole(actor.Policy, "reader"); err != nil {
		return nil, options, err
	}
	if err := validateInventoryPrefix(prefix); err != nil {
		return nil, options, err
	}
	if !inventoryPrefixReadable(actor.Policy, prefix) {
		return nil, options, &Error{"forbidden", "principal " + actor.Policy.Principal + " cannot read inventory under " + prefix}
	}
	if limit < 1 || limit > 100 {
		return nil, options, &Error{"validation_error", "inventory limit must be between 1 and 100"}
	}
	if cursor != nil && (!strings.HasPrefix(*cursor, prefix) || !strings.HasSuffix(*cursor, ".md")) {
		return nil, options, &Error{"validation_error", "inventory cursor must be a concept path under path_prefix"}
	}
	if fields == nil {
		fields = inventoryFields
	}
	selectedFields := []string{}
	seen := map[string]bool{}
	known := map[string]bool{}
	for _, field := range inventoryFields {
		known[field] = true
	}
	unknown := []string{}
	for _, field := range fields {
		if !seen[field] {
			seen[field] = true
			selectedFields = append(selectedFields, field)
			if !known[field] {
				unknown = append(unknown, field)
			}
		}
	}
	if len(selectedFields) == 0 {
		return nil, options, &Error{"validation_error", "inventory fields must not be empty"}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, options, &Error{"validation_error", "unsupported inventory fields: " + strings.Join(unknown, ", ")}
	}
	paths, err := repository.ListBundlePaths(c.Queue.Paths.CurrentDir, repository.BundleFilter{
		IncludePath: func(path string) bool {
			return strings.HasPrefix(path, prefix) && (strings.HasPrefix(prefix, "/trash/") || !strings.HasPrefix(path, "/trash/")) && readable(actor.Policy, path)
		},
		IncludeDirectory: func(directory string) bool {
			return (strings.HasPrefix(prefix, directory) || strings.HasPrefix(directory, prefix)) && inventoryPrefixReadable(actor.Policy, directory)
		},
	})
	if err != nil {
		return nil, options, err
	}
	candidates := []string{}
	for _, path := range paths {
		if cursor == nil || path > *cursor {
			candidates = append(candidates, path)
		}
	}
	selected := candidates[:min(limit, len(candidates))]
	entries := []any{}
	for _, path := range selected {
		entry, err := repository.ReadBundleEntry(c.Queue.Paths.CurrentDir, path)
		if err != nil {
			return nil, options, err
		}
		m := entry.Document.Frontmatter
		available := map[string]any{"path": entry.BundlePath, "id": m.ID, "title": m.Title, "status": m.Status, "type": m.Type, "tags": append([]string{}, m.Tags...), "created_at": modelTimestamp(m.CreatedAt), "updated_at": modelTimestamp(m.UpdatedAt), "updated_by": m.UpdatedBy}
		if seen["body_sha256"] || seen["body_bytes"] {
			body := []byte(entry.Document.Body)
			available["body_sha256"] = fmt.Sprintf("%x", sha256.Sum256(body))
			available["body_bytes"] = len(body)
		}
		if seen["assets"] {
			value, err := c.inventoryAssets(m.ID)
			if err != nil {
				return nil, options, err
			}
			available["assets"] = value
		}
		projected := map[string]any{}
		for _, field := range selectedFields {
			projected[field] = available[field]
		}
		entries = append(entries, projected)
	}
	var next any
	if len(candidates) > len(selected) && len(selected) > 0 {
		next = selected[len(selected)-1]
	}
	revision, err := repo.main(c.Queue.Paths)
	if err != nil {
		return nil, options, err
	}
	options.RepoRevision = &revision
	return map[string]any{"path_prefix": prefix, "fields": selectedFields, "entries": entries, "next_cursor": next}, options, nil
}
func (c *ProposalControls) inventoryAssets(concept string) ([]any, error) {
	result := []any{}
	root := c.Queue.Paths.CurrentDir
	kinds, err := assets.ListAssetKinds(root, concept)
	if err != nil {
		return nil, err
	}
	for _, kind := range kinds {
		versions, err := assets.ListAssetVersions(root, concept, kind)
		if err != nil {
			return nil, err
		}
		if len(versions) == 0 {
			continue
		}
		latest := versions[len(versions)-1]
		metadata, err := assets.LoadAssetMetadata(root, concept, kind, latest)
		if err != nil {
			return nil, err
		}
		digest, ok := metadata["zip_sha256"].(string)
		if !ok {
			return nil, &Error{"validation_error", "asset metadata is missing zip_sha256"}
		}
		result = append(result, map[string]any{"kind": kind, "versions": versions, "latest_version": latest, "latest_sha256": digest})
	}
	return result, nil
}
