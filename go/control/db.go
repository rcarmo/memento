// Package control ports the durable SQLite control plane. Schema presence does
// not imply service operations or authorisation have been implemented.
package control

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/rcarmo/memento/go/internal/pyjson"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	_ "modernc.org/sqlite"
)

type MigrationError struct{ Version string }

func (e *MigrationError) Error() string { return "unsupported control schema version: " + e.Version }

// Connect opens a pure-Go SQLite database. Every physical connection gets the
// source foreign-key/WAL settings and Python sqlite's default five-second busy
// timeout. One connection per handle mirrors the source; callers may open
// independent handles for concurrent workers and must retry only where safe.
func Connect(ctx context.Context, path string) (*sql.DB, error) {
	return connect(ctx, path, filepath.Abs, sql.Open)
}
func connect(ctx context.Context, path string, abs func(string) (string, error), open func(string, string) (*sql.DB, error)) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return nil, err
	}
	absolute, err := abs(path)
	if err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: absolute}
	query := url.Values{}
	for _, pragma := range []string{"busy_timeout(5000)", "journal_mode(WAL)", "foreign_keys(1)"} {
		query.Add("_pragma", pragma)
	}
	uri.RawQuery = query.Encode()
	db, err := open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

type migrationDB interface {
	sqlExecutor
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func Migrate(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return migrate(ctx, conn)
}
func migrate(ctx context.Context, db migrationDB) error {
	// Python sqlite's context does not start a transaction for DDL: on fresh
	// or unsupported versions, preceding CREATE statements persist. Use explicit
	// DDL phases, starting a transaction only for v5 data or version writes.
	for _, statement := range migrationsV1 {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	var current string
	err := db.QueryRowContext(ctx, "SELECT value FROM service_state WHERE key = 'schema_version'").Scan(&current)
	fresh := errors.Is(err, sql.ErrNoRows)
	if err != nil && !fresh {
		return err
	}
	if !fresh && current != "5" && current != "1" && current != "2" && current != "3" && current != "4" && current != "6" && current != "7" && current != "8" && current != "9" && current != SchemaVersion {
		return &MigrationError{Version: current}
	}
	if current == "5" {
		for _, statement := range migrationsV6 {
			if _, err = db.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		return migrateV5(ctx, db)
	}
	for _, group := range [][]string{migrationsV6, migrationsV7, migrationsV8, migrationsV9, migrationsV10} {
		for _, statement := range group {
			if _, err = db.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
	}
	if fresh {
		_, err = db.ExecContext(ctx, "INSERT INTO service_state(key, value, updated_at) VALUES('schema_version', ?, datetime('now'))", SchemaVersion)
	} else if current != SchemaVersion {
		_, err = db.ExecContext(ctx, "UPDATE service_state SET value = ?, updated_at = datetime('now') WHERE key = 'schema_version'", SchemaVersion)
	}
	return err
}

func migrateV5(ctx context.Context, db migrationDB) error {
	// Read all rows before any write, as Python fetchall does.
	rows, err := db.QueryContext(ctx, "SELECT * FROM skill_pack_proposals ORDER BY created_at, proposal_id")
	if err != nil {
		return err
	}
	return migrateV5Rows(ctx, db, rows)
}
func migrateV5Rows(ctx context.Context, db migrationDB, rows rowReader) error {
	legacy, err := readRows(rows)
	if err != nil {
		return err
	}
	var tx *sql.Tx
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	for _, row := range legacy {
		var exists string
		query := sqlExecutor(db)
		if tx != nil {
			query = tx
		}
		err = query.QueryRowContext(ctx, "SELECT proposal_id FROM proposals WHERE proposal_id = ?", pyString(row["proposal_id"])).Scan(&exists)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		name, version := pyString(row["skill_name"]), pyString(row["version"])
		assetID := "skill-pack:" + name + ":" + version
		conceptPath := "/skills/" + name + ".md"
		manifest, err := pyjson.Parse(pyString(row["manifest_json"]))
		if err != nil {
			return err
		}
		patch := map[string]any{"changes": []any{
			map[string]any{"kind": "create", "path": conceptPath, "concept_type": "concept", "title": pythonTitle(strings.ReplaceAll(name, "-", " ")), "body": pyString(row["skill_md"]), "description": "Versioned agent skill " + name + ".", "tags": []string{"skill"}, "aliases": []string{}},
			map[string]any{"kind": "attach_asset_pack", "path": conceptPath, "asset_id": assetID, "asset_kind": "skill", "zip_sha256": pyString(row["zip_sha256"]), "version": version, "manifest": manifest},
		}}
		patchJSON, err := pyjson.Dumps(patch)
		if err != nil {
			return err
		}
		sum := sha256.Sum256([]byte(patchJSON))
		if tx == nil {
			tx, err = db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO proposals(proposal_id,author_principal,client_instance_id,base_revision,intent,rationale,patch_json,patch_hash,status,reviewed_by,review_comment,applied_operation_id,applied_revision,created_at,updated_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, pyString(row["proposal_id"]), pyString(row["author_principal"]), nil, pyString(row["base_revision"]), "Attach skill asset "+name+" "+version, row["rationale"], patchJSON, fmt.Sprintf("%x", sum), pyString(row["status"]), row["reviewed_by"], row["review_comment"], row["applied_operation_id"], row["applied_revision"], pyString(row["created_at"]), pyString(row["updated_at"]), row["expires_at"]); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, pyString(row["proposal_id"]), assetID, conceptPath, "skill", version, "application/zip", pyString(row["zip_sha256"]), row["zip_bytes"], pyString(row["manifest_json"]), pyString(row["created_at"])); err != nil {
			return err
		}
	}
	executor := sqlExecutor(db)
	if tx != nil {
		executor = tx
	}
	for _, group := range [][]string{migrationsV6, migrationsV7, migrationsV8, migrationsV9, migrationsV10} {
		for _, statement := range group {
			if _, err = executor.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
	}
	if _, err = executor.ExecContext(ctx, "UPDATE service_state SET value = ?, updated_at = datetime('now') WHERE key = 'schema_version'", SchemaVersion); err != nil {
		return err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			_, _ = db.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
			return err
		}
		return nil
	}
	return nil
}

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type rowReader interface {
	Close() error
	Columns() ([]string, error)
	Next() bool
	Scan(...any) error
	Err() error
}

func readRows(rows rowReader) ([]map[string]any, error) {
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err = rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, name := range columns {
			row[name] = values[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
func pyString(value any) string {
	if value == nil {
		return "None"
	}
	if b, ok := value.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(value)
}
func pythonTitle(value string) string {
	var out strings.Builder
	previous := false
	title := cases.Title(language.Und, cases.NoLower)
	lower := cases.Lower(language.Und)
	for _, r := range value {
		cased := unicode.IsLower(r) || unicode.IsUpper(r) || unicode.IsTitle(r)
		if previous {
			out.WriteString(lower.String(string(r)))
		} else {
			out.WriteString(title.String(string(r)))
		}
		previous = cased
	}
	return out.String()
}
