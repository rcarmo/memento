package derived

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/gte"
	"github.com/rcarmo/memento/internal/repository"
)

type chunkClientStub struct {
	tokenizer *gte.Tokenizer
	calls     int
	onEmbed   func()
	fail      bool
	badVector bool
	chunkErr  error
	empty     bool
}

func (c *chunkClientStub) ModelInfo() SemanticModelInfo { return SemanticModelInfo{"m", 2, "v"} }
func (c *chunkClientStub) Chunk(text string, tokens, overlap, chars int) ([]string, error) {
	if c.chunkErr != nil {
		return nil, c.chunkErr
	}
	if c.empty {
		return nil, nil
	}
	return c.tokenizer.Chunk(text, tokens, overlap, chars)
}
func (c *chunkClientStub) Embed(text string) ([]float32, error) {
	c.calls++
	if c.onEmbed != nil {
		c.onEmbed()
	}
	if c.fail {
		return nil, errors.New("injected inference failure")
	}
	if c.badVector {
		return []float32{0, 0}, nil
	}
	if strings.Contains(text, "tailmarker") {
		return []float32{0, 1}, nil
	}
	return []float32{1, 0}, nil
}
func chunkFixture(t *testing.T) (*Index, *chunkClientStub, string) {
	t.Helper()
	index := semanticSearchIndex(t)
	vocab := make([]string, 110)
	for i := range vocab {
		vocab[i] = "unused"
	}
	vocab[104], vocab[105], vocab[106] = "word", "title", "tailmarker"
	tok, e := gte.NewTokenizer(vocab, 512)
	if e != nil {
		t.Fatal(e)
	}
	client := &chunkClientStub{tokenizer: tok}
	body := strings.Repeat("word ", 1400) + "tailmarker"
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	_, e = db.Exec("UPDATE concepts SET body=?,content_hash='long-doc' WHERE path='/a.md'", body)
	if e != nil {
		t.Fatal(e)
	}
	return index, client, body
}

var chunkConfig = SemanticRefreshConfig{ModelID: "m", Dimensions: 2, MaxInputChars: 4096, MaxBatch: 1}

func TestChunkRefreshTailSearchAndRollback(t *testing.T) {
	index, client, _ := chunkFixture(t)
	ctx := t.Context()
	if err := index.RefreshEmbeddingPaths(ctx, "main", []string{"/a.md"}, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	if client.calls < 4 {
		t.Fatal("not full-document", client.calls)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	var count int
	var hash string
	if err := db.QueryRow("SELECT count(*) FROM concept_embedding_chunks").Scan(&count); err != nil || count != 5 {
		t.Fatal(count, err)
	}
	if err := db.QueryRow("SELECT embedding_text_hash FROM concept_embeddings WHERE path='/a.md'").Scan(&hash); err != nil || !strings.HasPrefix(hash, chunkHashPrefix) {
		t.Fatal(hash, err)
	}
	options := SemanticSearchOptions{SearchOptions: SearchOptions{Query: "tailmarker", Syntax: "plain", Limit: 10}, MaxCandidates: 10}
	page, err := index.SearchSemantic(ctx, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, options, client)
	if err != nil || len(page.Results) != 2 || page.Results[0].Path != "/a.md" || page.Results[0].Score < .99 {
		t.Fatal(page, err)
	}
	// No duplicate results even though multiple chunks match the opening text.
	options.Query = "word"
	page, err = index.SearchSemantic(ctx, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, options, client)
	if err != nil || len(page.Results) != 2 {
		t.Fatal(page, err)
	}
	denied := access.EffectivePolicy{ReadPrefixes: []string{"/b.md"}}
	page, err = index.SearchSemantic(ctx, denied, options, client)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Results {
		if item.Path == "/a.md" {
			t.Fatal("authorization bypass")
		}
	}
	// Old readers can still read the valid original vector shape/row count.
	var bytes int
	if err = db.QueryRow("SELECT length(embedding_blob) FROM concept_embeddings WHERE path='/a.md'").Scan(&bytes); err != nil || bytes != 8 {
		t.Fatal(bytes, err)
	}
	// An old writer replaces the prefixed fingerprint. New reads must then
	// ignore leftover chunks rather than return pre-rollback document content.
	if _, err = db.Exec("UPDATE concept_embeddings SET embedding_text_hash='legacy' WHERE path='/a.md'"); err != nil {
		t.Fatal(err)
	}
	options.Query = "tailmarker"
	page, err = index.SearchSemantic(ctx, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, options, client)
	if err != nil || page.Results[0].Score != 0 {
		t.Fatal(page, err)
	}
}

func TestChunkRefreshFailureIsAtomic(t *testing.T) {
	index, client, _ := chunkFixture(t)
	ctx := t.Context()
	if err := index.RefreshEmbeddingPaths(ctx, "main", []string{"/a.md"}, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	var before int
	_ = db.QueryRow("SELECT count(*) FROM concept_embedding_chunks").Scan(&before)
	// Fail the second insert after DELETE and the first INSERT: transaction
	// rollback must retain the whole previous chunk set, never partial output.
	_, err := db.Exec("CREATE TRIGGER fail_chunk BEFORE INSERT ON concept_embedding_chunks WHEN NEW.ordinal=1 BEGIN SELECT RAISE(ABORT,'fault'); END")
	if err != nil {
		t.Fatal(err)
	}
	if err = index.RefreshEmbeddingPaths(ctx, "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("expected insert failure")
	}
	var after int
	_ = db.QueryRow("SELECT count(*) FROM concept_embedding_chunks").Scan(&after)
	if after != before {
		t.Fatal(before, after)
	}
	_, _ = db.Exec("DROP TRIGGER fail_chunk")
	client.fail = true
	if err = index.RefreshEmbeddingPaths(ctx, "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("refresh failure must be reported")
	}
	_ = db.QueryRow("SELECT count(*) FROM concept_embedding_chunks").Scan(&after)
	if after != before {
		t.Fatal("lost known-good vectors", before, after)
	}
	// A changed document has no valid old vector set to retain.
	_, _ = db.Exec("UPDATE concepts SET body='different body' WHERE path='/a.md'")
	if err = index.RefreshEmbeddingPaths(ctx, "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("inference error must propagate after recording failure")
	}
	var status string
	_ = db.QueryRow("SELECT status FROM concept_embeddings WHERE path='/a.md'").Scan(&status)
	_ = db.QueryRow("SELECT count(*) FROM concept_embedding_chunks").Scan(&after)
	if status != "error" || after != 0 {
		t.Fatal(status, after)
	}
	client.fail = false
	client.badVector = true
	if err = index.RefreshEmbeddingPaths(ctx, "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("invalid vector must fail the refresh")
	}
}
func TestChunkRefreshConcurrentEditAndCancellation(t *testing.T) {
	index, client, _ := chunkFixture(t)
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	client.onEmbed = func() {
		_, err := db.Exec("UPDATE concepts SET content_hash='changed',body='changed' WHERE path='/a.md'")
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, err := db.Exec("UPDATE concepts SET body=? WHERE path='/a.md'", strings.Repeat("word ", 1000))
	if err != nil {
		t.Fatal(err)
	}
	client.onEmbed = cancel
	if err := index.RefreshEmbeddingPaths(ctx, "main", []string{"/a.md"}, chunkConfig, client); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestChunkRefreshPlanningFailures(t *testing.T) {
	index, client, _ := chunkFixture(t)
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/missing.md"}, chunkConfig, client); err == nil {
		t.Fatal("missing")
	}
	client.chunkErr = errors.New("chunk")
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("chunk")
	}
	client.chunkErr = nil
	client.empty = true
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err == nil {
		t.Fatal("empty")
	}
	client.empty = false
	config := chunkConfig
	config.MaxInputChars = 0
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, config, client); err != nil {
		t.Fatal(err)
	}
}
func TestChunkTailEditInvalidates(t *testing.T) {
	index, client, body := chunkFixture(t)
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, chunkConfig, client); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", index.Path)
	defer db.Close()
	var id, title string
	if err := db.QueryRow("SELECT id,title FROM concepts WHERE path='/a.md'").Scan(&id, &title); err != nil {
		t.Fatal(err)
	}
	store := ContentStore{DB: db, DeferEmbeddings: true, MaxInputChars: 4096}
	entry := repository.BundleEntry{BundlePath: "/a.md", Document: repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{ID: id, Title: title}, Body: body}}
	if err := store.markEmbeddingStaleness(t.Context(), db, []repository.BundleEntry{entry}, "main"); err != nil {
		t.Fatal(err)
	}
	var status string
	_ = db.QueryRow("SELECT status FROM concept_embeddings WHERE path='/a.md'").Scan(&status)
	if status != "ready" {
		t.Fatal(status)
	}
	entry.Document.Body = strings.Replace(body, "tailmarker", "edited tail", 1)
	if err := store.markEmbeddingStaleness(t.Context(), db, []repository.BundleEntry{entry}, "next"); err != nil {
		t.Fatal(err)
	}
	_ = db.QueryRow("SELECT status FROM concept_embeddings WHERE path='/a.md'").Scan(&status)
	if status != "stale" {
		t.Fatal("tail edit failed to invalidate", status)
	}
}

// Closing the worker must cancel in-flight chunk inference instead of waiting
// for every remaining chunk/model timeout in a long document.
type cancellableChunkClient struct {
	*chunkClientStub
	started chan struct{}
}

func (c *cancellableChunkClient) EmbedBatchContext(ctx context.Context, texts []string) ([][]float32, error) {
	close(c.started)
	<-ctx.Done()
	return nil, ctx.Err()
}
func TestChunkWorkerShutdownCancels(t *testing.T) {
	index, base, _ := chunkFixture(t)
	client := &cancellableChunkClient{base, make(chan struct{})}
	worker := NewSemanticWorker(index, client, chunkConfig)
	worker.Enqueue("unused", "main", []string{"/a.md"}, false)
	select {
	case <-client.started:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	closed := make(chan struct{})
	go func() { worker.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not cancel")
	}
}

type contextChunkStub struct {
	*chunkClientStub
	vectors [][]float32
	err     error
}

func (c contextChunkStub) EmbedBatchContext(context.Context, []string) ([][]float32, error) {
	return c.vectors, c.err
}
func TestChunkContextBatch(t *testing.T) {
	for _, c := range []contextChunkStub{{vectors: [][]float32{{1, 0}}}, {}, {err: errors.New("inference")}} {
		result := embedChunkBatch(t.Context(), c, []string{"word"})
		if len(result) != 1 {
			t.Fatal(result)
		}
		if len(c.vectors) == 1 {
			if result[0].Err != nil || result[0].Vector[0] != 1 {
				t.Fatal(result)
			}
		} else if result[0].Err == nil {
			t.Fatal("missing error")
		}
	}
}

func TestChunkPacingAndAdmission(t *testing.T) {
	index, client, _ := chunkFixture(t)
	config := chunkConfig
	calls := 0
	config.BeforeBatch = func(context.Context) error { calls++; return nil }
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, config, client); err != nil || calls < 3 {
		t.Fatal(calls, err)
	}
	config.BeforeBatch = func(context.Context) error { return context.Canceled }
	if err := index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/a.md"}, config, client); err != context.Canceled {
		t.Fatal(err)
	}
	worker := NewProgressiveSemanticWorker(index, client, chunkConfig, SemanticWorkerPolicy{Enabled: true, Delay: time.Millisecond}, nil, nil, time.Now)
	if err := worker.admitChunkBatch(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := worker.admitChunkBatch(ctx); err != context.Canceled {
		t.Fatal(err)
	}
	worker.Close()
}
