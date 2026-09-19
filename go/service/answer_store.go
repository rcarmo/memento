package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcarmo/memento/go/control"
)

const UnknownAnswer = "UNKNOWN"

type AnswerCitation struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Revision string `json:"revision"`
}
type AnswerRecord struct {
	Answer       string                 `json:"answer"`
	AnswerSource string                 `json:"answer_source"`
	Confidence   string                 `json:"confidence"`
	Unresolved   []string               `json:"unresolved"`
	Citations    []AnswerCitation       `json:"citations"`
	Evidence     any                    `json:"evidence"`
	TraceID      *string                `json:"trace_id"`
	ModelChain   []control.ModelAttempt `json:"model_chain"`
}
type AnswerReadConcept struct {
	ID, Path, Title, Body, Revision, Status string
	Tags, SourceRefs, Supersedes            []string
	UpdatedAt                               time.Time
}
type AnswerSearchStep struct {
	Action string `json:"action"`
	Detail string `json:"detail"`
}
type DeepAnswerResult struct {
	Record       AnswerRecord
	ReadConcepts []AnswerReadConcept
	Steps        []AnswerSearchStep
	DurationMS   int
	Usage        map[string]int
}
type AnswerStore struct {
	DB  *sql.DB
	Now func() time.Time
}

func (s AnswerStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}
func answerISO(t time.Time) string      { return t.UTC().Truncate(time.Second).Format(time.RFC3339) }
func answerHash(v string) string        { x := sha256.Sum256([]byte(v)); return hex.EncodeToString(x[:]) }
func NormalizeQuestion(v string) string { return strings.ToLower(strings.Join(strings.Fields(v), " ")) }
func ScopeFingerprint(principal string, roles, reads, protected []string) string {
	dedupe := func(v []string) []string {
		m := map[string]bool{}
		for _, x := range v {
			m[x] = true
		}
		o := make([]string, 0, len(m))
		for x := range m {
			o = append(o, x)
		}
		sort.Strings(o)
		return o
	}
	encode := func(v any) string { b, _ := json.Marshal(v); return strings.ReplaceAll(string(b), ",", ", ") }
	// Python json.dumps(sort_keys=True) uses lexical key order and spaces after
	// separators; these exact bytes are part of the persisted cache key contract.
	material := `{"principal": ` + encode(principal) + `, "protected_read_prefixes": ` + encode(dedupe(protected)) + `, "read_prefixes": ` + encode(dedupe(reads)) + `, "roles": ` + encode(dedupe(roles)) + `}`
	return answerHash(material)
}
func ExactAnswerCacheKey(revision, question, scope, mode, policy, prompt, tool string) string {
	return answerHash(strings.Join([]string{revision, question, scope, mode, policy, prompt, tool}, "\n"))
}
func (s AnswerStore) Migrate(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS answer_cache(cache_key TEXT PRIMARY KEY,scope_key TEXT NOT NULL,repo_revision TEXT NOT NULL,normalized_question TEXT NOT NULL,answer_mode TEXT NOT NULL,response_json TEXT NOT NULL,cited_ids_json TEXT NOT NULL,read_ids_json TEXT NOT NULL,created_at TEXT NOT NULL,expires_at TEXT NOT NULL,last_accessed_at TEXT NOT NULL);CREATE TABLE IF NOT EXISTS hot_changed_concepts(scope_key TEXT NOT NULL,concept_id TEXT NOT NULL,observed_at TEXT NOT NULL,PRIMARY KEY(scope_key,concept_id));CREATE TABLE IF NOT EXISTS hot_answers(scope_key TEXT NOT NULL,question_hash TEXT NOT NULL,normalized_question TEXT NOT NULL,answer_mode TEXT NOT NULL,repo_revision TEXT NOT NULL,response_json TEXT NOT NULL,concept_ids_json TEXT NOT NULL,created_at TEXT NOT NULL,expires_at TEXT NOT NULL,PRIMARY KEY(scope_key,question_hash,answer_mode,repo_revision));CREATE TABLE IF NOT EXISTS answer_traces(trace_id TEXT PRIMARY KEY,principal TEXT NOT NULL,scope_key TEXT NOT NULL,question_hash TEXT NOT NULL,question_excerpt TEXT NOT NULL,repo_revision TEXT NOT NULL,answer_summary TEXT NOT NULL,steps_json TEXT NOT NULL,paths_json TEXT NOT NULL,model_chain_json TEXT NOT NULL,usage_json TEXT NOT NULL,duration_ms INTEGER NOT NULL,created_at TEXT NOT NULL);CREATE INDEX IF NOT EXISTS idx_answer_cache_lru ON answer_cache(last_accessed_at);CREATE INDEX IF NOT EXISTS idx_hot_changed_scope ON hot_changed_concepts(scope_key,observed_at DESC);CREATE INDEX IF NOT EXISTS idx_hot_answers_scope ON hot_answers(scope_key,answer_mode,repo_revision,created_at DESC);CREATE INDEX IF NOT EXISTS idx_answer_traces_created ON answer_traces(created_at DESC)`)
	return err
}
func (s AnswerStore) GetExact(ctx context.Context, key string) (*AnswerRecord, error) {
	now := answerISO(s.now())
	var raw, expires string
	err := s.DB.QueryRowContext(ctx, "SELECT response_json,expires_at FROM answer_cache WHERE cache_key=?", key).Scan(&raw, &expires)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if expires <= now {
		_, err = s.DB.ExecContext(ctx, "DELETE FROM answer_cache WHERE cache_key=?", key)
		return nil, err
	}
	if _, err = s.DB.ExecContext(ctx, "UPDATE answer_cache SET last_accessed_at=? WHERE cache_key=?", now, key); err != nil {
		return nil, err
	}
	var r AnswerRecord
	if err = json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	r.AnswerSource = "exact_cache"
	return &r, nil
}
func (s AnswerStore) PutExact(ctx context.Context, key, scope, revision, question, mode string, r AnswerRecord, cited, read []string, ttl, max int) error {
	raw, _ := json.Marshal(r)
	ids := func(v []string) string { sort.Strings(v); b, _ := json.Marshal(v); return string(b) }
	now := s.now()
	return withAnswerTx(ctx, s.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO answer_cache VALUES(?,?,?,?,?,?,?,?,?,?,?)`, key, scope, revision, question, mode, string(raw), ids(cited), ids(read), answerISO(now), answerISO(now.Add(time.Duration(ttl)*time.Second)), answerISO(now)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM answer_cache WHERE expires_at<=? OR cache_key NOT IN (SELECT cache_key FROM answer_cache ORDER BY last_accessed_at DESC LIMIT ?)", answerISO(now), max)
		return err
	})
}
func (s AnswerStore) GetHotContext(ctx context.Context, scope, question, mode, revision string) ([]string, *AnswerRecord, error) {
	now := s.now()
	if _, err := s.DB.ExecContext(ctx, "DELETE FROM hot_changed_concepts WHERE observed_at<=?;DELETE FROM hot_answers WHERE expires_at<=?", answerISO(now.Add(-3650*24*time.Hour)), answerISO(now)); err != nil {
		return nil, nil, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT concept_id FROM hot_changed_concepts WHERE scope_key=? ORDER BY observed_at DESC LIMIT 10", scope)
	if err != nil {
		return nil, nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Close(); err != nil {
		return nil, nil, err
	}
	var raw string
	err = s.DB.QueryRowContext(ctx, "SELECT response_json FROM hot_answers WHERE scope_key=? AND question_hash=? AND answer_mode=? AND repo_revision=? AND expires_at>?", scope, answerHash(question), mode, revision, answerISO(now)).Scan(&raw)
	if err == sql.ErrNoRows {
		return ids, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var r AnswerRecord
	if err = json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, nil, err
	}
	return ids, &r, nil
}
func (s AnswerStore) PutHotChanged(ctx context.Context, scope string, ids []string, max int) error {
	return withAnswerTx(ctx, s.DB, func(tx *sql.Tx) error {
		now := answerISO(s.now())
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO hot_changed_concepts VALUES(?,?,?)", scope, id, now); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM hot_changed_concepts WHERE scope_key=? AND concept_id NOT IN (SELECT concept_id FROM hot_changed_concepts WHERE scope_key=? ORDER BY observed_at DESC LIMIT ?)", scope, scope, max)
		return err
	})
}
func (s AnswerStore) PutHot(ctx context.Context, scope, question, mode, revision string, r AnswerRecord, ids []string, ttl, max int) error {
	raw, _ := json.Marshal(r)
	sort.Strings(ids)
	rawIDs, _ := json.Marshal(ids)
	now := s.now()
	return withAnswerTx(ctx, s.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO hot_answers VALUES(?,?,?,?,?,?,?,?,?)", scope, answerHash(question), question, mode, revision, string(raw), string(rawIDs), answerISO(now), answerISO(now.Add(time.Duration(ttl)*time.Second))); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM hot_answers WHERE scope_key=? AND rowid NOT IN (SELECT rowid FROM hot_answers WHERE scope_key=? ORDER BY created_at DESC LIMIT ?)", scope, scope, max)
		return err
	})
}
func (s AnswerStore) InvalidateHot(ctx context.Context, changed map[string]bool) error {
	if len(changed) == 0 {
		return nil
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT scope_key,question_hash,answer_mode,repo_revision,concept_ids_json FROM hot_answers")
	if err != nil {
		return err
	}
	defer rows.Close()
	type key [4]string
	var doomed []key
	for rows.Next() {
		var k key
		var raw string
		if err = rows.Scan(&k[0], &k[1], &k[2], &k[3], &raw); err != nil {
			return err
		}
		var ids []string
		if err = json.Unmarshal([]byte(raw), &ids); err != nil {
			return err
		}
		for _, id := range ids {
			if changed[id] {
				doomed = append(doomed, k)
				break
			}
		}
	}
	return withAnswerTx(ctx, s.DB, func(tx *sql.Tx) error {
		for _, k := range doomed {
			if _, err := tx.ExecContext(ctx, "DELETE FROM hot_answers WHERE scope_key=? AND question_hash=? AND answer_mode=? AND repo_revision=?", k[0], k[1], k[2], k[3]); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s AnswerStore) InsertTrace(ctx context.Context, principal, scope, question, revision string, result DeepAnswerResult, max, age int) (string, error) {
	id := uuid.NewString()
	if result.Record.TraceID != nil {
		id = *result.Record.TraceID
	}
	steps, _ := json.Marshal(result.Steps)
	paths := []string{}
	for _, c := range result.ReadConcepts {
		paths = append(paths, c.Path)
	}
	pathJSON, _ := json.Marshal(paths)
	chain, _ := json.Marshal(result.Record.ModelChain)
	usage, _ := json.Marshal(result.Usage)
	now := s.now()
	err := withAnswerTx(ctx, s.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO answer_traces VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", id, principal, scope, answerHash(question), truncateRunes(question, 200), revision, truncateRunes(result.Record.Answer, 200), string(steps), string(pathJSON), string(chain), string(usage), result.DurationMS, answerISO(now)); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM answer_traces WHERE created_at<? OR trace_id NOT IN (SELECT trace_id FROM answer_traces ORDER BY created_at DESC LIMIT ?)", answerISO(now.Add(-time.Duration(age)*24*time.Hour)), max)
		return err
	})
	return id, err
}
func withAnswerTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
func truncateRunes(v string, n int) string {
	r := []rune(v)
	if len(r) > n {
		return string(r[:n])
	}
	return v
}
