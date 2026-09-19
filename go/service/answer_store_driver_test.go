package service

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
)

var answerDriverID atomic.Int64

type answerFaultDriver struct{ mode string }
type answerFaultConn struct{ mode string }
type answerFaultRows struct {
	mode string
	done bool
}

func (d answerFaultDriver) Open(string) (driver.Conn, error) {
	return answerFaultConn(d), nil
}
func (answerFaultConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("prepare") }
func (answerFaultConn) Close() error                        { return nil }
func (answerFaultConn) Begin() (driver.Tx, error)           { return nil, errors.New("begin") }
func (c answerFaultConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}
func (c answerFaultConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	if c.mode == "query" {
		return nil, errors.New("query")
	}
	return &answerFaultRows{mode: c.mode}, nil
}
func (r *answerFaultRows) Columns() []string {
	if r.mode == "scan" {
		return []string{"one", "two"}
	}
	return []string{"concept_id"}
}
func (r *answerFaultRows) Close() error {
	if r.mode == "close" {
		return errors.New("close")
	}
	return nil
}
func (r *answerFaultRows) Next(values []driver.Value) error {
	if (r.mode == "scan" || r.mode == "close") && !r.done {
		r.done = true
		values[0] = "x"
		if len(values) > 1 {
			values[1] = "y"
		}
		return nil
	}
	if r.mode == "close" {
		return errors.New("rows")
	}
	return io.EOF
}
func faultAnswerDB(t *testing.T, mode string) *sql.DB {
	t.Helper()
	name := fmt.Sprintf("answer-fault-%d", answerDriverID.Add(1))
	sql.Register(name, answerFaultDriver{mode})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
func TestAnswerStoreDriverFaults(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"query", "scan", "close"} {
		t.Run(mode, func(t *testing.T) {
			s := AnswerStore{DB: faultAnswerDB(t, mode)}
			if _, _, err := s.GetHotContext(ctx, "s", "q", "m", "r"); err == nil {
				t.Fatal(mode)
			}
		})
	}
	s := AnswerStore{DB: faultAnswerDB(t, "query")}
	if err := s.InvalidateHot(context.Background(), map[string]bool{"x": true}); err == nil {
		t.Fatal("invalidate query")
	}
}
