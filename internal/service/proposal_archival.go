package service

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

// ArchivalImpact reads an existing derived index without creating or migrating
// it. Index construction remains a separate subsystem. The caller must hold the
// repository transaction lock so the current checkout matches main throughout.
// Changes must come from NormalizeProposalChanges, as for other policy helpers.
func (q ProposalQueue) ArchivalImpact(ctx context.Context, policy access.EffectivePolicy, changes []ProposalChange, revision, indexPath string) ([]any, error) {
	return q.archivalImpact(ctx, policy, changes, revision, indexPath, defaultProposalRepository(), openArchivalIndex)
}
func openArchivalIndex(path string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path}
	query := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(5000)"}}
	u.RawQuery = query.Encode()
	// control registers the pure-Go SQLite driver. Do not use control.Connect:
	// it enables WAL and accepts file creation, inappropriate for a derived read.
	return sql.Open("sqlite", u.String())
}
func (q ProposalQueue) archivalImpact(ctx context.Context, policy access.EffectivePolicy, changes []ProposalChange, revision, indexPath string, repo proposalRepository, open func(string) (*sql.DB, error)) ([]any, error) {
	archival := []ProposalChange{}
	for _, change := range changes {
		if change["kind"] == "trash" {
			archival = append(archival, change)
		}
	}
	if len(archival) == 0 {
		return []any{}, nil
	}
	if err := validateArchivalBatch(changes); err != nil {
		return nil, err
	}
	current, err := repo.main(q.Paths)
	if err != nil {
		return nil, err
	}
	if revision != current {
		return nil, &Error{"conflict", "archival impact requires the current repository revision"}
	}
	db, err := open(indexPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err = checkArchivalIndex(ctx, db, revision); err != nil {
		return nil, err
	}
	reports := []any{}
	for _, change := range archival {
		path := change["path"].(string)
		if _, err = access.AuthorizePath(policy, path, "read"); err != nil {
			return nil, err
		}
		if _, err = access.AuthorizePath(policy, path, "write"); err != nil {
			return nil, err
		}
		if strings.HasPrefix(path, "/trash/") || !strings.HasSuffix(path, ".md") {
			return nil, &repository.PathSafetyError{Message: "expected an active Markdown concept path"}
		}
		destination := "/trash" + path
		entry, err := repo.read(q.Paths.CurrentDir, path)
		if err != nil {
			return nil, err
		}
		target, err := repository.ValidateRepositoryWritePath(q.Paths.CurrentDir, destination)
		if err != nil {
			return nil, err
		}
		if err = checkArchivalTarget(target.AbsolutePath, os.Stat); err != nil {
			return nil, err
		}
		refs, err := archivalReferences(ctx, db, policy, path)
		if err != nil {
			return nil, err
		}
		accepted, err := archivalAssets(q.Paths.CurrentDir, entry.Document.Frontmatter.ID)
		if err != nil {
			return nil, err
		}
		reports = append(reports, map[string]any{"path": path, "destination": destination, "concept_id": entry.Document.Frontmatter.ID, "revision": revision, "inbound_references": refs, "reference_scope": "currently readable sources only; other namespaces may also refer to this item", "assets": accepted, "assets_action": "retained for restore; explicit purge removes current accepted versions", "links_action": "not rewritten; inbound links become unresolved until restore", "history_retained": true, "conflicts": []any{}})
	}
	current, err = repo.main(q.Paths)
	if err != nil {
		return nil, err
	}
	if revision != current {
		return nil, &Error{"conflict", "repository changed during archival impact inspection"}
	}
	return reports, nil
}
func checkArchivalTarget(path string, stat func(string) (os.FileInfo, error)) error {
	_, err := stat(path)
	if err == nil {
		return &Error{"conflict", "trash destination already exists"}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
func checkArchivalIndex(ctx context.Context, db *sql.DB, revision string) error {
	rows, err := db.QueryContext(ctx, "SELECT key,value FROM index_state")
	if err != nil {
		return err
	}
	return checkArchivalStateRows(rows, revision)
}
func checkArchivalStateRows(rows proposalIDRows, revision string) error {
	defer rows.Close()
	var err error
	state := map[string]string{}
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			return err
		}
		state[key] = value
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if state["index_revision"] != revision || state["status"] != "ready" {
		return &Error{"conflict", "archival impact requires a fresh content index"}
	}
	return nil
}
func archivalReferences(ctx context.Context, db *sql.DB, policy access.EffectivePolicy, path string) ([]any, error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT c.path,l.raw_target,l.resolution_state FROM links l JOIN concepts c ON c.id=l.source_id WHERE l.target_path=? AND c.path NOT LIKE '/trash/%' ORDER BY c.path,l.raw_target LIMIT 101`, path)
	if err != nil {
		return nil, err
	}
	return archivalReferenceRows(rows, policy)
}
func archivalReferenceRows(rows proposalIDRows, policy access.EffectivePolicy) ([]any, error) {
	defer rows.Close()
	var err error
	count := 0
	refs := []any{}
	for rows.Next() {
		var source, target, resolution string
		if err = rows.Scan(&source, &target, &resolution); err != nil {
			return nil, err
		}
		count++
		if _, err = access.AuthorizePath(policy, source, "read"); err == nil {
			refs = append(refs, map[string]any{"path": source, "target": target, "resolution": resolution, "after_archival": "unresolved"})
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if count > 100 {
		return nil, &Error{"validation_error", "archival impact exceeds 100 inbound references; curate references first"}
	}
	return refs, nil
}
func archivalAssets(root, id string) ([]any, error) {
	kinds, err := assets.ListAssetKinds(root, id)
	if err != nil {
		return nil, err
	}
	accepted := []any{}
	for _, kind := range kinds {
		versions, err := assets.ListAssetVersions(root, id, kind)
		if err != nil {
			return nil, err
		}
		for _, version := range versions {
			if len(accepted) >= 100 {
				return nil, &Error{"validation_error", "archival impact exceeds 100 accepted asset versions"}
			}
			metadata, err := assets.LoadAssetMetadata(root, id, kind, version)
			if err != nil {
				return nil, err
			}
			accepted = append(accepted, map[string]any{"kind": kind, "version": version, "sha256": metadata["zip_sha256"], "action": "retained"})
		}
	}
	return accepted, nil
}

// VisibleArchivalImpact re-scopes stored reports, or recomputes fresh reviewable
// reports so a curator can see references hidden from the original author.
func (q ProposalQueue) VisibleArchivalImpact(ctx context.Context, record control.ProposalRecord, policy access.EffectivePolicy, indexPath string) ([]any, error) {
	return q.visibleArchivalImpact(ctx, record, policy, indexPath, defaultProposalRepository())
}
func (q ProposalQueue) visibleArchivalImpact(ctx context.Context, record control.ProposalRecord, policy access.EffectivePolicy, indexPath string, repo proposalRepository) ([]any, error) {
	patch, err := record.Patch()
	if err != nil {
		return nil, err
	}
	reports := []any{}
	if value, exists := patch["archival_impact"]; exists {
		var ok bool
		reports, ok = value.([]any)
		if !ok {
			return nil, &ChangeValidationError{"archival impact must be a list"}
		}
	}
	if len(reports) > 0 && (record.Status == control.Submitted || record.Status == control.Approved) {
		revision, err := repo.main(q.Paths)
		if err != nil {
			return nil, err
		}
		if record.BaseRevision == revision {
			changes, err := proposalChanges(record)
			if err != nil {
				return nil, err
			}
			return q.archivalImpact(ctx, policy, changes, record.BaseRevision, indexPath, repo, openArchivalIndex)
		}
	}
	visible := []any{}
	for _, value := range reports {
		report, ok := value.(map[string]any)
		if !ok {
			return nil, &ChangeValidationError{"archival report must be an object"}
		}
		path, ok := report["path"].(string)
		if !ok {
			return nil, &ChangeValidationError{"archival path must be a string"}
		}
		if _, err = access.AuthorizePath(policy, path, "read"); err != nil {
			continue
		}
		rows, ok := report["inbound_references"].([]any)
		if !ok {
			return nil, &ChangeValidationError{"inbound references must be a list"}
		}
		refs := []any{}
		for _, value := range rows {
			row, ok := value.(map[string]any)
			if !ok {
				return nil, &ChangeValidationError{"inbound reference must be an object"}
			}
			source, ok := row["path"].(string)
			if !ok {
				return nil, &ChangeValidationError{"inbound path must be a string"}
			}
			if _, err = access.AuthorizePath(policy, source, "read"); err == nil {
				refs = append(refs, row)
			}
		}
		scoped := map[string]any{}
		for key, value := range report {
			scoped[key] = value
		}
		scoped["inbound_references"] = refs
		visible = append(visible, scoped)
	}
	return visible, nil
}
