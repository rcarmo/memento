package service

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/derived"
)

type metricsRowsStub struct {
	remaining int
	scanErr   error
	finalErr  error
	closeErr  error
}

func (r *metricsRowsStub) Next() bool {
	if r.remaining > 0 {
		r.remaining--
		return true
	}
	return false
}
func (r *metricsRowsStub) Scan(values ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*values[0].(*string) = "ready"
	*values[1].(*int64) = 2
	return nil
}
func (r *metricsRowsStub) Err() error   { return r.finalErr }
func (r *metricsRowsStub) Close() error { return r.closeErr }

type metricsFileInfo struct{ size int64 }

func (i metricsFileInfo) Name() string       { return "x" }
func (i metricsFileInfo) Size() int64        { return i.size }
func (i metricsFileInfo) Mode() fs.FileMode  { return 0 }
func (i metricsFileInfo) ModTime() time.Time { return time.Time{} }
func (i metricsFileInfo) IsDir() bool        { return false }
func (i metricsFileInfo) Sys() any           { return nil }

func TestReadEmbeddingMetricsFailures(t *testing.T) {
	boom := errors.New("boom")
	for name, rows := range map[string]*metricsRowsStub{
		"scan":  {remaining: 1, scanErr: boom},
		"rows":  {finalErr: boom},
		"close": {closeErr: boom},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readEmbeddingMetrics(rows); !errors.Is(err, boom) {
				t.Fatal(err)
			}
		})
	}
	result, err := readEmbeddingMetrics(&metricsRowsStub{remaining: 1})
	if err != nil || result["ready"] != 2 || result["pending"] != 0 {
		t.Fatal(result, err)
	}
}

func TestOpenReadOnlyMetricsDBFailures(t *testing.T) {
	boom := errors.New("boom")
	if _, err := openReadOnlyMetricsDBWith(t.Context(), "/x", func(string, string) (*sql.DB, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	closed, _ := sql.Open("sqlite", ":memory:")
	_ = closed.Close()
	if _, err := openReadOnlyMetricsDBWith(t.Context(), "/x", func(string, string) (*sql.DB, error) { return closed, nil }); err == nil {
		t.Fatal("closed database")
	}
}

func TestCollectSQLiteMetricsFailures(t *testing.T) {
	boom := errors.New("boom")
	stat := func(path string) (fs.FileInfo, error) {
		if path == "x" {
			return metricsFileInfo{size: 5}, nil
		}
		return nil, os.ErrNotExist
	}
	queries := 0
	value, err := collectSQLiteMetricsWith(nil, "x", stat, func(string, *int64) error { queries++; return nil })
	if err != nil || value.MainBytes != 5 || value.WALBytes != 0 || queries != 2 {
		t.Fatal(value, queries, err)
	}
	if _, err = collectSQLiteMetricsWith(nil, "x", func(string) (fs.FileInfo, error) { return nil, boom }, func(string, *int64) error { return nil }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	for _, failAt := range []int{1, 2} {
		calls := 0
		if _, err = collectSQLiteMetricsWith(nil, "x", stat, func(string, *int64) error {
			calls++
			if calls == failAt {
				return boom
			}
			return nil
		}); !errors.Is(err, boom) {
			t.Fatal(failAt, err)
		}
	}
}

func TestCollectLiveMetricsInjectedFailures(t *testing.T) {
	boom := errors.New("boom")
	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()
	service := &Runtime{DB: db, Paths: RuntimePaths{ControlDB: "c", DerivedDB: "d"}, Metrics: NewRuntimeMetrics()}
	base := liveMetricsOps{
		state: func(context.Context, string) (derived.IndexState, error) {
			return derived.IndexState{Status: "ready", RepoRevision: "r", IndexRevision: "r"}, nil
		},
		control:        func(*sql.DB, string) (sqliteMetrics, error) { return sqliteMetrics{}, nil },
		openDerived:    func(context.Context, string) (*sql.DB, error) { return sql.Open("sqlite", ":memory:") },
		derived:        func(*sql.DB, string) (sqliteMetrics, error) { return sqliteMetrics{}, nil },
		counts:         func(context.Context, *sql.DB, *liveMetricsSnapshot) error { return nil },
		embeddings:     func(context.Context, *sql.DB) (map[string]int64, error) { return map[string]int64{}, nil },
		processRuntime: func(*liveMetricsSnapshot) {},
	}
	for _, field := range []string{"state", "control", "open", "derived", "counts", "embeddings"} {
		ops := base
		switch field {
		case "state":
			ops.state = func(context.Context, string) (derived.IndexState, error) { return derived.IndexState{}, boom }
		case "control":
			ops.control = func(*sql.DB, string) (sqliteMetrics, error) { return sqliteMetrics{}, boom }
		case "open":
			ops.openDerived = func(context.Context, string) (*sql.DB, error) { return nil, boom }
		case "derived":
			ops.derived = func(*sql.DB, string) (sqliteMetrics, error) { return sqliteMetrics{}, boom }
		case "counts":
			ops.counts = func(context.Context, *sql.DB, *liveMetricsSnapshot) error { return boom }
		case "embeddings":
			ops.embeddings = func(context.Context, *sql.DB) (map[string]int64, error) { return nil, boom }
		}
		if _, err := collectLiveMetricsWith(t.Context(), service, ops); !errors.Is(err, boom) {
			t.Fatal(field, err)
		}
	}
	service.ServiceVersion = ""
	worker := derived.NewSemanticWorker(nil, nil, derived.SemanticRefreshConfig{})
	defer worker.Close()
	service.SemanticWorker = worker
	snapshot, err := collectLiveMetricsWith(t.Context(), service, base)
	if err != nil || snapshot.ServiceVersion != "unknown" || !snapshot.WorkerAvailable || !snapshot.Worker.Alive {
		t.Fatal(snapshot, err)
	}
}

func TestCollectEmbeddingMetricsQueryFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = collectEmbeddingMetrics(t.Context(), db); err == nil {
		t.Fatal("missing table")
	}
}

func TestLiveMetricsDefaultHandlerFailure(t *testing.T) {
	handler := LiveMetricsHTTP{}
	response, err := handler.Handle(t.Context(), "GET", liveMetricsPath, nil, nil, "")
	if err != nil || response.Status != 200 || len(response.Body) == 0 {
		t.Fatal(response, err)
	}
}
