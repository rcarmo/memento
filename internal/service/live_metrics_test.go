package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/derived"
)

func TestLiveMetricsHTTP(t *testing.T) {
	snapshot := liveMetricsSnapshot{
		ServiceVersion: "1.0.3\"\n\\", RepoRevision: "repo\nrevision", IndexRevision: "index\\revision", IndexReady: true, IndexStale: true,
		Concepts: 2, Links: 3, GraphMetrics: 2, Embedding: map[string]int64{"ready": 2, "error": 1},
		Control:         sqliteMetrics{MainBytes: 10, WALBytes: 11, SHMBytes: 12, Pages: 13, FreePages: 14},
		Derived:         sqliteMetrics{MainBytes: 20, WALBytes: 21, SHMBytes: 22, Pages: 23, FreePages: 24},
		WorkerAvailable: true, Worker: derived.SemanticWorkerState{Alive: true, Running: true, Pending: true, Completed: 7},
		IndexOperations: map[string]indexOperationMetric{"rebuild": {Success: 1, Error: 2, LastDuration: 3 * time.Second}, "update": {Success: 4, Error: 5, LastDuration: 6 * time.Second}},
		HeapAlloc:       30, HeapInUse: 31, HeapObjects: 32, Goroutines: 33,
	}
	calls := 0
	now := []time.Time{time.Unix(1, 0), time.Unix(1, 250000000)}
	handler := LiveMetricsHTTP{Collect: func(context.Context, *Runtime) (liveMetricsSnapshot, error) { calls++; return snapshot, nil }, Now: func() time.Time { value := now[0]; now = now[1:]; return value }}
	if response, err := handler.Handle(t.Context(), "GET", "/other", nil, nil, ""); err != nil || response != nil {
		t.Fatal(response, err)
	}
	for _, tc := range []struct {
		method string
		body   []byte
		status int
	}{{"POST", nil, 405}, {"GET", []byte("x"), 400}} {
		response, err := handler.Handle(t.Context(), tc.method, liveMetricsPath, nil, tc.body, "")
		if err != nil || response.Status != tc.status {
			t.Fatal(response, err)
		}
	}
	response, err := handler.Handle(t.Context(), "GET", liveMetricsPath, nil, nil, "")
	if err != nil || response.Status != 200 || calls != 1 || response.ContentType == nil || !strings.Contains(*response.ContentType, "text/plain") {
		t.Fatal(response, calls, err)
	}
	text := string(response.Body)
	for _, want := range []string{
		"memento_metrics_collect_success 1", "memento_metrics_collect_duration_seconds 0.25", `memento_build_info{version="1.0.3\"\n\\"} 1`,
		`memento_index_rows{table="links"} 3`, `memento_embedding_rows{status="ready"} 2`, `memento_embedding_rows{status="error"} 1`, `memento_embedding_rows{status="missing"} 0`, `memento_embedding_rows{status="other"} 0`,
		`memento_sqlite_file_bytes{database="derived",kind="wal"} 21`, `memento_sqlite_pages{database="control",state="free"} 14`, `memento_embedding_worker{state="running"} 1`,
		"memento_embedding_worker_completed_total 7", `memento_index_operations_total{operation="rebuild",result="error"} 2`,
		`memento_index_operation_last_duration_seconds{operation="update"} 6`, `memento_go_memory_bytes{kind="alloc"} 30`, "memento_go_goroutines 33",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "memento_index_revision_info") {
		t.Fatalf("unexpected live revision labels:\n%s", text)
	}
}

func TestLiveMetricsHTTPCollectionFailure(t *testing.T) {
	values := []time.Time{time.Unix(1, 0), time.Unix(1, 0)}
	handler := LiveMetricsHTTP{Collect: func(context.Context, *Runtime) (liveMetricsSnapshot, error) {
		return liveMetricsSnapshot{}, errors.New("boom")
	}, Now: func() time.Time { result := values[0]; values = values[1:]; return result }}
	response, err := handler.Handle(t.Context(), "GET", liveMetricsPath, nil, nil, "")
	text := string(response.Body)
	if err != nil || response.Status != 200 || !strings.Contains(text, "memento_service_up 1") || !strings.Contains(text, "memento_metrics_collect_success 0") || strings.Contains(text, "memento_index_ready") {
		t.Fatal(response, text, err)
	}
}

func TestLiveMetricsHTTPAdmissionBound(t *testing.T) {
	metrics := NewRuntimeMetrics()
	release, err := metrics.admitScrape(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	calls := 0
	handler := LiveMetricsHTTP{Runtime: &Runtime{Metrics: metrics}, Collect: func(context.Context, *Runtime) (liveMetricsSnapshot, error) {
		calls++
		return liveMetricsSnapshot{}, nil
	}}
	started := time.Now()
	response, err := handler.Handle(t.Context(), "GET", liveMetricsPath, nil, nil, "")
	elapsed := time.Since(started)
	if err != nil || response.Status != 200 || calls != 0 || !strings.Contains(string(response.Body), "memento_metrics_collect_success 0") || elapsed > 500*time.Millisecond {
		t.Fatal(response, calls, elapsed, err)
	}
}

func TestCollectLiveMetrics(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = t.TempDir() + "/runtime"
	service, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close(ctx)
	snapshot, err := collectLiveMetrics(ctx, service)
	if err != nil || !snapshot.IndexReady || snapshot.IndexStale || snapshot.Control.MainBytes == 0 || snapshot.Derived.MainBytes == 0 || snapshot.Embedding["ready"] != 0 || snapshot.Embedding["missing"] != 0 || snapshot.Goroutines < 1 {
		t.Fatal(snapshot, err)
	}
	if _, err = collectLiveMetrics(ctx, nil); err == nil {
		t.Fatal("nil runtime")
	}
	response, err := service.HTTPHooks.Route(ctx, "GET", liveMetricsPath, nil, nil, "")
	if err != nil || response.Status != 200 || !strings.Contains(string(response.Body), "memento_metrics_collect_success 1") {
		t.Fatal(response, err)
	}
	if err = service.DB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = collectLiveMetrics(ctx, service); err == nil {
		t.Fatal("closed database")
	}
	service.DB = nil
}

func TestRuntimeMetrics(t *testing.T) {
	var missing *RuntimeMetrics
	missing.ObserveIndex("rebuild", time.Second, nil)
	if len(missing.IndexSnapshot()) != 2 {
		t.Fatal("nil metrics")
	}
	metrics := NewRuntimeMetrics()
	metrics.ObserveIndex("rebuild", time.Second, nil)
	metrics.ObserveIndex("rebuild", 2*time.Second, errors.New("failed"))
	metrics.ObserveIndex("update", 3*time.Second, nil)
	snapshot := metrics.IndexSnapshot()
	if snapshot["rebuild"].Success != 1 || snapshot["rebuild"].Error != 1 || snapshot["rebuild"].LastDuration != 2*time.Second || snapshot["update"].Success != 1 {
		t.Fatal(snapshot)
	}
}

func TestLiveMetricsGateInitialization(t *testing.T) {
	var nilMetrics *RuntimeMetrics
	if nilMetrics.ensureScrapeGate() != nil {
		t.Fatal("nil")
	}
	empty := &RuntimeMetrics{}
	if empty.ensureScrapeGate() == nil {
		t.Fatal("gate")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := nilMetrics.admitScrape(ctx); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := collectMainRevisionMetrics(t.Context(), RuntimePaths{}); err != nil {
		t.Fatal(err)
	}
}
func TestLiveMetricsSnapshotFailures(t *testing.T) {
	for _, failure := range []string{"repo-before", "begin", "repo-after", "changed", "commit"} {
		service := auditMetricsRuntime(t)
		ops := defaultLiveMetricsOps()
		switch failure {
		case "repo-before":
			ops.repoRevision = func(context.Context, RuntimePaths) (string, error) { return "", errors.New("repo") }
		case "begin":
			ops.openDerived = func(context.Context, string) (*sql.DB, error) {
				db, _ := sql.Open("sqlite", ":memory:")
				_ = db.Close()
				return db, nil
			}
		case "repo-after", "changed":
			calls := 0
			ops.repoRevision = func(context.Context, RuntimePaths) (string, error) {
				calls++
				if calls == 1 {
					return "r", nil
				}
				if failure == "changed" {
					return "new", nil
				}
				return "", errors.New("repo")
			}
		case "commit":
			original := ops.embeddings
			ops.embeddings = func(ctx context.Context, q liveMetricsQueryer) (map[string]int64, error) {
				data, err := original(ctx, q)
				_ = q.(*sql.Tx).Rollback()
				return data, err
			}
		}
		if _, err := collectLiveMetricsWith(t.Context(), service, ops); err == nil {
			t.Fatal(failure)
		}
	}
}
