package graphdebug

import (
	"context"
	"database/sql"
	"net/url"
	"sort"

	_ "modernc.org/sqlite"
)

type Revisions struct {
	Repository string  `json:"repository"`
	Index      string  `json:"index"`
	Embedding  *string `json:"embedding"`
	Stale      bool    `json:"stale"`
}

var openSQL = sql.Open

type SnapshotError struct{ Message string }

func (e *SnapshotError) Error() string { return e.Message }

type SnapshotService struct {
	DerivedDBPath, ControlDBPath, RepositoryRoot string
	open                                         func(context.Context, string) (*sql.DB, error)
}

func NewSnapshotService(repositoryRoot, derivedDBPath, controlDBPath string) *SnapshotService {
	return &SnapshotService{RepositoryRoot: repositoryRoot, DerivedDBPath: derivedDBPath, ControlDBPath: controlDBPath, open: openReadDB}
}
func openReadDB(ctx context.Context, path string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(5000)"}}
	u.RawQuery = q.Encode()
	db, err := openSQL("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func (s *SnapshotService) Revisions(ctx context.Context) (Revisions, error) {
	db, err := s.open(ctx, s.DerivedDBPath)
	if err != nil {
		return Revisions{}, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT key,value FROM index_state WHERE key IN ('repo_revision','index_revision','semantic_embedding_revision')")
	if err != nil {
		return Revisions{}, err
	}
	defer rows.Close()
	state := map[string]string{}
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			return Revisions{}, err
		}
		state[key] = value
	}
	if err = rows.Err(); err != nil {
		return Revisions{}, err
	}
	r := Revisions{Repository: state["repo_revision"], Index: state["index_revision"]}
	if value := state["semantic_embedding_revision"]; value != "" {
		r.Embedding = &value
	}
	r.Stale = r.Repository != r.Index
	return r, nil
}
func (s *SnapshotService) PathsForIDs(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return []string{}, nil
	}
	db, err := s.open(ctx, s.DerivedDBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	unique := map[string]bool{}
	for _, id := range ids {
		unique[id] = true
	}
	ordered := make([]string, 0, len(unique))
	for id := range unique {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	query := "SELECT id,path FROM concepts WHERE id IN (?"
	args := make([]any, len(ordered))
	for i, id := range ordered {
		args[i] = id
		if i > 0 {
			query += ",?"
		}
	}
	query += ") ORDER BY id"
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]string{}
	for rows.Next() {
		var id, path string
		if err = rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		byID[id] = path
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result := []string{}
	for _, id := range ids {
		if path, ok := byID[id]; ok {
			result = append(result, path)
		}
	}
	return result, nil
}
