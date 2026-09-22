package derived

// Audit regression tests preserving the invariants that originally failed.
import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type auditQueueIndex struct {
	mu            sync.Mutex
	calls         []string
	entered       chan struct{}
	resume        chan struct{}
	alwaysPending bool
	queryEntered  chan struct{}
	queryResume   chan struct{}
	queries       int
}

func (i *auditQueueIndex) PendingEmbeddingPaths(ctx context.Context, _ int) ([]string, error) {
	i.mu.Lock()
	i.queries++
	q := i.queries
	i.mu.Unlock()
	if i.queryEntered != nil && q == 1 {
		close(i.queryEntered)
		<-i.queryResume
		return nil, nil
	}
	if i.alwaysPending {
		return []string{"/a.md"}, nil
	}
	return nil, nil
}
func (i *auditQueueIndex) RefreshEmbeddingPaths(_ context.Context, revision string, _ []string, _ SemanticRefreshConfig, _ SemanticClient) error {
	i.mu.Lock()
	i.calls = append(i.calls, revision)
	n := len(i.calls)
	i.mu.Unlock()
	if n == 1 && i.entered != nil {
		close(i.entered)
		<-i.resume
	}
	return errors.New("document changed during chunk embedding")
}
func (i *auditQueueIndex) revisions() []string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]string{}, i.calls...)
}
func TestAuditReenqueueIsNotLost(t *testing.T) {
	index := &auditQueueIndex{entered: make(chan struct{}), resume: make(chan struct{})}
	worker := NewSemanticWorker(index, nil, SemanticRefreshConfig{})
	defer worker.Close()
	worker.Enqueue("root", "r1", []string{"/a.md"}, false)
	<-index.entered
	worker.Enqueue("root", "r2", []string{"/a.md"}, false)
	close(index.resume)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := worker.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	calls := index.revisions()
	t.Logf("observed revisions: %v", calls)
	if len(calls) < 2 || calls[len(calls)-1] != "r2" {
		t.Fatal("newer enqueue silently dropped")
	}
}
func TestAuditTerminalFullRefreshStops(t *testing.T) {
	index := &auditQueueIndex{alwaysPending: true}
	worker := NewSemanticWorker(index, nil, SemanticRefreshConfig{})
	worker.Enqueue("root", "r1", nil, true)
	time.Sleep(80 * time.Millisecond)
	worker.Close()
	calls := index.revisions()
	t.Logf("terminal-error inference calls in 80ms: %d", len(calls))
	if len(calls) > 1 {
		t.Fatal("terminal full-refresh failure retries indefinitely")
	}
}
func TestAuditNewFullRequestNotCleared(t *testing.T) {
	index := &auditQueueIndex{queryEntered: make(chan struct{}), queryResume: make(chan struct{})}
	worker := NewSemanticWorker(index, nil, SemanticRefreshConfig{})
	worker.Enqueue("root", "r1", nil, true)
	<-index.queryEntered
	worker.Enqueue("root", "r2", nil, true)
	close(index.queryResume)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := worker.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	worker.Close()
	index.mu.Lock()
	queries := index.queries
	index.mu.Unlock()
	t.Logf("pending queries for two full enqueues: %d", queries)
	if queries < 2 {
		t.Fatal("new full request was cleared using old empty queue observation")
	}
}
func TestAuditModelChangeQueuesReadyChunks(t *testing.T) {
	index, client, _ := chunkFixture(t)

	paths, err := index.PendingEmbeddingPaths(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = index.RefreshEmbeddingPaths(t.Context(), "main", paths, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	if _, err = db.Exec("UPDATE concept_embeddings SET model_revision='previous-model'"); err != nil {
		t.Fatal(err)
	}
	paths, err = index.PendingEmbeddingPaths(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("eligible after model revision changed: %v", paths)
	if len(paths) != 2 {
		t.Fatal("full refresh does not select incompatible model vectors")
	}
}
func TestAuditFailedDocumentNotCountedCompleted(t *testing.T) {
	index, client, _ := chunkFixture(t)
	client.fail = true
	worker := NewSemanticWorker(index, client, chunkConfig)
	defer worker.Close()
	worker.Enqueue("root", "main", []string{"/a.md"}, false)
	// This checks error accounting, not storage latency. Shared CI runners can
	// spend over a second committing the failure and invalidating derived caches.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := worker.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	var status string
	if err := db.QueryRow("SELECT status FROM concept_embeddings WHERE path='/a.md'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	state := worker.State()
	t.Logf("stored=%s worker_completed=%d worker_error=%v", status, state.Completed, state.LastError)
	if status != "error" || state.Completed != 0 || state.LastError == nil {
		t.Fatal("worker reports successful completion for persisted embedding error")
	}
}
func TestAuditFullHashIncludesChunkPolicy(t *testing.T) {
	// Verify a character-limit change cannot leave a ready row unselected.
	index, client, _ := chunkFixture(t)

	first := chunkConfig
	first.MaxInputChars = 512
	paths, err := index.PendingEmbeddingPaths(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = index.RefreshEmbeddingPaths(t.Context(), "main", paths, first, client); err != nil {
		t.Fatal(err)
	}
	index.MaxInputChars = 4096
	paths, err = index.PendingEmbeddingPaths(t.Context(), 10)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("eligible after max_input_chars 512 -> 4096: %v", paths)
	if len(paths) != 2 {
		t.Fatal("chunk policy change not included in refresh identity")
	}
}
func TestAuditUnrelatedRevisionDoesNotRejectUnchangedDocument(t *testing.T) {
	index, client, _ := chunkFixture(t)
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	client.onEmbed = func() {
		if _, err := db.Exec("UPDATE concepts SET repo_revision='next'"); err != nil {
			t.Fatal(err)
		}
	}
	err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client)
	t.Logf("result for unchanged document: %v", err)
	if err != nil && strings.Contains(err.Error(), "changed") {
		t.Fatal("unrelated repository advance rejects unchanged document embeddings")
	}
}
