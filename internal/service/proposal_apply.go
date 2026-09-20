package service

import (
	"context"
	"fmt"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/internal/repository"
)

// ProposalApplyResult carries the source payload plus published revision; HTTP
// and MCP success/error envelopes are composed by the still-unported endpoints.
type ProposalApplyResult struct {
	Data        map[string]any `json:"data"`
	OperationID string         `json:"operation_id"`
	Revision    string         `json:"repo_revision"`
}
type proposalTransaction func(context.Context, repository.TransactionRequest, repository.MutationCallback) (repository.TransactionResult, error)

func (c *ProposalControls) Apply(ctx context.Context, actor ProposalActor, id, expected, key string) (ProposalApplyResult, error) {
	manager := repository.TransactionManager{Paths: c.Queue.Paths, Operations: control.Operations{DB: c.Queue.Proposals.DB, Now: c.Queue.Now}, Now: c.Queue.Now, DerivedUpdate: c.DerivedUpdate}
	var result ProposalApplyResult
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		result, err = c.apply(ctx, actor, id, expected, key, defaultProposalRepository(), manager.ApplyUnderLock)
		return err
	})
	return result, err
}
func (c *ProposalControls) apply(ctx context.Context, actor ProposalActor, id, expected, key string, repo proposalRepository, transaction proposalTransaction) (ProposalApplyResult, error) {
	empty := ProposalApplyResult{}
	if err := access.RequireRole(actor.Policy, "curator"); err != nil {
		return empty, err
	}
	q := c.Queue
	proposal, err := q.Proposals.Get(ctx, id)
	if err != nil {
		return empty, err
	}
	if err = RequireProposalAccess(actor.Policy, proposal, true); err != nil {
		return empty, err
	}
	proposal, err = q.refresh(ctx, proposal, "", nil, repo)
	if err != nil {
		return empty, err
	}
	existing, err := (control.Operations{DB: q.Proposals.DB}).ByIdempotency(ctx, actor.Policy.Principal, key)
	if err != nil {
		return empty, err
	}
	request, err := pyjson.Dumps(map[string]any{"proposal_id": id, "expected_revision": expected})
	if err != nil {
		return empty, err
	}
	mutator := WorktreeMutator{MaxConceptBytes: c.MaxConceptBytes, Proposals: q.Proposals, Now: q.Now, Random: c.Random}
	if proposal.Status == control.Applied && existing != nil {
		if existing.RequestHash != (control.OperationRequest{RequestJSON: request}).RequestHash() {
			return empty, &control.IdempotencyConflictError{}
		}
		if existing.State == control.Succeeded && existing.ResultRevision != nil && *existing.ResultRevision != "" {
			return c.replayApplied(ctx, proposal, *existing, repo, mutator)
		}
	}
	conflicts, err := q.conflicts(ctx, proposal, "", nil, nil, repo)
	if err != nil {
		return empty, err
	}
	for _, conflict := range conflicts {
		if conflict.Status != "clean" {
			return empty, &Error{"conflict", "proposal has conflicting changes; inspect and resolve before apply"}
		}
	}
	revision, err := repo.main(q.Paths)
	if err != nil {
		return empty, err
	}
	if proposal.BaseRevision != revision {
		return empty, &Error{"needs_rebase", "proposal needs rebase and review before apply"}
	}
	if proposal.Status != control.Approved {
		return empty, &Error{"conflict", fmt.Sprintf("proposal %s is %s", id, proposal.Status)}
	}
	changes, err := c.prepareAppliedChanges(ctx, actor.Policy, proposal, expected, repo)
	if err != nil {
		return empty, err
	}
	operationID, err := c.newOperationID()
	if err != nil {
		return empty, err
	}
	result, err := transaction(ctx, repository.TransactionRequest{Operation: control.OperationRequest{OpID: operationID, Principal: actor.Policy.Principal, IdempotencyKey: key, ToolName: "memory_proposal_apply", RequestJSON: request, ClientInstanceID: actor.ClientInstanceID, MCPSessionID: actor.MCPSessionID, SourceChat: actor.SourceChat}, ExpectedRevision: expected, CommitMessage: "proposal: apply " + id, AuthorName: "Rui Carmo", AuthorEmail: "rui.carmo@gmail.com"}, func(ctx context.Context, worktree string) ([]string, error) {
		return mutator.ApplyChanges(ctx, worktree, changes, actor.Policy.Principal, actor.Policy, &id)
	})
	if err != nil {
		return empty, err
	}
	// Deliberately after publication/operation completion, matching Python. If
	// this update fails, recovery must inspect both operation and proposal state;
	// this method must not pretend it atomically committed with Git.
	updated, err := q.Proposals.UpdateStatus(ctx, id, control.ProposalStatusUpdate{Status: control.Applied, ReviewedBy: proposal.ReviewedBy, ReviewComment: proposal.ReviewComment, AppliedOperationID: &result.Operation.OpID, AppliedRevision: &result.ResultRevision})
	if err != nil {
		return empty, err
	}
	if err = q.refreshAll(ctx, repo); err != nil {
		return empty, err
	}
	if c.ChangedConcepts != nil {
		if err = c.ChangedConcepts(ctx, actor.Policy, result.ChangedPaths); err != nil {
			return empty, err
		}
	}
	return c.applyPayload(ctx, updated, changes, result.ChangedPaths, result.Replayed, result.Operation.OpID, result.ResultRevision, repo, mutator)
}
func (c *ProposalControls) replayApplied(ctx context.Context, proposal control.ProposalRecord, existing control.OperationRecord, repo proposalRepository, mutator WorktreeMutator) (ProposalApplyResult, error) {
	replay, err := existing.ReplayPayload()
	if err != nil {
		return ProposalApplyResult{}, err
	}
	changes, err := proposalChanges(proposal)
	if err != nil {
		return ProposalApplyResult{}, err
	}
	paths := []string{}
	if values, ok := replay["changed_paths"].([]any); ok {
		for _, value := range values {
			if path, ok := value.(string); ok {
				paths = append(paths, path)
			}
		}
	}
	return c.applyPayload(ctx, proposal, changes, paths, true, existing.OpID, *existing.ResultRevision, repo, mutator)
}
func (c *ProposalControls) prepareAppliedChanges(ctx context.Context, policy access.EffectivePolicy, proposal control.ProposalRecord, expected string, repo proposalRepository) ([]ProposalChange, error) {
	changes, err := proposalChanges(proposal)
	if err != nil {
		return nil, err
	}
	changes = AdaptExistingAssetConcepts(c.Queue.Paths.CurrentDir, changes)
	if err = ValidateChangeAuthorization(policy, changes, "write"); err != nil {
		return nil, err
	}
	if _, err = c.Queue.archivalImpact(ctx, policy, changes, expected, c.DerivedIndexPath, repo, openArchivalIndex); err != nil {
		return nil, err
	}
	return changes, nil
}
func (c *ProposalControls) applyPayload(ctx context.Context, proposal control.ProposalRecord, changes []ProposalChange, paths []string, replayed bool, operationID, revision string, repo proposalRepository, mutator WorktreeMutator) (ProposalApplyResult, error) {
	preview, err := mutator.previewChanges(c.Queue.Paths.CurrentDir, changes, defaultMutationIO())
	if err != nil {
		return ProposalApplyResult{}, err
	}
	payload, err := c.Queue.payload(ctx, proposal, preview, repo)
	if err != nil {
		return ProposalApplyResult{}, err
	}
	return ProposalApplyResult{Data: map[string]any{"proposal": payload, "changed_paths": paths, "replayed": replayed}, OperationID: operationID, Revision: revision}, nil
}
