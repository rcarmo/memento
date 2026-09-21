package service

// Audit regression tests for metrics cancellation, freshness, and snapshots.
import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/derived"
)

func auditMetricsRuntime(t *testing.T) *Runtime {
	t.Helper()
	var cfg RuntimeConfig
	cfg.Repository.RootPath = filepath.Join(t.TempDir(), "state")
	service, _, err := BuildModelsOffRuntime(t.Context(), cfg, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close(context.Background()) })
	return service
}
func TestAuditMetricsCancellationBoundsControlWait(t *testing.T) {
	service := auditMetricsRuntime(t)
	service.DB.SetMaxOpenConns(1)
	conn, err := service.DB.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan error, 1)
	go func() { _, e := collectLiveMetrics(ctx, service); done <- e }()
	select {
	case <-done:
	case <-time.After(80 * time.Millisecond):
		_ = conn.Close()
		<-done
		t.Fatal("cancelled scrape waits indefinitely for control DB connection")
	}
}
func TestAuditMetricsDetectsGitAdvance(t *testing.T) {
	service := auditMetricsRuntime(t)
	before, err := collectLiveMetrics(t.Context(), service)
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=Audit", "-c", "user.email=audit@example.invalid", "--git-dir", service.Paths.Repository.BareDir}, args...)...)
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatal(e, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	tree := git("rev-parse", "refs/heads/main^{tree}")
	next := git("commit-tree", tree, "-p", before.RepoRevision, "-m", "disposable audit revision")
	git("update-ref", "refs/heads/main", next, before.RepoRevision)
	after, err := collectLiveMetrics(t.Context(), service)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Git HEAD=%s exported repo=%s exported index=%s stale=%v", next, after.RepoRevision, after.IndexRevision, after.IndexStale)
	if !after.IndexStale {
		t.Fatal("real repo moved but metrics still reports fresh")
	}
}
func TestAuditMetricsSnapshotConsistency(t *testing.T) {
	service := auditMetricsRuntime(t)
	writer, err := sql.Open("sqlite", service.Paths.DerivedDB)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	ops := defaultLiveMetricsOps()
	state := ops.state
	ops.state = func(ctx context.Context, db liveMetricsQueryer) (derived.IndexState, error) {
		previous, err := state(ctx, db)
		if err != nil {
			return previous, err
		}
		// Writer publishes an entire new state between this scrape's reads.
		_, err = writer.Exec(`BEGIN; INSERT INTO concepts(id,path,type,title,status,tags_json,aliases_json,body,content_hash,updated_at,repo_revision) VALUES('audit','/audit.md','concept','Audit','active','[]','[]','body','h','now','new'); UPDATE index_state SET value='new' WHERE key IN ('repo_revision','index_revision');COMMIT`)
		return previous, err
	}
	snapshot, err := collectLiveMetricsWith(t.Context(), service, ops)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("exported revision=%s rows=%d", snapshot.IndexRevision, snapshot.Concepts)
	if snapshot.IndexRevision != "new" && snapshot.Concepts != 0 {
		t.Fatal("scrape combines old revision with new row count")
	}
}
func TestAuditMetricsCountsMissingEmbeddings(t *testing.T) {
	service := auditMetricsRuntime(t)
	db, err := sql.Open("sqlite", service.Paths.DerivedDB)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO concepts(id,path,type,title,status,tags_json,aliases_json,body,content_hash,updated_at,repo_revision) VALUES('audit','/audit.md','concept','Audit','active','[]','[]','body','h','now','main')`)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := collectLiveMetrics(t.Context(), service)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("concepts=%d embeddings=%v", snapshot.Concepts, snapshot.Embedding)
	if snapshot.Embedding["pending"]+snapshot.Embedding["missing"] == 0 {
		t.Fatal("unembedded concept absent from pending/missing counts")
	}
}
