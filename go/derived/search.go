package derived

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rcarmo/memento/go/access"
	"golang.org/x/text/cases"
)

type SearchError struct{ Message string }

func (e *SearchError) Error() string { return e.Message }

type SearchOptions struct {
	Query                           string
	Syntax                          string // explicit plain or fts5; callers provide the source default fts5
	ConceptType, Status, PathPrefix *string
	Tags                            []string
	Limit                           int
	Cursor                          *string
	Strict                          bool
	Timeout                         time.Duration
}
type SearchResult struct {
	ConceptID   string   `json:"concept_id"`
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	ConceptType string   `json:"concept_type"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
	Score       float64  `json:"score"`
	Snippet     string   `json:"snippet"`
}
type SearchPage struct {
	Results       []SearchResult `json:"results"`
	NextCursor    *string        `json:"next_cursor"`
	RepoRevision  string         `json:"repo_revision"`
	IndexRevision string         `json:"index_revision"`
	Warnings      []string       `json:"warnings"`
}

// SearchLexical is the eventual, models-disabled search kernel. The daemon must
// migrate/initialise and supply trusted policy before calling it. Strict polling,
// semantic fallback and corruption quarantine are separate lifecycle work.
func (s ContentStore) SearchLexical(ctx context.Context, policy access.EffectivePolicy, options SearchOptions) (SearchPage, error) {
	empty := SearchPage{}
	if options.Strict {
		if _, err := s.WaitForFreshness(ctx, options.Timeout); err != nil {
			return empty, err
		}
	}
	offset, err := decodeOffset(options.Cursor)
	if err != nil {
		return empty, err
	}
	limit := max(1, min(options.Limit, 100))
	status, err := getState(ctx, s.DB, "status")
	if err != nil {
		return empty, err
	}
	if status != nil && *status == "quarantined" {
		return empty, &CorruptionError{"derived index is quarantined"}
	}
	repo, err := requiredState(ctx, s.DB, "repo_revision")
	if err != nil {
		return empty, err
	}
	index, err := requiredState(ctx, s.DB, "index_revision")
	if err != nil {
		return empty, err
	}
	query, err := LexicalQuery(options.Query, options.Syntax)
	if err != nil {
		return empty, err
	}
	rows, err := searchRows(ctx, s.DB, policy, options, query, limit+1, offset)
	if err != nil {
		return empty, err
	}
	page := SearchPage{Results: rows[:min(limit, len(rows))], RepoRevision: repo, IndexRevision: index, Warnings: []string{}}
	if len(rows) > limit {
		cursor := "offset:" + strconv.Itoa(offset+limit)
		page.NextCursor = &cursor
	}
	return page, nil
}
func requiredState(ctx context.Context, db executor, key string) (string, error) {
	value, err := getState(ctx, db, key)
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", &CorruptionError{"missing index state: " + key}
	}
	return *value, nil
}
func searchRows(ctx context.Context, db executor, policy access.EffectivePolicy, options SearchOptions, query string, limit, offset int) ([]SearchResult, error) {
	scope, args := authorizedPrefixes(policy, "c")
	conditions := []string{"concept_fts MATCH ?", scope}
	parameters := []any{query}
	parameters = append(parameters, args...)
	for _, filter := range []struct {
		field string
		value *string
	}{{"c.type", options.ConceptType}, {"c.status", options.Status}, {"c.path", options.PathPrefix}} {
		if filter.value != nil {
			if filter.field == "c.path" {
				conditions = append(conditions, `c.path LIKE ? ESCAPE '\'`)
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
	for i, c := range conditions {
		conditions[i] = "(" + c + ")"
	}
	parameters = append(parameters, limit, offset)
	rows, err := db.QueryContext(ctx, `SELECT c.id,c.path,c.title,c.type,c.status,c.tags_json,snippet(concept_fts,5,'','',' … ',16) AS snippet,bm25(concept_fts,10.0,5.0,5.0,4.0,1.0,5.0) AS score FROM concept_fts JOIN concepts c ON c.id=concept_fts.concept_id WHERE `+strings.Join(conditions, " AND ")+" ORDER BY score,c.id LIMIT ? OFFSET ?", parameters...)
	if err != nil {
		return nil, searchQueryError(err)
	}
	return readSearchRows(rows)
}
func readSearchRows(rows rows) ([]SearchResult, error) {
	defer rows.Close()
	out := []SearchResult{}
	for rows.Next() {
		var r SearchResult
		var tags string
		var snippet sql.NullString
		var score sql.NullFloat64
		if err := rows.Scan(&r.ConceptID, &r.Path, &r.Title, &r.ConceptType, &r.Status, &tags, &snippet, &score); err != nil {
			return nil, searchQueryError(err)
		}
		if err := json.Unmarshal([]byte(tags), &r.Tags); err != nil {
			return nil, err
		}
		if r.Tags == nil {
			r.Tags = []string{}
		}
		if score.Valid {
			r.Score = -score.Float64
		}
		text := snippet.String
		if text == "" {
			text = r.Title
		}
		r.Snippet = boundedSnippet(text)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, searchQueryError(err)
	}
	return out, nil
}
func searchQueryError(err error) error {
	message := strings.ToLower(err.Error())
	for _, part := range []string{"unterminated string", "malformed match expression", "fts5: syntax error", "no such column"} {
		if strings.Contains(message, part) {
			return &SearchError{"invalid FTS query"}
		}
	}
	return err
}
func prefixConditions(prefixes []string, column string) (string, []any) {
	clauses := []string{}
	args := []any{}
	for _, prefix := range prefixes {
		clauses = append(clauses, "("+column+" = substr(?,1,length(?) - 1) OR "+column+` LIKE ? ESCAPE '\')`)
		args = append(args, prefix, prefix, escapeLike(prefix)+"%")
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}
func authorizedPrefixes(policy access.EffectivePolicy, alias string) (string, []any) {
	column := alias + ".path"
	allowed, args := prefixConditions(policy.ReadPrefixes, column)
	sql := allowed + " AND " + column + " NOT LIKE '/trash/%'"
	for _, grant := range access.ProtectedReadGrants(policy) {
		protected, params := prefixConditions([]string{grant.Prefix}, column)
		args = append(args, params...)
		if len(grant.Explicit) > 0 {
			explicit, params := prefixConditions(grant.Explicit, column)
			sql += " AND (NOT " + protected + " OR " + explicit + ")"
			args = append(args, params...)
		} else {
			sql += " AND NOT " + protected
		}
	}
	return "(" + sql + ")", args
}
func escapeLike(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}

var stopWords = func() map[string]bool {
	out := map[string]bool{}
	for _, word := range strings.Fields("a about an and are at be been being by can could did do does for from how in is it me of on please should tell the this to was were what when where which who whom whose why will with would") {
		out[word] = true
	}
	return out
}()

func LexicalQuery(query, syntax string) (string, error) {
	if syntax == "fts5" {
		if strings.TrimFunc(query, pythonSpace) == "" {
			return "", &SearchError{"search query must not be empty"}
		}
		return query, nil
	}
	if syntax != "plain" {
		return "", &SearchError{"unsupported query_syntax: " + syntax}
	}
	terms := strings.FieldsFunc(query, func(r rune) bool { return !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_') })
	if len(terms) == 0 {
		return "", &SearchError{"plain search query must contain at least one word"}
	}
	significant := []string{}
	for _, term := range terms {
		if !stopWords[cases.Fold().String(term)] {
			significant = append(significant, term)
		}
	}
	if len(significant) > 0 {
		terms = significant
	}
	seen := map[string]bool{}
	quoted := []string{}
	for _, term := range terms {
		if !seen[term] {
			seen[term] = true
			quoted = append(quoted, `"`+term+`"`)
		}
	}
	return strings.Join(quoted, " OR "), nil
}
func pythonSpace(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }
func boundedSnippet(text string) string {
	var out strings.Builder
	out.Grow(min(len(text), 240))
	space, count := true, 0
	for _, value := range text {
		if pythonSpace(value) {
			space = true
			continue
		}
		if space && out.Len() > 0 {
			if count >= 240 {
				break
			}
			out.WriteByte(' ')
			count++
		}
		if count >= 240 {
			break
		}
		out.WriteRune(value)
		space = false
		count++
	}
	return out.String()
}
func decodeOffset(cursor *string) (int, error) {
	if cursor == nil {
		return 0, nil
	}
	if !strings.HasPrefix(*cursor, "offset:") {
		return 0, errors.New("unsupported cursor")
	}
	raw := strings.TrimFunc(strings.TrimPrefix(*cursor, "offset:"), pythonSpace)
	// Finite Go offsets are deliberate: arbitrary-size Python int offsets and
	// Unicode-decimal grammar require a wider cursor coercion port.
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor offset: %s", raw)
	}
	if value < 0 {
		return 0, nil
	}
	if value > int64(^uint(0)>>1)-101 {
		return 0, errors.New("cursor offset exceeds supported range")
	}
	return int(value), nil
}
