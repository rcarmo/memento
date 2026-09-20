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

func schedulerStore(t *testing.T) (*sql.DB, Scheduler) {
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
	return db, Scheduler{DB: db, Now: func() time.Time { return now }, UUID: func() string { return "run-one" }}
}
func TestSchedulerLifecycle(t *testing.T) {
	ctx := context.Background()
	db, s := schedulerStore(t)
	base := "base"
	claim, err := s.Claim(ctx, "dream", "one", &base)
	if err != nil || !claim.Created || claim.Record.RunID != "run-one" || claim.Record.State != "running" {
		t.Fatal(claim, err)
	}
	if _, err = s.Claim(ctx, "dream", "two", &base); err == nil {
		t.Fatal("overlap")
	}
	end := "end"
	message := "done"
	chain := []ModelAttempt{{"m1", "error"}, {"m2", "success"}}
	record, err := s.Finish(ctx, "run-one", "succeeded", &end, 3, 1, chain, &message)
	if err != nil || record.State != "succeeded" || record.SignalCount != 3 || !reflect.DeepEqual(record.ModelChain, chain) || record.FinishedAt == nil {
		t.Fatal(record, err)
	}
	duplicate, err := s.Claim(ctx, "dream", "one", &base)
	if err != nil || duplicate.Created || duplicate.Record.RunID != "run-one" {
		t.Fatal(duplicate, err)
	}
	if _, err = db.Exec(`INSERT INTO scheduler_runs(run_id,job_name,window_key,state,model_chain_json,started_at) VALUES('legacy','other','x','done','["old"]','x')`); err != nil {
		t.Fatal(err)
	}
	legacy, err := s.Get(ctx, "legacy")
	if err != nil || !reflect.DeepEqual(legacy.ModelChain, []ModelAttempt{{"old", "success"}}) {
		t.Fatal(legacy, err)
	}
}
func TestSchedulerDefaultsAndNullChain(t *testing.T) {
	ctx := context.Background()
	db, _ := schedulerStore(t)
	s := Scheduler{DB: db}
	claim, err := s.Claim(ctx, "default", "window", nil)
	if err != nil || claim.Record.RunID == "" || claim.Record.StartedAt == "" {
		t.Fatal(claim, err)
	}
	if _, err = db.Exec(`INSERT INTO scheduler_runs(run_id,job_name,window_key,state,model_chain_json,started_at) VALUES('null','null','null','done',NULL,'x')`); err != nil {
		t.Fatal(err)
	}
	record, err := s.Get(ctx, "null")
	if err != nil || len(record.ModelChain) != 0 {
		t.Fatal(record, err)
	}
}
func TestSchedulerConflictText(t *testing.T) {
	if (&SchedulerConflictError{"dream"}).Error() != "job dream is already running" {
		t.Fatal("text")
	}
}
func TestSchedulerFailures(t *testing.T) {
	ctx := context.Background()
	db, s := schedulerStore(t)
	if _, err := s.Get(ctx, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE VIEW scheduler_runs_real AS SELECT * FROM scheduler_runs`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO scheduler_runs(run_id,job_name,window_key,state,model_chain_json,started_at) VALUES('duplicate-bad','duplicate','window','done','{','x')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(ctx, "duplicate", "window", nil); err == nil {
		t.Fatal("duplicate scan")
	}
	for id, raw := range map[string]string{"bad-json": "{", "bad-object": "[{}]", "bad-scalar": "[1]"} {
		if _, err := db.Exec(`INSERT INTO scheduler_runs(run_id,job_name,window_key,state,model_chain_json,started_at) VALUES(?,?,?,'done',?,'x')`, id, id, id, raw); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, id); err == nil {
			t.Fatal(id)
		}
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_scheduler_insert BEFORE INSERT ON scheduler_runs BEGIN SELECT RAISE(FAIL,'insert'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(ctx, "insert", "x", nil); err == nil {
		t.Fatal("insert")
	}
	db.Exec(`DROP TRIGGER fail_scheduler_insert`)
	if _, err := db.Exec(`CREATE TRIGGER remove_scheduler AFTER INSERT ON scheduler_runs BEGIN DELETE FROM scheduler_runs WHERE run_id=NEW.run_id; END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(ctx, "gone", "x", nil); err == nil {
		t.Fatal("get after insert")
	}
	db.Exec(`DROP TRIGGER remove_scheduler`)
	claim, err := s.Claim(ctx, "finish", "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER fail_scheduler_update BEFORE UPDATE ON scheduler_runs BEGIN SELECT RAISE(FAIL,'update'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Finish(ctx, claim.Record.RunID, "done", nil, 0, 0, nil, nil); err == nil {
		t.Fatal("update")
	}
	db.Exec(`DROP TRIGGER fail_scheduler_update`)
	s.UUID = func() string { return "run-two" }
	claim, err = s.Claim(ctx, "finish-two", "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := s.Finish(ctx, claim.Record.RunID, "done", nil, 0, 0, nil, nil); err == nil {
		t.Fatal("finish closed")
	}
	if _, err := s.Claim(ctx, "x", "x", nil); err == nil {
		t.Fatal("claim closed")
	}
	if _, err := s.Finish(ctx, "x", "x", nil, 0, 0, nil, nil); err == nil {
		t.Fatal("finish closed")
	}
}
