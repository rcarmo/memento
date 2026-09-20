package derived

import (
	"context"
	"database/sql"
	"strings"

	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/repository"
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
func assetPathsForBundle(root string, bundle repository.RepositoryBundle) (map[string]map[string]bool, error) {
	result := map[string]map[string]bool{}
	for _, entry := range bundle.Entries {
		paths, err := assets.AcceptedAssetPaths(root, entry.Document.Frontmatter.ID)
		if err != nil {
			return nil, err
		}
		result[entry.Document.Frontmatter.ID] = paths
	}
	return result, nil
}
func assetPathsForConceptRows(ctx context.Context, root string, db executor) (map[string]map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT id FROM concepts ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		paths, err := assets.AcceptedAssetPaths(root, id)
		if err != nil {
			return nil, err
		}
		result[id] = paths
	}
	return result, rows.Err()
}

func recomputeLinks(ctx context.Context, db executor, revision string) error {
	return recomputeLinksWithAssets(ctx, db, revision, nil)
}
func recomputeLinksWithAssets(ctx context.Context, db executor, revision string, assetPaths map[string]map[string]bool) error {
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
			_, fragment, has := strings.Cut(link.Href, "#")
			var anchor any
			if has && fragment != "" {
				anchor = fragment
			}
			external := repository.IsExternalLink(link.Href)
			var targetPath any
			var target any
			if resolvedPath, internal := repository.ResolveLinkPath(c.Path, link.Href); internal {
				targetPath = resolvedPath
				if id, ok := pathToID[resolvedPath]; ok {
					target = id
				}
			}
			resolution := "broken"
			if external {
				resolution = "external"
			} else if target != nil {
				resolution = "resolved"
			} else if assetPath, candidate := repository.ResolveAssetLinkPath(link.Href); candidate && assetPaths[c.ID][assetPath] {
				resolution = "asset"
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
	// Preserve duplicate/self/broken concept-link degree semantics. Attached
	// resources are valid references but do not create graph connectivity.
	_, err := db.ExecContext(ctx, `INSERT INTO graph_metrics(concept_id,inbound_degree,outbound_degree,broken_link_count,orphan_flag)
 SELECT id,(SELECT COUNT(*) FROM links WHERE target_id=c.id),
 (SELECT COUNT(*) FROM links WHERE source_id=c.id AND target_path IS NOT NULL AND resolution_state != 'asset'),
 (SELECT COUNT(*) FROM links WHERE source_id=c.id AND resolution_state='broken'),
 CASE WHEN NOT EXISTS(SELECT 1 FROM links WHERE target_id=c.id) AND NOT EXISTS(SELECT 1 FROM links WHERE source_id=c.id AND target_path IS NOT NULL AND resolution_state != 'asset') THEN 1 ELSE 0 END
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
