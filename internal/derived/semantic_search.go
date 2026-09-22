package derived

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/rcarmo/memento/internal/access"
)

type SemanticSearchOptions struct {
	SearchOptions
	Hybrid        bool
	MaxCandidates int
	model         *SemanticModelInfo
	policy        string
}
type semanticRow struct {
	result SearchResult
	blob   []byte
	norm   float64
}

func (i *Index) SearchSemantic(ctx context.Context, policy access.EffectivePolicy, options SemanticSearchOptions, client SemanticClient) (page SearchPage, err error) {
	if client == nil {
		return page, errors.New("semantic search embedding client is unavailable")
	}
	if _, ok := client.(SemanticChunkClient); !ok {
		return page, errors.New("semantic embedding client does not support chunking")
	}
	info := client.ModelInfo()
	results := embedChunkBatch(ctx, client, []string{options.Query})
	if results[0].Err != nil {
		return page, results[0].Err
	}
	queryVector := results[0].Vector
	if _, _, err = validateSemanticVector(queryVector, info.Dimensions); err != nil {
		return page, err
	}
	queryNorm, _ := semanticNorm(queryVector)
	options.model = &info
	options.policy = ChunkPolicy(info, i.MaxInputChars)
	err = i.withChunkStore(ctx, func(s ContentStore) error {
		offset, e := decodeOffset(options.Cursor)
		if e != nil {
			return e
		}
		limit := max(1, min(options.Limit, 100))
		repo, e := requiredState(ctx, s.DB, "repo_revision")
		if e != nil {
			return e
		}
		index, e := requiredState(ctx, s.DB, "index_revision")
		if e != nil {
			return e
		}
		rows, e := semanticCandidateRows(ctx, s.DB, policy, options)
		if e != nil {
			return e
		}
		candidates := make([]SemanticCandidate, 0, len(rows))
		best := map[string]SemanticCandidate{}
		byID := map[string]SearchResult{}
		for _, row := range rows {
			cosine, e := semanticBlobCosine(queryVector, queryNorm, row.blob, row.norm, info.Dimensions)
			if e != nil {
				return e
			}
			previous, found := best[row.result.ConceptID]
			if !found || cosine > previous.Cosine {
				best[row.result.ConceptID] = SemanticCandidate{row.result.ConceptID, row.result.Path, cosine}
				byID[row.result.ConceptID] = row.result
			}
		}
		for _, candidate := range best {
			candidates = append(candidates, candidate)
		}
		lexicalIDs := []string{}
		if options.Hybrid {
			query, e := LexicalQuery(options.Query, options.Syntax)
			if e != nil {
				return e
			}
			lexical, e := searchRows(ctx, s.DB, policy, options.SearchOptions, query, options.MaxCandidates, 0)
			if e != nil {
				return e
			}
			for _, item := range lexical {
				lexicalIDs = append(lexicalIDs, item.ConceptID)
			}
		}
		ranked := RankSemanticCandidates(candidates, lexicalIDs, options.Hybrid, offset, limit)
		results := make([]SearchResult, 0, min(limit, len(ranked)))
		for _, item := range ranked[:min(limit, len(ranked))] {
			result := byID[item.ID]
			result.Score = item.Cosine
			results = append(results, result)
		}
		page = SearchPage{Results: results, RepoRevision: repo, IndexRevision: index, Warnings: []string{}}
		if len(ranked) > limit {
			cursor := "offset:" + strconv.Itoa(offset+limit)
			page.NextCursor = &cursor
		}
		return nil
	})
	return page, err
}
func semanticCandidateRows(ctx context.Context, db executor, policy access.EffectivePolicy, options SemanticSearchOptions) ([]semanticRow, error) {
	var tables int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('concept_embedding_chunks','concept_embedding_policy')").Scan(&tables); err != nil {
		return nil, err
	}
	if tables != 2 {
		return []semanticRow{}, nil
	}
	scope, args := authorizedPrefixes(policy, "e")
	conditions := []string{"e.status = 'ready'", scope, "EXISTS(SELECT 1 FROM concept_embedding_policy p WHERE p.concept_id=e.concept_id AND p.policy=?)"}
	parameters := append(append([]any{}, args...), options.policy)
	if options.model != nil {
		conditions = append(conditions, "e.model_id = ?", "e.model_revision = ?", "e.dimensions = ?")
		parameters = append(parameters, options.model.ModelID, options.model.Revision, options.model.Dimensions)
	}
	for _, filter := range []struct {
		field string
		value *string
	}{{"c.type", options.ConceptType}, {"c.status", options.Status}, {"e.path", options.PathPrefix}} {
		if filter.value != nil {
			if filter.field == "e.path" {
				conditions = append(conditions, `e.path LIKE ? ESCAPE '\'`)
				parameters = append(parameters, escapeLike(*filter.value)+"%")
			} else {
				conditions = append(conditions, filter.field+" = ?")
				parameters = append(parameters, *filter.value)
			}
		}
	}
	for _, tag := range options.Tags {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM json_each(c.tags_json) WHERE value = ?)")
		parameters = append(parameters, tag)
	}
	for index := range conditions {
		conditions[index] = "(" + conditions[index] + ")"
	}
	// Score all authorized chunks before taking the best per item and ranking.
	parameters = append(parameters, -1)
	rows, err := db.QueryContext(ctx, `SELECT c.id,c.path,c.title,c.type,c.status,c.tags_json,k.text,k.embedding_blob,k.embedding_norm FROM concept_embeddings e JOIN concepts c ON c.id=e.concept_id JOIN concept_embedding_chunks k ON k.concept_id=c.id AND k.document_hash=e.embedding_text_hash AND k.model_revision=e.model_revision WHERE `+strings.Join(conditions, " AND ")+` ORDER BY e.path,c.id,k.ordinal LIMIT ?`, parameters...)
	if err != nil {
		return nil, err
	}
	return readSemanticRows(rows)
}

type semanticRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

func readSemanticRows(rows semanticRows) ([]semanticRow, error) {
	defer rows.Close()
	out := []semanticRow{}
	for rows.Next() {
		var row semanticRow
		var tags string
		if err := rows.Scan(&row.result.ConceptID, &row.result.Path, &row.result.Title, &row.result.ConceptType, &row.result.Status, &tags, &row.result.Snippet, &row.blob, &row.norm); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(tags), &row.result.Tags); err != nil {
			return nil, err
		}
		if row.result.Tags == nil {
			row.result.Tags = []string{}
		}
		row.result.Snippet = boundedSnippet(row.result.Snippet)
		out = append(out, row)
	}
	return out, rows.Err()
}
func semanticNorm(vector []float32) (float64, error) {
	sum := 0.0
	for _, value := range vector {
		converted := float64(value)
		if !finite(converted) {
			return 0, errors.New("embedding contains non-finite value")
		}
		sum += converted * converted
	}
	if sum <= 0 {
		return 0, errors.New("embedding has zero or invalid norm")
	}
	return math.Sqrt(sum), nil
}
func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func semanticBlobCosine(left []float32, leftNorm float64, blob []byte, rightNorm float64, dimensions int) (float64, error) {
	if len(left) != dimensions || len(blob) != dimensions*4 {
		return 0, errors.New("vector dimension mismatch")
	}
	if leftNorm <= 0 || rightNorm <= 0 || !finite(rightNorm) {
		return 0, errors.New("embedding has zero or invalid norm")
	}
	dot := 0.0
	for index, l := range left {
		r := math.Float32frombits(binary.LittleEndian.Uint32(blob[index*4:]))
		if !finite(float64(r)) {
			return 0, errors.New("embedding contains non-finite value")
		}
		dot += float64(l) * float64(r)
	}
	return dot / (leftNorm * rightNorm), nil
}
