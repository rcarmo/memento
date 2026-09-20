package assets

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func syntheticZIP(t *testing.T) []byte {
	t.Helper()
	for _, c := range packFixtures(t) {
		if c.Name == "stored" {
			return c.ZIP
		}
	}
	t.Fatal("fixture")
	return nil
}
func TestStagingFailures(t *testing.T) {
	ctx := context.Background()
	raw := syntheticZIP(t)
	s, _ := stagingStore(t)
	for _, pair := range [][2]string{{"asset", "bad"}, {"INVALID", "1.0.0"}} {
		if _, _, err := s.BeginUpload(ctx, "p", "bad", pair[0], pair[1]); err == nil {
			t.Fatal(pair)
		}
	}
	s.Random = strings.NewReader("")
	if _, _, err := s.BeginUpload(ctx, "p", "empty-random", "asset", "1.0.0"); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	if _, _, err := s.Put(ctx, "p", "empty-random", "asset", "1.0.0", raw); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	s.Random = nil
	if _, err := s.id(); err != nil {
		t.Fatal(err)
	}
	s.Now = nil
	if s.now().IsZero() {
		t.Fatal("clock")
	}
	s, _ = stagingStore(t)
	if _, _, err := s.PutWithTicket(ctx, "absent", raw); err == nil {
		t.Fatal("unknown ticket")
	}
	_, token, err := s.BeginUpload(ctx, "p", "ticket", "asset", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = s.DB.Exec("UPDATE asset_upload_tickets SET expires_at='bad'")
	if _, _, err = s.BeginUpload(ctx, "p", "ticket", "asset", "1.0.0"); err == nil {
		t.Fatal("malformed ticket state")
	}
	if _, _, err = s.PutWithTicket(ctx, token, raw); err == nil {
		t.Fatal("malformed ticket")
	}
	_, _ = s.DB.Exec("UPDATE asset_upload_tickets SET expires_at='9999-01-01T00:00:00Z'")
	if _, _, err = s.PutWithTicket(ctx, token, []byte("invalid")); err == nil {
		t.Fatal("bad zip")
	}
	staged, _, err := s.PutWithTicket(ctx, token, raw)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = s.DB.Exec("UPDATE staged_assets SET manifest_json='{' WHERE staged_asset_id=?", staged.StagedAssetID)
	if _, err = s.Get(ctx, "p", staged.StagedAssetID, false); err == nil {
		t.Fatal("bad manifest")
	}
	if _, _, err = s.PutWithTicket(ctx, token, raw); err == nil {
		t.Fatal("replay bad manifest")
	}
	if _, _, err = s.Put(ctx, "p", "ticket:ticket", "asset", "1.0.0", raw); err == nil {
		t.Fatal("bad stored replay")
	}
	if err = s.Consume(ctx, "p", nil, "proposal"); err != nil {
		t.Fatal(err)
	}
	s.DB.Close()
	for _, run := range []func() error{func() error { _, _, err := s.BeginUpload(ctx, "p", "x", "asset", "1.0.0"); return err }, func() error { _, _, err := s.PutWithTicket(ctx, token, raw); return err }, func() error { _, _, err := s.Put(ctx, "p", "x", "asset", "1.0.0", raw); return err }, func() error { _, err := s.Get(ctx, "p", "x", false); return err }} {
		if err := run(); err == nil {
			t.Fatal("closed DB")
		}
	}
}
func TestStagingSQLRollback(t *testing.T) {
	ctx := context.Background()
	raw := syntheticZIP(t)
	for _, kind := range []string{"ticket-insert", "ticket-readback", "stage-insert", "ticket-update", "expire-before-upload", "consume-fk", "zero-ticket-update"} {
		t.Run(kind, func(t *testing.T) {
			s, clock := stagingStore(t)
			var token string
			if kind == "ticket-insert" {
				_, _ = s.DB.Exec(`CREATE TRIGGER blocked BEFORE INSERT ON asset_upload_tickets BEGIN SELECT RAISE(ABORT,'test'); END`)
				if _, _, err := s.BeginUpload(ctx, "p", "k", "asset", "1.0.0"); err == nil {
					t.Fatal(kind)
				}
				return
			}
			if kind == "ticket-readback" {
				_, _ = s.DB.Exec(`CREATE TRIGGER deleted AFTER INSERT ON asset_upload_tickets BEGIN DELETE FROM asset_upload_tickets; END`)
				if _, value, err := s.BeginUpload(ctx, "p", "k", "asset", "1.0.0"); err == nil || value != "" {
					t.Fatal("token with failed readback")
				}
				return
			}
			_, token, err := s.BeginUpload(ctx, "p", "ticket", "asset", "1.0.0")
			if err != nil {
				t.Fatal(err)
			}
			expired, _, err := s.Put(ctx, "p", "old", "asset", "1.0.0", raw)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = s.DB.Exec("UPDATE staged_assets SET expires_at=? WHERE staged_asset_id=?", timestamp(clock.Add(-time.Hour)), expired.StagedAssetID)
			switch kind {
			case "stage-insert":
				_, _ = s.DB.Exec(`CREATE TRIGGER blocked BEFORE INSERT ON staged_assets BEGIN SELECT RAISE(ABORT,'test'); END`)
			case "ticket-update":
				_, _ = s.DB.Exec(`CREATE TRIGGER blocked BEFORE UPDATE ON asset_upload_tickets BEGIN SELECT RAISE(ABORT,'test'); END`)
			case "zero-ticket-update":
				_, _ = s.DB.Exec(`CREATE TRIGGER ignored BEFORE UPDATE ON asset_upload_tickets BEGIN SELECT RAISE(IGNORE); END`)
			case "expire-before-upload":
				_, _ = s.DB.Exec(`CREATE TRIGGER blocked BEFORE UPDATE ON staged_assets BEGIN SELECT RAISE(ABORT,'test'); END`)
			case "consume-fk":
				fresh, _, err := s.Put(ctx, "p", "fresh", "asset", "1.0.0", raw)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.Consume(ctx, "p", []string{fresh.StagedAssetID}, "missing-proposal"); err == nil {
					t.Fatal(kind)
				}
				return
			}
			if _, _, err = s.PutWithTicket(ctx, token, raw); err == nil {
				t.Fatal(kind)
			}
			var count int
			_ = s.DB.QueryRow("SELECT COUNT(*) FROM staged_assets WHERE idempotency_key='ticket:ticket'").Scan(&count)
			if count != 0 {
				t.Fatal("ticket insertion survived rollback")
			}
			if kind != "expire-before-upload" {
				got, err := s.Get(ctx, "p", expired.StagedAssetID, false)
				if err != nil || got.State != "expired" || len(got.BlobBytes) != 0 {
					t.Fatal("expiry side effect lost", got, err)
				}
			}
		})
	}
}

type failResult struct{}

func (failResult) LastInsertId() (int64, error) { return 0, nil }
func (failResult) RowsAffected() (int64, error) { return 0, io.ErrClosedPipe }
func TestStagedRowAndManifestValidation(t *testing.T) {
	if err := consumedOne(failResult{}, "x"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	empty := ""
	value := &empty
	nullableEmpty(&value)
	if value != nil {
		t.Fatal("empty nullable")
	}
	good := Manifest{Entries: []ManifestEntry{}, SHA256: strings.Repeat("a", 64)}
	raw := manifestJSON(good)
	if _, err := ParseManifest(raw); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"{}", "null", "[]", raw + " {}", strings.Replace(raw, `"entries":[]`, `"entries":null`, 1), strings.Replace(raw, `"file_count":0`, `"file_count":-1`, 1), strings.Replace(raw, strings.Repeat("a", 64), "bad", 1), strings.TrimSuffix(raw, "}") + `,"extra":1}`} {
		if _, err := ParseManifest(bad); err == nil {
			t.Fatal(bad)
		}
	}
	good.Entries = []ManifestEntry{{Path: "", SHA256: strings.Repeat("b", 64), MediaType: "x"}}
	if _, err := ParseManifest(manifestJSON(good)); err == nil {
		t.Fatal("bad entry")
	}
	var fields map[string]any
	_ = json.Unmarshal([]byte(manifestJSON(good)), &fields)
	entries := fields["entries"].([]any)
	delete(entries[0].(map[string]any), "size")
	encoded, _ := json.Marshal(fields)
	if _, err := ParseManifest(string(encoded)); err == nil {
		t.Fatal("missing entry field")
	}
	s, clock := stagingStore(t)
	staged, _, err := s.Put(context.Background(), "p", "k", "asset", "1.0.0", syntheticZIP(t))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = s.DB.Exec("UPDATE staged_assets SET expires_at='bad'")
	if _, err = scanStaged(s.DB.QueryRow("SELECT "+stagedColumns+" FROM staged_assets"), *clock); err == nil {
		t.Fatal("bad expiry")
	}
	_, _ = s.DB.Exec("UPDATE staged_assets SET expires_at=?", timestamp(clock.Add(-time.Hour)))
	row, err := scanStaged(s.DB.QueryRow("SELECT "+stagedColumns+" FROM staged_assets WHERE staged_asset_id=?", staged.StagedAssetID), *clock)
	if err != nil || row.State != "expired" {
		t.Fatal(row, err)
	}
}
func TestConsumeCallerTransactionAndConcurrency(t *testing.T) {
	s, _ := stagingStore(t)
	ctx := context.Background()
	raw := syntheticZIP(t)
	staged, _, err := s.Put(ctx, "p", "k", "asset", "1.0.0", raw)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ConsumeInTx(ctx, tx, "p", []string{staged.StagedAssetID}, "proposal"); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback()
	if _, err = s.Get(ctx, "p", staged.StagedAssetID, true); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	wins := 0
	var mu sync.Mutex
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := s.Consume(ctx, "p", []string{staged.StagedAssetID}, "proposal"); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	workers.Wait()
	if wins != 1 {
		t.Fatal("multiple consumers", wins)
	}
}

var _ sql.Result = failResult{}

func TestTicketDeferredFailureAndUploadedExpiry(t *testing.T) {
	ctx := context.Background()
	s, clock := stagingStore(t)
	raw := syntheticZIP(t)
	_, token, err := s.BeginUpload(ctx, "p", "k", "asset", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.Exec(`CREATE TABLE deferred_asset(id TEXT REFERENCES staged_assets(staged_asset_id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_upload_commit AFTER INSERT ON staged_assets BEGIN INSERT INTO deferred_asset VALUES('missing'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.PutWithTicket(ctx, token, raw); err == nil {
		t.Fatal("deferred commit accepted")
	}
	ticket, err := s.TicketStatus(ctx, "p", "k")
	if err != nil || ticket.StagedAssetID != nil {
		t.Fatal(ticket, err)
	}
	var count int
	_ = s.DB.QueryRow("SELECT COUNT(*) FROM staged_assets").Scan(&count)
	if count != 0 {
		t.Fatal("partial staging")
	}
	_, _ = s.DB.Exec("DROP TRIGGER fail_upload_commit")
	staged, _, err := s.PutWithTicket(ctx, token, raw)
	if err != nil {
		t.Fatal(err)
	}
	*clock = clock.Add(25 * time.Hour)
	replay, replayed, err := s.PutWithTicket(ctx, token, raw)
	if err != nil || !replayed || replay.State != "expired" || len(replay.BlobBytes) != 0 || replay.StagedAssetID != staged.StagedAssetID {
		t.Fatal(replay, replayed, err)
	}
}
