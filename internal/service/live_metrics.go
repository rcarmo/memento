package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

const (
	liveMetricsPath             = "/metrics"
	liveMetricsAdmissionTimeout = 100 * time.Millisecond
	liveMetricsCollectTimeout   = 2 * time.Second
)

var errLiveMetricsChangedSnapshot = errors.New("live metrics snapshot changed during collection")

type indexOperationMetric struct {
	Success, Error uint64
	LastDuration   time.Duration
}

type RuntimeMetrics struct {
	mu     sync.Mutex
	index  map[string]indexOperationMetric
	scrape chan struct{}
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{index: map[string]indexOperationMetric{"rebuild": {}, "update": {}}, scrape: newLiveMetricsGate()}
}

func newLiveMetricsGate() chan struct{} {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return gate
}

func (m *RuntimeMetrics) ensureScrapeGate() chan struct{} {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scrape == nil {
		m.scrape = newLiveMetricsGate()
	}
	return m.scrape
}

func (m *RuntimeMetrics) admitScrape(ctx context.Context) (func(), error) {
	if m == nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return func() {}, nil
	}
	gate := m.ensureScrapeGate()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-gate:
		return func() {
			select {
			case gate <- struct{}{}:
			default:
			}
		}, nil
	}
}

func (m *RuntimeMetrics) ObserveIndex(operation string, duration time.Duration, err error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	value := m.index[operation]
	if err == nil {
		value.Success++
	} else {
		value.Error++
	}
	value.LastDuration = duration
	m.index[operation] = value
}

func (m *RuntimeMetrics) IndexSnapshot() map[string]indexOperationMetric {
	result := map[string]indexOperationMetric{"rebuild": {}, "update": {}}
	if m == nil {
		return result
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, value := range m.index {
		result[key] = value
	}
	return result
}

type sqliteMetrics struct {
	MainBytes, WALBytes, SHMBytes int64
	Pages, FreePages              int64
}

type liveMetricsSnapshot struct {
	ServiceVersion                    string
	RepoRevision, IndexRevision       string
	IndexReady, IndexStale            bool
	Concepts, Links, GraphMetrics     int64
	Embedding                         map[string]int64
	Control, Derived                  sqliteMetrics
	Worker                            derived.SemanticWorkerState
	WorkerAvailable                   bool
	IndexOperations                   map[string]indexOperationMetric
	HeapAlloc, HeapInUse, HeapObjects uint64
	Goroutines                        int
}

type LiveMetricsHTTP struct {
	Runtime *Runtime
	Collect func(context.Context, *Runtime) (liveMetricsSnapshot, error)
	Now     func() time.Time
}

func (h LiveMetricsHTTP) Handle(ctx context.Context, method, path string, _ map[string]string, body []byte, _ string) (*umcp.HTTPResponse, error) {
	if path != liveMetricsPath {
		return nil, nil
	}
	if method != "GET" {
		return &umcp.HTTPResponse{Status: 405, Headers: [][2]string{{"Allow", "GET"}}}, nil
	}
	if len(body) != 0 {
		return &umcp.HTTPResponse{Status: 400}, nil
	}
	collect := h.Collect
	if collect == nil {
		collect = collectLiveMetrics
	}
	now := h.Now
	if now == nil {
		now = time.Now
	}
	started := now()
	collectCtx, cancel := context.WithTimeout(ctx, liveMetricsCollectTimeout)
	defer cancel()
	var (
		snapshot   liveMetricsSnapshot
		collectErr error
	)
	release, err := h.admitScrape(collectCtx)
	if err != nil {
		collectErr = err
	} else {
		defer release()
		snapshot, collectErr = collect(collectCtx, h.Runtime)
	}
	elapsed := now().Sub(started).Seconds()
	text := renderLivePrometheus(snapshot, collectErr == nil, elapsed)
	mime := "text/plain; version=0.0.4; charset=utf-8"
	return &umcp.HTTPResponse{Status: 200, Body: []byte(text), ContentType: &mime, Headers: [][2]string{{"Cache-Control", "no-store"}, {"X-Content-Type-Options", "nosniff"}}}, nil
}

func (h LiveMetricsHTTP) admitScrape(ctx context.Context) (func(), error) {
	var metrics *RuntimeMetrics
	if h.Runtime != nil {
		metrics = h.Runtime.Metrics
	}
	waitCtx, cancel := context.WithTimeout(ctx, liveMetricsAdmissionTimeout)
	defer cancel()
	return metrics.admitScrape(waitCtx)
}

type liveMetricsQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type liveMetricsOps struct {
	repoRevision   func(context.Context, RuntimePaths) (string, error)
	state          func(context.Context, liveMetricsQueryer) (derived.IndexState, error)
	control        func(context.Context, liveMetricsQueryer, string) (sqliteMetrics, error)
	openDerived    func(context.Context, string) (*sql.DB, error)
	derived        func(context.Context, liveMetricsQueryer, string) (sqliteMetrics, error)
	counts         func(context.Context, liveMetricsQueryer, *liveMetricsSnapshot) error
	embeddings     func(context.Context, liveMetricsQueryer) (map[string]int64, error)
	processRuntime func(*liveMetricsSnapshot)
}

func defaultLiveMetricsOps() liveMetricsOps {
	return liveMetricsOps{
		repoRevision: collectMainRevisionMetrics,
		state:        collectIndexStateMetrics,
		control:      collectSQLiteMetrics,
		openDerived:  openReadOnlyMetricsDB,
		derived:      collectSQLiteMetrics,
		counts: func(ctx context.Context, db liveMetricsQueryer, snapshot *liveMetricsSnapshot) error {
			return db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM concepts),(SELECT COUNT(*) FROM links),(SELECT COUNT(*) FROM graph_metrics)`).Scan(&snapshot.Concepts, &snapshot.Links, &snapshot.GraphMetrics)
		},
		embeddings: collectEmbeddingMetrics,
		processRuntime: func(snapshot *liveMetricsSnapshot) {
			var memory runtime.MemStats
			runtime.ReadMemStats(&memory)
			snapshot.HeapAlloc = memory.HeapAlloc
			snapshot.HeapInUse = memory.HeapInuse
			snapshot.HeapObjects = memory.HeapObjects
			snapshot.Goroutines = runtime.NumGoroutine()
		},
	}
}

func collectLiveMetrics(ctx context.Context, service *Runtime) (liveMetricsSnapshot, error) {
	return collectLiveMetricsWith(ctx, service, defaultLiveMetricsOps())
}

func collectLiveMetricsWith(ctx context.Context, service *Runtime, ops liveMetricsOps) (liveMetricsSnapshot, error) {
	var snapshot liveMetricsSnapshot
	if service == nil || service.DB == nil {
		return snapshot, errors.New("runtime database is unavailable")
	}
	snapshot.ServiceVersion = service.ServiceVersion
	if snapshot.ServiceVersion == "" {
		snapshot.ServiceVersion = "unknown"
	}
	var err error
	if snapshot.Control, err = ops.control(ctx, service.DB, service.Paths.ControlDB); err != nil {
		return snapshot, err
	}
	repoBefore, repoAfter := "", ""
	if ops.repoRevision != nil && service.Paths.Repository.BareDir != "" {
		repoBefore, err = ops.repoRevision(ctx, service.Paths)
		if err != nil {
			return snapshot, err
		}
	}
	derivedDB, err := ops.openDerived(ctx, service.Paths.DerivedDB)
	if err != nil {
		return snapshot, err
	}
	defer derivedDB.Close()
	tx, err := derivedDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return snapshot, err
	}
	defer func() { _ = tx.Rollback() }()
	state, err := ops.state(ctx, tx)
	if err != nil {
		return snapshot, err
	}
	snapshot.RepoRevision = state.RepoRevision
	if repoBefore != "" {
		snapshot.RepoRevision = repoBefore
	}
	snapshot.IndexRevision = state.IndexRevision
	snapshot.IndexReady = state.Status == "ready"
	snapshot.IndexStale = snapshot.RepoRevision != snapshot.IndexRevision
	if snapshot.Derived, err = ops.derived(ctx, tx, service.Paths.DerivedDB); err != nil {
		return snapshot, err
	}
	if err = ops.counts(ctx, tx, &snapshot); err != nil {
		return snapshot, err
	}
	if snapshot.Embedding, err = ops.embeddings(ctx, tx); err != nil {
		return snapshot, err
	}
	if repoBefore != "" {
		repoAfter, err = ops.repoRevision(ctx, service.Paths)
		if err != nil {
			return snapshot, err
		}
		if repoAfter != repoBefore {
			return snapshot, errLiveMetricsChangedSnapshot
		}
	}
	if err = tx.Commit(); err != nil {
		return snapshot, err
	}
	if service.SemanticWorker != nil {
		snapshot.WorkerAvailable = true
		snapshot.Worker = service.SemanticWorker.State()
	}
	snapshot.IndexOperations = service.Metrics.IndexSnapshot()
	ops.processRuntime(&snapshot)
	return snapshot, nil
}

func collectMainRevisionMetrics(_ context.Context, paths RuntimePaths) (string, error) {
	if paths.Repository.BareDir == "" {
		return "", nil
	}
	return repository.GetMainRevision(paths.Repository)
}

func collectIndexStateMetrics(ctx context.Context, db liveMetricsQueryer) (derived.IndexState, error) {
	rows, err := db.QueryContext(ctx, "SELECT key,value FROM index_state WHERE key IN ('repo_revision','index_revision','schema_version','status')")
	if err != nil {
		return derived.IndexState{}, err
	}
	return readIndexStateMetrics(rows)
}

func readIndexStateMetrics(rows metricsRows) (derived.IndexState, error) {
	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			_ = rows.Close()
			return derived.IndexState{}, err
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return derived.IndexState{}, err
	}
	if err := rows.Close(); err != nil {
		return derived.IndexState{}, err
	}
	for _, key := range []string{"repo_revision", "index_revision", "schema_version", "status"} {
		if values[key] == "" {
			return derived.IndexState{}, fmt.Errorf("derived index state is missing %s", key)
		}
	}
	return derived.IndexState{RepoRevision: values["repo_revision"], IndexRevision: values["index_revision"], SchemaVersion: values["schema_version"], Status: values["status"]}, nil
}

type metricsRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

func collectEmbeddingMetrics(ctx context.Context, db liveMetricsQueryer) (map[string]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT COALESCE(e.status,'missing'),COUNT(*) FROM concepts AS c LEFT JOIN concept_embeddings AS e ON e.concept_id=c.id GROUP BY COALESCE(e.status,'missing')`)
	if err != nil {
		return nil, err
	}
	return readEmbeddingMetrics(rows)
}

func readEmbeddingMetrics(rows metricsRows) (map[string]int64, error) {
	result := map[string]int64{"ready": 0, "pending": 0, "stale": 0, "error": 0, "missing": 0, "legacy": 0, "other": 0}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		switch status {
		case "ready", "pending", "stale", "error", "missing", "legacy":
			result[status] += count
		default:
			result["other"] += count
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

func openReadOnlyMetricsDB(ctx context.Context, path string) (*sql.DB, error) {
	return openReadOnlyMetricsDBWith(ctx, path, sql.Open)
}

func openReadOnlyMetricsDBWith(ctx context.Context, path string, open func(string, string) (*sql.DB, error)) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path}
	query := url.Values{"mode": {"ro"}, "_pragma": {"query_only(1)", "busy_timeout(250)"}}
	u.RawQuery = query.Encode()
	db, err := open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func collectSQLiteMetrics(ctx context.Context, db liveMetricsQueryer, path string) (sqliteMetrics, error) {
	return collectSQLiteMetricsWith(ctx, path, os.Stat, func(ctx context.Context, query string, target *int64) error {
		return db.QueryRowContext(ctx, query).Scan(target)
	})
}

func collectSQLiteMetricsWith(ctx context.Context, path string, stat func(string) (fs.FileInfo, error), query func(context.Context, string, *int64) error) (sqliteMetrics, error) {
	var result sqliteMetrics
	for suffix, target := range map[string]*int64{"": &result.MainBytes, "-wal": &result.WALBytes, "-shm": &result.SHMBytes} {
		info, err := stat(path + suffix)
		if err == nil {
			*target = info.Size()
		} else if !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
	}
	if err := query(ctx, "PRAGMA page_count", &result.Pages); err != nil {
		return result, err
	}
	if err := query(ctx, "PRAGMA freelist_count", &result.FreePages); err != nil {
		return result, err
	}
	return result, nil
}

func renderLivePrometheus(snapshot liveMetricsSnapshot, success bool, elapsed float64) string {
	boolean := func(value bool) int {
		if value {
			return 1
		}
		return 0
	}
	lines := []string{
		"# HELP memento_service_up Memento process health.",
		"# TYPE memento_service_up gauge",
		"memento_service_up 1",
		"# HELP memento_metrics_collect_success Whether the latest scrape collection completed.",
		"# TYPE memento_metrics_collect_success gauge",
		fmt.Sprintf("memento_metrics_collect_success %d", boolean(success)),
		"# HELP memento_metrics_collect_duration_seconds Time spent collecting this scrape.",
		"# TYPE memento_metrics_collect_duration_seconds gauge",
		fmt.Sprintf("memento_metrics_collect_duration_seconds %s", strconv.FormatFloat(elapsed, 'g', -1, 64)),
	}
	if !success {
		return strings.Join(append(lines, ""), "\n")
	}
	lines = append(lines,
		"# HELP memento_build_info Memento build information.",
		"# TYPE memento_build_info gauge",
		fmt.Sprintf("memento_build_info{version=\"%s\"} 1", prometheusLabelValue(snapshot.ServiceVersion)),
		"# HELP memento_index_ready Whether the derived index reports ready.",
		"# TYPE memento_index_ready gauge",
		fmt.Sprintf("memento_index_ready %d", boolean(snapshot.IndexReady)),
		"# HELP memento_index_stale Whether the derived index revision differs from the repository revision.",
		"# TYPE memento_index_stale gauge",
		fmt.Sprintf("memento_index_stale %d", boolean(snapshot.IndexStale)),
		"# HELP memento_index_rows Number of rows in core derived-index tables.",
		"# TYPE memento_index_rows gauge",
		fmt.Sprintf("memento_index_rows{table=\"concepts\"} %d", snapshot.Concepts),
		fmt.Sprintf("memento_index_rows{table=\"links\"} %d", snapshot.Links),
		fmt.Sprintf("memento_index_rows{table=\"graph_metrics\"} %d", snapshot.GraphMetrics),
		"# HELP memento_embedding_rows Number of concepts by embedding refresh state.",
		"# TYPE memento_embedding_rows gauge",
	)
	for _, status := range []string{"ready", "pending", "stale", "error", "missing", "legacy", "other"} {
		lines = append(lines, fmt.Sprintf("memento_embedding_rows{status=\"%s\"} %d", prometheusLabelValue(status), snapshot.Embedding[status]))
	}
	lines = append(lines,
		"# HELP memento_sqlite_file_bytes SQLite main, WAL, and shared-memory file sizes.",
		"# TYPE memento_sqlite_file_bytes gauge",
	)
	for _, database := range []struct {
		name string
		data sqliteMetrics
	}{{"control", snapshot.Control}, {"derived", snapshot.Derived}} {
		lines = append(lines,
			fmt.Sprintf("memento_sqlite_file_bytes{database=\"%s\",kind=\"main\"} %d", prometheusLabelValue(database.name), database.data.MainBytes),
			fmt.Sprintf("memento_sqlite_file_bytes{database=\"%s\",kind=\"wal\"} %d", prometheusLabelValue(database.name), database.data.WALBytes),
			fmt.Sprintf("memento_sqlite_file_bytes{database=\"%s\",kind=\"shm\"} %d", prometheusLabelValue(database.name), database.data.SHMBytes),
		)
	}
	lines = append(lines,
		"# HELP memento_sqlite_pages SQLite allocated and free page counts.",
		"# TYPE memento_sqlite_pages gauge",
		fmt.Sprintf("memento_sqlite_pages{database=\"control\",state=\"allocated\"} %d", snapshot.Control.Pages),
		fmt.Sprintf("memento_sqlite_pages{database=\"control\",state=\"free\"} %d", snapshot.Control.FreePages),
		fmt.Sprintf("memento_sqlite_pages{database=\"derived\",state=\"allocated\"} %d", snapshot.Derived.Pages),
		fmt.Sprintf("memento_sqlite_pages{database=\"derived\",state=\"free\"} %d", snapshot.Derived.FreePages),
		"# HELP memento_embedding_worker Worker state and completed document refreshes.",
		"# TYPE memento_embedding_worker gauge",
		fmt.Sprintf("memento_embedding_worker{state=\"available\"} %d", boolean(snapshot.WorkerAvailable)),
		fmt.Sprintf("memento_embedding_worker{state=\"alive\"} %d", boolean(snapshot.WorkerAvailable && snapshot.Worker.Alive)),
		fmt.Sprintf("memento_embedding_worker{state=\"running\"} %d", boolean(snapshot.WorkerAvailable && snapshot.Worker.Running)),
		fmt.Sprintf("memento_embedding_worker{state=\"pending\"} %d", boolean(snapshot.WorkerAvailable && snapshot.Worker.Pending)),
		"# HELP memento_embedding_worker_completed_total Document embedding refreshes completed by this process.",
		"# TYPE memento_embedding_worker_completed_total counter",
		fmt.Sprintf("memento_embedding_worker_completed_total %d", snapshot.Worker.Completed),
		"# HELP memento_index_operations_total Derived-index operations by operation and result.",
		"# TYPE memento_index_operations_total counter",
	)
	for _, operation := range []string{"rebuild", "update"} {
		value := snapshot.IndexOperations[operation]
		lines = append(lines,
			fmt.Sprintf("memento_index_operations_total{operation=\"%s\",result=\"success\"} %d", prometheusLabelValue(operation), value.Success),
			fmt.Sprintf("memento_index_operations_total{operation=\"%s\",result=\"error\"} %d", prometheusLabelValue(operation), value.Error),
		)
	}
	lines = append(lines,
		"# HELP memento_index_operation_last_duration_seconds Duration of the latest derived-index operation.",
		"# TYPE memento_index_operation_last_duration_seconds gauge",
		fmt.Sprintf("memento_index_operation_last_duration_seconds{operation=\"rebuild\"} %s", strconv.FormatFloat(snapshot.IndexOperations["rebuild"].LastDuration.Seconds(), 'g', -1, 64)),
		fmt.Sprintf("memento_index_operation_last_duration_seconds{operation=\"update\"} %s", strconv.FormatFloat(snapshot.IndexOperations["update"].LastDuration.Seconds(), 'g', -1, 64)),
		"# HELP memento_go_memory_bytes Go heap memory by kind.",
		"# TYPE memento_go_memory_bytes gauge",
		fmt.Sprintf("memento_go_memory_bytes{kind=\"alloc\"} %d", snapshot.HeapAlloc),
		fmt.Sprintf("memento_go_memory_bytes{kind=\"inuse\"} %d", snapshot.HeapInUse),
		"# HELP memento_go_heap_objects Number of allocated Go heap objects.",
		"# TYPE memento_go_heap_objects gauge",
		fmt.Sprintf("memento_go_heap_objects %d", snapshot.HeapObjects),
		"# HELP memento_go_goroutines Number of current goroutines.",
		"# TYPE memento_go_goroutines gauge",
		fmt.Sprintf("memento_go_goroutines %d", snapshot.Goroutines),
		"",
	)
	return strings.Join(lines, "\n")
}

func prometheusLabelValue(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\"", "\\\"")
	return replacer.Replace(value)
}
