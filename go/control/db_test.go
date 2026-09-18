package control

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type migrationFixture struct {
	Mode     string
	Database []byte
	Expected map[string]any
	Error    string
}

func migrationFixtures(t *testing.T) []migrationFixture {
	t.Helper()
	raw, err := os.ReadFile("../testdata/parity/control-db.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Cases []migrationFixture }
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data.Cases
}
func fixtureDB(t *testing.T, fixture migrationFixture) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "control.sqlite")
	if err := os.WriteFile(path, fixture.Database, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := Connect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}
func snapshotDB(t *testing.T, db *sql.DB) map[string]any {
	t.Helper()
	ctx := context.Background()
	result := map[string]any{}
	for _, table := range []string{"service_state", "proposals", "proposal_assets"} {
		var exists string
		err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			result[table] = nil
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		rows, err := db.QueryContext(ctx, "SELECT * FROM "+table)
		if err != nil {
			t.Fatal(err)
		}
		records, err := readRows(rows)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range records {
			if table == "service_state" {
				row["updated_at"] = "<clock>"
			}
			if blob, ok := row["blob_bytes"].([]byte); ok {
				row["blob_bytes"] = base64.StdEncoding.EncodeToString(blob)
			}
		}
		result[table] = records
	}
	rows, err := db.QueryContext(ctx, "SELECT type,name,tbl_name AS 'table',sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := readRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	result["schema"] = schema
	// Compare decoded JSON so sqlite integer/Go container representation is immaterial.
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var normalized map[string]any
	_ = json.Unmarshal(raw, &normalized)
	return normalized
}
func TestPythonControlMigrationFixtures(t *testing.T) {
	for _, fixture := range migrationFixtures(t) {
		t.Run(fixture.Mode, func(t *testing.T) {
			db, _ := fixtureDB(t, fixture)
			err := Migrate(context.Background(), db)
			if fixture.Error == "" {
				if err != nil {
					t.Fatal(err)
				}
				if err = Migrate(context.Background(), db); err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("migration accepted invalid source")
			}
			got := snapshotDB(t, db)
			if !reflect.DeepEqual(got, fixture.Expected) {
				a, _ := json.MarshalIndent(got, "", "  ")
				b, _ := json.MarshalIndent(fixture.Expected, "", "  ")
				t.Fatalf("snapshot differs:\n%s\n!=\n%s", a, b)
			}
		})
	}
}
func TestControlPragmasReopenAndErrors(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "control.sqlite")
	db, err := Connect(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for pragma, want := range map[string]any{"journal_mode": "wal", "foreign_keys": int64(1), "busy_timeout": int64(5000)} {
		var got any
		if err = db.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil || got != want {
			t.Fatal(pragma, got, err)
		}
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = Connect(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if err = Migrate(ctx, db); err == nil {
		t.Fatal("closed DB")
	}
	if _, err = Connect(ctx, filepath.Join(path, "child")); err == nil {
		t.Fatal("parent file")
	}
	if _, err = Connect(ctx, t.TempDir()); err == nil {
		t.Fatal("directory DB")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = Connect(canceled, filepath.Join(t.TempDir(), "db")); err == nil {
		t.Fatal("cancelled open")
	}
	if (&MigrationError{Version: "11"}).Error() != "unsupported control schema version: 11" {
		t.Fatal("error text")
	}
	if pyString(nil) != "None" || pyString([]byte("x")) != "x" {
		t.Fatal("python values")
	}
	if pythonTitle("demo ß skill") != "Demo Ss Skill" {
		t.Fatal(pythonTitle("demo ß skill"))
	}
}

func findMigration(t *testing.T, mode string) migrationFixture {
	t.Helper()
	for _, f := range migrationFixtures(t) {
		if f.Mode == mode {
			return f
		}
	}
	t.Fatal(mode)
	return migrationFixture{}
}

type faultMigrationDB struct {
	*sql.DB
	fail string
}

func (d faultMigrationDB) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	if d.fail != "" && strings.Contains(q, d.fail) {
		return nil, io.ErrClosedPipe
	}
	return d.DB.ExecContext(ctx, q, args...)
}
func (d faultMigrationDB) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	return nil, io.ErrClosedPipe
}
func TestMigrationFailuresAndRollback(t *testing.T) {
	ctx := context.Background()
	if _, err := connect(ctx, filepath.Join(t.TempDir(), "db"), func(string) (string, error) { return "", os.ErrPermission }, sql.Open); err == nil {
		t.Fatal("abs")
	}
	if _, err := connect(ctx, filepath.Join(t.TempDir(), "db"), filepath.Abs, func(string, string) (*sql.DB, error) { return nil, io.ErrClosedPipe }); err == nil {
		t.Fatal("open")
	}
	for _, mode := range []string{"fresh", "5"} {
		for _, target := range []string{"CREATE TABLE IF NOT EXISTS proposal_assets", "CREATE TABLE IF NOT EXISTS access_meta"} {
			db, _ := fixtureDB(t, findMigration(t, mode))
			if err := migrate(ctx, faultMigrationDB{db, target}); err == nil {
				t.Fatal(mode, target)
			}
		}
	}
	db, _ := fixtureDB(t, findMigration(t, "fresh"))
	_, _ = db.Exec("CREATE TABLE service_state(other TEXT)")
	if err := Migrate(ctx, db); err == nil {
		t.Fatal("bad schema query")
	}
	db, _ = fixtureDB(t, findMigration(t, "5"))
	_, _ = db.Exec("DROP TABLE skill_pack_proposals")
	if err := Migrate(ctx, db); err == nil {
		t.Fatal("missing legacy")
	}
	db, _ = fixtureDB(t, findMigration(t, "5"))
	_, _ = db.Exec("DROP TABLE proposals")
	_, _ = db.Exec("CREATE TABLE proposals(other TEXT)")
	if err := Migrate(ctx, db); err == nil {
		t.Fatal("legacy existing query")
	}
	for _, target := range []string{"proposals", "proposal_assets", "service_state"} {
		db, _ := fixtureDB(t, findMigration(t, "5"))
		for _, statement := range migrationsV6 {
			_, _ = db.Exec(statement)
		}
		verb := "INSERT"
		if target == "service_state" {
			verb = "UPDATE"
		}
		_, err := db.Exec("CREATE TRIGGER blocked BEFORE " + verb + " ON " + target + " BEGIN SELECT RAISE(ABORT, 'synthetic'); END")
		if err != nil {
			t.Fatal(err)
		}
		if err = Migrate(ctx, db); err == nil {
			t.Fatal(target)
		}
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM proposals").Scan(&count)
		if count != 0 {
			t.Fatal("partial migration committed")
		}
	}
	db, _ = fixtureDB(t, findMigration(t, "5"))
	_, _ = db.Exec("DELETE FROM skill_pack_proposals")
	if err := Migrate(ctx, db); err != nil {
		t.Fatal("no data migration", err)
	}
	db, _ = fixtureDB(t, findMigration(t, "5"))
	_, _ = db.Exec("UPDATE skill_pack_proposals SET manifest_json=?", strings.Repeat("[", 102)+"0"+strings.Repeat("]", 102))
	if err := Migrate(ctx, db); err == nil {
		t.Fatal("patch depth error")
	}
	// DDL inside the v5 write transaction fails and rolls back prior lifted rows.
	db, _ = fixtureDB(t, findMigration(t, "5"))
	_, _ = db.Exec("CREATE TABLE access_audit(other TEXT)")
	if err := Migrate(ctx, db); err == nil {
		t.Fatal("DDL failure")
	}
}

type failedRows struct {
	fail string
	next bool
}

func (r *failedRows) Close() error { return nil }
func (r *failedRows) Columns() ([]string, error) {
	if r.fail == "columns" {
		return nil, io.ErrClosedPipe
	}
	return []string{"one"}, nil
}
func (r *failedRows) Next() bool {
	if r.next {
		return false
	}
	r.next = true
	return true
}
func (r *failedRows) Scan(...any) error { return io.ErrClosedPipe }
func (r *failedRows) Err() error        { return nil }
func TestMigrationRowErrors(t *testing.T) {
	for _, kind := range []string{"columns", "scan"} {
		if _, err := readRows(&failedRows{fail: kind}); err == nil {
			t.Fatal(kind)
		}
	}
}

func TestLegacyRowFailure(t *testing.T) {
	db, _ := fixtureDB(t, findMigration(t, "5"))
	if err := migrateV5Rows(context.Background(), db, &failedRows{fail: "columns"}); err == nil {
		t.Fatal("row failure")
	}
}

func TestV5DeferredCommitFailureCleansConnection(t *testing.T) {
	db, _ := fixtureDB(t, findMigration(t, "5"))
	ctx := context.Background()
	_, err := db.Exec(`CREATE TABLE migration_deferred(proposal_id TEXT REFERENCES proposals(proposal_id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_migration_commit AFTER INSERT ON proposals BEGIN INSERT INTO migration_deferred VALUES('missing'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err == nil {
		t.Fatal("deferred constraint ignored")
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM proposals").Scan(&count); err != nil || count != 0 {
		t.Fatal("failed commit leaked rows", count, err)
	}
	if _, err = db.Exec("BEGIN; ROLLBACK"); err != nil {
		t.Fatal("connection retained failed transaction", err)
	}
}

type initialDDLFailure struct{ *sql.DB }

func (d initialDDLFailure) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, io.ErrClosedPipe
}
func TestInitialMigrationDDLFailure(t *testing.T) {
	db, _ := fixtureDB(t, findMigration(t, "fresh"))
	if err := migrate(context.Background(), initialDDLFailure{db}); err == nil {
		t.Fatal("initial DDL")
	}
}

func TestPinnedTransactionBeginFailure(t *testing.T) {
	db, _ := fixtureDB(t, findMigration(t, "fresh"))
	ctx := context.Background()
	if _, err := db.Exec("BEGIN"); err != nil {
		t.Fatal(err)
	}
	if err := WithTransaction(ctx, db, func(*sql.Tx) error { return nil }); err == nil {
		t.Fatal("nested begin")
	}
	if _, err := db.Exec("ROLLBACK"); err != nil {
		t.Fatal(err)
	}
}
