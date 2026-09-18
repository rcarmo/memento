package derived

import (
	"context"
	"database/sql"
	"strings"

	"github.com/rcarmo/memento/go/repository"
)

type contentRow struct{ ID, Path, Body string }
type rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}

func contentRows(rows rows) ([]contentRow, error) {
	defer rows.Close()
	out := []contentRow{}
	for rows.Next() {
		var r contentRow
		if err := rows.Scan(&r.ID, &r.Path, &r.Body); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func recomputeLinks(ctx context.Context, db executor, revision string) error {
	if _, err := db.ExecContext(ctx, "DELETE FROM links"); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, "SELECT id,path,body FROM concepts ORDER BY path")
	if err != nil {
		return err
	}
	concepts, err := contentRows(rows)
	if err != nil {
		return err
	}
	pathToID := map[string]string{}
	for _, c := range concepts {
		pathToID[c.Path] = c.ID
	}
	for _, c := range concepts {
		for _, link := range repository.ExtractStructuralLinks(c.Body) {
			path, fragment, has := strings.Cut(link.Href, "#")
			var anchor any
			if has && fragment != "" {
				anchor = fragment
			}
			var target any
			if strings.HasPrefix(path, "/") {
				if id, ok := pathToID[path]; ok {
					target = id
				}
			}
			external := repository.IsExternalLink(link.Href)
			resolution := "broken"
			if external {
				resolution = "external"
			} else if target != nil {
				resolution = "resolved"
			}
			var targetPath any
			if strings.HasPrefix(path, "/") && !external {
				targetPath = path
			}
			if _, err = db.ExecContext(ctx, `INSERT INTO links(source_id,target_id,raw_target,target_path,anchor,link_kind,resolution_state,first_seen_revision,last_checked_revision) VALUES(?,?,?,?,?,'markdown',?,?,?)`, c.ID, target, link.Href, targetPath, anchor, resolution, revision, revision); err != nil {
				return err
			}
		}
	}
	return nil
}
func recomputeMetrics(ctx context.Context, db executor) error {
	if _, err := db.ExecContext(ctx, "DELETE FROM graph_metrics"); err != nil {
		return err
	}
	// Equivalent to the source's ordered per-ID counts, including duplicate links,
	// broken absolute outbound targets, relative broken links and self references.
	_, err := db.ExecContext(ctx, `INSERT INTO graph_metrics(concept_id,inbound_degree,outbound_degree,broken_link_count,orphan_flag)
 SELECT id,(SELECT COUNT(*) FROM links WHERE target_id=c.id),
 (SELECT COUNT(*) FROM links WHERE source_id=c.id AND target_path IS NOT NULL),
 (SELECT COUNT(*) FROM links WHERE source_id=c.id AND resolution_state='broken'),
 CASE WHEN NOT EXISTS(SELECT 1 FROM links WHERE target_id=c.id) AND NOT EXISTS(SELECT 1 FROM links WHERE source_id=c.id AND target_path IS NOT NULL) THEN 1 ELSE 0 END
 FROM concepts c ORDER BY id`)
	return err
}
func classifyExternal(ctx context.Context, db executor) error {
	rows, err := db.QueryContext(ctx, "SELECT rowid,raw_target FROM links")
	if err != nil {
		return err
	}
	ids, err := externalRows(rows)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = db.ExecContext(ctx, "UPDATE links SET resolution_state='external',target_path=NULL,target_id=NULL WHERE rowid=?", id); err != nil {
			return err
		}
	}
	return nil
}
func externalRows(rows rows) ([]int64, error) {
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		var target sql.NullString
		if err := rows.Scan(&id, &target); err != nil {
			return nil, err
		}
		if repository.IsExternalLink(target.String) {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}
