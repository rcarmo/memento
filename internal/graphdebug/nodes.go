package graphdebug

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
)

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
type EmbeddingState struct {
	Status            string  `json:"status"`
	ModelID           *string `json:"model_id"`
	Dimensions        *int    `json:"dimensions"`
	EmbeddingRevision *string `json:"embedding_revision"`
	ModelRevision     *string `json:"model_revision"`
	UpdatedAt         *string `json:"updated_at"`
	Error             *string `json:"error"`
}
type Node struct {
	ID                   string         `json:"id"`
	Path                 string         `json:"path"`
	Title                string         `json:"title"`
	Type                 string         `json:"type"`
	Status               string         `json:"status"`
	Tags                 []string       `json:"tags"`
	Namespace            string         `json:"namespace"`
	UpdatedAt            string         `json:"updated_at"`
	UpdatedBy            *string        `json:"updated_by"`
	ProvenanceKeys       []string       `json:"provenance_keys"`
	MarkdownBytes        int64          `json:"markdown_bytes"`
	AssetBytes           int64          `json:"asset_bytes"`
	CombinedBytes        int64          `json:"combined_bytes"`
	ExplicitInDegree     int            `json:"explicit_in_degree"`
	ExplicitOutDegree    int            `json:"explicit_out_degree"`
	BrokenLinkCount      int            `json:"broken_link_count"`
	Orphan               bool           `json:"orphan"`
	ProposalCount        int            `json:"proposal_count"`
	PendingProposalCount int            `json:"pending_proposal_count"`
	Embedding            EmbeddingState `json:"embedding"`
	CoarsePosition       Position       `json:"coarse_position"`
	AnomalyIDs           []string       `json:"anomaly_ids"`
}

func namespace(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	if len(parts) == 0 {
		return "/"
	}
	return "/" + parts[0] + "/"
}
func position(id string) Position {
	digest := sha256.Sum256([]byte(id))
	angle := float64(binary.BigEndian.Uint64(digest[:8])) / math.Exp2(64) * 2 * math.Pi
	radius := .5 + float64(binary.BigEndian.Uint32(digest[8:12]))/math.Exp2(32)*.5
	z := float64(binary.BigEndian.Uint32(digest[12:16]))/math.Exp2(32)*2 - 1
	return Position{math.Cos(angle) * radius, math.Sin(angle) * radius, z}
}
func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}
func (s *SnapshotService) repositoryPath(path string) (string, error) {
	root, _ := filepath.Abs(s.RepositoryRoot)
	candidate, _ := filepath.Abs(filepath.Join(root, strings.TrimPrefix(path, "/")))
	relative, _ := filepath.Rel(root, candidate)
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", os.ErrNotExist
	}
	info, err := os.Stat(candidate)
	if err != nil || !info.Mode().IsRegular() {
		return "", os.ErrNotExist
	}
	return candidate, nil
}
func (s *SnapshotService) assetBytes(id string) int64 {
	entries, _ := filepath.Glob(filepath.Join(s.RepositoryRoot, ".assets", id, "*", "*.json"))
	sort.Strings(entries)
	var total int64
	for _, meta := range entries {
		if info, err := os.Stat(meta); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		if info, err := os.Stat(strings.TrimSuffix(meta, ".json") + ".zip"); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
	}
	return total
}
func (s *SnapshotService) filesystemFields(path, id string) (int64, *string, []string, int64) {
	candidate, err := s.repositoryPath(path)
	if err != nil {
		return 0, nil, []string{}, s.assetBytes(id)
	}
	info, _ := os.Stat(candidate) // repositoryPath already verified this file
	document, err := repository.ParseConceptFile(candidate)
	if err != nil {
		return info.Size(), nil, []string{}, s.assetBytes(id)
	}
	updated := document.Frontmatter.UpdatedBy
	unique := map[string]bool{}
	for _, ref := range document.Frontmatter.SourceRefs {
		sum := sha256.Sum256([]byte(ref))
		unique[hex.EncodeToString(sum[:])] = true
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return info.Size(), &updated, keys, s.assetBytes(id)
}
func nullableInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}
func (s *SnapshotService) Nodes(ctx context.Context, ids []string, limit int, policy *access.EffectivePolicy, includeTrash bool) ([]Node, error) {
	counts, err := s.ProposalCounts(ctx, policy)
	if err != nil {
		return nil, err
	}
	db, err := s.open(ctx, s.DerivedDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	clauses := []string{}
	args := []any{}
	if !includeTrash && ids == nil {
		clauses = append(clauses, "c.path NOT LIKE '/trash/%'")
	}
	if ids != nil {
		if len(ids) == 0 {
			return []Node{}, nil
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		clauses = append(clauses, "c.id IN ("+placeholders+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	if policy != nil {
		scope, scopeArgs := pathScopeSQL("c.path", policy)
		clauses = append(clauses, scope)
		args = append(args, scopeArgs...)
	}
	query := "SELECT c.id,c.path,c.title,c.type,c.status,c.tags_json,c.updated_at,COALESCE(g.inbound_degree,0),COALESCE(g.outbound_degree,0),COALESCE(g.broken_link_count,0),COALESCE(g.orphan_flag,0),e.status,e.model_id,e.dimensions,e.embedding_revision,e.model_revision,e.updated_at,e.error_message FROM concepts c LEFT JOIN graph_metrics g ON g.concept_id=c.id LEFT JOIN concept_embeddings e ON e.concept_id=c.id"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY c.id LIMIT ?"
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Node{}
	for rows.Next() {
		var n Node
		var tags string
		var orphan int
		var status, model, revision, modelRevision, updated, errorMessage sql.NullString
		var dimensions sql.NullInt64
		if err = rows.Scan(&n.ID, &n.Path, &n.Title, &n.Type, &n.Status, &tags, &n.UpdatedAt, &n.ExplicitInDegree, &n.ExplicitOutDegree, &n.BrokenLinkCount, &orphan, &status, &model, &dimensions, &revision, &modelRevision, &updated, &errorMessage); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(tags), &n.Tags); err != nil {
			return nil, err
		}
		n.Orphan = orphan != 0
		n.Namespace = namespace(n.Path)
		n.MarkdownBytes, n.UpdatedBy, n.ProvenanceKeys, n.AssetBytes = s.filesystemFields(n.Path, n.ID)
		n.CombinedBytes = n.MarkdownBytes + n.AssetBytes
		count := counts[n.Path]
		n.ProposalCount, n.PendingProposalCount = count.Total, count.Pending
		n.AnomalyIDs = []string{}
		n.Embedding = EmbeddingState{Status: "missing", ModelID: nullableString(model), Dimensions: nullableInt(dimensions), EmbeddingRevision: nullableString(revision), ModelRevision: nullableString(modelRevision), UpdatedAt: nullableString(updated), Error: nullableString(errorMessage)}
		if status.Valid {
			n.Embedding.Status = status.String
		}
		n.CoarsePosition = position(n.ID)
		out = append(out, n)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
