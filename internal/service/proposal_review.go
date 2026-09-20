package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

// Review accepts trusted identity, not MCP-supplied principal data. An empty key
// generates a fresh key for legacy clients, matching the source optional field.
func (c *ProposalControls) Review(ctx context.Context, actor ProposalActor, id, decision string, comment *string, key string) (ProposalControlResult, error) {
	var result ProposalControlResult
	err := repository.WithTransactionLock(ctx, c.Queue.Paths, func() error {
		var err error
		result, err = c.review(ctx, actor, id, decision, comment, key, defaultProposalRepository())
		return err
	})
	return result, err
}
func (c *ProposalControls) review(ctx context.Context, actor ProposalActor, id, decision string, comment *string, key string, repo proposalRepository) (ProposalControlResult, error) {
	empty := ProposalControlResult{}
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
	if key == "" {
		key, err = c.newOperationID()
		if err != nil {
			return empty, err
		}
	}
	request, replay, err := c.controlReplay(ctx, actor.Policy, key, "memory_proposal_review", map[string]any{"proposal_id": id, "decision": decision, "comment": nullableText(comment)})
	if err != nil {
		return empty, err
	}
	if replay != nil {
		payload, err := replay.ReplayPayload()
		if err != nil {
			return empty, err
		}
		if payload == nil {
			payload = map[string]any{}
		}
		payload["replayed"] = true
		return ProposalControlResult{payload, replay.OpID}, nil
	}
	proposal, err = q.refresh(ctx, proposal, "", nil, repo)
	if err != nil {
		return empty, err
	}
	if proposal.Status == control.Applied || proposal.Status == control.Expired {
		return empty, &Error{"conflict", fmt.Sprintf("proposal %s is already %s", id, proposal.Status)}
	}
	revision, err := repo.main(q.Paths)
	if err != nil {
		return empty, err
	}
	if decision == "approve" {
		conflicts, err := q.conflicts(ctx, proposal, "", nil, nil, repo)
		if err != nil {
			return empty, err
		}
		for _, conflict := range conflicts {
			if conflict.Status != "clean" {
				return empty, &Error{"conflict", "proposal has conflicting changes; inspect and resolve before approval"}
			}
		}
		if proposal.BaseRevision != revision {
			return empty, &Error{"needs_rebase", "proposal needs rebase before approval"}
		}
		if err = c.approvalArchival(ctx, actor.Policy, proposal, repo); err != nil {
			return empty, err
		}
	}
	status, ok := map[string]control.ProposalStatus{"approve": control.Approved, "reject": control.Rejected, "request_changes": control.Draft}[decision]
	if !ok {
		return empty, &Error{"validation_error", "unsupported proposal decision: " + decision}
	}
	var operationID string
	var result map[string]any
	err = control.WithTransaction(ctx, q.Proposals.DB, func(tx *sql.Tx) error {
		if err := q.event(ctx, tx, proposal, actor.Policy.Principal, "review", status, revision, map[string]any{"decision": decision, "comment": nullableText(comment), "previous_reviewed_by": nullableText(proposal.ReviewedBy), "previous_review_comment": nullableText(proposal.ReviewComment)}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE proposals SET status=?,reviewed_by=?,review_comment=?,updated_at=? WHERE proposal_id=?", status, actor.Policy.Principal, comment, q.now(), id); err != nil {
			return err
		}
		updated, err := q.Proposals.GetInTx(ctx, tx, id)
		if err != nil {
			return err
		}
		assets, err := q.Proposals.ListAssetsInTx(ctx, tx, control.ProposalAssetQuery{ProposalID: &id})
		if err != nil {
			return err
		}
		summary, err := q.summary(ctx, updated, assets, true, repo)
		if err != nil {
			return err
		}
		summary["review_comment"] = nullableText(comment)
		result = map[string]any{"proposal": summary, "proposal_id": id, "replayed": false}
		operationID, err = c.journalControl(ctx, tx, actor, key, "memory_proposal_review", request, revision, result)
		return err
	})
	if err != nil {
		return empty, err
	}
	return ProposalControlResult{result, operationID}, nil
}

func (c *ProposalControls) approvalArchival(ctx context.Context, policy access.EffectivePolicy, proposal control.ProposalRecord, repo proposalRepository) error {
	patch, err := proposal.Patch()
	if err != nil {
		return err
	}
	if !jsonTruthy(patch["archival_impact"]) {
		return nil
	}
	changes, err := proposalChanges(proposal)
	if err != nil {
		return err
	}
	_, err = c.Queue.archivalImpact(ctx, policy, changes, proposal.BaseRevision, c.DerivedIndexPath, repo, openArchivalIndex)
	return err
}

// Only the JSON-domain truthiness needed for stored proposal fields. Floats and
// integers are decoded as json.Number by the persisted-patch reader.
func jsonTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	case json.Number:
		n, _ := v.Float64()
		return n != 0
	default:
		return true
	}
}
