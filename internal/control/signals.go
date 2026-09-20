package control

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/rcarmo/memento/internal/pyjson"
	"strings"
	"time"
)

type DreamSignal struct {
	SignalID                                                                                               string   `json:"signal_id"`
	SignalType                                                                                             string   `json:"signal_type"`
	EntityRefs                                                                                             []string `json:"entity_refs"`
	Severity, RepoRevision, DedupeKey, Status, EvidenceHash, EvidenceJSON, FirstDetectedAt, LastDetectedAt string
	ResolvedRevision                                                                                       *string
}
type DetectedSignal struct {
	SignalType          string
	EntityRefs          []string
	Severity, DedupeKey string
	Evidence            map[string]any
}

func (d DetectedSignal) evidence() (string, string, error) {
	raw, err := pyjson.DumpsCompact(d.Evidence)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(raw))
	pretty, _ := pyjson.Dumps(d.Evidence)
	return hex.EncodeToString(sum[:]), pretty, nil
}

type Signals struct {
	DB   *sql.DB
	Now  func() time.Time
	UUID func() string
}

func (s Signals) now() string {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	return now.UTC().Truncate(time.Second).Format(time.RFC3339)
}
func (s Signals) id() string {
	if s.UUID != nil {
		return s.UUID()
	}
	return uuid.NewString()
}

const signalColumns = "signal_id,signal_type,entity_refs_json,severity,repo_revision,dedupe_key,status,evidence_hash,evidence_json,first_detected_at,last_detected_at,resolved_revision"

func (s Signals) List(ctx context.Context) ([]DreamSignal, error) {
	return s.list(ctx, "SELECT "+signalColumns+" FROM dream_signals ORDER BY signal_type,dedupe_key")
}
func (s Signals) Actionable(ctx context.Context) ([]DreamSignal, error) {
	return s.list(ctx, "SELECT "+signalColumns+" FROM dream_signals WHERE status IN ('open','acknowledged') ORDER BY signal_type,dedupe_key")
}
func (s Signals) list(ctx context.Context, query string) ([]DreamSignal, error) {
	rows, err := s.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []DreamSignal{}
	for rows.Next() {
		item, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s Signals) Upsert(ctx context.Context, revision string, detections []DetectedSignal) ([]DreamSignal, error) {
	existing, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	byKey := map[string]DreamSignal{}
	for _, item := range existing {
		byKey[item.DedupeKey] = item
	}
	seen := map[string]bool{}
	now := s.now()
	err = WithTransaction(ctx, s.DB, func(tx *sql.Tx) error {
		for _, detection := range detections {
			seen[detection.DedupeKey] = true
			hash, evidence, encodeErr := detection.evidence()
			if encodeErr != nil {
				return encodeErr
			}
			refs, _ := pyjson.Dumps(detection.EntityRefs)
			current, ok := byKey[detection.DedupeKey]
			if !ok {
				if _, e := tx.ExecContext(ctx, "INSERT INTO dream_signals(signal_id,signal_type,entity_refs_json,severity,repo_revision,dedupe_key,status,evidence_hash,evidence_json,first_detected_at,last_detected_at,resolved_revision) VALUES(?,?,?,?,?,?,'open',?,?,?,?,NULL)", s.id(), detection.SignalType, refs, detection.Severity, revision, detection.DedupeKey, hash, evidence, now, now); e != nil {
					return e
				}
				continue
			}
			status, resolved := current.Status, current.ResolvedRevision
			if (status == "resolved" || status == "ignored") && current.EvidenceHash != hash {
				status = "open"
				resolved = nil
			}
			if _, e := tx.ExecContext(ctx, "UPDATE dream_signals SET signal_type=?,entity_refs_json=?,severity=?,repo_revision=?,status=?,evidence_hash=?,evidence_json=?,last_detected_at=?,resolved_revision=? WHERE dedupe_key=?", detection.SignalType, refs, detection.Severity, revision, status, hash, evidence, now, resolved, detection.DedupeKey); e != nil {
				return e
			}
		}
		for _, item := range existing {
			if !seen[item.DedupeKey] && (item.Status == "open" || item.Status == "acknowledged" || item.Status == "proposed") {
				if _, e := tx.ExecContext(ctx, "UPDATE dream_signals SET status='resolved',resolved_revision=? WHERE dedupe_key=?", revision, item.DedupeKey); e != nil {
					return e
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.List(ctx)
}
func (s Signals) MarkStatus(ctx context.Context, keys []string, status string) error {
	if len(keys) == 0 {
		return nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	args := []any{status}
	for _, key := range keys {
		args = append(args, key)
	}
	_, err := s.DB.ExecContext(ctx, "UPDATE dream_signals SET status=? WHERE dedupe_key IN ("+marks+")", args...)
	return err
}
func SetServiceState(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(ctx, "INSERT INTO service_state(key,value,updated_at) VALUES(?,?,datetime('now')) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=datetime('now')", key, value)
	return err
}
func GetServiceState(ctx context.Context, db *sql.DB, key string) (*string, error) {
	var value string
	err := db.QueryRowContext(ctx, "SELECT value FROM service_state WHERE key=?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}
func scanSignal(row rowScanner) (DreamSignal, error) {
	var item DreamSignal
	var refs string
	if err := row.Scan(&item.SignalID, &item.SignalType, &refs, &item.Severity, &item.RepoRevision, &item.DedupeKey, &item.Status, &item.EvidenceHash, &item.EvidenceJSON, &item.FirstDetectedAt, &item.LastDetectedAt, &item.ResolvedRevision); err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(refs), &item.EntityRefs); err != nil {
		return item, err
	}
	return item, nil
}
