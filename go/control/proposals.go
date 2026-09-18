package control

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcarmo/memento/go/internal/pyjson"
)

type ProposalStatus string

const (
	Draft       ProposalStatus = "draft"
	Submitted   ProposalStatus = "submitted"
	Approved    ProposalStatus = "approved"
	Rejected    ProposalStatus = "rejected"
	Applied     ProposalStatus = "applied"
	Stale       ProposalStatus = "stale"
	NeedsRebase ProposalStatus = "needs_rebase"
	Conflicted  ProposalStatus = "conflicted"
	Expired     ProposalStatus = "expired"
)

func UnresolvedProposalStatuses() []ProposalStatus {
	return []ProposalStatus{Approved, Conflicted, Draft, NeedsRebase, Stale, Submitted}
}
func validProposalStatus(status ProposalStatus) bool {
	switch status {
	case Draft, Submitted, Approved, Rejected, Applied, Stale, NeedsRebase, Conflicted, Expired:
		return true
	}
	return false
}

type ProposalRecord struct {
	ProposalID         string         `json:"proposal_id"`
	AuthorPrincipal    string         `json:"author_principal"`
	ClientInstanceID   *string        `json:"client_instance_id"`
	BaseRevision       string         `json:"base_revision"`
	Intent             string         `json:"intent"`
	Rationale          *string        `json:"rationale"`
	PatchJSON          string         `json:"patch_json"`
	PatchHash          string         `json:"patch_hash"`
	Status             ProposalStatus `json:"status"`
	ReviewedBy         *string        `json:"reviewed_by"`
	ReviewComment      *string        `json:"review_comment"`
	AppliedOperationID *string        `json:"applied_operation_id"`
	AppliedRevision    *string        `json:"applied_revision"`
	CreatedAt          string         `json:"created_at"`
	UpdatedAt          string         `json:"updated_at"`
	ExpiresAt          *string        `json:"expires_at"`
}

func (r ProposalRecord) Patch() (map[string]any, error) {
	return objectJSON(r.PatchJSON, "proposal patch must decode to an object")
}

type ProposalAssetInput struct {
	AssetID      string `json:"asset_id"`
	ConceptPath  string `json:"concept_path"`
	AssetKind    string `json:"asset_kind"`
	Version      string `json:"version"`
	MediaType    string `json:"media_type"`
	SHA256       string `json:"sha256"`
	BlobBytes    []byte `json:"blob_bytes"`
	ManifestJSON string `json:"manifest_json"`
}
type ProposalAssetRecord struct {
	ProposalAssetInput
	ProposalID string `json:"proposal_id"`
	CreatedAt  string `json:"created_at"`
}

func (r ProposalAssetRecord) Manifest() (map[string]any, error) {
	return objectJSON(r.ManifestJSON, "proposal asset manifest must decode to an object")
}
func objectJSON(raw, message string) (map[string]any, error) {
	v, err := pyjson.Parse(raw)
	if err != nil {
		return nil, err
	}
	value, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New(message)
	}
	return value, nil
}

type ProposalRequest struct {
	ProposalID, AuthorPrincipal, BaseRevision, Intent string
	ClientInstanceID, Rationale                       *string
	Patch                                             map[string]any
	ExpiresInDays                                     *int
	Assets                                            []ProposalAssetInput
}
type Proposals struct {
	DB  *sql.DB
	Now func() time.Time
}

func (p Proposals) now() time.Time {
	if p.Now != nil {
		return p.Now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}
func (p Proposals) Create(ctx context.Context, request ProposalRequest) (ProposalRecord, error) {
	var record ProposalRecord
	err := WithTransaction(ctx, p.DB, func(tx *sql.Tx) error { var err error; record, err = p.CreateInTx(ctx, tx, request); return err })
	if err != nil {
		return ProposalRecord{}, err
	}
	return record, nil
}

// CreateInTx is the explicit manage_transaction=False equivalent. Callers own
// commit/rollback, including atomic combination with access/idempotency events.
func (p Proposals) CreateInTx(ctx context.Context, tx *sql.Tx, request ProposalRequest) (ProposalRecord, error) {
	raw, err := pyjson.Dumps(request.Patch)
	if err != nil {
		return ProposalRecord{}, err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	now := p.now().Format(time.RFC3339)
	days := 30
	if request.ExpiresInDays != nil {
		days = *request.ExpiresInDays
	}
	if days < -3652058 || days > 3652058 {
		return ProposalRecord{}, errors.New("proposal expiry is out of range")
	}
	expiry := p.now().AddDate(0, 0, days)
	if expiry.Year() < 1 || expiry.Year() > 9999 {
		return ProposalRecord{}, errors.New("proposal expiry is out of range")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO proposals(proposal_id,author_principal,client_instance_id,base_revision,intent,rationale,patch_json,patch_hash,status,created_at,updated_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, request.ProposalID, request.AuthorPrincipal, request.ClientInstanceID, request.BaseRevision, request.Intent, request.Rationale, raw, hash, Submitted, now, now, expiry.Format(time.RFC3339))
	if err != nil {
		return ProposalRecord{}, err
	}
	for _, asset := range request.Assets {
		blob := asset.BlobBytes
		if blob == nil {
			blob = []byte{}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, request.ProposalID, asset.AssetID, asset.ConceptPath, asset.AssetKind, asset.Version, asset.MediaType, asset.SHA256, blob, asset.ManifestJSON, now)
		if err != nil {
			return ProposalRecord{}, err
		}
	}
	return proposalByID(ctx, tx, request.ProposalID)
}

const proposalColumns = "proposal_id,author_principal,client_instance_id,base_revision,intent,rationale,patch_json,patch_hash,status,reviewed_by,review_comment,applied_operation_id,applied_revision,created_at,updated_at,expires_at"
const proposalAssetColumns = "proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at"

type ProposalNotFoundError struct{ ProposalID, AssetID string }

func (e *ProposalNotFoundError) Error() string {
	if e.AssetID != "" {
		return "unknown proposal asset: " + e.ProposalID + "/" + e.AssetID
	}
	return "unknown proposal: " + e.ProposalID
}
func (p Proposals) Get(ctx context.Context, id string) (ProposalRecord, error) {
	return proposalByID(ctx, p.DB, id)
}

// GetInTx reads the updated proposal before the caller commits its review.
func (p Proposals) GetInTx(ctx context.Context, tx *sql.Tx, id string) (ProposalRecord, error) {
	return proposalByID(ctx, tx, id)
}
func proposalByID(ctx context.Context, db sqlExecutor, id string) (ProposalRecord, error) {
	record, err := scanProposal(db.QueryRowContext(ctx, "SELECT "+proposalColumns+" FROM proposals WHERE proposal_id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return ProposalRecord{}, &ProposalNotFoundError{ProposalID: id}
	}
	return record, err
}
func scanProposal(row rowScanner) (ProposalRecord, error) {
	var r ProposalRecord
	err := row.Scan(&r.ProposalID, &r.AuthorPrincipal, &r.ClientInstanceID, &r.BaseRevision, &r.Intent, &r.Rationale, &r.PatchJSON, &r.PatchHash, &r.Status, &r.ReviewedBy, &r.ReviewComment, &r.AppliedOperationID, &r.AppliedRevision, &r.CreatedAt, &r.UpdatedAt, &r.ExpiresAt)
	if err != nil {
		return ProposalRecord{}, err
	}
	if !validProposalStatus(r.Status) {
		return ProposalRecord{}, fmt.Errorf("invalid proposal status: %s", r.Status)
	}
	return r, nil
}
func (p Proposals) GetAsset(ctx context.Context, id, assetID string) (ProposalAssetRecord, error) {
	record, err := scanProposalAsset(p.DB.QueryRowContext(ctx, "SELECT "+proposalAssetColumns+" FROM proposal_assets WHERE proposal_id=? AND asset_id=?", id, assetID))
	if errors.Is(err, sql.ErrNoRows) {
		return ProposalAssetRecord{}, &ProposalNotFoundError{ProposalID: id, AssetID: assetID}
	}
	return record, err
}
func scanProposalAsset(row rowScanner) (ProposalAssetRecord, error) {
	var r ProposalAssetRecord
	err := row.Scan(&r.ProposalID, &r.AssetID, &r.ConceptPath, &r.AssetKind, &r.Version, &r.MediaType, &r.SHA256, &r.BlobBytes, &r.ManifestJSON, &r.CreatedAt)
	return r, err
}

type ProposalQuery struct {
	Status               *ProposalStatus
	AuthorPrincipal      *string
	Unresolved           bool
	CurrentRevision, Now *string
	Limit                *int
	Cursor               *string
}

func (p Proposals) List(ctx context.Context, q ProposalQuery) ([]ProposalRecord, error) {
	conditions := []string{}
	args := []any{}
	if q.Status != nil {
		if q.CurrentRevision != nil && q.Now != nil {
			conditions = append(conditions, "(CASE WHEN expires_at IS NOT NULL AND expires_at < ? AND status != 'applied' THEN 'expired' ELSE status END) = ?")
			args = append(args, *q.Now, *q.Status)
		} else {
			conditions = append(conditions, "status = ?")
			args = append(args, *q.Status)
		}
	}
	if q.Unresolved {
		values := UnresolvedProposalStatuses()
		marks := []string{}
		for _, s := range values {
			marks = append(marks, "?")
			args = append(args, s)
		}
		conditions = append(conditions, "status IN ("+strings.Join(marks, ",")+")")
	}
	if q.Cursor != nil {
		var created, id string
		err := p.DB.QueryRowContext(ctx, "SELECT created_at,proposal_id FROM proposals WHERE proposal_id=?", *q.Cursor).Scan(&created, &id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid proposal list cursor")
		}
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, "(created_at,proposal_id) > (?,?)")
		args = append(args, created, id)
	}
	if q.AuthorPrincipal != nil {
		conditions = append(conditions, "author_principal = ?")
		args = append(args, *q.AuthorPrincipal)
	}
	query := "SELECT " + proposalColumns + " FROM proposals" + whereClause(conditions) + " ORDER BY created_at, proposal_id"
	if q.Limit != nil {
		if *q.Limit < 1 {
			return nil, errors.New("proposal query limit must be positive")
		}
		query += " LIMIT ?"
		args = append(args, *q.Limit)
	}
	rows, err := p.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProposals(rows)
}
func whereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}
func scanProposals(rows operationRows) ([]ProposalRecord, error) {
	out := []ProposalRecord{}
	for rows.Next() {
		r, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type ProposalAssetQuery struct{ ProposalID, ConceptPath, AssetKind *string }

func (p Proposals) ListAssets(ctx context.Context, q ProposalAssetQuery) ([]ProposalAssetRecord, error) {
	return listProposalAssets(ctx, p.DB, q)
}
func (p Proposals) ListAssetsInTx(ctx context.Context, tx *sql.Tx, q ProposalAssetQuery) ([]ProposalAssetRecord, error) {
	return listProposalAssets(ctx, tx, q)
}

type proposalAssetQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func listProposalAssets(ctx context.Context, db proposalAssetQuerier, q ProposalAssetQuery) ([]ProposalAssetRecord, error) {
	conditions := []string{}
	args := []any{}
	for _, field := range []struct {
		name  string
		value *string
	}{{"proposal_id", q.ProposalID}, {"concept_path", q.ConceptPath}, {"asset_kind", q.AssetKind}} {
		if field.value != nil {
			conditions = append(conditions, field.name+" = ?")
			args = append(args, *field.value)
		}
	}
	rows, err := db.QueryContext(ctx, "SELECT "+proposalAssetColumns+" FROM proposal_assets"+whereClause(conditions)+" ORDER BY created_at, proposal_id, asset_id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProposalAssets(rows)
}
func scanProposalAssets(rows operationRows) ([]ProposalAssetRecord, error) {
	out := []ProposalAssetRecord{}
	for rows.Next() {
		r, err := scanProposalAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateStatus is the low-level blind update, not a review/apply policy check.
// Omitting optional review/application fields clears their prior values.
type ProposalStatusUpdate struct {
	Status                                                         ProposalStatus
	ReviewedBy, ReviewComment, AppliedOperationID, AppliedRevision *string
}

func (p Proposals) UpdateStatus(ctx context.Context, id string, update ProposalStatusUpdate) (ProposalRecord, error) {
	if !validProposalStatus(update.Status) {
		return ProposalRecord{}, fmt.Errorf("invalid proposal status: %s", update.Status)
	}
	current, err := p.Get(ctx, id)
	if err != nil {
		return ProposalRecord{}, err
	}
	assignments := []string{"status=?", "updated_at=?"}
	args := []any{update.Status, p.now().Format(time.RFC3339)}
	for _, field := range []struct {
		name       string
		value, old *string
	}{{"reviewed_by", update.ReviewedBy, current.ReviewedBy}, {"review_comment", update.ReviewComment, current.ReviewComment}, {"applied_operation_id", update.AppliedOperationID, current.AppliedOperationID}, {"applied_revision", update.AppliedRevision, current.AppliedRevision}} {
		if field.value != nil || field.old != nil {
			assignments = append(assignments, field.name+"=?")
			args = append(args, field.value)
		}
	}
	args = append(args, id)
	if _, err = p.DB.ExecContext(ctx, "UPDATE proposals SET "+strings.Join(assignments, ",")+" WHERE proposal_id=?", args...); err != nil {
		return ProposalRecord{}, err
	}
	return p.Get(ctx, id)
}
