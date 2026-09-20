package service

import (
	"context"

	"github.com/rcarmo/memento/internal/control"
)

// Payload includes retained proposal history without authorising the caller.
// The service must check visibility before exposing it through an endpoint.
func (q ProposalQueue) Payload(ctx context.Context, record control.ProposalRecord, preview string) (map[string]any, error) {
	return q.payload(ctx, record, preview, defaultProposalRepository())
}
func (q ProposalQueue) payload(ctx context.Context, record control.ProposalRecord, preview string, repo proposalRepository) (map[string]any, error) {
	patch, err := record.Patch()
	if err != nil {
		return nil, err
	}
	changes, exists := patch["changes"]
	if !exists {
		return nil, &ChangeValidationError{"missing proposal changes"}
	}
	revision, err := repo.main(q.Paths)
	if err != nil {
		return nil, err
	}
	rows, err := q.Proposals.DB.QueryContext(ctx, `SELECT event_id,actor,action,from_status,to_status,base_revision,repo_revision,details_json,created_at FROM proposal_events WHERE proposal_id=? ORDER BY event_id DESC LIMIT 50`, record.ProposalID)
	if err != nil {
		return nil, err
	}
	history, err := proposalHistory(rows)
	if err != nil {
		return nil, err
	}
	conflicts, err := q.conflicts(ctx, record, "", nil, nil, repo)
	if err != nil {
		return nil, err
	}
	optional := func(key string) any {
		value, exists := patch[key]
		if !exists {
			return []any{}
		}
		return value
	}
	return map[string]any{"proposal_id": record.ProposalID, "author_principal": record.AuthorPrincipal, "base_revision": record.BaseRevision, "intent": record.Intent, "rationale": nullableText(record.Rationale), "status": string(record.Status), "reviewed_by": nullableText(record.ReviewedBy), "review_comment": nullableText(record.ReviewComment), "applied_operation_id": nullableText(record.AppliedOperationID), "applied_revision": nullableText(record.AppliedRevision), "expires_at": nullableText(record.ExpiresAt), "current_revision": revision, "created_at": record.CreatedAt, "updated_at": record.UpdatedAt, "changes": changes, "history": history, "history_limit": 50, "conflicts": conflictDetails(conflicts), "consulted_concepts": optional("consulted_concepts"), "contradictions": optional("contradictions"), "reciprocal_links": optional("reciprocal_links"), "target_hint": patch["target_hint"], "diff": preview}, nil
}
func proposalHistory(rows proposalIDRows) ([]any, error) {
	defer rows.Close()
	result := []any{}
	for rows.Next() {
		var id int64
		var actor, action, from, to, base, revision, details, stamp string
		if err := rows.Scan(&id, &actor, &action, &from, &to, &base, &revision, &details, &stamp); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"event_id": id, "actor": actor, "action": action, "from_status": from, "to_status": to, "base_revision": base, "repo_revision": revision, "details_json": details, "created_at": stamp})
	}
	return result, rows.Err()
}
