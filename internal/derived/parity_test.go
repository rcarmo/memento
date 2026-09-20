package derived

import (
	"context"
	"database/sql"
	"errors"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"testing"
)

func parityRoot(t *testing.T) string {
	root := t.TempDir()
	raw := `---
id: '12345678'
type: concept
title: One
status: active
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
updated_by: test
---
Body.
`
	if err := os.WriteFile(filepath.Join(root, "one.md"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}
func parityIndex(t *testing.T, root, revision string) *Index {
	t.Helper()
	index := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	if err := index.Rebuild(context.Background(), root, revision); err != nil {
		t.Fatal(err)
	}
	return index
}
func TestParityCheck(t *testing.T) {
	root := parityRoot(t)
	one := parityIndex(t, root, "r")
	two := parityIndex(t, root, "r")
	report, err := one.ParityCheck(context.Background(), two, "r")
	if err != nil || !report.Matches || report.Details != "match" || report.CurrentRevision != "r" {
		t.Fatal(report, err)
	}
	db, err := sql.Open("sqlite", two.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE concepts SET title='changed'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	report, err = one.ParityCheck(context.Background(), two, "r")
	if err != nil || report.Matches || report.Details != "normalized derived state differs" {
		t.Fatal(report, err)
	}
}

type fakeParityRows struct {
	columns                    []string
	columnErr, scanErr, rowErr error
	rows                       [][]any
	at                         int
}

func (f *fakeParityRows) Columns() ([]string, error) { return f.columns, f.columnErr }
func (f *fakeParityRows) Next() bool                 { return f.at < len(f.rows) }
func (f *fakeParityRows) Scan(targets ...any) error {
	if f.scanErr != nil {
		return f.scanErr
	}
	for i, value := range f.rows[f.at] {
		*(targets[i].(*any)) = value
	}
	f.at++
	return nil
}
func (f *fakeParityRows) Err() error { return f.rowErr }
func TestDumpRows(t *testing.T) {
	boom := errors.New("boom")
	for _, rows := range []*fakeParityRows{{columnErr: boom}, {columns: []string{"x"}, rows: [][]any{{"x"}}, scanErr: boom}, {columns: []string{"x"}, rowErr: boom}} {
		if _, err := dumpRows(rows); !errors.Is(err, boom) {
			t.Fatal(err)
		}
	}
	rows, err := dumpRows(&fakeParityRows{columns: []string{"x", "y"}, rows: [][]any{{[]byte("bytes"), int64(2)}}})
	if err != nil || rows[0][0] != "bytes" || rows[0][1] != int64(2) {
		t.Fatal(rows, err)
	}
}
func TestRunParityQueriesFailures(t *testing.T) {
	boom := errors.New("boom")
	if _, err := runParityQueries(func(string) (parityRows, func(), error) { return nil, func() {}, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	closed := false
	if _, err := runParityQueries(func(string) (parityRows, func(), error) {
		return &fakeParityRows{columns: []string{"x"}, rows: [][]any{{1}}, scanErr: boom}, func() { closed = true }, nil
	}); !errors.Is(err, boom) || !closed {
		t.Fatal(err, closed)
	}
}
func TestParityFailures(t *testing.T) {
	ctx := context.Background()
	root := parityRoot(t)
	good := parityIndex(t, root, "r")
	badPath := filepath.Join(t.TempDir(), "bad.sqlite")
	if err := os.WriteFile(badPath, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	bad := &Index{Path: badPath}
	if _, err := bad.ParityCheck(ctx, good, "r"); err == nil {
		t.Fatal("state")
	}
	db, err := sql.Open("sqlite", good.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("DROP TABLE links"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err = good.ParityCheck(ctx, bad, "r"); err == nil {
		t.Fatal("dump")
	}
	clean := parityIndex(t, root, "r")
	cleanDB, err := sql.Open("sqlite", clean.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cleanDB.Exec("DROP TABLE links"); err != nil {
		t.Fatal(err)
	}
	cleanDB.Close()
	fresh := parityIndex(t, root, "r")
	if _, err = fresh.ParityCheck(ctx, clean, "r"); err == nil {
		t.Fatal("clean dump")
	}
}
