package service

import (
	"context"
	_ "embed"
	"encoding/json"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

//go:embed manifest_tools.json
var manifestToolDefinitions []byte

// RegisterManifestTools registers the 25 implemented development tools.
func (j *Jobs) RegisterManifestTools(server *umcp.Server, notify ProposalNotifier) error {
	return registerProposalTools(server, j.callStagingOrProposalTool, notify, manifestToolDefinitions)
}
func manifestInvalid(message string) error { return &Error{"validation_error", message} }

var manifestSHA = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func unknownManifestFields(value map[string]any, allowed ...string) []string {
	known := map[string]bool{}
	for _, field := range allowed {
		known[field] = true
	}
	unknown := []string{}
	for field := range value {
		if !known[field] {
			unknown = append(unknown, field)
		}
	}
	sort.Strings(unknown)
	return unknown
}
func boundedManifestString(value any, limit int) (string, bool) {
	text, ok := value.(string)
	return text, ok && text != "" && utf8.RuneCountInString(text) <= limit
}
func manifestBytes(value any) (json.Number, bool) {
	n, ok := value.(json.Number)
	if !ok {
		return "", false
	}
	integer, ok := new(big.Int).SetString(string(n), 10)
	if !ok || integer.Sign() < 0 {
		return "", false
	}
	return json.Number(integer.String()), true
}
func validateManifestTarget(target, prefix string) error {
	if !strings.HasPrefix(target, "/") || !strings.HasSuffix(target, ".md") || !strings.HasPrefix(target, prefix) || strings.ContainsAny(target, "\\\x00") || strings.Contains(target, "//") {
		return manifestInvalid("manifest memento_path must be a concept path under path_prefix")
	}
	for _, part := range strings.Split(strings.TrimPrefix(target, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			return manifestInvalid("manifest memento_path is unsafe")
		}
	}
	return nil
}

type manifestItem struct {
	payload map[string]any
	updated time.Time
}

func (c *ProposalControls) CompareManifest(ctx context.Context, actor ProposalActor, prefix string, items []any, match map[string]any, includeAssets bool) (map[string]any, SuccessOptions, error) {
	return c.compareManifest(ctx, actor, prefix, items, match, includeAssets, defaultProposalRepository())
}
func (c *ProposalControls) compareManifest(ctx context.Context, actor ProposalActor, prefix string, items []any, match map[string]any, includeAssets bool, repo proposalRepository) (map[string]any, SuccessOptions, error) {
	options := SuccessOptions{}
	if err := access.RequireRole(actor.Policy, "reader"); err != nil {
		return nil, options, err
	}
	if err := validateInventoryPrefix(prefix); err != nil {
		return nil, options, err
	}
	if !inventoryPrefixReadable(actor.Policy, prefix) {
		return nil, options, &Error{"forbidden", "principal " + actor.Policy.Principal + " cannot compare manifests under " + prefix}
	}
	if len(items) == 0 || len(items) > 50 {
		return nil, options, manifestInvalid("manifest comparison requires between 1 and 50 items")
	}
	if unknown := unknownManifestFields(match, "path_template", "aliases"); len(unknown) > 0 {
		return nil, options, manifestInvalid("unsupported manifest match fields: " + strings.Join(unknown, ", "))
	}
	var template *string
	if value := match["path_template"]; value != nil {
		text, ok := value.(string)
		if !ok || utf8.RuneCountInString(text) > 1024 {
			return nil, options, manifestInvalid("manifest path_template must be a bounded string")
		}
		if strings.Count(text, "{name}") != 1 || strings.ContainsAny(strings.Replace(text, "{name}", "", 1), "{}") {
			return nil, options, manifestInvalid("manifest path_template must contain only one {name} field")
		}
		template = &text
	}
	aliases := map[string]string{}
	if value, exists := match["aliases"]; exists {
		mapping, ok := value.(map[string]any)
		if !ok || len(mapping) > 50 {
			return nil, options, manifestInvalid("manifest aliases must be a mapping with at most 50 entries")
		}
		for alias, mapped := range mapping {
			text, ok := mapped.(string)
			if !ok {
				return nil, options, manifestInvalid("manifest aliases must map strings to strings")
			}
			if alias == "" || utf8.RuneCountInString(alias) > 128 || text == "" || utf8.RuneCountInString(text) > 128 {
				return nil, options, manifestInvalid("manifest alias names must contain 1 to 128 characters")
			}
			aliases[alias] = text
		}
	}
	normalized := []manifestItem{}
	names, targets := map[string]bool{}, map[string]bool{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, options, umcp.ExecutionError{Type: "TypeError", Message: "manifest items must be mappings"}
		}
		if unknown := unknownManifestFields(item, "name", "local_path", "memento_path", "local_updated_at", "local_body_sha256", "local_bytes"); len(unknown) > 0 {
			return nil, options, manifestInvalid("unsupported manifest item fields: " + strings.Join(unknown, ", "))
		}
		name, ok := boundedManifestString(item["name"], 128)
		if !ok {
			return nil, options, manifestInvalid("manifest item name must contain 1 to 128 characters")
		}
		if names[name] {
			return nil, options, manifestInvalid("duplicate manifest item name: " + name)
		}
		localPath, ok := boundedManifestString(item["local_path"], 512)
		if !ok {
			return nil, options, manifestInvalid("manifest local_path must contain 1 to 512 characters")
		}
		digest, ok := item["local_body_sha256"].(string)
		if !ok || !manifestSHA.MatchString(digest) {
			return nil, options, manifestInvalid("manifest local_body_sha256 must be a SHA-256 hex digest")
		}
		size, ok := manifestBytes(item["local_bytes"])
		if !ok {
			return nil, options, manifestInvalid("manifest local_bytes must be a non-negative integer")
		}
		updated, err := manifestTimestamp(item["local_updated_at"])
		if err != nil {
			return nil, options, err
		}
		target := item["memento_path"]
		if target == nil {
			if template == nil {
				return nil, options, manifestInvalid("manifest items without memento_path require match.path_template")
			}
			mapped := name
			if alias, ok := aliases[name]; ok {
				mapped = alias
			}
			target = strings.ReplaceAll(*template, "{name}", mapped)
		}
		path, ok := target.(string)
		if !ok {
			return nil, options, manifestInvalid("manifest memento_path must be a string")
		}
		if err = validateManifestTarget(path, prefix); err != nil {
			return nil, options, err
		}
		if _, err = access.AuthorizePath(actor.Policy, path, "read"); err != nil {
			return nil, options, err
		}
		if _, err = repository.ValidateRepositoryWritePath(c.Queue.Paths.CurrentDir, path); err != nil {
			return nil, options, err
		}
		if targets[path] {
			return nil, options, manifestInvalid("duplicate manifest memento_path: " + path)
		}
		targets[path] = true
		names[name] = true
		normalized = append(normalized, manifestItem{map[string]any{"name": name, "local_path": localPath, "memento_path": path, "local_updated_at": modelTimestamp(updated), "local_body_sha256": strings.ToLower(digest), "local_bytes": size}, updated})
	}
	fields := []string{"path", "updated_at", "body_sha256", "body_bytes"}
	if includeAssets {
		fields = append(fields, "assets")
	}
	// Python calls memory_inventory, which resolves live policy a second time.
	if c.inventoryPolicy != nil {
		policy, err := c.inventoryPolicy(ctx)
		if err != nil {
			return nil, options, err
		}
		actor.Policy = policy
	}
	inventory, inventoryOptions, err := c.inventory(ctx, actor, prefix, fields, 51, nil, repo)
	if err != nil {
		return nil, options, err
	}
	entries := inventory["entries"].([]any)
	if len(entries) > 50 || inventory["next_cursor"] != nil {
		return nil, options, manifestInvalid("manifest comparison namespace exceeds 50 concepts; use a narrower path_prefix")
	}
	byPath := map[string]map[string]any{}
	onlyPaths := []string{}
	for _, value := range entries {
		entry := value.(map[string]any)
		path := entry["path"].(string)
		byPath[path] = entry
		if !targets[path] {
			onlyPaths = append(onlyPaths, path)
		}
	}
	if len(normalized)+len(onlyPaths) > 50 {
		return nil, options, manifestInvalid("manifest comparison exceeds 50 combined records; narrow the manifest or path_prefix")
	}
	matching, differing, localOnly, remoteOnly := []any{}, []any{}, []any{}, []any{}
	for _, item := range normalized {
		payload := item.payload
		remote := byPath[payload["memento_path"].(string)]
		payload["local_present"] = true
		payload["memento_present"] = remote != nil
		if remote == nil {
			payload["body_match"] = nil
			localOnly = append(localOnly, payload)
			continue
		}
		bodyMatch := payload["local_body_sha256"] == remote["body_sha256"]
		// Inventory emits already-validated, UTC-normalised model timestamps.
		remoteUpdated, _ := manifestTimestamp(remote["updated_at"])
		payload["body_match"] = bodyMatch
		payload["body_bytes_match"] = string(payload["local_bytes"].(json.Number)) == strconv.Itoa(remote["body_bytes"].(int))
		payload["memento_updated_at"] = remote["updated_at"]
		payload["memento_body_sha256"] = remote["body_sha256"]
		payload["memento_bytes"] = remote["body_bytes"]
		if includeAssets {
			for key, value := range manifestAssetSummary(remote["assets"]) {
				payload[key] = value
			}
		}
		if bodyMatch {
			matching = append(matching, payload)
		} else {
			newer := "unknown"
			if item.updated.After(remoteUpdated) {
				newer = "local"
			} else if item.updated.Before(remoteUpdated) {
				newer = "memento"
			}
			payload["likely_newer"] = newer
			differing = append(differing, payload)
		}
	}
	sort.Strings(onlyPaths)
	for _, path := range onlyPaths {
		remote := byPath[path]
		payload := map[string]any{"memento_path": path, "local_present": false, "memento_present": true, "memento_updated_at": remote["updated_at"], "memento_body_sha256": remote["body_sha256"], "memento_bytes": remote["body_bytes"]}
		if includeAssets {
			for key, value := range manifestAssetSummary(remote["assets"]) {
				payload[key] = value
			}
		}
		remoteOnly = append(remoteOnly, payload)
	}
	options.RepoRevision = inventoryOptions.RepoRevision
	return map[string]any{"path_prefix": prefix, "matching": matching, "differing": differing, "local_only": localOnly, "memento_only": remoteOnly, "counts": map[string]any{"matching": len(matching), "differing": len(differing), "local_only": len(localOnly), "memento_only": len(remoteOnly)}}, options, nil
}
func manifestAssetSummary(value any) map[string]any {
	items, _ := value.([]any)
	summaries := []any{}
	for _, raw := range items[:min(20, len(items))] {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, k := item["kind"].(string)
		version, v := item["latest_version"].(string)
		digest, d := item["latest_sha256"].(string)
		if k && v && d {
			summaries = append(summaries, map[string]any{"kind": kind, "latest_version": version, "latest_sha256": digest})
		}
	}
	return map[string]any{"asset_present": len(items) > 0, "assets": summaries, "asset_kinds_truncated": len(items) > len(summaries)}
}

// JSON-domain Python truthiness, after uMCP's annotation coercion.
func manifestTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case json.Number:
		n, ok := new(big.Rat).SetString(string(v))
		return !ok || n.Sign() != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}
