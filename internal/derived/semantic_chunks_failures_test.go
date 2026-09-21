package derived

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
)

func TestChunkPublicationSQLFaults(t *testing.T) {
	run := func(fail int, inferenceFailure bool) (int, error) {
		store, fault := faultStore(t)
		if err := store.Migrate(t.Context()); err != nil {
			t.Fatal(err)
		}
		_, err := store.DB.Exec(`INSERT INTO concepts(id,path,type,title,status,tags_json,aliases_json,body,content_hash,updated_at,repo_revision) VALUES('id','/a.md','concept','Title','active','[]','[]','body','h','now','r')`)
		if err != nil {
			t.Fatal(err)
		}
		doc := chunkDocument{id: "id", path: "/a.md", contentHash: "h", revision: "r", digest: fullEmbeddingHash("Title\n\nbody"), info: SemanticModelInfo{"m", 2, "v"}, chunks: []string{"Title\n\nbody"}, vectors: []encodedChunk{{semanticBlob(1, 0), 1}}}
		if inferenceFailure {
			doc.embedErr = errors.New("inference")
		}
		fault.count = 0
		fault.remaining = fail
		err = publishChunks(t.Context(), store.DB, doc)
		calls := fault.count
		fault.remaining = 0
		if err != nil {
			var n int
			if e := store.DB.QueryRow("SELECT count(*) FROM concept_embeddings").Scan(&n); e != nil || n != 0 {
				t.Fatalf("partial publication at %d: %d %v", fail, n, e)
			}
		}
		return calls, err
	}
	for _, failed := range []bool{false, true} {
		count, err := run(0, failed)
		if err != nil {
			t.Fatal(err)
		}
		for n := 1; n <= count; n++ {
			_, err = run(n, failed)
			if err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(n, err)
			}
		}
	}
	db, _ := sql.Open("sqlite", ":memory:")
	_ = db.Close()
	if err := publishChunks(t.Context(), db, chunkDocument{}); err == nil {
		t.Fatal("closed DB")
	}
}
func TestChunkStoreAndSearchFailures(t *testing.T) {
	index, client, _ := chunkFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := index.withChunkStore(ctx, func(ContentStore) error { return nil }); err == nil {
		t.Fatal("cancelled")
	}
	invalid := &Index{Path: t.TempDir() + "/bad"}
	if err := os.WriteFile(invalid.Path, []byte("invalid sqlite header"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := invalid.withChunkStore(t.Context(), func(ContentStore) error { return nil }); err == nil {
		t.Fatal("invalid header")
	}
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	rows := []semanticRow{{result: SearchResult{ConceptID: "id"}}}
	_, _ = db.Exec("ALTER TABLE concept_embedding_chunks RENAME COLUMN text TO missing")
	if _, err := bestSemanticChunks(t.Context(), db, rows, []float32{1, 0}, 1, 2); err == nil {
		t.Fatal("query")
	}
	if _, err := index.SearchSemantic(t.Context(), access.EffectivePolicy{ReadPrefixes: []string{"/"}}, SemanticSearchOptions{SearchOptions: SearchOptions{Query: "word", Limit: 1}}, client); err == nil {
		t.Fatal("broken chunk query")
	}
	// Search errors currently quarantine the index through the legacy lifecycle.
	// Restore this fixture's path to inspect the decode-failure leg separately.
	index, client, _ = chunkFixture(t)
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	db, _ = sql.Open("sqlite", index.Path)
	defer db.Close()
	_, _ = db.Exec("ALTER TABLE concept_embedding_chunks RENAME COLUMN text TO missing")
	_, _ = db.Exec("ALTER TABLE concept_embedding_chunks RENAME COLUMN missing TO text; UPDATE concepts SET tags_json='bad' WHERE id='id'")
	if _, err := bestSemanticChunks(t.Context(), db, rows, []float32{1, 0}, 1, 2); err == nil {
		t.Fatal("decode")
	}
	_ = db.Close()
	if _, err := bestSemanticChunks(t.Context(), db, rows, []float32{1, 0}, 1, 2); err == nil {
		t.Fatal("closed")
	}
}
func TestChunkQueueAndCleanup(t *testing.T) {
	index, client, _ := chunkFixture(t)
	index.ChunkEmbeddings = true
	paths, err := index.PendingEmbeddingPaths(t.Context(), 10)
	if err != nil || len(paths) != 2 {
		t.Fatal(paths, err)
	}
	if err = index.RefreshEmbeddingPaths(t.Context(), "main", paths, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	paths, err = index.PendingEmbeddingPaths(t.Context(), 10)
	if err != nil || len(paths) != 0 {
		t.Fatal(paths, err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	// Existing delete/rename paths, including old binaries, delete the parent
	// embedding row. The optional trigger removes all associated chunk text.
	_, err = db.Exec("DELETE FROM concept_embeddings WHERE path='/a.md'")
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow("SELECT count(*) FROM concept_embedding_chunks WHERE concept_id='id'").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	// A failed chunk attempt should not continuously requeue itself.
	client.fail = true
	if err = index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("failed inference must be reported")
	}
	paths, err = index.PendingEmbeddingPaths(t.Context(), 10)
	if err != nil || len(paths) != 0 {
		t.Fatal(paths, err)
	}
}
func TestChunkMetadataChangedDuringInference(t *testing.T) {
	index, client, _ := chunkFixture(t)
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	client.onEmbed = func() {
		_, err := db.Exec("UPDATE concepts SET title='new title' WHERE path='/a.md'")
		if err != nil {
			t.Fatal(err)
		}
	}
	err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client)
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatal(err)
	}
	client.onEmbed = func() {
		_, err := db.Exec("DELETE FROM concepts WHERE path='/a.md'")
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("deleted")
	}
}

func TestChunkInvalidVectorRead(t *testing.T) {
	index, client, _ := chunkFixture(t)
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	if _, err := db.Exec("UPDATE concept_embedding_chunks SET embedding_blob=x'00'"); err != nil {
		t.Fatal(err)
	}
	if _, err := bestSemanticChunks(t.Context(), db, []semanticRow{{result: SearchResult{ConceptID: "id"}}}, []float32{1, 0}, 1, 2); err == nil {
		t.Fatal("invalid vector")
	}
}

func TestChunkPolicyAndPendingFailures(t *testing.T) {
	index, client, _ := chunkFixture(t)
	index.ConfigureChunkModel(client.ModelInfo())
	if chunkPolicy(client.ModelInfo(), 0) != chunkPolicy(client.ModelInfo(), 4096) {
		t.Fatal("default policy")
	}
	db, _ := sql.Open("sqlite", index.Path)
	_ = db.Close()
	if _, err := index.pendingChunkRows(t.Context(), db, 2); err == nil {
		t.Fatal("closed")
	}
}

func TestWorkerPendingQueryGenerationAndClose(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		index := &workerPendingBarrier{entered: make(chan struct{}), resume: make(chan struct{})}
		worker := NewSemanticWorker(index, nil, SemanticRefreshConfig{})
		worker.Enqueue("root", "r1", nil, true)
		<-index.entered
		done := make(chan struct{})
		if shutdown {
			go func() { worker.Close(); close(done) }()
			for worker.State().Alive {
				time.Sleep(time.Millisecond)
			}
		} else {
			worker.Enqueue("root", "r2", nil, true)
		}
		close(index.resume)
		if shutdown {
			<-done
		} else {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			if err := worker.WaitIdle(ctx); err != nil {
				t.Fatal(err)
			}
			cancel()
			worker.Close()
		}
	}
}

type workerPendingBarrier struct {
	entered, resume chan struct{}
	calls           int
}

func (i *workerPendingBarrier) PendingEmbeddingPaths(context.Context, int) ([]string, error) {
	i.calls++
	if i.calls == 1 {
		close(i.entered)
		<-i.resume
		return []string{"/a.md"}, nil
	}
	return nil, nil
}
func (i *workerPendingBarrier) RefreshEmbeddingPaths(context.Context, string, []string, SemanticRefreshConfig, SemanticClient) error {
	return nil
}

func TestChunkSearchBeyondCandidatePrefix(t *testing.T) {
	index, client, _ := chunkFixture(t)
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	// Put the answer after the first authorized path, then set the old cap to 1.
	_, err := db.Exec("UPDATE concepts SET path='/z.md' WHERE id='id';UPDATE concept_embeddings SET path='/z.md' WHERE concept_id='id'")
	if err != nil {
		t.Fatal(err)
	}
	page, err := index.SearchSemantic(t.Context(), access.EffectivePolicy{ReadPrefixes: []string{"/"}}, SemanticSearchOptions{SearchOptions: SearchOptions{Query: "tailmarker", Limit: 1}, MaxCandidates: 1}, client)
	if err != nil || len(page.Results) != 1 || page.Results[0].Path != "/z.md" {
		t.Fatal(page, err)
	}
}
func TestIndexLockCancellation(t *testing.T) {
	index, _, _ := chunkFixture(t)
	index.mu.Lock()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { _, err := index.PendingEmbeddingPaths(ctx, 1); done <- err }()
	time.Sleep(5 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("lock cancellation")
	}
	index.mu.Unlock()
}

func TestChunkSearchInferenceAndConnectFailure(t *testing.T) {
	index, client, _ := chunkFixture(t)
	client.fail = true
	if _, err := index.SearchSemantic(t.Context(), access.EffectivePolicy{}, SemanticSearchOptions{}, client); err == nil {
		t.Fatal("inference")
	}
	// Header validation permits a missing file, but connection cannot create a
	// database under a regular-file parent.
	parent := t.TempDir() + "/file"
	if err := os.WriteFile(parent, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	bad := &Index{Path: parent + "/db"}
	if err := bad.withChunkStore(t.Context(), func(ContentStore) error { return nil }); err == nil {
		t.Fatal("open")
	}
	// A valid SQLite header with a truncated body reaches connection/migration.
	bad.Path = t.TempDir() + "/broken"
	if err := os.WriteFile(bad.Path, append([]byte("SQLite format 3\x00"), make([]byte, 100)...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := bad.withChunkStore(t.Context(), func(ContentStore) error { return nil }); err == nil {
		t.Fatal("corrupt connect")
	}
}
