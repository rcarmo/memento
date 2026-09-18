package graphdebug

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

type snapshotFaultDriver struct{}
type snapshotFaultConn struct{}
type snapshotFaultRows struct {
	mode    string
	emitted bool
	count   int
}

var snapshotFaultMode atomic.Value
var snapshotFaultRegistered atomic.Bool

func (snapshotFaultDriver) Open(string) (driver.Conn, error)  { return snapshotFaultConn{}, nil }
func (snapshotFaultConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("prepare") }
func (snapshotFaultConn) Close() error                        { return nil }
func (snapshotFaultConn) Begin() (driver.Tx, error)           { return nil, errors.New("begin") }
func (snapshotFaultConn) Ping(context.Context) error {
	if snapshotFaultMode.Load().(string) == "ping" {
		return io.ErrClosedPipe
	}
	return nil
}
func (snapshotFaultConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	mode := snapshotFaultMode.Load().(string)
	if mode == "query" {
		return nil, io.ErrClosedPipe
	}
	if len(query) >= 12 && query[:12] == "SELECT id,pa" {
		return &snapshotFaultRows{mode: "paths-" + mode}, nil
	}
	if len(query) >= 12 && query[:12] == "SELECT rowid" {
		return &snapshotFaultRows{mode: "edges-" + mode}, nil
	}
	if strings.HasPrefix(query, "SELECT status,") {
		return &snapshotFaultRows{mode: "proposals-" + mode}, nil
	}
	if strings.HasPrefix(query, "SELECT concept_id,embedding_blob") {
		return &snapshotFaultRows{mode: "semantic-" + mode}, nil
	}
	if strings.HasPrefix(query, "SELECT c.id,c.path,c.title,c.type,c.tags_json,snippet") {
		return &snapshotFaultRows{mode: "search-" + mode}, nil
	}
	if strings.HasPrefix(query, "SELECT c.id") {
		return &snapshotFaultRows{mode: "nodes-" + mode}, nil
	}
	return &snapshotFaultRows{mode: "revisions-" + mode}, nil
}
func (r *snapshotFaultRows) Columns() []string {
	if len(r.mode) >= 6 && r.mode[:6] == "paths-" {
		return []string{"id", "path"}
	}
	if len(r.mode) >= 6 && r.mode[:6] == "edges-" {
		return []string{"rowid", "source_id", "target_id", "raw_target", "resolution_state", "anchor", "first_seen_revision", "last_checked_revision"}
	}
	if len(r.mode) >= 6 && r.mode[:6] == "nodes-" {
		return make([]string, 18)
	}
	if len(r.mode) >= 10 && r.mode[:10] == "proposals-" {
		return []string{"status", "patch_json", "author_principal"}
	}
	if len(r.mode) >= 9 && r.mode[:9] == "semantic-" {
		return []string{"concept_id", "embedding_blob", "embedding_norm", "model_id", "embedding_revision"}
	}
	if len(r.mode) >= 7 && r.mode[:7] == "search-" {
		return []string{"id", "path", "title", "type", "tags_json", "snippet"}
	}
	return []string{"key", "value"}
}
func (*snapshotFaultRows) Close() error { return nil }
func (r *snapshotFaultRows) Next(values []driver.Value) error {
	if r.emitted {
		if r.mode == "revisions-rows" || r.mode == "paths-rows" || r.mode == "edges-rows" || r.mode == "nodes-rows" || r.mode == "proposals-rows" || r.mode == "semantic-rows" || r.mode == "search-rows" {
			return io.ErrClosedPipe
		}
		if r.mode != "edges-multi" || r.count >= 2 {
			return io.EOF
		}
	}
	r.emitted = true
	r.count++
	if r.mode == "revisions-scan" || r.mode == "paths-scan" || r.mode == "edges-scan" || r.mode == "nodes-scan" || r.mode == "proposals-scan" || r.mode == "semantic-scan" || r.mode == "search-scan" {
		values[0] = nil
		values[1] = "value"
	} else if len(r.mode) >= 7 && r.mode[:7] == "search-" {
		values[0] = "a"
		values[1] = "/a"
		values[2] = "Title"
		values[3] = "concept"
		values[4] = "[]"
		values[5] = ""
	} else if len(r.mode) >= 9 && r.mode[:9] == "semantic-" {
		values[0] = "a"
		values[1] = blob(1, 0)
		values[2] = float64(1)
		values[3] = "m"
		values[4] = "main"
	} else if len(r.mode) >= 10 && r.mode[:10] == "proposals-" {
		values[0] = "draft"
		values[1] = `{"path":"/a"}`
		values[2] = "actor"
	} else if len(r.mode) >= 6 && r.mode[:6] == "nodes-" {
		values[0] = "id"
		values[1] = "/a.md"
		values[2] = "Title"
		values[3] = "concept"
		values[4] = "active"
		values[5] = "[]"
		values[6] = "now"
		values[7] = int64(0)
		values[8] = int64(0)
		values[9] = int64(0)
		values[10] = int64(0)
		for i := 11; i < 18; i++ {
			values[i] = nil
		}
	} else if len(r.mode) >= 6 && r.mode[:6] == "edges-" {
		values[0] = int64(1)
		values[1] = "a"
		values[2] = "b"
		values[3] = "/b"
		values[4] = "resolved"
		values[5] = nil
		values[6] = "r"
		values[7] = "r"
	} else if len(r.mode) >= 6 && r.mode[:6] == "paths-" {
		values[0] = "a"
		values[1] = "/a.md"
	} else {
		values[0] = "repo_revision"
		values[1] = "main"
	}
	return nil
}

func faultOpen(t *testing.T, mode string) func(context.Context, string) (*sql.DB, error) {
	t.Helper()
	snapshotFaultMode.Store(mode)
	return func(context.Context, string) (*sql.DB, error) { return sql.Open("graphdebug-fault", "") }
}
func TestSnapshotDatabaseFailureBranches(t *testing.T) {
	if snapshotFaultRegistered.CompareAndSwap(false, true) {
		sql.Register("graphdebug-fault", snapshotFaultDriver{})
	}
	ctx := context.Background()
	service := NewSnapshotService("root", "derived", "control")
	service.open = func(context.Context, string) (*sql.DB, error) { return nil, io.ErrClosedPipe }
	if _, err := service.Nodes(ctx, nil, 1, nil, false); err == nil {
		t.Fatal("nodes open")
	}
	if _, err := service.Revisions(ctx); err == nil {
		t.Fatal("revision open")
	}
	if _, err := service.PathsForIDs(ctx, []string{"a"}); err == nil {
		t.Fatal("paths open")
	}
	for _, mode := range []string{"query", "scan", "rows"} {
		service.open = faultOpen(t, mode)
		if _, err := service.Revisions(ctx); err == nil {
			t.Fatal("revision", mode)
		}
		service.open = faultOpen(t, mode)
		if _, err := service.PathsForIDs(ctx, []string{"a"}); err == nil {
			t.Fatal("paths", mode)
		}
		service.open = faultOpen(t, mode)
		if _, err := service.ExplicitEdges(ctx, nil, nil, nil, 2, nil); err == nil {
			t.Fatal("edges", mode)
		}
		fault := faultOpen(t, mode)
		control := emptyControlDB(t)
		service.ControlDBPath = control
		service.open = func(ctx context.Context, path string) (*sql.DB, error) {
			if path == control {
				return openReadDB(ctx, path)
			}
			return fault(ctx, path)
		}
		if _, err := service.Nodes(ctx, nil, 2, nil, false); err == nil {
			t.Fatal("nodes", mode)
		}
	}
	service.open = faultOpen(t, "multi")
	edges, err := service.ExplicitEdges(ctx, []string{"x"}, nil, nil, 1, nil)
	if err != nil || len(edges) != 0 {
		t.Fatal(edges, err)
	}
	service.open = faultOpen(t, "multi")
	edges, err = service.ExplicitEdges(ctx, nil, nil, nil, 1, nil)
	if err != nil || len(edges) != 1 {
		t.Fatal(edges, err)
	}
}
func TestOpenReadDBPingFailure(t *testing.T) {
	original := openSQL
	defer func() { openSQL = original }()
	openSQL = func(string, string) (*sql.DB, error) { return nil, io.ErrClosedPipe }
	if _, err := openReadDB(context.Background(), "unused"); err == nil {
		t.Fatal("open")
	}
	openSQL = original
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := openReadDB(ctx, snapshotDB(t)); err == nil {
		t.Fatal("cancelled ping")
	}
}
