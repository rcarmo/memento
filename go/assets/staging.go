package assets

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rcarmo/memento/go/control"
)

const (
	StagingTTL      = 24 * time.Hour
	UploadTicketTTL = time.Hour
)

type StagedAssetError struct{ Message string }

func (e *StagedAssetError) Error() string { return e.Message }
func stagingError(message string) error   { return &StagedAssetError{message} }
func blank(s string) bool {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }) == ""
}
func timestamp(t time.Time) string               { return t.UTC().Truncate(time.Second).Format(time.RFC3339) }
func parseTimestamp(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

type UploadTicket struct {
	Principal      string  `json:"principal"`
	IdempotencyKey string  `json:"idempotency_key"`
	AssetKind      string  `json:"asset_kind"`
	Version        string  `json:"version"`
	ExpiresAt      string  `json:"expires_at"`
	StagedAssetID  *string `json:"staged_asset_id"`
	ConsumedAt     *string `json:"consumed_at"`
}

func (t UploadTicket) State(now time.Time) (string, error) {
	if t.StagedAssetID != nil {
		return "uploaded", nil
	}
	expires, err := parseTimestamp(t.ExpiresAt)
	if err != nil {
		return "", err
	}
	if !expires.After(now) {
		return "expired", nil
	}
	return "pending", nil
}

type StagedAsset struct {
	StagedAssetID string   `json:"staged_asset_id"`
	Principal     string   `json:"principal"`
	AssetKind     string   `json:"asset_kind"`
	Version       string   `json:"version"`
	MediaType     string   `json:"media_type"`
	SHA256        string   `json:"sha256"`
	BlobBytes     []byte   `json:"blob_bytes"`
	Manifest      Manifest `json:"manifest"`
	State         string   `json:"state"`
	ProposalID    *string  `json:"proposal_id"`
	CreatedAt     string   `json:"created_at"`
	ExpiresAt     string   `json:"expires_at"`
	ConsumedAt    *string  `json:"consumed_at"`
}

func (s StagedAsset) PublicPayload() map[string]any {
	return map[string]any{"staged_asset_id": s.StagedAssetID, "asset_kind": s.AssetKind, "version": s.Version, "media_type": s.MediaType, "sha256": s.SHA256, "manifest": s.Manifest, "state": s.State, "proposal_id": s.ProposalID, "created_at": s.CreatedAt, "expires_at": s.ExpiresAt, "consumed_at": s.ConsumedAt}
}

type StagingStore struct {
	DB       *sql.DB
	Now      func() time.Time
	Random   io.Reader
	randomMu sync.Mutex
}

func (s *StagingStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}
func (s *StagingStore) random(n int) ([]byte, error) {
	s.randomMu.Lock()
	defer s.randomMu.Unlock()
	source := s.Random
	if source == nil {
		source = rand.Reader
	}
	raw := make([]byte, n)
	_, err := io.ReadFull(source, raw)
	return raw, err
}
func (s *StagingStore) id() (string, error) {
	raw, err := s.random(16)
	if err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 15) | 64
	raw[8] = (raw[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[:4], raw[4:6], raw[6:8], raw[8:10], raw[10:]), nil
}
func tokenDigest(token string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(token))) }

const ticketColumns = "principal,idempotency_key,asset_kind,version,expires_at,staged_asset_id,consumed_at"
const stagedColumns = "staged_asset_id,principal,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,state,proposal_id,created_at,expires_at,consumed_at"

type scanner interface{ Scan(...any) error }
type stageExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func nullableEmpty(value **string) {
	if *value != nil && **value == "" {
		*value = nil
	}
}
func scanTicket(row scanner) (UploadTicket, error) {
	var t UploadTicket
	err := row.Scan(&t.Principal, &t.IdempotencyKey, &t.AssetKind, &t.Version, &t.ExpiresAt, &t.StagedAssetID, &t.ConsumedAt)
	nullableEmpty(&t.StagedAssetID)
	nullableEmpty(&t.ConsumedAt)
	return t, err
}
func scanStaged(row scanner, now time.Time) (StagedAsset, error) {
	var s StagedAsset
	var raw string
	if err := row.Scan(&s.StagedAssetID, &s.Principal, &s.AssetKind, &s.Version, &s.MediaType, &s.SHA256, &s.BlobBytes, &raw, &s.State, &s.ProposalID, &s.CreatedAt, &s.ExpiresAt, &s.ConsumedAt); err != nil {
		return s, err
	}
	if s.BlobBytes == nil {
		s.BlobBytes = []byte{}
	}
	if s.State == "ready" {
		expires, err := parseTimestamp(s.ExpiresAt)
		if err != nil {
			return s, err
		}
		if !expires.After(now) {
			s.State = "expired"
		}
	}
	manifest, err := ParseManifest(raw)
	if err != nil {
		return s, err
	}
	s.Manifest = manifest
	nullableEmpty(&s.ProposalID)
	nullableEmpty(&s.ConsumedAt)
	return s, nil
}
func ParseManifest(raw string) (Manifest, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return Manifest{}, err
	}
	for _, key := range []string{"entries", "sha256", "total_uncompressed_bytes", "file_count"} {
		if value, ok := fields[key]; !ok || string(value) == "null" {
			return Manifest{}, invalid("invalid asset manifest")
		}
	}
	var manifest Manifest
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&manifest); err != nil {
		return manifest, err
	}
	if manifest.Entries == nil || len(manifest.SHA256) != 64 || manifest.FileCount < 0 {
		return manifest, invalid("invalid asset manifest")
	}
	var entries []map[string]json.RawMessage
	_ = json.Unmarshal(fields["entries"], &entries)
	for _, entry := range entries {
		for _, key := range []string{"path", "size", "media_type", "sha256"} {
			if value, ok := entry[key]; !ok || string(value) == "null" {
				return manifest, invalid("invalid asset manifest entry")
			}
		}
	}
	for _, entry := range manifest.Entries {
		if entry.Path == "" || entry.MediaType == "" || len(entry.SHA256) != 64 {
			return manifest, invalid("invalid asset manifest entry")
		}
	}
	return manifest, nil
}
func manifestJSON(m Manifest) string {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	// Manifest contains only strings, non-negative integers and slices;
	// encoding to bytes.Buffer has no dynamic values or I/O failure path.
	_ = enc.Encode(m)
	return strings.TrimSuffix(out.String(), "\n")
}
func (s *StagingStore) BeginUpload(ctx context.Context, principal, key, kind, version string) (UploadTicket, string, error) {
	if blank(key) {
		return UploadTicket{}, "", stagingError("idempotency_key is required")
	}
	existing, err := scanTicket(s.DB.QueryRowContext(ctx, "SELECT "+ticketColumns+" FROM asset_upload_tickets WHERE principal=? AND idempotency_key=?", principal, key))
	if err == nil {
		if existing.AssetKind != kind || existing.Version != version {
			return UploadTicket{}, "", stagingError("idempotency key was already used for a different upload")
		}
		state, err := existing.State(s.now())
		if err != nil {
			return UploadTicket{}, "", err
		}
		return UploadTicket{}, "", stagingError("upload ticket already issued: " + state)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return UploadTicket{}, "", err
	}
	if _, err = ParseStableSemver(version); err != nil {
		return UploadTicket{}, "", err
	}
	if err = ValidateKind(kind); err != nil {
		return UploadTicket{}, "", err
	}
	now := s.now()
	random, err := s.random(32)
	if err != nil {
		return UploadTicket{}, "", err
	}
	token := "memento_upload_" + base64.RawURLEncoding.EncodeToString(random)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO asset_upload_tickets(token_digest,principal,idempotency_key,asset_kind,version,created_at,expires_at,staged_asset_id,consumed_at) VALUES(?,?,?,?,?,?,?,NULL,NULL)`, tokenDigest(token), principal, key, kind, version, timestamp(now), timestamp(now.Add(UploadTicketTTL)))
	if err != nil {
		return UploadTicket{}, "", err
	}
	ticket, err := s.TicketStatus(ctx, principal, key)
	if err != nil {
		return UploadTicket{}, "", err
	}
	return ticket, token, nil
}
func (s *StagingStore) TicketStatus(ctx context.Context, principal, key string) (UploadTicket, error) {
	ticket, err := scanTicket(s.DB.QueryRowContext(ctx, "SELECT "+ticketColumns+" FROM asset_upload_tickets WHERE principal=? AND idempotency_key=?", principal, key))
	if errors.Is(err, sql.ErrNoRows) {
		return UploadTicket{}, stagingError("upload ticket not found")
	}
	return ticket, err
}
func (s *StagingStore) Expire(ctx context.Context) (int64, error) {
	result, err := s.DB.ExecContext(ctx, "UPDATE staged_assets SET state='expired',blob_bytes=X'' WHERE state='ready' AND expires_at<=?", timestamp(s.now()))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
func (s *StagingStore) Get(ctx context.Context, principal, id string, requireReady bool) (StagedAsset, error) {
	if _, err := s.Expire(ctx); err != nil {
		return StagedAsset{}, err
	}
	staged, err := scanStaged(s.DB.QueryRowContext(ctx, "SELECT "+stagedColumns+" FROM staged_assets WHERE staged_asset_id=? AND principal=?", id, principal), s.now())
	if errors.Is(err, sql.ErrNoRows) {
		return StagedAsset{}, stagingError("staged asset not found")
	}
	if err != nil {
		return StagedAsset{}, err
	}
	if requireReady && staged.State != "ready" {
		return StagedAsset{}, stagingError("staged asset is not ready: " + staged.State)
	}
	return staged, nil
}
func (s *StagingStore) Put(ctx context.Context, principal, key, kind, version string, raw []byte) (StagedAsset, bool, error) {
	if blank(key) {
		return StagedAsset{}, false, stagingError("Idempotency-Key header is required")
	}
	if _, err := s.Expire(ctx); err != nil {
		return StagedAsset{}, false, err
	}
	return s.put(ctx, s.DB, principal, key, kind, version, raw)
}

// put assumes expiry already ran outside any surrounding transaction. The
// source's inner expire() commits those side effects even if insertion fails.
func (s *StagingStore) put(ctx context.Context, db stageExecutor, principal, key, kind, version string, raw []byte) (StagedAsset, bool, error) {
	existing, err := scanStaged(db.QueryRowContext(ctx, "SELECT "+stagedColumns+" FROM staged_assets WHERE principal=? AND idempotency_key=?", principal, key), s.now())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return StagedAsset{}, false, err
	}
	pack, validationErr := ValidateAssetPack(kind, version, raw, "", "")
	if validationErr != nil {
		return StagedAsset{}, false, validationErr
	}
	if err == nil {
		if existing.AssetKind != kind || existing.Version != version || existing.SHA256 != pack.Manifest.SHA256 {
			return StagedAsset{}, false, stagingError("idempotency key was already used for a different asset")
		}
		return existing, true, nil
	}
	id, err := s.id()
	if err != nil {
		return StagedAsset{}, false, err
	}
	now := s.now()
	staged := StagedAsset{StagedAssetID: id, Principal: principal, AssetKind: kind, Version: version, MediaType: "application/zip", SHA256: pack.Manifest.SHA256, BlobBytes: append([]byte{}, raw...), Manifest: pack.Manifest, State: "ready", CreatedAt: timestamp(now), ExpiresAt: timestamp(now.Add(StagingTTL))}
	manifest := manifestJSON(staged.Manifest)
	_, err = db.ExecContext(ctx, `INSERT INTO staged_assets(staged_asset_id,principal,idempotency_key,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,state,proposal_id,created_at,expires_at,consumed_at) VALUES(?,?,?,?,?,?,?,?,?,?,NULL,?,?,NULL)`, id, principal, key, kind, version, staged.MediaType, staged.SHA256, raw, manifest, staged.State, staged.CreatedAt, staged.ExpiresAt)
	if err != nil {
		return StagedAsset{}, false, err
	}
	return staged, false, nil
}
func (s *StagingStore) PutWithTicket(ctx context.Context, token string, raw []byte) (StagedAsset, bool, error) {
	digest := tokenDigest(token)
	ticket, err := scanTicket(s.DB.QueryRowContext(ctx, "SELECT "+ticketColumns+" FROM asset_upload_tickets WHERE token_digest=?", digest))
	if errors.Is(err, sql.ErrNoRows) {
		return StagedAsset{}, false, stagingError("upload ticket not found")
	}
	if err != nil {
		return StagedAsset{}, false, err
	}
	state, err := ticket.State(s.now())
	if err != nil {
		return StagedAsset{}, false, err
	}
	if state == "expired" {
		return StagedAsset{}, false, stagingError("upload ticket expired")
	}
	if ticket.StagedAssetID != nil {
		staged, err := s.Get(ctx, ticket.Principal, *ticket.StagedAssetID, false)
		if err != nil {
			return StagedAsset{}, false, err
		}
		if staged.SHA256 != fmt.Sprintf("%x", sha256.Sum256(raw)) {
			return StagedAsset{}, false, stagingError("upload ticket was already used for a different asset")
		}
		return staged, true, nil
	}
	if _, err = s.Expire(ctx); err != nil {
		return StagedAsset{}, false, err
	}
	var staged StagedAsset
	var replayed bool
	err = control.WithTransaction(ctx, s.DB, func(tx *sql.Tx) error {
		var err error
		staged, replayed, err = s.put(ctx, tx, ticket.Principal, "ticket:"+ticket.IdempotencyKey, ticket.AssetKind, ticket.Version, raw)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, "UPDATE asset_upload_tickets SET staged_asset_id=?,consumed_at=? WHERE token_digest=? AND staged_asset_id IS NULL", staged.StagedAssetID, timestamp(s.now()), digest)
		if err != nil {
			return err
		}
		return consumedOne(result, "upload ticket could not be consumed")
	})
	if err != nil {
		return StagedAsset{}, false, err
	}
	return staged, replayed, nil
}
func consumedOne(result sql.Result, message string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return stagingError(message)
	}
	return nil
}
func (s *StagingStore) Consume(ctx context.Context, principal string, ids []string, proposal string) error {
	if len(ids) == 0 {
		return nil
	}
	return control.WithTransaction(ctx, s.DB, func(tx *sql.Tx) error { return s.ConsumeInTx(ctx, tx, principal, ids, proposal) })
}
func (s *StagingStore) ConsumeInTx(ctx context.Context, tx *sql.Tx, principal string, ids []string, proposal string) error {
	now := timestamp(s.now())
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, "UPDATE staged_assets SET state='consumed',proposal_id=?,consumed_at=?,blob_bytes=X'' WHERE staged_asset_id=? AND principal=? AND state='ready'", proposal, now, id, principal)
		if err != nil {
			return err
		}
		if err = consumedOne(result, "staged asset could not be consumed"); err != nil {
			return err
		}
	}
	return nil
}
