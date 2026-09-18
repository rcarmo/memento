// Package service incrementally composes Memento policy with durable stores.
// The daemon and full endpoint surface are not implemented yet.
package service

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/internal/pyjson"
	"github.com/rcarmo/memento/go/repository"
)

type ProposalConflict struct {
	Index            int      `json:"index"`
	Status           string   `json:"status"`
	ConflictingPaths []string `json:"conflicting_paths"`
	Reason           string   `json:"reason,omitempty"`
}
type RevisionDiffs map[string]map[string]bool

const unavailableBase = "\x00base_revision_unavailable"

// ProposalQueue refreshes status/history only. It never approves, rebases,
// applies or authorises a proposal. The enclosing service must serialise it
// with review/apply and provide an authorised view to external callers.
type ProposalQueue struct {
	Proposals control.Proposals
	Paths     repository.GitRepositoryPaths
	Now       func() time.Time
}
type proposalRepository struct {
	main func(repository.GitRepositoryPaths) (string, error)
	diff func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error)
	read func(string, string) (repository.BundleEntry, error)
}

func defaultProposalRepository() proposalRepository {
	return proposalRepository{repository.GetMainRevision, repository.DiffMainPaths, repository.ReadBundleEntry}
}
func (q ProposalQueue) now() string {
	now := time.Now()
	if q.Now != nil {
		now = q.Now()
	}
	now = now.UTC()
	text := now.Format("2006-01-02T15:04:05")
	if now.Nanosecond()/1000 != 0 {
		text += now.Format(".000000")
	}
	return text + "Z"
}
func (q ProposalQueue) Conflicts(ctx context.Context, record control.ProposalRecord, revision string, limit *int, diffs RevisionDiffs) ([]ProposalConflict, error) {
	return q.conflicts(ctx, record, revision, limit, diffs, defaultProposalRepository())
}
func (q ProposalQueue) conflicts(ctx context.Context, record control.ProposalRecord, revision string, limit *int, diffs RevisionDiffs, repo proposalRepository) ([]ProposalConflict, error) {
	var err error
	if revision == "" {
		revision, err = repo.main(q.Paths)
		if err != nil {
			return nil, err
		}
	}
	changed := diffs[record.BaseRevision]
	if changed == nil {
		changed = map[string]bool{}
		if record.BaseRevision != revision {
			paths, diffErr := repo.diff(ctx, q.Paths, record.BaseRevision, revision)
			if diffErr != nil {
				if errors.Is(diffErr, context.Canceled) || errors.Is(diffErr, context.DeadlineExceeded) {
					return nil, diffErr
				}
				// Python wraps Git command failures as GitError; pure-Go object/ref errors
				// have several concrete types. Confirm live main then fail the old base closed.
				if _, err = repo.main(q.Paths); err != nil {
					return nil, err
				}
				changed[unavailableBase] = true
			} else {
				for _, path := range paths {
					changed[path] = true
				}
			}
		}
		if diffs != nil {
			diffs[record.BaseRevision] = changed
		}
	}
	patch, err := record.Patch()
	if err != nil {
		return nil, err
	}
	changes, ok := patch["changes"].([]any)
	if !ok {
		if _, exists := patch["changes"]; exists {
			return nil, errors.New("proposal changes must be a list")
		}
		changes = []any{}
	}
	if limit != nil {
		n := *limit
		if n < 0 {
			n = max(0, len(changes)+n)
		}
		changes = changes[:min(n, len(changes))]
	}
	result := []ProposalConflict{}
	for i, value := range changes {
		change, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("proposal change must be an object")
		}
		path, err := proposalPathValue(change, "path")
		if err != nil {
			return nil, err
		}
		paths := []string{path}
		kind, _ := change["kind"].(string)
		if kind == "rename" {
			next, err := proposalPathValue(change, "new_path")
			if err != nil {
				return nil, err
			}
			paths = append(paths, next)
		}
		if kind == "trash" {
			paths = append(paths, "/trash"+path)
		}
		if kind == "attach_asset_pack" {
			entry, readErr := repo.read(q.Paths.CurrentDir, path)
			if readErr == nil {
				prefix := "/.assets/" + entry.Document.Frontmatter.ID + "/"
				for changedPath := range changed {
					if strings.HasPrefix(changedPath, prefix) {
						copy := map[string]bool{}
						for k, v := range changed {
							copy[k] = v
						}
						copy[path] = true
						changed = copy
						break
					}
				}
			} else {
				var bundle *repository.BundleError
				var frontmatter *repository.FrontmatterError
				var safety *repository.PathSafetyError
				if !errors.Is(readErr, fs.ErrNotExist) && !errors.As(readErr, &bundle) && !errors.As(readErr, &frontmatter) && !errors.As(readErr, &safety) {
					return nil, readErr
				}
			}
		}
		conflicts := map[string]bool{}
		for _, path := range paths {
			if changed[unavailableBase] || changed[path] {
				conflicts[path] = true
			}
		}
		conflicting := []string{}
		for path := range conflicts {
			conflicting = append(conflicting, path)
		}
		sort.Strings(conflicting)
		item := ProposalConflict{Index: i, Status: "clean", ConflictingPaths: conflicting}
		if len(conflicting) > 0 {
			item.Status = "conflict"
		}
		if changed[unavailableBase] {
			item.Reason = "base_revision_unavailable; inspect current content and file a fresh proposal"
		}
		result = append(result, item)
	}
	return result, nil
}
func proposalPathValue(change map[string]any, key string) (string, error) {
	value, exists := change[key]
	if !exists {
		return "", nil
	}
	path, ok := value.(string)
	if !ok {
		return "", errors.New("non-string proposal path is not supported")
	}
	return path, nil
}
func (q ProposalQueue) Refresh(ctx context.Context, record control.ProposalRecord, revision string, diffs RevisionDiffs) (control.ProposalRecord, error) {
	return q.refresh(ctx, record, revision, diffs, defaultProposalRepository())
}
func (q ProposalQueue) refresh(ctx context.Context, record control.ProposalRecord, revision string, diffs RevisionDiffs, repo proposalRepository) (control.ProposalRecord, error) {
	var err error
	if revision == "" {
		revision, err = repo.main(q.Paths)
		if err != nil {
			return control.ProposalRecord{}, err
		}
	}
	now := q.now()
	status := record.Status
	action := "repository_advanced"
	conflicts := []ProposalConflict{}
	if record.ExpiresAt != nil && *record.ExpiresAt < now && status != control.Applied {
		status = control.Expired
		action = "expired"
	} else if record.BaseRevision != revision && (status == control.Submitted || status == control.Approved || status == control.Stale || status == control.NeedsRebase || status == control.Conflicted) {
		conflicts, err = q.conflicts(ctx, record, revision, nil, diffs, repo)
		if err != nil {
			return control.ProposalRecord{}, err
		}
		status = control.NeedsRebase
		for _, item := range conflicts {
			if item.Status != "clean" {
				status = control.Conflicted
				break
			}
		}
	}
	if status == record.Status {
		return record, nil
	}
	details := []any{}
	for _, c := range conflicts {
		item := map[string]any{"index": c.Index, "status": c.Status, "conflicting_paths": c.ConflictingPaths}
		if c.Reason != "" {
			item["reason"] = c.Reason
		}
		details = append(details, item)
	}
	err = control.WithTransaction(ctx, q.Proposals.DB, func(tx *sql.Tx) error {
		if err := q.event(ctx, tx, record, "system", action, status, revision, map[string]any{"conflicts": details}); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE proposals SET status=?,updated_at=? WHERE proposal_id=?", status, now, record.ProposalID)
		return err
	})
	if err != nil {
		return control.ProposalRecord{}, err
	}
	return q.Proposals.Get(ctx, record.ProposalID)
}
func (q ProposalQueue) event(ctx context.Context, tx *sql.Tx, record control.ProposalRecord, actor, action string, status control.ProposalStatus, revision string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	raw, err := pyjson.Dumps(details)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO proposal_events(proposal_id,actor,action,from_status,to_status,base_revision,repo_revision,details_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, record.ProposalID, actor, action, record.Status, status, record.BaseRevision, revision, raw, q.now())
	return err
}
func (q ProposalQueue) RefreshAll(ctx context.Context) error {
	return q.refreshAll(ctx, defaultProposalRepository())
}
func (q ProposalQueue) refreshAll(ctx context.Context, repo proposalRepository) error {
	revision, err := repo.main(q.Paths)
	if err != nil {
		return err
	}
	after := ""
	diffs := RevisionDiffs{}
	for {
		rows, err := q.Proposals.DB.QueryContext(ctx, "SELECT proposal_id FROM proposals WHERE proposal_id > ? AND status NOT IN ('applied','expired') ORDER BY proposal_id LIMIT 100", after)
		if err != nil {
			return err
		}
		ids, err := q.refreshPage(ctx, rows, revision, diffs, repo)
		if err != nil {
			return err
		}
		if len(ids) < 100 {
			return nil
		}
		after = ids[len(ids)-1]
	}
}
func (q ProposalQueue) refreshPage(ctx context.Context, rows proposalIDRows, revision string, diffs RevisionDiffs, repo proposalRepository) ([]string, error) {
	ids, err := proposalIDs(rows)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		record, err := q.Proposals.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if _, err = q.refresh(ctx, record, revision, diffs, repo); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

type proposalIDRows interface {
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
}

func proposalIDs(rows proposalIDRows) ([]string, error) {
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
