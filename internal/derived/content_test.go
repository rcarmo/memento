package derived

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testStore(t testing.TB) ContentStore {
	t.Helper()
	db, err := Connect(context.Background(), filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return ContentStore{DB: db}
}
func normal(v any) any {
	raw, _ := json.Marshal(v)
	var result any
	_ = json.Unmarshal(raw, &result)
	return result
}
func queryRows(t *testing.T, db *sql.DB, query string) []map[string]any {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	out := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err = rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		item := map[string]any{}
		for i, key := range cols {
			item[key] = values[i]
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
func snapshot(t *testing.T, db *sql.DB) map[string]any {
	t.Helper()
	out := map[string]any{}
	for _, table := range []string{"concepts", "concept_fts", "links", "graph_metrics", "index_state", "concept_embeddings"} {
		rows := queryRows(t, db, "SELECT * FROM "+table+" ORDER BY rowid")
		for _, row := range rows {
			if table == "index_state" {
				if _, err := time.Parse("2006-01-02 15:04:05", row["updated_at"].(string)); err != nil {
					t.Fatal(err)
				}
				delete(row, "updated_at")
			}
			if table == "concept_embeddings" {
				if value, ok := row["embedding_blob"].([]byte); ok {
					row["embedding_blob"] = hex.EncodeToString(value)
				}
			}
		}
		out[table] = rows
	}
	return out
}
func TestContentReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/derived-content.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Sequences []struct {
			Defer bool
			Steps []struct {
				Action, Revision string
				Paths            []string
				Files            map[string]*string
				ErrorType        string `json:"error_type"`
				Snapshot         map[string]any
			}
		}
		Migrations []struct {
			Scenario   string
			Statements []string
			ErrorType  string `json:"error_type"`
			Snapshot   map[string]any
			Schema     []map[string]any
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, sequence := range fixture.Sequences {
		s := testStore(t)
		s.DeferEmbeddings = sequence.Defer
		root := t.TempDir()
		if err = s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
		for _, step := range sequence.Steps {
			for path, text := range step.Files {
				target := filepath.Join(root, path)
				if text == nil {
					if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
				} else {
					if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(target, []byte(*text), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			switch step.Action {
			case "seed-embeddings":
				// Fixture snapshot contains exact source rows (including text hashes).
				rows := step.Snapshot["concept_embeddings"].([]any)
				for _, value := range rows {
					r := value.(map[string]any)
					blob, err := hex.DecodeString(r["embedding_blob"].(string))
					if err != nil {
						t.Fatal(err)
					}
					if _, err = s.DB.Exec(`INSERT INTO concept_embeddings VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, r["concept_id"], r["path"], r["embedding_text_hash"], r["model_id"], r["dimensions"], r["embedding_revision"], r["status"], r["model_revision"], blob, r["embedding_norm"], r["updated_at"], r["error_message"]); err != nil {
						t.Fatal(err)
					}
				}
			case "rebuild", "duplicate-rebuild":
				err = s.Rebuild(ctx, root, step.Revision)
			case "update":
				err = s.UpdatePaths(ctx, root, step.Revision, step.Paths)
			}
			if (err != nil) != (step.ErrorType != "") {
				t.Fatal(sequence.Defer, step.Action, step.Revision, err, step.ErrorType)
			}
			got := snapshot(t, s.DB)
			if !reflect.DeepEqual(normal(got), step.Snapshot) {
				for table, want := range step.Snapshot {
					if !reflect.DeepEqual(normal(got[table]), want) {
						t.Error(sequence.Defer, step.Revision, table, got[table], want)
					}
				}
				t.FailNow()
			}
		}
	}
	for _, c := range fixture.Migrations {
		t.Run(c.Scenario, func(t *testing.T) {
			s := testStore(t)
			if err := s.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			for _, statement := range c.Statements {
				if _, err := s.DB.Exec(statement); err != nil {
					t.Fatal(err)
				}
			}
			err := s.Migrate(ctx)
			if (err != nil) != (c.ErrorType != "") {
				t.Fatal(err, c.ErrorType)
			}
			if got := snapshot(t, s.DB); !reflect.DeepEqual(normal(got), c.Snapshot) {
				t.Fatal(got, c.Snapshot)
			}
			// Ignore only SQL formatting, not names/columns/types/index definitions.
			got := queryRows(t, s.DB, "SELECT name,sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY name")
			for _, rows := range [][]map[string]any{got, c.Schema} {
				for _, r := range rows {
					r["sql"] = strings.Join(strings.Fields(r["sql"].(string)), " ")
				}
			}
			if !reflect.DeepEqual(normal(got), normal(c.Schema)) {
				t.Fatal(got, c.Schema)
			}
		})
	}
}
