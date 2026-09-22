package derived

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"
)

var registerGraphRows sync.Once

type graphRowsDriver struct{}
type graphRowsConn struct{ mode string }
type graphRows struct {
	columns []string
	scan    bool
	done    bool
}

func (graphRowsDriver) Open(mode string) (driver.Conn, error) { return graphRowsConn{mode}, nil }
func (graphRowsConn) Prepare(string) (driver.Stmt, error)     { return nil, io.ErrClosedPipe }
func (graphRowsConn) Close() error                            { return nil }
func (graphRowsConn) Begin() (driver.Tx, error)               { return nil, io.ErrClosedPipe }
func (graphRowsConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}
func (c graphRowsConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	columns := []string{"id"}
	if strings.Contains(q, "SELECT embedding_blob") {
		columns = []string{"blob"}
	}
	if strings.Contains(q, "SELECT g.concept_id") {
		columns = []string{"id", "blob", "norm"}
	}
	return &graphRows{columns: columns, scan: c.mode == "scan"}, nil
}
func (r *graphRows) Columns() []string { return r.columns }
func (*graphRows) Close() error        { return nil }
func (r *graphRows) Next(values []driver.Value) error {
	if r.done {
		return io.ErrClosedPipe
	}
	r.done = true
	if r.scan {
		values[0] = struct{}{}
		return nil
	}
	if len(values) == 3 {
		values[0] = "peer"
		values[1] = semanticBlob(1, 0)
		values[2] = float64(1)
	} else if r.columns[0] == "blob" {
		values[0] = semanticBlob(1, 0)
	} else {
		values[0] = "item"
	}
	return nil
}
func TestSemanticGraphReadErrors(t *testing.T) {
	registerGraphRows.Do(func() { sql.Register("semantic-graph-rows", graphRowsDriver{}) })
	for _, mode := range []string{"scan", "rows"} {
		db, err := sql.Open("semantic-graph-rows", mode)
		if err != nil {
			t.Fatal(err)
		}
		identity := semanticGraphIdentity{conceptID: "item", dimensions: 2}
		if err = prepareSemanticGraphCache(t.Context(), db); err == nil {
			t.Fatal(mode, "prepare")
		}
		if _, _, _, err = semanticGraphAggregate(t.Context(), db, identity); err == nil {
			t.Fatal(mode, "aggregate")
		}
		if _, err = semanticGraphPeerRows(t.Context(), db, identity); err == nil {
			t.Fatal(mode, "peers")
		}
		db.Close()
	}
}
