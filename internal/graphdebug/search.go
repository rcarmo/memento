package graphdebug

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rcarmo/memento/internal/access"
)

var graphWords = regexp.MustCompile(`[\pL\pN_]+`)

type SearchResult struct {
	ID      string   `json:"id"`
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Type    string   `json:"type"`
	Tags    []string `json:"tags"`
	Snippet string   `json:"snippet"`
}
type SearchResults struct {
	SchemaVersion int            `json:"schema_version"`
	Results       []SearchResult `json:"results"`
}

func plainFTSQuery(query string) (string, error) {
	terms := graphWords.FindAllString(query, -1)
	if len(terms) == 0 {
		return "", errors.New("search query must contain words")
	}
	for i, term := range terms {
		terms[i] = `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
	}
	return strings.Join(terms, " "), nil
}
func graphSnippet(value string) string {
	compact := strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(compact) <= 240 {
		return compact
	}
	runes := []rune(compact)
	return string(runes[:240])
}
func (s *SnapshotService) Search(ctx context.Context, query string, policy *access.EffectivePolicy) (SearchResults, error) {
	empty := SearchResults{}
	expression, err := plainFTSQuery(strings.TrimSpace(query))
	if err != nil {
		return empty, err
	}
	db, err := s.open(ctx, s.DerivedDBPath)
	if err != nil {
		return empty, err
	}
	defer db.Close()
	conditions := []string{"concept_fts MATCH ?", "c.path NOT LIKE '/trash/%'"}
	args := []any{expression}
	if policy != nil {
		scope, scopeArgs := pathScopeSQL("c.path", policy)
		conditions = append(conditions, scope)
		args = append(args, scopeArgs...)
	}
	args = append(args, 20)
	rows, err := db.QueryContext(ctx, `SELECT c.id,c.path,c.title,c.type,c.tags_json,snippet(concept_fts,5,'','',' … ',16) FROM concept_fts JOIN concepts c ON c.id=concept_fts.concept_id WHERE `+strings.Join(conditions, " AND ")+` ORDER BY bm25(concept_fts,10.0,5.0,5.0,4.0,1.0,5.0),c.id LIMIT ?`, args...)
	if err != nil {
		return empty, err
	}
	defer rows.Close()
	results := []SearchResult{}
	for rows.Next() {
		var item SearchResult
		var tags, snippet string
		if err = rows.Scan(&item.ID, &item.Path, &item.Title, &item.Type, &tags, &snippet); err != nil {
			return empty, err
		}
		if err = decodeStrings(tags, &item.Tags); err != nil {
			return empty, err
		}
		if snippet == "" {
			snippet = item.Title
		}
		item.Snippet = graphSnippet(snippet)
		results = append(results, item)
	}
	if err = rows.Err(); err != nil {
		return empty, err
	}
	return SearchResults{SchemaVersion: 1, Results: results}, nil
}
func decodeStrings(raw string, target *[]string) error { return json.Unmarshal([]byte(raw), target) }
