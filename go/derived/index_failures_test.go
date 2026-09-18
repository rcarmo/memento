package derived

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
)

type badHeaderReader struct {
	io.Reader
	readErr, closeErr error
}

func (r badHeaderReader) Read(p []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	return r.Reader.Read(p)
}
func (r badHeaderReader) Close() error { return r.closeErr }
func TestIndexFileErrors(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "index.sqlite")
	ops := defaultIndexIO()
	if err := os.WriteFile(path, []byte("SQLite format 3\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"stat", "open", "read", "close", "short"} {
		local := ops
		switch stage {
		case "stat":
			local.stat = func(string) (fs.FileInfo, error) { return nil, io.ErrClosedPipe }
		case "open":
			local.openFile = func(string) (io.ReadCloser, error) { return nil, io.ErrClosedPipe }
		case "read":
			local.openFile = func(string) (io.ReadCloser, error) { return badHeaderReader{readErr: io.ErrClosedPipe}, nil }
		case "close":
			local.openFile = func(string) (io.ReadCloser, error) {
				return badHeaderReader{Reader: bytes.NewReader([]byte("SQLite format 3\x00")), closeErr: io.ErrClosedPipe}, nil
			}
		case "short":
			local.openFile = func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(nil)), nil }
		}
		if err := validateIndexHeader(path, local); err == nil {
			t.Fatal(stage)
		} else if stage == "short" && err.Error() != "invalid sqlite header" {
			t.Fatal(err)
		}
	}
	if _, err := openIndexDB(ctx, filepath.Join(path, "file")); err == nil {
		t.Fatal("parent file")
	}
	if _, err := openIndexDBWith(ctx, path, func(string, string) (*sql.DB, error) { return nil, io.ErrClosedPipe }); err == nil {
		t.Fatal("sql open")
	}
	if _, err := openIndexDB(ctx, t.TempDir()); err == nil {
		t.Fatal("directory DB")
	}
	if !transientDatabaseError(errors.New("DATABASE IS BUSY")) {
		t.Fatal("busy")
	}
}
func TestIndexConnectFailures(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"open", "stat", "stat-after", "pragma", "migrate"} {
		t.Run(stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "index.sqlite")
			if err := os.WriteFile(path, nil, 0600); err != nil {
				t.Fatal(err)
			}
			i := &Index{Path: path}
			ops := defaultIndexIO()
			s, f := faultStore(t)
			ops.openDB = func(context.Context, string) (*sql.DB, error) { return s.DB, nil }
			switch stage {
			case "open":
				ops.openDB = func(context.Context, string) (*sql.DB, error) { return nil, io.ErrClosedPipe }
			case "stat":
				ops.stat = func(string) (fs.FileInfo, error) { return nil, io.ErrClosedPipe }
			case "stat-after":
				calls := 0
				ops.stat = func(path string) (fs.FileInfo, error) {
					calls++
					if calls == 2 {
						return nil, io.ErrClosedPipe
					}
					return os.Stat(path)
				}
			case "pragma":
				f.remaining = 1
			case "migrate":
				f.remaining = 3
			}
			if _, err := i.connect(ctx, ops); err == nil {
				t.Fatal(stage)
			}
		})
	}
}
func TestQuarantineFailuresAndSidecars(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"missing", "stat", "collision", "target-stat", "rename", "side-stat", "side-remove", "success"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "index.sqlite")
			i := &Index{Path: path, Now: func() time.Time { return time.UnixMilli(123) }}
			ops := defaultIndexIO()
			if stage != "missing" {
				if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			target := filepath.Join(root, "index.quarantine-123.sqlite")
			if stage == "collision" {
				if err := os.WriteFile(target, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if stage == "stat" {
				ops.stat = func(string) (fs.FileInfo, error) { return nil, io.ErrClosedPipe }
			}
			if stage == "target-stat" {
				ops.stat = func(p string) (fs.FileInfo, error) {
					if p == target {
						return nil, io.ErrClosedPipe
					}
					return os.Stat(p)
				}
			}
			if stage == "rename" {
				ops.rename = func(string, string) error { return io.ErrClosedPipe }
			}
			if stage == "side-stat" {
				ops.stat = func(p string) (fs.FileInfo, error) {
					if p == path+"-wal" {
						return nil, io.ErrClosedPipe
					}
					return os.Stat(p)
				}
			}
			if stage == "side-remove" || stage == "success" {
				for _, suffix := range []string{"-wal", "-shm"} {
					if err := os.WriteFile(path+suffix, nil, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if stage == "side-remove" {
				ops.remove = func(string) error { return io.ErrClosedPipe }
			}
			got, err := i.quarantine(ops)
			if stage == "success" {
				if err != nil || got != target {
					t.Fatal(got, err)
				}
				for _, suffix := range []string{"-wal", "-shm"} {
					if _, err = os.Stat(path + suffix); !os.IsNotExist(err) {
						t.Fatal(err)
					}
				}
			} else if stage == "missing" {
				if err != nil || got != filepath.Join(root, "index.missing") {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal(stage)
			}
		})
	}
	for _, stage := range []string{"quarantine", "connect", "state", "path"} {
		t.Run("after/"+stage, func(t *testing.T) {
			i := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
			ops := defaultIndexIO()
			if stage == "quarantine" {
				ops.stat = func(string) (fs.FileInfo, error) { return nil, io.ErrClosedPipe }
			}
			if stage == "connect" {
				ops.openDB = func(context.Context, string) (*sql.DB, error) { return nil, io.ErrClosedPipe }
			}
			if stage == "state" || stage == "path" {
				original := ops.openDB
				ops.openDB = func(ctx context.Context, path string) (*sql.DB, error) {
					db, err := original(ctx, path)
					if err != nil {
						return nil, err
					}
					if err = (ContentStore{DB: db}).Migrate(ctx); err != nil {
						return nil, err
					}
					value := "status"
					if stage == "path" {
						value = "quarantine_path"
					}
					_, err = db.Exec("CREATE TRIGGER blocked BEFORE INSERT ON index_state WHEN NEW.key='" + value + "' BEGIN SELECT RAISE(ABORT,'fail');END")
					return db, err
				}
			}
			if err := i.quarantineAfter(ctx, io.ErrClosedPipe, ops); err == nil {
				t.Fatal(stage)
			}
		})
	}
}
func TestIndexPublicReadsAndWaiting(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	installConcept(t, root)
	i := &Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	if err := i.Rebuild(ctx, root, "r1"); err != nil {
		t.Fatal(err)
	}
	if got, err := i.Status(ctx, policy); err != nil || got.VisibleConcepts != 1 {
		t.Fatal(got, err)
	}
	if _, err := i.Metrics(ctx, "id"); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Graph(ctx, policy, "id", GraphOptions{Depth: 2, Strict: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := i.SearchLexical(ctx, policy, SearchOptions{Query: "self", Syntax: "plain", Strict: true}); err != nil {
		t.Fatal(err)
	}
	if err := i.SetRepoRevision(ctx, "r2"); err != nil {
		t.Fatal(err)
	}
	if _, err := i.Graph(ctx, policy, "id", GraphOptions{Strict: true}); err == nil {
		t.Fatal("stale graph")
	}
	if _, err := i.SearchLexical(ctx, policy, SearchOptions{Query: "self", Syntax: "plain", Strict: true}); err == nil {
		t.Fatal("stale search")
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		if err := i.UpdatePaths(ctx, root, "r2", nil); err != nil {
			t.Error(err)
		}
	}()
	if state, err := i.WaitForFreshness(ctx, time.Second); err != nil || state.IndexRevision != "r2" {
		t.Fatal(state, err)
	}
	wg.Wait()
	if err := i.withCore(ctx, false, func(ContentStore) error { return io.ErrClosedPipe }); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}
