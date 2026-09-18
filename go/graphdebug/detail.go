package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var readAssetFile = os.ReadFile
var statAsset = os.Stat

type AssetSummary struct {
	AssetKind        string  `json:"asset_kind"`
	Version          string  `json:"version"`
	MetadataBytes    int64   `json:"metadata_bytes"`
	PayloadBytes     int64   `json:"payload_bytes"`
	SourceProposalID *string `json:"source_proposal_id"`
}
type ProposalSummary struct {
	ProposalID      string  `json:"proposal_id"`
	Author          string  `json:"author"`
	Status          string  `json:"status"`
	Intent          string  `json:"intent"`
	BaseRevision    string  `json:"base_revision"`
	AppliedRevision *string `json:"applied_revision"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}
type Detail struct {
	SchemaVersion    int               `json:"schema_version"`
	Revisions        Revisions         `json:"revisions"`
	Node             Node              `json:"node"`
	Preview          string            `json:"preview"`
	PreviewTruncated bool              `json:"preview_truncated"`
	Outbound         []Edge            `json:"outbound"`
	Inbound          []Edge            `json:"inbound"`
	Assets           []AssetSummary    `json:"assets"`
	Proposals        []ProposalSummary `json:"proposals"`
}

func (s *SnapshotService) Assets(id string, limit int) []AssetSummary {
	files, _ := filepath.Glob(filepath.Join(s.RepositoryRoot, ".assets", id, "*", "*.json"))
	sort.Strings(files)
	out := []AssetSummary{}
	for _, file := range files {
		raw, err := readAssetFile(file)
		if err != nil {
			continue
		}
		var payload map[string]any
		if json.Unmarshal(raw, &payload) != nil {
			continue
		}
		kind, _ := payload["asset_kind"].(string)
		if kind == "" {
			kind = filepath.Base(filepath.Dir(file))
		}
		version, _ := payload["version"].(string)
		if version == "" {
			version = strings.TrimSuffix(filepath.Base(file), ".json")
		}
		info, err := statAsset(file)
		if err != nil {
			continue
		}
		var zipBytes int64
		if zipInfo, err := statAsset(strings.TrimSuffix(file, ".json") + ".zip"); err == nil && zipInfo.Mode().IsRegular() {
			zipBytes = zipInfo.Size()
		}
		var source *string
		if value, ok := payload["source_proposal_id"].(string); ok && value != "" {
			source = &value
		}
		out = append(out, AssetSummary{kind, version, info.Size(), zipBytes, source})
		if len(out) >= limit {
			break
		}
	}
	return out
}
func (s *SnapshotService) Proposals(ctx context.Context, path string, policy *access.EffectivePolicy, limit int) ([]ProposalSummary, error) {
	db, err := s.open(ctx, s.ControlDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT proposal_id,author_principal,status,intent,base_revision,applied_revision,created_at,updated_at,patch_json FROM proposals ORDER BY created_at,proposal_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProposalSummary{}
	for rows.Next() {
		var item ProposalSummary
		var applied sql.NullString
		var patch string
		if err = rows.Scan(&item.ProposalID, &item.Author, &item.Status, &item.Intent, &item.BaseRevision, &applied, &item.CreatedAt, &item.UpdatedAt, &patch); err != nil {
			return nil, err
		}
		paths := proposalPaths(patch)
		if !proposalVisible(item.Author, paths, policy) || !containsPath(paths, path) {
			continue
		}
		item.AppliedRevision = nullableString(applied)
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
func containsPath(paths []string, path string) bool {
	for _, item := range paths {
		if item == path {
			return true
		}
	}
	return false
}
func (s *SnapshotService) Detail(ctx context.Context, id string, policy *access.EffectivePolicy, previewChars, edgeLimit, summaryLimit int) (Detail, error) {
	empty := Detail{}
	nodes, err := s.Nodes(ctx, []string{id}, 1, policy, false)
	if err != nil {
		return empty, err
	}
	if len(nodes) == 0 {
		return empty, &SnapshotError{"unknown memory"}
	}
	revisions, err := s.Revisions(ctx)
	if err != nil {
		return empty, err
	}
	candidate, err := s.repositoryPath(nodes[0].Path)
	if err != nil {
		return empty, &SnapshotError{"memory file unavailable"}
	}
	document, err := repository.ParseConceptFile(candidate)
	if err != nil {
		return empty, err
	}
	body := []rune(document.Body)
	preview := body
	if len(preview) > previewChars {
		preview = preview[:previewChars]
	}
	visible, err := s.Nodes(ctx, nil, 100000, policy, false)
	if err != nil {
		return empty, err
	}
	ids := make([]string, len(visible))
	for i, node := range visible {
		ids[i] = node.ID
	}
	source, target := id, id
	outbound, err := s.ExplicitEdges(ctx, ids, &source, nil, edgeLimit, policy)
	if err != nil {
		return empty, err
	}
	inbound, err := s.ExplicitEdges(ctx, ids, nil, &target, edgeLimit, policy)
	if err != nil {
		return empty, err
	}
	center := ScopedNodes(nodes, append(append([]Edge{}, outbound...), inbound...))[0]
	proposals, err := s.Proposals(ctx, center.Path, policy, summaryLimit)
	if err != nil {
		return empty, err
	}
	return Detail{1, revisions, center, string(preview), len(body) > len(preview), outbound, inbound, s.Assets(center.ID, summaryLimit), proposals}, nil
}
