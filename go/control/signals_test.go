package control

import (
	"context"
	"database/sql"
	"errors"
	_ "modernc.org/sqlite"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func signalStore(t *testing.T) (*sql.DB, Signals) {
	t.Helper()
	db, err := Connect(context.Background(), filepath.Join(t.TempDir(), "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := time.Date(2026, 7, 17, 13, 0, 0, 0, time.UTC)
	ids := 0
	return db, Signals{DB: db, Now: func() time.Time { return now }, UUID: func() string { ids++; return map[int]string{1: "id-one", 2: "id-two"}[ids] }}
}
func TestSignalsLifecycle(t *testing.T) {
	ctx := context.Background()
	db, s := signalStore(t)
	detections := []DetectedSignal{{"orphan", []string{"b"}, "low", "b", map[string]any{"z": 1, "a": "x"}}, {"broken_link", []string{"a"}, "high", "a", map[string]any{"path": "/missing"}}}
	items, err := s.Upsert(ctx, "r1", detections)
	if err != nil || len(items) != 2 || items[0].SignalType != "broken_link" || items[1].SignalType != "orphan" {
		t.Fatal(items, err)
	}
	if items[1].EvidenceJSON != `{"a": "x", "z": 1}` || items[1].EvidenceHash != "8d6a75ac86d8b51bb56acfbb96108ed81474aa3504c317f77c0c576bde387cd3" {
		t.Fatal(items[1])
	}
	if err = s.MarkStatus(ctx, []string{"b"}, "ignored"); err != nil {
		t.Fatal(err)
	}
	items, err = s.Upsert(ctx, "r2", detections)
	if err != nil || items[1].Status != "ignored" {
		t.Fatal(items, err)
	}
	detections[0].Evidence["z"] = 2
	items, err = s.Upsert(ctx, "r3", detections[:1])
	if err != nil || items[1].Status != "open" || items[1].ResolvedRevision != nil || items[0].Status != "resolved" || *items[0].ResolvedRevision != "r3" {
		t.Fatal(items, err)
	}
	actionable, err := s.Actionable(ctx)
	if err != nil || len(actionable) != 1 || actionable[0].DedupeKey != "b" {
		t.Fatal(actionable, err)
	}
	if err = s.MarkStatus(ctx, nil, "x"); err != nil {
		t.Fatal(err)
	}
	if err = SetServiceState(ctx, db, "last", "r1"); err != nil {
		t.Fatal(err)
	}
	if err = SetServiceState(ctx, db, "last", "r2"); err != nil {
		t.Fatal(err)
	}
	value, err := GetServiceState(ctx, db, "last")
	if err != nil || value == nil || *value != "r2" {
		t.Fatal(value, err)
	}
	value, err = GetServiceState(ctx, db, "missing")
	if err != nil || value != nil {
		t.Fatal(value, err)
	}
}

type failingSignalRow struct{ err error }

func (r failingSignalRow) Scan(...any) error { return r.err }
func TestSignalScanFailure(t *testing.T) {
	boom := errors.New("boom")
	if _, err := scanSignal(failingSignalRow{boom}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
func TestSignalEvidenceFailure(t *testing.T) {
	signal := DetectedSignal{Evidence: map[string]any{"bad": make(chan int)}}
	if _, _, err := signal.evidence(); err == nil {
		t.Fatal("evidence")
	}
	defaults := Signals{}
	if defaults.id() == "" {
		t.Fatal("uuid")
	}
	_, store := signalStore(t)
	if _, err := store.Upsert(context.Background(), "r", []DetectedSignal{signal}); err == nil {
		t.Fatal("upsert evidence")
	}
	if _, err := store.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if defaults.id() == "" {
		t.Fatal("uuid")
	}
}
func TestSignalsDefaultsAndFailures(t *testing.T) {
	ctx := context.Background()
	db, s := signalStore(t)
	defaults := Signals{DB: db}
	items, err := defaults.Upsert(ctx, "r", []DetectedSignal{{"x", []string{}, "low", "x", map[string]any{}}})
	if err != nil || items[0].SignalID == "" || items[0].FirstDetectedAt == "" {
		t.Fatal(items, err)
	}
	if _, err = db.Exec(`INSERT INTO dream_signals(signal_id,signal_type,entity_refs_json,severity,repo_revision,dedupe_key,status,evidence_hash,evidence_json,first_detected_at,last_detected_at) VALUES('bad','bad','{','x','r','bad','open','x','{}','x','x')`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.List(ctx); err == nil {
		t.Fatal("malformed refs")
	}
	db.Exec(`DELETE FROM dream_signals WHERE signal_id='bad'`)
	if _, err = db.Exec(`CREATE TRIGGER fail_signal_insert BEFORE INSERT ON dream_signals BEGIN SELECT RAISE(FAIL,'insert'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Upsert(ctx, "r", []DetectedSignal{{"y", nil, "x", "y", map[string]any{}}}); err == nil {
		t.Fatal("insert")
	}
	db.Exec(`DROP TRIGGER fail_signal_insert`)
	if _, err = db.Exec(`CREATE TRIGGER fail_signal_update BEFORE UPDATE ON dream_signals BEGIN SELECT RAISE(FAIL,'update'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Upsert(ctx, "r", []DetectedSignal{{"x", nil, "x", "x", map[string]any{"changed": true}}}); err == nil {
		t.Fatal("update")
	}
	db.Exec(`DROP TRIGGER fail_signal_update`)
	if _, err = s.Upsert(ctx, "r", []DetectedSignal{{"z", nil, "x", "z", map[string]any{}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_signal_resolve BEFORE UPDATE ON dream_signals BEGIN SELECT RAISE(FAIL,'resolve'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Upsert(ctx, "r", nil); err == nil {
		t.Fatal("resolve")
	}
	db.Exec(`DROP TRIGGER fail_signal_resolve`)
	if err = s.MarkStatus(ctx, []string{"x"}, "acknowledged"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Actionable(ctx); err != nil || !reflect.DeepEqual(got[0].EntityRefs, []string{}) {
		t.Fatal(got, err)
	}
	if _, err = db.Exec(`CREATE TRIGGER rollback_signal AFTER INSERT ON dream_signals BEGIN SELECT RAISE(ROLLBACK,'rollback'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Upsert(ctx, "r", []DetectedSignal{{"commit", nil, "x", "commit", map[string]any{}}}); err == nil {
		t.Fatal("commit")
	}
	db.Close()
	if _, err := GetServiceState(ctx, db, "direct"); err == nil {
		t.Fatal("state closed")
	}
	for _, run := range []func() error{func() error { _, e := s.List(ctx); return e }, func() error { _, e := s.Actionable(ctx); return e }, func() error { _, e := s.Upsert(ctx, "r", nil); return e }, func() error { return s.MarkStatus(ctx, []string{"x"}, "x") }, func() error { return SetServiceState(ctx, db, "x", "x") }, func() error { _, e := GetServiceState(ctx, db, "x"); return e }} {
		if err := run(); err == nil {
			t.Fatal("closed")
		}
	}
	_ = errors.Is(sql.ErrNoRows, sql.ErrNoRows)
}
