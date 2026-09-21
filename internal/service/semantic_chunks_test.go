package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/gte"
)

type modelChunkStub struct{ gteSemanticStub }

func (modelChunkStub) EmbedBatch(_ []string, _ gte.BatchOptions, checkpoint gte.Checkpoint) ([][]float32, error) {
	return nil, checkpoint("test")
}

func (modelChunkStub) Chunk(text string, _, _, _ int) ([]string, error) { return []string{text}, nil }
func TestSemanticClientChunkAdapters(t *testing.T) {
	client := &GTESemanticClient{Model: gteSemanticStub{}}
	if _, err := client.Chunk("test", 384, 64, 4096); err == nil {
		t.Fatal("missing chunk support")
	}
	client.Model = modelChunkStub{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.EmbedBatchContext(ctx, []string{"test"}); err != context.Canceled {
		t.Fatal(err)
	}
	chunks, err := client.Chunk("test", 384, 64, 4096)
	if err != nil || len(chunks) != 1 || chunks[0] != "test" {
		t.Fatal(chunks, err)
	}
	sub := &subprocessSemanticClient{ModelPath: filepath.Join(t.TempDir(), "missing")}
	if _, err = sub.Chunk("test", 384, 64, 4096); err == nil {
		t.Fatal("missing model")
	}
	sub.ModelPath = "../../models/gte/gte-small.gtemodel"
	if _, err = os.Stat(sub.ModelPath); err != nil {
		t.Skip("model not installed")
	}
	chunks, err = sub.Chunk(strings.Repeat("word ", 600)+"tailmarker", 384, 64, 4096)
	if err != nil || len(chunks) < 2 || !strings.HasSuffix(chunks[len(chunks)-1], "tailmarker") {
		t.Fatal(chunks, err)
	}
}

// Offline retrieval test with the actual pinned GTE model. A concept whose
// answer occurs after 4096 characters must outrank a related distractor.
func TestRealChunkTailRetrieval(t *testing.T) {
	path := os.Getenv("GTE_MODEL_PATH")
	if path == "" {
		t.Skip("set GTE_MODEL_PATH")
	}
	client, err := LoadGTESemanticClient(path, "gte-test", 384, 1, 4096)
	if err != nil {
		t.Fatal(err)
	}
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	runtime, _, err := BuildModelsOffRuntime(t.Context(), config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())
	// The actual repository/ACL fixtures are tested elsewhere; here isolate the
	// long-document embedding and retrieval behavior against the real model.
	index := &derived.Index{Path: runtime.Paths.DerivedDB, DeferEmbeddings: true}
	body := strings.Repeat("The monthly gardening report describes watering plants, growing vegetables and maintaining the lawn. ", 75) + "\n\n# Database recovery\nTo recover PostgreSQL after a crash, replay the write-ahead log from the last checkpoint. WAL records restore committed transactions and preserve durability."
	// Populate the derived database with the same schema used by normal indexing.
	store, err := derived.Connect(t.Context(), index.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.Exec(`INSERT INTO concepts(id,path,type,title,status,tags_json,aliases_json,body,content_hash,updated_at,repo_revision) VALUES('long','/long.md','concept','Operations notes','active','[]','[]',?,'long','now','main'),('other','/other.md','concept','Database tutorial','active','[]','[]','PostgreSQL is a relational database. SQL queries filter tables and join rows. Indexes help query planning.','other','now','main'); UPDATE index_state SET value='main' WHERE key IN ('repo_revision','index_revision')`, body)
	if err != nil {
		t.Fatal(err)
	}
	if err = index.RefreshEmbeddingPaths(t.Context(), "main", []string{"/long.md", "/other.md"}, derived.SemanticRefreshConfig{ModelID: "gte-test", Dimensions: 384, MaxBatch: 1, MaxInputChars: 4096}, client); err != nil {
		t.Fatal(err)
	}
	page, err := index.SearchSemantic(t.Context(), access.EffectivePolicy{ReadPrefixes: []string{"/"}}, derived.SemanticSearchOptions{SearchOptions: derived.SearchOptions{Query: "How does PostgreSQL recover committed transactions after a crash?", Limit: 2}, MaxCandidates: 10}, client)
	if err != nil || len(page.Results) != 2 || page.Results[0].Path != "/long.md" {
		t.Fatal(page, err)
	}
	t.Logf("real model tail retrieval: first=%s score=%.4f distractor=%.4f", page.Results[0].Path, page.Results[0].Score, page.Results[1].Score)
	tok, err := gte.LoadTokenizer(path)
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := tok.Chunk(body, 384, 64, 4096)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("document chars=%d chunks=%d", len(body), len(chunks))
}

func TestRealChunkSubprocessRefresh(t *testing.T) {
	path := os.Getenv("GTE_MODEL_PATH")
	if path == "" {
		t.Skip("set GTE_MODEL_PATH")
	}
	worker, err := filepath.Abs("../../build/memento-embed-go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(worker); err != nil {
		t.Skip("build worker first")
	}
	config := DefaultSemanticSearchConfig()
	config.ModelPath = &path
	config.WorkerPath = worker
	config.MaxBatchSize = 1
	client, err := LoadSubprocessSemanticClient(config)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Repeat("An operational note about gardening and growing vegetables. ", 40) + "\n\n# Recovery\nPostgreSQL replays the write-ahead log to recover committed transactions after a crash."
	chunks, err := client.Chunk(text, 384, 64, 4096)
	if err != nil || len(chunks) < 2 {
		t.Fatal(err)
	}
	for _, chunk := range chunks {
		result, err := client.EmbedBatchContext(t.Context(), []string{chunk})
		if err != nil || len(result) != 1 || len(result[0]) != 384 {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = client.EmbedBatchContext(ctx, []string{"cancel"}); err == nil {
		t.Fatal("cancellation")
	}
	t.Logf("real subprocess embedded %d bounded chunks", len(chunks))
}
