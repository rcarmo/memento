package service

import (
	"context"
	"database/sql"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

// Propose mirrors the non-model submission path. Identity is resolved by the
// caller, not supplied as a tool argument. Source submissions have no durable
// idempotency key: a repeated call generates another proposal ID.
func (c *ProposalControls) Propose(ctx context.Context, actor ProposalActor, intent, base string, changes []any, rationale *string) (map[string]any, error) {
	var result map[string]any
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		result, err = c.propose(ctx, actor, intent, base, changes, rationale, defaultProposalRepository())
		return err
	})
	return result, err
}
func (c *ProposalControls) propose(ctx context.Context, actor ProposalActor, intent, base string, changes []any, rationale *string, repo proposalRepository) (map[string]any, error) {
	if err := access.RequireRole(actor.Policy, "proposer"); err != nil {
		return nil, err
	}
	id, err := c.newOperationID()
	if err != nil {
		return nil, err
	}
	prepared, err := c.prepareAssets(ctx, changes, actor.Policy.Principal, repo)
	if err != nil {
		return nil, err
	}
	normalized, err := NormalizeProposalChanges(prepared.Changes)
	if err != nil {
		return nil, err
	}
	if err = ValidateChangeAuthorization(actor.Policy, normalized, "write"); err != nil {
		return nil, err
	}
	if err = c.validateSkillBindings(normalized, repo); err != nil {
		return nil, err
	}
	mutator := WorktreeMutator{MaxConceptBytes: c.MaxConceptBytes}
	preview, err := mutator.PreviewChanges(c.Queue.Paths.CurrentDir, normalized)
	if err != nil {
		return nil, err
	}
	impact, err := c.Queue.archivalImpact(ctx, actor.Policy, normalized, base, c.DerivedIndexPath, repo, openArchivalIndex)
	if err != nil {
		return nil, err
	}
	stored := make([]any, 0, len(normalized))
	for _, change := range normalized {
		stored = append(stored, map[string]any(change))
	}
	var record control.ProposalRecord
	err = control.WithTransaction(ctx, c.Queue.Proposals.DB, func(tx *sql.Tx) error {
		var err error
		record, err = c.Queue.Proposals.CreateInTx(ctx, tx, control.ProposalRequest{ProposalID: id, AuthorPrincipal: actor.Policy.Principal, ClientInstanceID: actor.ClientInstanceID, BaseRevision: base, Intent: intent, Rationale: rationale, Patch: map[string]any{"changes": stored, "archival_impact": impact}, Assets: prepared.Assets})
		if err != nil {
			return err
		}
		if c.Staging != nil {
			return c.Staging.ConsumeInTx(ctx, tx, actor.Policy.Principal, prepared.StagedIDs, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	payload, err := c.Queue.payload(ctx, record, preview, repo)
	if err != nil {
		return nil, err
	}
	visible, err := c.Queue.visibleArchivalImpact(ctx, record, actor.Policy, c.DerivedIndexPath, repo)
	if err != nil {
		return nil, err
	}
	payload["archival_impact"] = visible
	return map[string]any{"proposal": payload}, nil
}
