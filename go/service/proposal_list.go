package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/internal/pyjson"
	"github.com/rcarmo/memento/go/repository"
)

func (c *ProposalControls) ListProposals(ctx context.Context, actor ProposalActor, status *string, limit int, cursor *string) (map[string]any, error) {
	var result map[string]any
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		result, err = c.listProposals(ctx, actor, status, limit, cursor, defaultProposalRepository())
		return err
	})
	return result, err
}
func (c *ProposalControls) listProposals(ctx context.Context, actor ProposalActor, status *string, limit int, cursor *string, repo proposalRepository) (map[string]any, error) {
	if err := access.RequireRole(actor.Policy, "proposer"); err != nil {
		return nil, err
	}
	if err := c.Queue.refreshAll(ctx, repo); err != nil {
		return nil, err
	}
	var requested *control.ProposalStatus
	unresolved := status != nil && *status == "unresolved"
	if status != nil && !unresolved {
		value := control.ProposalStatus(*status)
		if value == control.Stale {
			value = control.NeedsRebase
		}
		switch value {
		case control.Draft, control.Submitted, control.Approved, control.Rejected, control.Applied, control.Expired, control.NeedsRebase, control.Conflicted:
		default:
			return nil, &Error{"validation_error", fmt.Sprintf("'%s' is not a valid ProposalStatus", *status)}
		}
		requested = &value
	}
	if limit < 1 || limit > 200 {
		return nil, &Error{"validation_error", "proposal list limit must be between 1 and 200"}
	}
	revision, err := repo.main(c.Queue.Paths)
	if err != nil {
		return nil, err
	}
	scope := proposalListScope(actor.Policy, status, revision)
	var after *string
	if cursor != nil {
		value, err := c.decodeProposalCursor(*cursor, scope)
		if err != nil {
			return nil, err
		}
		after = &value
	}
	author := &actor.Policy.Principal
	for _, role := range actor.Policy.Roles {
		if role == "curator" {
			author = nil
			break
		}
	}
	now := c.Queue.now()
	count := limit + 1
	candidates, err := c.Queue.Proposals.List(ctx, control.ProposalQuery{Status: requested, Unresolved: unresolved, AuthorPrincipal: author, CurrentRevision: &revision, Now: &now, Limit: &count, Cursor: after})
	if err != nil {
		return nil, err
	}
	selected := []control.ProposalRecord{}
	for _, record := range candidates[:min(limit, len(candidates))] {
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
	}
	var next any
	if len(candidates) > limit {
		token, err := c.encodeProposalCursor(scope, candidates[limit-1].ProposalID)
		if err != nil {
			return nil, err
		}
		next = token
	}
	summaries := []any{}
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
