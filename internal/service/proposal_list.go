package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/internal/repository"
)

func (c *ProposalControls) ListProposals(ctx context.Context, actor ProposalActor, status *string, limit int, cursor *string) (map[string]any, error) {
	var result map[string]any
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		result, err = c.listProposals(ctx, actor, status, limit, cursor, defaultProposalRepository(), ProposalListOptions{})
		return err
	})
	return result, err
}

// ProposalListOptions selects a recent actionable queue, or explicit history.
type ProposalListOptions struct {
	ExcludeStatuses []string
	SortBy          string
	SortOrder       string
}

func normalizeProposalList(status *string, opts ProposalListOptions) (string, ProposalListOptions, error) {
	filter := "pending"
	if status != nil {
		filter = *status
	}
	if filter == "stale" {
		filter = "needs_rebase"
	}
	switch filter {
	case "all", "pending", "unresolved", "draft", "submitted", "approved", "rejected", "applied", "expired", "needs_rebase", "conflicted":
	default:
		return "", opts, &Error{"validation_error", fmt.Sprintf("'%s' is not a valid ProposalStatus", filter)}
	}
	if opts.SortBy == "" {
		opts.SortBy = "created_at"
	}
	if opts.SortOrder == "" {
		opts.SortOrder = "desc"
	}
	switch opts.SortBy {
	case "created_at", "updated_at", "proposal_id":
	default:
		return "", opts, &Error{"validation_error", "sort_by must be created_at, updated_at or proposal_id"}
	}
	if opts.SortOrder != "asc" && opts.SortOrder != "desc" {
		return "", opts, &Error{"validation_error", "sort_order must be asc or desc"}
	}
	excluded := make([]string, 0, len(opts.ExcludeStatuses))
	for _, value := range opts.ExcludeStatuses {
		if value == "stale" {
			value = "needs_rebase"
		}
		switch value {
		case "draft", "submitted", "approved", "rejected", "applied", "expired", "needs_rebase", "conflicted":
		default:
			return "", opts, &Error{"validation_error", "invalid exclude_statuses value"}
		}
		excluded = append(excluded, value)
	}
	sort.Strings(excluded)
	opts.ExcludeStatuses = slices.Compact(excluded)
	return filter, opts, nil
}

func (c *ProposalControls) listProposals(ctx context.Context, actor ProposalActor, status *string, limit int, cursor *string, repo proposalRepository, options ...ProposalListOptions) (map[string]any, error) {
	if err := access.RequireRole(actor.Policy, "proposer"); err != nil {
		return nil, err
	}
	opts := ProposalListOptions{}
	if len(options) > 0 {
		opts = options[0]
	}
	filter, opts, err := normalizeProposalList(status, opts)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 {
		return nil, &Error{"validation_error", "proposal list limit must be between 1 and 200"}
	}
	if err = c.Queue.refreshAll(ctx, repo); err != nil {
		return nil, err
	}
	revision, err := repo.main(c.Queue.Paths)
	if err != nil {
		return nil, err
	}
	scope := proposalListScope(actor.Policy, &filter, revision)
	scope["sort_by"], scope["sort_order"], scope["exclude_statuses"] = opts.SortBy, opts.SortOrder, opts.ExcludeStatuses
	var after *control.ProposalPosition
	if cursor != nil {
		raw, err := c.decodeProposalCursor(*cursor, scope)
		if err != nil {
			return nil, err
		}
		after = &control.ProposalPosition{}
		if json.Unmarshal([]byte(raw), after) != nil || after.ID == "" || after.Value == "" {
			return nil, &Error{"validation_error", "invalid proposal list cursor"}
		}
	}
	author := &actor.Policy.Principal
	for _, role := range actor.Policy.Roles {
		if role == "curator" {
			author = nil
			break
		}
	}
	now := c.Queue.now()
	batch := limit + 1
	query := control.ProposalQuery{AuthorPrincipal: author, CurrentRevision: &revision, Now: &now, Limit: &batch, SortBy: opts.SortBy, SortOrder: opts.SortOrder}
	switch filter {
	case "all":
	case "pending":
		query.Pending = true
	case "unresolved":
		query.Unresolved = true
	default:
		v := control.ProposalStatus(filter)
		query.Status = &v
	}
	for _, s := range opts.ExcludeStatuses {
		query.ExcludeStatuses = append(query.ExcludeStatuses, control.ProposalStatus(s))
	}
	// Scan to limit+1 visible records; hidden rows must not produce empty pages
	// or a cursor that exposes their identifiers/timestamps.
	selected := make([]control.ProposalRecord, 0, batch)
	for len(selected) < batch {
		query.After = after
		candidates, err := c.Queue.Proposals.List(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, record := range candidates {
			position := proposalPosition(record, opts.SortBy)
			after = &position
			allowed, err := CanAccessProposal(actor.Policy, record, true)
			if err != nil {
				return nil, err
			}
			if !allowed {
				continue
			}
			record, err = c.Queue.refresh(ctx, record, "", nil, repo)
			if err != nil {
				return nil, err
			}
			selected = append(selected, record)
			if len(selected) == batch {
				break
			}
		}
		if len(candidates) < batch {
			break
		}
	}
	var next any
	if len(selected) > limit {
		position := proposalPosition(selected[limit-1], opts.SortBy)
		// This fixed struct contains only strings, so marshaling cannot fail.
		raw, _ := json.Marshal(position)
		next, err = c.encodeProposalCursor(scope, string(raw))
		if err != nil {
			return nil, err
		}
		selected = selected[:limit]
	}
	summaries := make([]any, 0, len(selected))
	for _, record := range selected {
		assetRows, err := c.Queue.Proposals.ListAssets(ctx, control.ProposalAssetQuery{ProposalID: &record.ProposalID})
		if err != nil {
			return nil, err
		}
		summary, err := c.Queue.summary(ctx, record, assetRows, false, repo)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return map[string]any{"proposals": summaries, "next_cursor": next}, nil
}

func proposalPosition(record control.ProposalRecord, field string) control.ProposalPosition {
	value := record.CreatedAt
	if field == "updated_at" {
		value = record.UpdatedAt
	}
	if field == "proposal_id" {
		value = record.ProposalID
	}
	return control.ProposalPosition{Value: value, ID: record.ProposalID}
}
func proposalListScope(policy access.EffectivePolicy, status *string, revision string) map[string]any {
	// Preserve list order, even when grants are semantically equivalent.
	return map[string]any{"principal": policy.Principal, "status": nullableText(status), "revision": revision, "reads": append([]string{}, policy.ReadPrefixes...), "writes": append([]string{}, policy.WritePrefixes...), "protected": append([]string{}, policy.ProtectedReadPrefixes...), "roles": append([]string{}, policy.Roles...)}
}
func (c *ProposalControls) cursorCipher() (proposalCursor, error) {
	c.cursorMu.Lock()
	defer c.cursorMu.Unlock()
	if c.cursor != nil {
		return *c.cursor, nil
	}
	source := c.Random
	if source == nil {
		source = rand.Reader
	}
	var key [32]byte
	if _, err := io.ReadFull(source, key[:]); err != nil {
		return proposalCursor{}, err
	}
	c.cursor = &proposalCursor{key: key}
	return *c.cursor, nil
}
func (c *ProposalControls) encodeProposalCursor(scope map[string]any, after string) (string, error) {
	codec, err := c.cursorCipher()
	if err != nil {
		return "", err
	}
	// Token payload ordering is not an identity contract; decrypt/compare exact
	// JSON values. Scope/value coercion remains strict for internally minted keys.
	raw, err := pyjson.Dumps(map[string]any{"scope": scope, "after": after})
	if err != nil {
		return "", err
	}
	now := time.Now()
	if c.Queue.Now != nil {
		now = c.Queue.Now()
	}
	source := c.Random
	if source == nil {
		source = rand.Reader
	}
	return codec.encrypt([]byte(raw), now, source)
}
func (c *ProposalControls) decodeProposalCursor(token string, scope map[string]any) (string, error) {
	codec, err := c.cursorCipher()
	if err != nil {
		return "", err
	}
	raw, err := codec.decrypt(token)
	if err != nil {
		return "", &Error{"validation_error", errProposalCursor.Error()}
	}
	var decoded struct {
		Scope map[string]any `json:"scope"`
		After *string        `json:"after"`
	}
	if err = json.Unmarshal(raw, &decoded); err != nil || decoded.After == nil {
		return "", &Error{"validation_error", errProposalCursor.Error()}
	}
	expected, _ := pyjson.Dumps(scope)
	actual, _ := pyjson.Dumps(decoded.Scope)
	// Both objects are known JSON values. A sorted encoding preserves null/[]
	// distinctions while avoiding Go []any versus []string representation drift.
	if actual != expected {
		return "", &Error{"validation_error", errProposalCursor.Error()}
	}
	return *decoded.After, nil
}
