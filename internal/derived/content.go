// Package derived builds disposable content/FTS/link indexes. Semantic workers,
// search/graph APIs and automatic corruption quarantine are separate port slices.
package derived

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/internal/repository"
)

const SchemaVersion = "2"

// LinkResolutionVersion changes only disposable derived data, not SQLite DDL.
const LinkResolutionVersion = "relative-assets-1"

type CorruptionError struct{ Message string }

func (e *CorruptionError) Error() string { return e.Message }

// ContentStore is the models-disabled index kernel. Callers own database life,
// writer serialisation and corruption recovery. It does not quarantine files.
type ContentStore struct {
	DB              *sql.DB
	DeferEmbeddings bool
	MaxInputChars   int  // zero selects the source default of 4096
	initialized     bool // lifecycle owner already migrated this file identity
}

func Connect(ctx context.Context, path string) (*sql.DB, error) {
	return connect(ctx, path, control.Connect)
}
func connect(ctx context.Context, path string, open func(context.Context, string) (*sql.DB, error)) (*sql.DB, error) {
	db, err := open(ctx, path)
	if err != nil {
		return nil, err
	}
	if _, err = db.ExecContext(ctx, "PRAGMA application_id=1296646996"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Migrate preserves source DDL-before-DML boundaries: creation of schema tables
// precedes the state transaction. Existing unsupported schema is never reset.
func (s ContentStore) Migrate(ctx context.Context) error {
	if s.initialized {
		return nil
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, statement := range migrations {
		if _, err = conn.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return stateTransaction(ctx, conn, func() error {
		version, err := getState(ctx, conn, "schema_version")
		if err != nil {
			return err
		}
		if version == nil {
			if err = setState(ctx, conn, "schema_version", SchemaVersion); err != nil {
				return err
			}
		} else if *version != SchemaVersion {
			return &CorruptionError{"unsupported derived schema version"}
		}
		for _, pair := range [][2]string{{"repo_revision", ""}, {"index_revision", ""}, {"status", "ready"}, {"semantic_embedding_revision", ""}} {
			value, err := getState(ctx, conn, pair[0])
			if err != nil {
				return err
			}
			if value == nil {
				if err = setState(ctx, conn, pair[0], pair[1]); err != nil {
					return err
				}
			}
		}
		classified, err := getState(ctx, conn, "external_links_classified")
		if err != nil {
			return err
		}
		if classified == nil || *classified != "1" {
			if err = classifyExternal(ctx, conn); err != nil {
				return err
			}
			if err = recomputeMetrics(ctx, conn); err != nil {
				return err
			}
			return setState(ctx, conn, "external_links_classified", "1")
		}
		return nil
	})
}
func stateTransaction(ctx context.Context, conn *sql.Conn, fn func() error) error {
	if _, err := conn.ExecContext(ctx, "BEGIN"); err != nil {
		return err
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
	if err := fn(); err != nil {
		return err
	}
	_, err := conn.ExecContext(ctx, "COMMIT")
	return err
}
func getState(ctx context.Context, db executor, key string) (*string, error) {
	var value string
	err := db.QueryRowContext(ctx, "SELECT value FROM index_state WHERE key=?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}
func setState(ctx context.Context, db executor, key, value string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO index_state(key,value,updated_at) VALUES(?,?,datetime('now')) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=datetime('now')`, key, value)
	return err
}
func (s ContentStore) Rebuild(ctx context.Context, root, revision string) error {
	if err := s.Migrate(ctx); err != nil {
		return err
	}
	bundle, err := repository.ScanBundle(root, repository.BundleFilter{})
	if err != nil {
		return err
	}
	return s.contentTransaction(ctx, func(conn *sql.Conn) error {
		for _, table := range []string{"concepts", "concept_fts", "links", "graph_metrics"} {
			if _, err := conn.ExecContext(ctx, "DELETE FROM "+table); err != nil {
				return err
			}
		}
		if err := setState(ctx, conn, "status", "rebuilding"); err != nil {
			return err
		}
		if err := setState(ctx, conn, "repo_revision", revision); err != nil {
			return err
		}
		for _, entry := range bundle.Entries {
			if err := upsertEntry(ctx, conn, entry, revision); err != nil {
				return err
			}
		}
		assetPaths, err := assetPathsForBundle(root, bundle)
		if err != nil {
			return err
		}
		if err := recomputeLinksWithAssets(ctx, conn, revision, assetPaths); err != nil {
			return err
		}
		if err := recomputeMetrics(ctx, conn); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, "DELETE FROM concept_embeddings WHERE concept_id NOT IN (SELECT id FROM concepts)"); err != nil {
			return err
		}
		if err := s.markEmbeddingStaleness(ctx, conn, bundle.Entries, revision); err != nil {
			return err
		}
		if !s.DeferEmbeddings {
			if err := setState(ctx, conn, "semantic_embedding_revision", "disabled"); err != nil {
				return err
			}
		}
		return finishContent(ctx, conn, revision)
	})
}
func (s ContentStore) UpdatePaths(ctx context.Context, root, revision string, paths []string) error {
	if err := s.Migrate(ctx); err != nil {
		return err
	}
	state, err := getState(ctx, s.DB, "status")
	if err != nil {
		return err
	}
	if state != nil && *state == "quarantined" {
		return &CorruptionError{"derived index is quarantined"}
	}
	ordered := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		if !seen[path] {
			ordered = append(ordered, path)
			seen[path] = true
		}
	}
	sort.Strings(ordered)
	return s.contentTransaction(ctx, func(conn *sql.Conn) error {
		if err := setState(ctx, conn, "repo_revision", revision); err != nil {
			return err
		}
		for _, path := range ordered {
			if err := deletePath(ctx, conn, path); err != nil {
				return err
			}
		}
		documents := 0
		for _, path := range ordered {
			if !strings.HasSuffix(path, ".md") || repository.IsReservedBundlePath(path) {
				continue
			}
			// Unlike Python's unconstrained direct parse, reject traversal/symlinks.
			// These paths come from Git; hostile replacement races remain a wider gate.
			if err := repository.ValidateBundlePath(path); err != nil {
				return err
			}
			absolute := filepath.Join(root, strings.TrimPrefix(path, "/"))
			info, err := os.Stat(absolute)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				continue
			}
			entry, err := repository.ReadBundleEntry(root, path)
			if err != nil {
				return err
			}
			documents++
			if err = upsertEntry(ctx, conn, entry, revision); err != nil {
				return err
			}
			if err = commitEntry(ctx, conn); err != nil {
				return err
			}
		}
		if _, err := conn.ExecContext(ctx, "UPDATE concepts SET repo_revision=?", revision); err != nil {
			return err
		}
		assetPaths, err := assetPathsForConceptRows(ctx, root, conn)
		if err != nil {
			return err
		}
		if err := recomputeLinksWithAssets(ctx, conn, revision, assetPaths); err != nil {
			return err
		}
		if err := recomputeMetrics(ctx, conn); err != nil {
			return err
		}
		if !s.DeferEmbeddings {
			if err := setState(ctx, conn, "semantic_embedding_revision", "disabled"); err != nil {
				return err
			}
		} else if documents == 0 {
			if _, err := conn.ExecContext(ctx, "UPDATE concept_embeddings SET embedding_revision=? WHERE status='ready'", revision); err != nil {
				return err
			}
			if err := setEmbeddingRevision(ctx, conn, revision); err != nil {
				return err
			}
		}
		return finishContent(ctx, conn, revision)
	})
}
func (s ContentStore) contentTransaction(ctx context.Context, fn func(*sql.Conn) error) error {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return stateTransaction(ctx, conn, func() error { return fn(conn) })
}
func commitEntry(ctx context.Context, conn executor) error {
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	_, err := conn.ExecContext(ctx, "BEGIN")
	return err
}
func finishContent(ctx context.Context, conn executor, revision string) error {
	if err := setState(ctx, conn, "link_resolution_version", LinkResolutionVersion); err != nil {
		return err
	}
	if err := setState(ctx, conn, "index_revision", revision); err != nil {
		return err
	}
	return setState(ctx, conn, "status", "ready")
}
func upsertEntry(ctx context.Context, db executor, entry repository.BundleEntry, revision string) error {
	m := entry.Document.Frontmatter
	tags, err := pyjson.Dumps(m.Tags)
	if err != nil {
		return err
	}
	aliases, err := pyjson.Dumps(m.Aliases)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO concepts(id,path,type,title,description,status,tags_json,aliases_json,body,content_hash,updated_at,repo_revision) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET path=excluded.path,type=excluded.type,title=excluded.title,description=excluded.description,status=excluded.status,tags_json=excluded.tags_json,aliases_json=excluded.aliases_json,body=excluded.body,content_hash=excluded.content_hash,updated_at=excluded.updated_at,repo_revision=excluded.repo_revision`, m.ID, entry.BundlePath, m.Type, m.Title, m.Description, m.Status, tags, aliases, entry.Document.Body, fmt.Sprintf("%x", sha256.Sum256([]byte(entry.Document.Body))), isoTime(m.UpdatedAt), revision)
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, "DELETE FROM concept_fts WHERE concept_id=?", m.ID); err != nil {
		return err
	}
	description := ""
	if m.Description != nil {
		description = *m.Description
	}
	_, err = db.ExecContext(ctx, "INSERT INTO concept_fts(concept_id,title,description,aliases,tags,body,path) VALUES(?,?,?,?,?,?,?)", m.ID, m.Title, description, strings.Join(m.Aliases, " "), strings.Join(m.Tags, " "), entry.Document.Body, entry.BundlePath)
	return err
}
func isoTime(t time.Time) string {
	t = t.UTC()
	text := t.Format("2006-01-02T15:04:05")
	if t.Nanosecond()/1000 != 0 {
		text += t.Format(".000000")
	}
	return text + "Z"
}
func deletePath(ctx context.Context, db executor, path string) error {
	var id string
	err := db.QueryRowContext(ctx, "SELECT id FROM concepts WHERE path=?", path).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, query := range []string{"DELETE FROM concept_fts WHERE concept_id=?", "DELETE FROM links WHERE source_id=? OR target_id=?", "DELETE FROM graph_metrics WHERE concept_id=?", "DELETE FROM concept_embeddings WHERE concept_id=?", "DELETE FROM concepts WHERE id=?"} {
		args := []any{id}
		if strings.Contains(query, "OR") {
			args = append(args, id)
		}
		if _, err = db.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}
func (s ContentStore) markEmbeddingStaleness(ctx context.Context, db executor, entries []repository.BundleEntry, revision string) error {
	limit := s.MaxInputChars
	if limit == 0 {
		limit = 4096
	}
	for _, entry := range entries {
		m := entry.Document.Frontmatter
		parts := []string{m.Title}
		if m.Description != nil && *m.Description != "" {
			parts = append(parts, *m.Description)
		}
		parts = append(parts, entry.Document.Body)
		text := []string{}
		for _, part := range parts {
			part = strings.TrimFunc(part, func(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f })
			if part != "" {
				text = append(text, part)
			}
		}
		runes := []rune(strings.Join(text, "\n\n"))
		n := limit
		if n < 0 {
			n = max(0, len(runes)+n)
		}
		digest := sha256.Sum256([]byte(string(runes[:min(n, len(runes))])))
		if _, err := db.ExecContext(ctx, `UPDATE concept_embeddings SET path=?,embedding_revision=?,status=CASE WHEN embedding_text_hash=? THEN status ELSE 'stale' END WHERE concept_id=?`, entry.BundlePath, revision, fmt.Sprintf("%x", digest), m.ID); err != nil {
			return err
		}
	}
	return setEmbeddingRevision(ctx, db, revision)
}
func setEmbeddingRevision(ctx context.Context, db executor, revision string) error {
	var concepts, ready int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM concepts").Scan(&concepts); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM concept_embeddings WHERE status='ready'").Scan(&ready); err != nil {
		return err
	}
	state := ""
	if ready == concepts {
		state = revision
	} else if ready > 0 {
		state = "partial"
	}
	return setState(ctx, db, "semantic_embedding_revision", state)
}
