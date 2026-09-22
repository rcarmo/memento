package derived

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
)

const semanticGraphAlgorithm = "mean-normalized-chunks-v1"

const semanticGraphSchema = `CREATE TABLE IF NOT EXISTS semantic_graph_items (
 concept_id TEXT PRIMARY KEY,
 document_hash TEXT NOT NULL,
 model_id TEXT,
 model_revision TEXT,
 dimensions INTEGER,
 policy TEXT,
 algorithm TEXT,
 embedding_blob BLOB,
 embedding_norm REAL
);
CREATE TABLE IF NOT EXISTS semantic_graph_pairs (
 source_id TEXT,
 target_id TEXT,
 similarity REAL,
 PRIMARY KEY(source_id,target_id),
 CHECK(source_id<target_id)
);
CREATE INDEX IF NOT EXISTS idx_semantic_graph_pairs_target ON semantic_graph_pairs(target_id);
CREATE TRIGGER IF NOT EXISTS delete_semantic_graph_item_pairs AFTER DELETE ON semantic_graph_items
BEGIN
 DELETE FROM semantic_graph_pairs WHERE source_id=OLD.concept_id OR target_id=OLD.concept_id;
END;
CREATE TRIGGER IF NOT EXISTS invalidate_semantic_graph_on_embedding_delete AFTER DELETE ON concept_embeddings
BEGIN
 DELETE FROM semantic_graph_items WHERE concept_id=OLD.concept_id;
END;
CREATE TRIGGER IF NOT EXISTS invalidate_semantic_graph_on_embedding_update AFTER UPDATE OF status,embedding_text_hash,model_id,model_revision,dimensions ON concept_embeddings
WHEN OLD.status IS NOT NEW.status
 OR OLD.embedding_text_hash IS NOT NEW.embedding_text_hash
 OR OLD.model_id IS NOT NEW.model_id
 OR OLD.model_revision IS NOT NEW.model_revision
 OR OLD.dimensions IS NOT NEW.dimensions
BEGIN
 DELETE FROM semantic_graph_items WHERE concept_id=NEW.concept_id;
END;
CREATE TRIGGER IF NOT EXISTS invalidate_semantic_graph_on_policy_delete AFTER DELETE ON concept_embedding_policy
BEGIN
 DELETE FROM semantic_graph_items WHERE concept_id=OLD.concept_id;
END;
CREATE TRIGGER IF NOT EXISTS invalidate_semantic_graph_on_policy_update AFTER UPDATE OF policy ON concept_embedding_policy
WHEN OLD.policy IS NOT NEW.policy
BEGIN
 DELETE FROM semantic_graph_items WHERE concept_id=NEW.concept_id;
END;
CREATE TRIGGER IF NOT EXISTS invalidate_semantic_graph_on_chunk_insert AFTER INSERT ON concept_embedding_chunks
BEGIN
 DELETE FROM semantic_graph_items WHERE concept_id=NEW.concept_id;
END;
CREATE TRIGGER IF NOT EXISTS invalidate_semantic_graph_on_chunk_update AFTER UPDATE ON concept_embedding_chunks
BEGIN
 DELETE FROM semantic_graph_items WHERE concept_id=OLD.concept_id;
 DELETE FROM semantic_graph_items WHERE concept_id=NEW.concept_id;
END;
CREATE TRIGGER IF NOT EXISTS invalidate_semantic_graph_on_chunk_delete AFTER DELETE ON concept_embedding_chunks
BEGIN
 DELETE FROM semantic_graph_items WHERE concept_id=OLD.concept_id;
END`

// PrepareSemanticGraphCache backfills persisted graph scores without inference.
// Existing compatible aggregates are left untouched; web readers never call it.
func PrepareSemanticGraphCache(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, semanticGraphSchema); err != nil {
		return err
	}
	return stateTransaction(ctx, conn, func() error { return prepareSemanticGraphCache(ctx, conn) })
}

type semanticGraphIdentity struct {
	conceptID, documentHash, modelID, modelRevision, policy string
	dimensions                                              int
}

type semanticGraphCachedItem struct {
	conceptID string
	blob      []byte
	norm      float64
}

func refreshSemanticGraphItem(ctx context.Context, db executor, id string) error {
	if _, err := db.ExecContext(ctx, "DELETE FROM semantic_graph_items WHERE concept_id=?", id); err != nil {
		return err
	}
	identity, ok, err := semanticGraphIdentityRow(ctx, db, id)
	if err != nil || !ok {
		return err
	}
	blob, norm, ok, err := semanticGraphAggregate(ctx, db, identity)
	if err != nil || !ok {
		return err
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO semantic_graph_items(concept_id,document_hash,model_id,model_revision,dimensions,policy,algorithm,embedding_blob,embedding_norm) VALUES(?,?,?,?,?,?,?,?,?)`, identity.conceptID, identity.documentHash, identity.modelID, identity.modelRevision, identity.dimensions, identity.policy, semanticGraphAlgorithm, blob, norm); err != nil {
		return err
	}
	if norm <= 0 || !finite(norm) {
		return nil
	}
	left := make([]float32, identity.dimensions)
	for i := range left {
		left[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	others, err := semanticGraphPeerRows(ctx, db, identity)
	if err != nil {
		return err
	}
	for _, other := range others {
		cosine, ok := semanticGraphCosine(left, norm, other.blob, other.norm, identity.dimensions)
		if !ok {
			continue
		}
		sourceID, targetID := identity.conceptID, other.conceptID
		if sourceID > targetID {
			sourceID, targetID = targetID, sourceID
		}
		if _, err = db.ExecContext(ctx, "INSERT INTO semantic_graph_pairs(source_id,target_id,similarity) VALUES(?,?,?)", sourceID, targetID, cosine); err != nil {
			return err
		}
	}
	return nil
}

func prepareSemanticGraphCache(ctx context.Context, db executor) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM semantic_graph_items
WHERE NOT EXISTS (
 SELECT 1
 FROM concepts c
 JOIN concept_embeddings e ON e.concept_id=semantic_graph_items.concept_id
 JOIN concept_embedding_policy p ON p.concept_id=semantic_graph_items.concept_id
 WHERE c.id=semantic_graph_items.concept_id
 AND e.status='ready'
 AND semantic_graph_items.document_hash=e.embedding_text_hash
 AND semantic_graph_items.model_id=e.model_id
 AND semantic_graph_items.model_revision=e.model_revision
 AND semantic_graph_items.dimensions=e.dimensions
 AND semantic_graph_items.policy=p.policy
 AND semantic_graph_items.algorithm=?
 AND EXISTS (
  SELECT 1 FROM concept_embedding_chunks k
  WHERE k.concept_id=e.concept_id AND k.document_hash=e.embedding_text_hash AND k.model_revision=e.model_revision
 )
 AND length(semantic_graph_items.embedding_blob)=e.dimensions*4
 AND semantic_graph_items.embedding_norm IS NOT NULL
 AND semantic_graph_items.embedding_norm>=0
 AND ABS(semantic_graph_items.embedding_norm)<=1.0e308
)`, semanticGraphAlgorithm); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT e.concept_id
FROM concepts c
JOIN concept_embeddings e ON e.concept_id=c.id
JOIN concept_embedding_policy p ON p.concept_id=e.concept_id
WHERE e.status='ready'
AND e.dimensions>0
AND EXISTS (
 SELECT 1 FROM concept_embedding_chunks k
 WHERE k.concept_id=e.concept_id AND k.document_hash=e.embedding_text_hash AND k.model_revision=e.model_revision
)
AND NOT EXISTS (
 SELECT 1 FROM semantic_graph_items g
 WHERE g.concept_id=e.concept_id
 AND g.document_hash=e.embedding_text_hash
 AND g.model_id=e.model_id
 AND g.model_revision=e.model_revision
 AND g.dimensions=e.dimensions
 AND g.policy=p.policy
 AND g.algorithm=?
 AND length(g.embedding_blob)=e.dimensions*4
 AND g.embedding_norm IS NOT NULL
 AND g.embedding_norm>=0
 AND ABS(g.embedding_norm)<=1.0e308
)
ORDER BY c.path,e.concept_id`, semanticGraphAlgorithm)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err = refreshSemanticGraphItem(ctx, db, id); err != nil {
			return err
		}
	}
	return nil
}

func semanticGraphIdentityRow(ctx context.Context, db executor, id string) (semanticGraphIdentity, bool, error) {
	item := semanticGraphIdentity{}
	err := db.QueryRowContext(ctx, `SELECT e.concept_id,e.embedding_text_hash,e.model_id,e.model_revision,e.dimensions,p.policy
FROM concept_embeddings e
JOIN concepts c ON c.id=e.concept_id
JOIN concept_embedding_policy p ON p.concept_id=e.concept_id
WHERE e.concept_id=? AND e.status='ready'`, id).Scan(&item.conceptID, &item.documentHash, &item.modelID, &item.modelRevision, &item.dimensions, &item.policy)
	if err == nil && item.dimensions > 0 {
		return item, true, nil
	}
	if err == nil || err == sql.ErrNoRows {
		return semanticGraphIdentity{}, false, nil
	}
	return semanticGraphIdentity{}, false, err
}

func semanticGraphAggregate(ctx context.Context, db executor, item semanticGraphIdentity) ([]byte, float64, bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT embedding_blob
FROM concept_embedding_chunks
WHERE concept_id=? AND document_hash=? AND model_revision=?
ORDER BY ordinal`, item.conceptID, item.documentHash, item.modelRevision)
	if err != nil {
		return nil, 0, false, err
	}
	defer rows.Close()
	sum := make([]float64, item.dimensions)
	matched := 0
	for rows.Next() {
		var blob []byte
		if err = rows.Scan(&blob); err != nil {
			return nil, 0, false, err
		}
		matched++
		semanticGraphAddNormalizedChunk(sum, blob)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, false, err
	}
	if matched == 0 {
		return nil, 0, false, nil
	}
	blob, norm := semanticGraphEncode(sum)
	return blob, norm, true, nil
}

func semanticGraphAddNormalizedChunk(sum []float64, blob []byte) {
	if len(blob) != len(sum)*4 {
		return
	}
	normSquared := 0.0
	for index := range sum {
		value := float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[index*4:])))
		if !finite(value) {
			return
		}
		normSquared += value * value
	}
	norm := math.Sqrt(normSquared)
	if !finite(norm) || norm <= 0 {
		return
	}
	for index := range sum {
		value := float64(math.Float32frombits(binary.LittleEndian.Uint32(blob[index*4:])))
		sum[index] += value / norm
	}
}

func semanticGraphEncode(sum []float64) ([]byte, float64) {
	blob := make([]byte, len(sum)*4)
	normSquared := 0.0
	for _, value := range sum {
		if !finite(value) {
			return blob, 0
		}
		normSquared += value * value
	}
	norm := math.Sqrt(normSquared)
	if !finite(norm) || norm <= 0 {
		return blob, 0
	}
	storedSquared := 0.0
	for index, value := range sum {
		normalized := float32(value / norm)
		converted := float64(normalized)
		binary.LittleEndian.PutUint32(blob[index*4:], math.Float32bits(normalized))
		storedSquared += converted * converted
	}
	storedNorm := math.Sqrt(storedSquared)
	return blob, storedNorm
}

func semanticGraphPeerRows(ctx context.Context, db executor, item semanticGraphIdentity) ([]semanticGraphCachedItem, error) {
	rows, err := db.QueryContext(ctx, `SELECT g.concept_id,g.embedding_blob,g.embedding_norm
FROM semantic_graph_items g
JOIN concepts c ON c.id=g.concept_id
JOIN concept_embeddings e ON e.concept_id=g.concept_id
JOIN concept_embedding_policy p ON p.concept_id=g.concept_id
WHERE g.concept_id<>?
AND g.algorithm=?
AND g.model_id=?
AND g.model_revision=?
AND g.dimensions=?
AND g.policy=?
AND e.status='ready'
AND g.document_hash=e.embedding_text_hash
AND g.model_id=e.model_id
AND g.model_revision=e.model_revision
AND g.dimensions=e.dimensions
AND g.policy=p.policy
AND EXISTS (
 SELECT 1 FROM concept_embedding_chunks k
 WHERE k.concept_id=g.concept_id AND k.document_hash=g.document_hash AND k.model_revision=g.model_revision
)
ORDER BY g.concept_id`, item.conceptID, semanticGraphAlgorithm, item.modelID, item.modelRevision, item.dimensions, item.policy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []semanticGraphCachedItem{}
	for rows.Next() {
		var cached semanticGraphCachedItem
		if err = rows.Scan(&cached.conceptID, &cached.blob, &cached.norm); err != nil {
			return nil, err
		}
		out = append(out, cached)
	}
	return out, rows.Err()
}

func semanticGraphCosine(left []float32, leftNorm float64, blob []byte, rightNorm float64, dimensions int) (float64, bool) {
	cosine, err := semanticBlobCosine(left, leftNorm, blob, rightNorm, dimensions)
	if err != nil || !finite(cosine) {
		return 0, false
	}
	return cosine, true
}
