package graphdebug

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync/atomic"
	"testing"
)

type snapshotFaultDriver struct{}
type snapshotFaultConn struct{}
type snapshotFaultRows struct {
	mode    string
	emitted bool
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
	return &snapshotFaultRows{mode: "revisions-" + mode}, nil
}
func (r *snapshotFaultRows) Columns() []string {
	if len(r.mode) >= 6 && r.mode[:6] == "paths-" {
		return []string{"id", "path"}
	}
	return []string{"key", "value"}
}
func (*snapshotFaultRows) Close() error { return nil }
func (r *snapshotFaultRows) Next(values []driver.Value) error {
	if r.emitted {
		if r.mode == "revisions-rows" || r.mode == "paths-rows" {
			return io.ErrClosedPipe
		}
		return io.EOF
	}
	r.emitted = true
	if r.mode == "revisions-scan" || r.mode == "paths-scan" {
		values[0] = nil
		values[1] = "value"
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
