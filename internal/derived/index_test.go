package derived

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
)

func TestLifecycleReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/derived-lifecycle.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario                           string
		Statements, Files, QuarantineFiles []string
		ErrorType                          string `json:"error_type"`
		Error                              string
		FinalState                         map[string]string `json:"final_state"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "bundle")
			if err := os.Mkdir(bundle, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "index.sqlite")
			index := &Index{Path: path, Now: func() time.Time { return time.UnixMilli(1789689600125) }}
			switch c.Scenario {
			case "invalid-write", "invalid-read":
				if err := os.WriteFile(path, []byte("invalid header"), 0600); err != nil {
					t.Fatal(err)
				}
			case "zero":
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "missing":
			default:
				if _, err := index.State(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if len(c.Statements) > 0 {
				db, err := openIndexDB(ctx, path)
				if err != nil {
					t.Fatal(err)
				}
				for _, q := range c.Statements {
					if _, err = db.Exec(q); err != nil {
						t.Fatal(err)
					}
				}
				db.Close()
			}
			if c.Scenario == "unsupported-write" || c.Scenario == "unsupported-read" {
				index.identity = nil
			}
			if c.Scenario == "replaced-inode" {
				if err := os.Rename(path, filepath.Join(root, "old.sqlite")); err != nil {
					t.Fatal(err)
				}
				db, err := openIndexDB(ctx, path)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = db.Exec("CREATE TABLE marker(value TEXT)"); err != nil {
					t.Fatal(err)
				}
				db.Close()
			}
			if c.Scenario == "bad-concept" {
				if err := os.WriteFile(filepath.Join(bundle, "bad.md"), []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var err error
			switch c.Scenario {
			case "busy":
				lock, openErr := openIndexDB(ctx, path)
				if openErr != nil {
					t.Fatal(openErr)
				}
				defer lock.Close()
				if _, openErr = lock.Exec("BEGIN IMMEDIATE"); openErr != nil {
					t.Fatal(openErr)
				}
				ops := defaultIndexIO()
				ops.openDB = func(ctx context.Context, path string) (*sql.DB, error) {
					db, err := openIndexDB(ctx, path)
					if err == nil {
						_, err = db.Exec("PRAGMA busy_timeout=1")
					}
					return db, err
				}
				err = index.withCoreIO(ctx, true, func(s ContentStore) error { return s.SetRepoRevision(ctx, "r") }, ops)
				_, _ = lock.Exec("ROLLBACK")
			case "invalid-read", "unsupported-read", "quarantined-read":
				_, err = index.State(ctx)
			case "missing-table":
				_, err = index.Metrics(ctx, "unknown")
			case "quarantined-write":
				err = index.UpdatePaths(ctx, bundle, "r", nil)
			case "query-error":
				_, err = index.SearchLexical(ctx, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, SearchOptions{Query: `"unterminated`, Syntax: "fts5"})
			default:
				err = index.Rebuild(ctx, bundle, "r")
			}
			if (err != nil) != (c.ErrorType != "") {
				t.Fatal(err, c.ErrorType)
			}
			if c.ErrorType == "DerivedIndexCorruptionError" {
				var target *CorruptionError
				if !errors.As(err, &target) {
					t.Fatal(err)
				}
			}
			if c.ErrorType == "DerivedIndexUnavailableError" {
				var target *UnavailableError
				if !errors.As(err, &target) || target.Error() == "" {
					t.Fatal(err)
				}
			}
			paths, err := filepath.Glob(filepath.Join(root, "*.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			files := []string{}
			for _, p := range paths {
				files = append(files, filepath.Base(p))
			}
			if !reflect.DeepEqual(files, c.Files) {
				t.Fatal(files, c.Files)
			}
			if c.FinalState != nil {
				db, err := openIndexDB(ctx, path)
				if err != nil {
					t.Fatal(err)
				}
				state := map[string]string{}
				for _, row := range queryRows(t, db, "SELECT key,value FROM index_state") {
					value := row["value"].(string)
					if row["key"] == "quarantine_path" {
						value = filepath.Base(value)
					}
					state[row["key"].(string)] = value
				}
				db.Close()
				if !reflect.DeepEqual(state, c.FinalState) {
					t.Fatal(state, c.FinalState)
				}
			}
		})
	}
}

func TestQuarantinePreservesDatabaseAndRebuilds(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	i := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	if err := i.Rebuild(ctx, root, "r1"); err != nil {
		t.Fatal(err)
	}
	db, err := openIndexDB(ctx, i.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("DROP TABLE graph_metrics"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err = i.Metrics(ctx, "id"); err == nil {
		t.Fatal("missing table should quarantine")
	}
	state, err := i.State(ctx)
	if err != nil || state.Status != "quarantined" || state.QuarantinePath == nil {
		t.Fatal(state, err)
	}
	saved, err := openIndexDB(ctx, *state.QuarantinePath)
	if err != nil {
		t.Fatal(err)
	}
	var body string
	if err = saved.QueryRow("SELECT body FROM concepts WHERE id='id'").Scan(&body); err != nil || body == "" {
		t.Fatal("quarantine content lost", body, err)
	}
	saved.Close()
	if err = i.Rebuild(ctx, root, "r2"); err != nil {
		t.Fatal(err)
	}
	next, err := i.State(ctx)
	if err != nil || next.Status != "ready" || next.IndexRevision != "r2" || next.QuarantinePath == nil || *next.QuarantinePath != *state.QuarantinePath {
		t.Fatal(next, err)
	}
	if _, err = os.Stat(*state.QuarantinePath); err != nil {
		t.Fatal("rebuild deleted quarantine", err)
	}
}
