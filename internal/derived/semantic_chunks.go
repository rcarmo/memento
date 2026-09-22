package derived

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// SemanticChunkClient exposes tokenizer-aware document chunking for the
// derived semantic runtime.
type SemanticChunkClient interface {
	SemanticClient
	Chunk(string, int, int, int) ([]string, error)
}

// SemanticContextBatchClient shares the caller's deadline, including worker shutdown.
type SemanticContextBatchClient interface {
	EmbedBatchContext(context.Context, []string) ([][]float32, error)
}

func embedChunkBatch(ctx context.Context, client SemanticClient, texts []string) []SemanticEmbeddingResult {
	if contextual, ok := client.(SemanticContextBatchClient); ok {
		vectors, err := contextual.EmbedBatchContext(ctx, texts)
		if err == nil && len(vectors) != len(texts) {
			err = fmt.Errorf("embedding worker returned wrong chunk count")
		}
		results := make([]SemanticEmbeddingResult, len(texts))
		for j := range results {
			results[j].Err = err
			if err == nil {
				results[j].Vector = vectors[j]
			}
		}
		return results
	}
	return EmbedSemanticBatch(client, texts, len(texts))
}

type encodedChunk struct {
	blob []byte
	norm float64
}
type chunkDocument struct {
	id, path, digest, contentHash, revision string
	info                                    SemanticModelInfo
	chunks                                  []string
	vectors                                 []encodedChunk
	embedErr                                error
	policy                                  string
}

const chunkHashPrefix = "chunks-v1:"

func fullEmbeddingHash(text string) string {
	return fmt.Sprintf("%s%x", chunkHashPrefix, sha256.Sum256([]byte(text)))
}

// The optional additive table leaves schema-v2 concept_embeddings readable by
// prior images. Its first chunk is mirrored in that table for rollback readers
// only. Current graph/search readers use matching chunk input/model and policy.
const chunkSchema = `CREATE TABLE IF NOT EXISTS concept_embedding_chunks (
 concept_id TEXT NOT NULL, ordinal INTEGER NOT NULL,
 document_hash TEXT NOT NULL, model_revision TEXT NOT NULL,
 text TEXT NOT NULL, embedding_blob BLOB NOT NULL, embedding_norm REAL NOT NULL,
 PRIMARY KEY(concept_id,ordinal)
);
CREATE TABLE IF NOT EXISTS concept_embedding_policy (
 concept_id TEXT PRIMARY KEY, policy TEXT NOT NULL
);
CREATE TRIGGER IF NOT EXISTS delete_concept_embedding_chunks AFTER DELETE ON concept_embeddings
BEGIN DELETE FROM concept_embedding_chunks WHERE concept_id=OLD.concept_id;
DELETE FROM concept_embedding_policy WHERE concept_id=OLD.concept_id; END`

func chunkTableExists(ctx context.Context, db executor) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='concept_embedding_chunks'").Scan(&count)
	return count != 0, err
}

// Refresh captures input briefly under the index lock, then runs inference
// without that lock or a SQLite transaction. Publication checks the captured
// document/repo state again and atomically replaces all chunks for one concept.
func (i *Index) refreshChunks(ctx context.Context, revision string, paths []string, config SemanticRefreshConfig, client SemanticChunkClient) error {
	info := client.ModelInfo()
	i.mu.Lock()
	i.chunkModel = info
	i.MaxInputChars = config.MaxInputChars
	i.mu.Unlock()
	for _, path := range paths {
		var id, title, body, contentHash string
		var description *string
		err := i.withChunkStore(ctx, func(s ContentStore) error {
			return s.DB.QueryRowContext(ctx, "SELECT id,title,description,body,content_hash FROM concepts WHERE path=?", path).Scan(&id, &title, &description, &body, &contentHash)
		})
		if err != nil {
			return err
		}
		text := embeddingText(title, description, body)
		digest := fullEmbeddingHash(text)
		limit := config.MaxInputChars
		if limit <= 0 {
			limit = 4096
		}
		chunks, err := client.Chunk(text, 384, 64, limit)
		if err != nil {
			return err
		}
		if len(chunks) == 0 {
			return fmt.Errorf("embedding document has no content")
		}

		vectors := make([]encodedChunk, len(chunks))
		var embedErr error
		// Bound worker batches (one on the NAS). Check cancellation between
		// batches; at most one existing client call can remain in flight.
		for first := 0; first < len(chunks); {
			if err = ctx.Err(); err != nil {
				return err
			}
			if first > 0 && config.BeforeBatch != nil {
				if err = config.BeforeBatch(ctx); err != nil {
					return err
				}
			}
			last := min(len(chunks), first+max(1, config.MaxBatch))
			results := embedChunkBatch(ctx, client, chunks[first:last])
			for j, result := range results {
				if result.Err != nil {
					embedErr = result.Err
					break
				}
				vectors[first+j].blob, vectors[first+j].norm, embedErr = validateSemanticVector(result.Vector, info.Dimensions)
				if embedErr != nil {
					break
				}
			}
			if embedErr != nil {
				break
			}
			first = last
		}
		err = i.withChunkStore(ctx, func(s ContentStore) error {
			return publishChunks(ctx, s.DB, chunkDocument{id: id, path: path, digest: digest, contentHash: contentHash, revision: revision, info: info, chunks: chunks, vectors: vectors, embedErr: embedErr, policy: ChunkPolicy(info, limit)})
		})
		if err != nil {
			return err
		}
		if embedErr != nil {
			return embedErr
		}
	}
	return nil
}

func publishChunks(ctx context.Context, db *sql.DB, doc chunkDocument) error {
	id, path, digest, contentHash, revision, info, chunks, vectors, embedErr := doc.id, doc.path, doc.digest, doc.contentHash, doc.revision, doc.info, doc.chunks, doc.vectors, doc.embedErr
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, chunkSchema+";"+semanticGraphSchema); err != nil {
		return err
	}
	return stateTransaction(ctx, conn, func() error {
		var currentHash, currentRevision, currentTitle, currentBody string
		var currentDescription *string
		if err := conn.QueryRowContext(ctx, "SELECT content_hash,repo_revision,title,description,body FROM concepts WHERE id=? AND path=?", id, path).Scan(&currentHash, &currentRevision, &currentTitle, &currentDescription, &currentBody); err != nil {
			return err
		}
		if currentHash != contentHash || fullEmbeddingHash(embeddingText(currentTitle, currentDescription, currentBody)) != digest {
			return fmt.Errorf("document changed during chunk embedding")
		}
		revision = currentRevision
		if embedErr != nil {
			var previousHash, previousModel, previousStatus, previousPolicy string
			err := conn.QueryRowContext(ctx, "SELECT e.embedding_text_hash,e.model_revision,e.status,COALESCE(p.policy,'') FROM concept_embeddings e LEFT JOIN concept_embedding_policy p ON p.concept_id=e.concept_id WHERE e.concept_id=?", id).Scan(&previousHash, &previousModel, &previousStatus, &previousPolicy)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if previousHash == digest && previousModel == info.Revision && previousStatus == "ready" && previousPolicy == doc.policy {
				return embedErr // keep last-known-good vectors for an unchanged document
			}
		}
		if _, err := conn.ExecContext(ctx, "DELETE FROM concept_embedding_chunks WHERE concept_id=?", id); err != nil {
			return err
		}
		status := "ready"
		var blob any = vectors[0].blob
		var norm any = vectors[0].norm
		var message any
		if embedErr != nil {
			status, blob, norm, message = "error", nil, nil, embedErr.Error()
		} else {
			for ordinal, chunk := range chunks {
				if _, err := conn.ExecContext(ctx, "INSERT INTO concept_embedding_chunks VALUES(?,?,?,?,?,?,?)", id, ordinal, digest, info.Revision, chunk, vectors[ordinal].blob, vectors[ordinal].norm); err != nil {
					return err
				}
			}
		}
		_, err := conn.ExecContext(ctx, `INSERT INTO concept_embeddings VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(concept_id) DO UPDATE SET path=excluded.path,embedding_text_hash=excluded.embedding_text_hash,model_id=excluded.model_id,dimensions=excluded.dimensions,embedding_revision=excluded.embedding_revision,status=excluded.status,model_revision=excluded.model_revision,embedding_blob=excluded.embedding_blob,embedding_norm=excluded.embedding_norm,updated_at=excluded.updated_at,error_message=excluded.error_message`, id, path, digest, info.ModelID, info.Dimensions, revision, status, info.Revision, blob, norm, time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), message)
		if err != nil {
			return err
		}
		if _, err = conn.ExecContext(ctx, `INSERT INTO concept_embedding_policy VALUES(?,?) ON CONFLICT(concept_id) DO UPDATE SET policy=excluded.policy`, id, doc.policy); err != nil {
			return err
		}
		if _, err = conn.ExecContext(ctx, "DELETE FROM concept_embedding_chunks WHERE concept_id NOT IN (SELECT concept_id FROM concept_embeddings)"); err != nil {
			return err
		}
		if err := refreshSemanticGraphItem(ctx, conn, id); err != nil {
			return err
		}
		return setEmbeddingRevision(ctx, conn, revision)
	})
}

// Chunk publication errors (constraint, cancellation, full disk) must roll back,
// not quarantine and replace an otherwise readable index. Explicit lifecycle
// recovery remains the responsibility of startup/rebuild.
func (i *Index) withChunkStore(ctx context.Context, fn func(ContentStore) error) error {
	if err := lockIndex(ctx, &i.mu); err != nil {
		return err
	}
	defer i.mu.Unlock()
	ops := defaultIndexIO()
	if err := validateIndexHeader(i.Path, ops); err != nil {
		return err
	}
	s, err := i.connect(ctx, ops)
	if err != nil {
		return err
	}
	defer s.DB.Close()
	return fn(s)
}

func lockIndex(ctx context.Context, mu *sync.Mutex) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if mu.TryLock() {
			return nil
		}
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func ChunkPolicy(info SemanticModelInfo, chars int) string {
	if chars <= 0 {
		chars = 4096
	}
	raw, _ := json.Marshal(struct {
		Version                string
		Tokens, Overlap, Chars int
		Model                  SemanticModelInfo
	}{"chunks-v1", 384, 64, chars, info})
	return string(raw)
}

// PrepareChunkEmbeddings classifies stored rows before serving searches. Legacy
// rows keep their rollback-readable bytes but are queued for chunk regeneration.
// A policy change invalidates only affected items; repository revision is unused.
func (i *Index) PrepareChunkEmbeddings(ctx context.Context) error {
	return i.withChunkStore(ctx, func(s ContentStore) error { return prepareChunkEmbeddings(ctx, s.DB, i.chunkModel, i.MaxInputChars) })
}
func prepareChunkEmbeddings(ctx context.Context, db *sql.DB, model SemanticModelInfo, maxInputChars int) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, chunkSchema+";"+semanticGraphSchema); err != nil {
		return err
	}
	return stateTransaction(ctx, conn, func() error {
		if _, err = conn.ExecContext(ctx, `UPDATE concept_embeddings SET status=CASE WHEN embedding_text_hash NOT LIKE 'chunks-v1:%' OR NOT EXISTS(SELECT 1 FROM concept_embedding_chunks k WHERE k.concept_id=concept_embeddings.concept_id AND k.document_hash=concept_embeddings.embedding_text_hash AND k.model_revision=concept_embeddings.model_revision) THEN 'legacy' ELSE 'stale' END WHERE status='ready' AND (embedding_text_hash NOT LIKE 'chunks-v1:%' OR model_id!=? OR model_revision!=? OR dimensions!=? OR NOT EXISTS(SELECT 1 FROM concept_embedding_policy p WHERE p.concept_id=concept_embeddings.concept_id AND p.policy=?) OR NOT EXISTS(SELECT 1 FROM concept_embedding_chunks k WHERE k.concept_id=concept_embeddings.concept_id AND k.document_hash=concept_embeddings.embedding_text_hash AND k.model_revision=concept_embeddings.model_revision))`, model.ModelID, model.Revision, model.Dimensions, ChunkPolicy(model, maxInputChars)); err != nil {
			return err
		}
		revision, err := requiredState(ctx, conn, "repo_revision")
		if err != nil {
			return err
		}
		if err := prepareSemanticGraphCache(ctx, conn); err != nil {
			return err
		}
		return setEmbeddingRevision(ctx, conn, revision)
	})
}

// ConfigureChunkModel sets immutable runtime model identity before the worker starts.
func (i *Index) ConfigureChunkModel(info SemanticModelInfo) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.chunkModel = info
}
func (i *Index) pendingChunkRows(ctx context.Context, db *sql.DB, limit int) (embeddingPathRows, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='concept_embedding_policy'`).Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		return db.QueryContext(ctx, `SELECT path FROM concepts ORDER BY path LIMIT ?`, limit)
	}
	return db.QueryContext(ctx, `SELECT c.path FROM concepts c LEFT JOIN concept_embeddings e ON e.concept_id=c.id LEFT JOIN concept_embedding_policy p ON p.concept_id=c.id WHERE e.concept_id IS NULL OR e.status IN ('stale','pending','legacy') OR e.model_id!=? OR e.model_revision!=? OR e.dimensions!=? OR COALESCE(p.policy,'')!=? OR (e.status='ready' AND NOT EXISTS (SELECT 1 FROM concept_embedding_chunks k WHERE k.concept_id=c.id AND k.document_hash=e.embedding_text_hash AND k.model_revision=e.model_revision)) ORDER BY CASE WHEN e.status='stale' THEN 0 ELSE 1 END,c.path LIMIT ?`, i.chunkModel.ModelID, i.chunkModel.Revision, i.chunkModel.Dimensions, ChunkPolicy(i.chunkModel, i.MaxInputChars), limit)
}
