package derived

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/rcarmo/memento/go/access"
)

type SemanticSearchOptions struct {
	SearchOptions
	Hybrid        bool
	MaxCandidates int
}
type semanticRow struct {
	result SearchResult
	blob   []byte
}

func (i *Index) SearchSemantic(ctx context.Context, policy access.EffectivePolicy, options SemanticSearchOptions, client SemanticClient) (page SearchPage, err error) {
	if client == nil {
		return page, errors.New("semantic search embedding client is unavailable")
	}
	err = i.withCore(ctx, false, func(s ContentStore) error {
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
		queryVector, e := client.Embed(options.Query)
		if e != nil {
			return e
		}
		info := client.ModelInfo()
		if _, _, e = validateSemanticVector(queryVector, info.Dimensions); e != nil {
			return e
		}
		rows, e := semanticCandidateRows(ctx, s.DB, policy, options)
		if e != nil {
			return e
		}
		candidates := make([]SemanticCandidate, 0, len(rows))
		byID := map[string]SearchResult{}
		for _, row := range rows {
			vector, e := unpackSemanticBlob(row.blob, info.Dimensions)
			if e != nil {
				return e
			}
			cosine, e := semanticCosine(queryVector, vector)
			if e != nil {
				return e
			}
			candidates = append(candidates, SemanticCandidate{row.result.ConceptID, row.result.Path, cosine})
			byID[row.result.ConceptID] = row.result
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
	scope, args := authorizedPrefixes(policy, "e")
	conditions := []string{"e.status = 'ready'", scope}
	parameters := append([]any{}, args...)
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
	maxCandidates := options.MaxCandidates
	if maxCandidates < 1 {
		maxCandidates = 200
	}
	parameters = append(parameters, maxCandidates)
	rows, err := db.QueryContext(ctx, `SELECT c.id,c.path,c.title,c.type,c.status,c.tags_json,c.title,e.embedding_blob FROM concept_embeddings e JOIN concepts c ON c.id=e.concept_id WHERE `+strings.Join(conditions, " AND ")+` ORDER BY e.path,c.id LIMIT ?`, parameters...)
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
		if err := rows.Scan(&row.result.ConceptID, &row.result.Path, &row.result.Title, &row.result.ConceptType, &row.result.Status, &tags, &row.result.Snippet, &row.blob); err != nil {
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
func unpackSemanticBlob(blob []byte, dimensions int) ([]float32, error) {
	if len(blob) != dimensions*4 {
		return nil, errors.New("invalid embedding blob length")
	}
	out := make([]float32, dimensions)
	for index := range out {
		out[index] = math.Float32frombits(binary.LittleEndian.Uint32(blob[index*4:]))
	}
	return out, nil
}
func semanticCosine(left, right []float32) (float64, error) {
	if len(left) != len(right) {
		return 0, errors.New("vector dimension mismatch")
	}
	dot, ln, rn := 0.0, 0.0, 0.0
	for index, l := range left {
		r := right[index]
		if math.IsNaN(float64(l)) || math.IsInf(float64(l), 0) || math.IsNaN(float64(r)) || math.IsInf(float64(r), 0) {
			return 0, errors.New("embedding contains non-finite value")
		}
		dot += float64(l) * float64(r)
		ln += float64(l) * float64(l)
		rn += float64(r) * float64(r)
	}
	if ln <= 0 || rn <= 0 {
		return 0, errors.New("embedding has zero or invalid norm")
	}
	return dot / (math.Sqrt(ln) * math.Sqrt(rn)), nil
}
