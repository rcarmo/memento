package derived

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/repository"
	sqlite "modernc.org/sqlite"
)

// Inject a failure at every individual SQL boundary while retaining a real
// SQLite engine, including COMMIT/BEGIN after source-compatible inner commits.
type faultState struct {
	remaining int
	count     int
	fired     bool
	queries   []string
}

func (f *faultState) hit(query string) error {
	f.count++
	f.queries = append(f.queries, query)
	if f.remaining > 0 {
		f.remaining--
		if f.remaining == 0 {
			f.fired = true
			return io.ErrClosedPipe
		}
	}
	return nil
}

type faultDriver struct{ state *faultState }

func (d faultDriver) Open(name string) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(name)
	if err != nil {
		return nil, err
	}
	return faultConn{Conn: conn, state: d.state}, nil
}

type faultConn struct {
	driver.Conn
	state *faultState
}

func (c faultConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.state.hit(q); err != nil {
		return nil, err
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
}
func (c faultConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.state.hit(q); err != nil {
		return nil, err
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
}

var driverMu sync.Mutex
var driverCount int

func faultStore(t *testing.T) (ContentStore, *faultState) {
	t.Helper()
	f := &faultState{}
	driverMu.Lock()
	driverCount++
	name := fmt.Sprintf("derived-fault-%d", driverCount)
	sql.Register(name, faultDriver{f})
	driverMu.Unlock()
	db, err := sql.Open(name, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return ContentStore{DB: db}, f
}

const testConcept = "---\nid: 'id'\ntype: concept\ntitle: Title\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: actor\n---\n[self](/a.md) [external](https://example.org)\n"

func installConcept(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte(testConcept), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestEveryContentSQLFailure(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	for _, kind := range []string{"migrate", "legacy-migrate", "rebuild", "update", "empty-update", "delete-update"} {
		t.Run(kind, func(t *testing.T) {
			run := func(fail int) (int, bool, error) {
				s, f := faultStore(t)
				if kind != "migrate" {
					if err := s.Rebuild(ctx, root, "before"); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "legacy-migrate" {
					if _, err := s.DB.Exec(`DELETE FROM index_state WHERE key='external_links_classified';UPDATE links SET resolution_state='broken',target_path='/wrong',target_id='id' WHERE raw_target LIKE 'https:%'`); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "empty-update" {
					s.DeferEmbeddings = true
				}
				f.count = 0
				f.remaining = fail
				f.queries = nil
				var err error
				switch kind {
				case "migrate", "legacy-migrate":
					err = s.Migrate(ctx)
				case "rebuild":
					err = s.Rebuild(ctx, root, "after")
				case "update":
					err = s.UpdatePaths(ctx, root, "after", []string{"/a.md"})
				case "empty-update":
					err = s.UpdatePaths(ctx, root, "after", nil)
				case "delete-update":
					err = s.UpdatePaths(ctx, t.TempDir(), "after", []string{"/a.md"})
				}
				// The deferred ROLLBACK after a successful commit is expected to fail and
				// ignored. Every other injected failure must be returned to the caller.
				if f.fired && err == nil && f.queries[fail-1] != "ROLLBACK" {
					t.Fatalf("ignored SQL failure %d %s", fail, f.queries[fail-1])
				}
				s.DB.Close()
				return f.count, f.fired, err
			}
			count, _, err := run(0)
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= count; i++ {
				_, fired, _ := run(i)
				if !fired {
					t.Fatal("not injected", i)
				}
			}
		})
	}
}
func TestContentFilesystemAndCorruptRows(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	root := t.TempDir()
	installConcept(t, root)
	if err := s.Rebuild(ctx, root, "r"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../a.md", "relative.md", "/../a.md"} {
		if err := s.UpdatePaths(ctx, root, "next", []string{path}); err == nil {
			t.Fatal(path)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "dir.md"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePaths(ctx, root, "next", []string{"/dir.md"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("loop.md", filepath.Join(root, "loop.md")); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePaths(ctx, root, "next", []string{"/loop.md"}); err == nil {
		t.Fatal("stat loop")
	}
	if _, err := s.DB.Exec("UPDATE index_state SET value='quarantined' WHERE key='status'"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePaths(ctx, root, "next", nil); err == nil || err.Error() != "derived index is quarantined" {
		t.Fatal(err)
	}
	s.DB.Close()
	if err := s.contentTransaction(ctx, func(*sql.Conn) error { return nil }); err == nil {
		t.Fatal("closed connection")
	}
	if err := s.Migrate(ctx); err == nil {
		t.Fatal("closed migrate")
	}
	if _, err := Connect(ctx, filepath.Join(root, "a.md", "no")); err == nil {
		t.Fatal("invalid parent")
	}
	db := testStore(t).DB
	db.Close()
	if _, err := connect(ctx, "", func(context.Context, string) (*sql.DB, error) { return db, nil }); err == nil {
		t.Fatal("pragma failure")
	}
	if got := isoTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); got != "2026-01-01T00:00:00Z" {
		t.Fatal(got)
	}
	s = testStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	entry := repository.BundleEntry{BundlePath: "/a.md", Document: repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{Tags: []string{"\xff"}}}}
	if err := upsertEntry(ctx, s.DB, entry, "r"); err == nil {
		t.Fatal("invalid tag JSON")
	}
	entry.Document.Frontmatter.Tags = nil
	entry.Document.Frontmatter.Aliases = []string{"\xff"}
	if err := upsertEntry(ctx, s.DB, entry, "r"); err == nil {
		t.Fatal("invalid alias JSON")
	}
	s.MaxInputChars = -1
	if err := s.markEmbeddingStaleness(ctx, s.DB, []repository.BundleEntry{{Document: repository.ConceptDocument{Body: " \x1c "}}}, "r"); err != nil {
		t.Fatal(err)
	}
	for _, setup := range []string{`DROP TABLE concepts;CREATE TABLE concepts(id TEXT,path TEXT,body TEXT);INSERT INTO concepts VALUES(NULL,'/a.md','')`, `DROP TABLE links;CREATE TABLE links(raw_target TEXT,resolution_state TEXT,target_path TEXT,target_id TEXT);INSERT INTO links(raw_target) VALUES('https://example.org')`} {
		other := testStore(t)
		if err := other.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := other.DB.Exec(setup); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(setup, "DROP TABLE concepts") {
			if err := recomputeLinks(ctx, other.DB, "r"); err == nil {
				t.Fatal("bad concept row")
			}
		} else {
			if err := classifyExternal(ctx, other.DB); err != nil {
				t.Fatal(err)
			}
		}
	}
}

type badRows struct {
	scan, iteration error
	done, closed    bool
}

func (r *badRows) Next() bool {
	if r.done {
		return false
	}
	r.done = true
	return r.scan != nil
}
func (r *badRows) Scan(...any) error { return r.scan }
func (r *badRows) Err() error        { return r.iteration }
func (r *badRows) Close() error      { r.closed = true; return nil }
func TestContentRowFailures(t *testing.T) {
	for _, scan := range []bool{false, true} {
		for _, external := range []bool{false, true} {
			r := &badRows{}
			if scan {
				r.scan = io.ErrClosedPipe
			} else {
				r.iteration = io.ErrClosedPipe
			}
			var err error
			if external {
				_, err = externalRows(r)
			} else {
				_, err = contentRows(r)
			}
			if !errors.Is(err, io.ErrClosedPipe) || !r.closed {
				t.Fatal(err, r)
			}
		}
	}
}

func TestExternalClassificationMalformedRow(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`DROP TABLE links;CREATE VIEW links AS SELECT 'bad-id' AS rowid,'https://example.org' AS raw_target`); err != nil {
		t.Fatal(err)
	}
	if err := classifyExternal(ctx, s.DB); err == nil {
		t.Fatal("invalid rowid")
	}
}
