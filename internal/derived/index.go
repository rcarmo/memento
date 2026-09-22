package derived

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcarmo/memento/internal/access"
	sqlite "modernc.org/sqlite"
)

type UnavailableError struct{ Message string }

func (e *UnavailableError) Error() string { return e.Message }

type InvalidHeaderError struct{}

func (*InvalidHeaderError) Error() string { return "invalid sqlite header" }

// Index owns models-off connections, identity-based initialisation and recovery.
// One object serialises its operations; cross-process writes require the daemon
// writer lease. Strict waiting releases this lock between state reads.
type Index struct {
	Path            string
	DeferEmbeddings bool
	MaxInputChars   int
	chunkModel      SemanticModelInfo
	Now             func() time.Time
	mu              sync.Mutex
	identity        fs.FileInfo
}
type indexIO struct {
	stat     func(string) (fs.FileInfo, error)
	openFile func(string) (io.ReadCloser, error)
	openDB   func(context.Context, string) (*sql.DB, error)
	rename   func(string, string) error
	remove   func(string) error
}

func defaultIndexIO() indexIO {
	return indexIO{os.Stat, func(path string) (io.ReadCloser, error) { return os.Open(path) }, openIndexDB, os.Rename, os.Remove}
}
func openIndexDB(ctx context.Context, path string) (*sql.DB, error) {
	return openIndexDBWith(ctx, path, sql.Open)
}
func openIndexDBWith(ctx context.Context, path string, open func(string, string) (*sql.DB, error)) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	query := url.Values{"_pragma": {"foreign_keys(1)", "busy_timeout(5000)"}}
	u.RawQuery = query.Encode()
	db, err := open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func (i *Index) withCore(ctx context.Context, write bool, fn func(ContentStore) error) error {
	if err := lockIndex(ctx, &i.mu); err != nil {
		return err
	}
	defer i.mu.Unlock()
	return i.withCoreIO(ctx, write, fn, defaultIndexIO())
}
func (i *Index) withCoreIO(ctx context.Context, write bool, fn func(ContentStore) error, ops indexIO) error {
	// Source header validation is outside _with_quarantine's catch block.
	if write {
		if err := validateIndexHeader(i.Path, ops); err != nil {
			return err
		}
	}
	store, err := i.connect(ctx, ops)
	if err == nil {
		err = fn(store)
		store.DB.Close()
	}
	if err == nil {
		return nil
	}
	var database *sqlite.Error
	var corruption *CorruptionError
	if errors.As(err, &database) {
		if transientDatabaseError(err) {
			return &UnavailableError{err.Error()}
		}
		return i.quarantineAfter(ctx, err, ops)
	}
	if write && errors.As(err, &corruption) {
		return i.quarantineAfter(ctx, err, ops)
	}
	return err
}
func (i *Index) connect(ctx context.Context, ops indexIO) (ContentStore, error) {
	empty := ContentStore{}
	db, err := ops.openDB(ctx, i.Path)
	if err != nil {
		return empty, err
	}
	store := ContentStore{DB: db, DeferEmbeddings: i.DeferEmbeddings, MaxInputChars: i.MaxInputChars}
	identity, err := ops.stat(i.Path)
	if err != nil {
		db.Close()
		return empty, err
	}
	if i.identity == nil || !os.SameFile(i.identity, identity) {
		for _, query := range []string{"PRAGMA application_id=1296646996", "PRAGMA journal_mode=WAL"} {
			if _, err = db.ExecContext(ctx, query); err != nil {
				db.Close()
				return empty, err
			}
		}
		if err = store.Migrate(ctx); err != nil {
			db.Close()
			return empty, err
		}
		i.identity, err = ops.stat(i.Path)
		if err != nil {
			db.Close()
			return empty, err
		}
	}
	store.initialized = true
	return store, nil
}
func validateIndexHeader(path string, ops indexIO) error {
	info, err := ops.stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return nil
	}
	file, err := ops.openFile(path)
	if err != nil {
		return err
	}
	var header [16]byte
	_, err = io.ReadFull(file, header[:])
	closeErr := file.Close()
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	if string(header[:]) != "SQLite format 3\x00" {
		return &InvalidHeaderError{}
	}
	return closeErr
}
func transientDatabaseError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database is busy")
}
func (i *Index) quarantineAfter(ctx context.Context, cause error, ops indexIO) error {
	path, err := i.quarantine(ops)
	if err != nil {
		return err
	}
	store, err := i.connect(ctx, ops)
	if err != nil {
		return err
	}
	defer store.DB.Close()
	err = store.contentTransaction(ctx, func(conn *sql.Conn) error {
		if err := setState(ctx, conn, "status", "quarantined"); err != nil {
			return err
		}
		return setState(ctx, conn, "quarantine_path", path)
	})
	if err != nil {
		return err
	}
	return &CorruptionError{cause.Error()}
}
func (i *Index) quarantine(ops indexIO) (string, error) {
	i.identity = nil
	if _, err := ops.stat(i.Path); errors.Is(err, os.ErrNotExist) {
		return strings.TrimSuffix(i.Path, filepath.Ext(i.Path)) + ".missing", nil
	} else if err != nil {
		return "", err
	}
	now := time.Now()
	if i.Now != nil {
		now = i.Now()
	}
	ext := filepath.Ext(i.Path)
	target := strings.TrimSuffix(i.Path, ext) + fmt.Sprintf(".quarantine-%d", now.UnixMilli()) + ext
	// Refuse collisions rather than overwrite an earlier recovery file. Keep
	// same-directory rename; source's cross-filesystem shutil fallback is unused.
	if _, err := ops.stat(target); err == nil {
		return "", errors.New("derived quarantine target already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := ops.rename(i.Path, target); err != nil {
		return "", err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		path := i.Path + suffix
		if _, err := ops.stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return "", err
		}
		if err := ops.remove(path); err != nil {
			return "", err
		}
	}
	return target, nil
}
func (i *Index) Rebuild(ctx context.Context, root, revision string) error {
	return i.withCore(ctx, true, func(s ContentStore) error { return s.Rebuild(ctx, root, revision) })
}
func (i *Index) UpdatePaths(ctx context.Context, root, revision string, paths []string) error {
	return i.withCore(ctx, true, func(s ContentStore) error { return s.UpdatePaths(ctx, root, revision, paths) })
}
func (i *Index) State(ctx context.Context) (IndexState, error) {
	var state IndexState
	err := i.withCore(ctx, false, func(s ContentStore) error { var err error; state, err = s.State(ctx); return err })
	return state, err
}
func (i *Index) Status(ctx context.Context, policy access.EffectivePolicy) (StatusSnapshot, error) {
	var result StatusSnapshot
	err := i.withCore(ctx, false, func(s ContentStore) error { var err error; result, err = s.Status(ctx, policy); return err })
	return result, err
}
func (i *Index) PendingEmbeddingPaths(ctx context.Context, limit int) ([]string, error) {
	return i.pendingEmbeddingPaths(ctx, limit, i.pendingChunkRows)
}
func (i *Index) pendingEmbeddingPaths(ctx context.Context, limit int, query func(context.Context, *sql.DB, int) (embeddingPathRows, error)) (paths []string, err error) {
	if limit < 1 {
		return nil, errors.New("limit must be positive")
	}
	err = i.withCore(ctx, false, func(s ContentStore) error {
		rows, queryErr := query(ctx, s.DB, limit)
		if queryErr != nil {
			return queryErr
		}
		paths, queryErr = readEmbeddingPaths(rows)
		return queryErr
	})
	return paths, err
}

type embeddingPathRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

func readEmbeddingPaths(rows embeddingPathRows) ([]string, error) {
	defer rows.Close()
	paths := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}
func (i *Index) SetRepoRevision(ctx context.Context, revision string) error {
	return i.withCore(ctx, false, func(s ContentStore) error { return s.SetRepoRevision(ctx, revision) })
}
func (i *Index) WaitForFreshness(ctx context.Context, timeout time.Duration) (IndexState, error) {
	return waitForState(ctx, timeout, time.Now, sleepFreshness, i.State)
}
func (i *Index) SearchLexical(ctx context.Context, policy access.EffectivePolicy, options SearchOptions) (SearchPage, error) {
	if options.Strict {
		if _, err := i.WaitForFreshness(ctx, options.Timeout); err != nil {
			return SearchPage{}, err
		}
		options.Strict = false
	}
	var page SearchPage
	err := i.withCore(ctx, false, func(s ContentStore) error {
		var err error
		page, err = s.SearchLexical(ctx, policy, options)
		return err
	})
	return page, err
}
func (i *Index) Graph(ctx context.Context, policy access.EffectivePolicy, id string, options GraphOptions) (GraphNeighborhood, error) {
	if options.Strict {
		if _, err := i.WaitForFreshness(ctx, options.Timeout); err != nil {
			return GraphNeighborhood{}, err
		}
		options.Strict = false
	}
	var graph GraphNeighborhood
	err := i.withCore(ctx, false, func(s ContentStore) error { var err error; graph, err = s.Graph(ctx, policy, id, options); return err })
	return graph, err
}
func (i *Index) Metrics(ctx context.Context, id string) (GraphMetrics, error) {
	var metrics GraphMetrics
	err := i.withCore(ctx, false, func(s ContentStore) error { var err error; metrics, err = s.Metrics(ctx, id); return err })
	return metrics, err
}
