package service

import (
	"context"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

type OperationOutcome struct {
	Data                  map[string]any
	Revision, OperationID *string
}
type transactionProbe func(context.Context, repository.GitRepositoryPaths, func() error) (bool, error)

// OperationGet deliberately does not take the writer lock for the whole call:
// it must remain usable while a timed-out mutation is still running.
func (c *ProposalControls) OperationGet(ctx context.Context, actor ProposalActor, key, id *string) (OperationOutcome, error) {
	return c.operationGet(ctx, actor, key, id, defaultProposalRepository(), repository.TryTransactionLock)
}
func (c *ProposalControls) operationGet(ctx context.Context, actor ProposalActor, key, id *string, repo proposalRepository, probe transactionProbe) (OperationOutcome, error) {
	empty := OperationOutcome{}
	curator, proposer := false, false
	for _, role := range actor.Policy.Roles {
		curator = curator || role == "curator"
		proposer = proposer || role == "proposer"
	}
	if !curator && !proposer {
		return empty, access.RequireRole(actor.Policy, "curator")
	}
	if (key == nil) == (id == nil) {
		return empty, &Error{"validation_error", "provide exactly one of idempotency_key or operation_id"}
	}
	operations := control.Operations{DB: c.Queue.Proposals.DB}
	var operation *control.OperationRecord
	var err error
	if key != nil {
		operation, err = operations.ByIdempotency(ctx, actor.Policy.Principal, *key)
		if err != nil {
			return empty, err
		}
		if operation == nil {
			acquired, err := probe(ctx, c.Queue.Paths, func() error {
				var err error
				operation, err = operations.ByIdempotency(ctx, actor.Policy.Principal, *key)
				return err
			})
			if err != nil {
				return empty, err
			}
			if !acquired {
				return missingOperation(false), nil
			}
			if operation == nil {
				return missingOperation(true), nil
			}
		}
	} else {
		record, err := operations.Get(ctx, *id)
		if err != nil {
			return empty, err
		}
		operation = &record
		if operation.Principal != actor.Policy.Principal {
			return empty, &Error{"forbidden", "operation belongs to another principal"}
		}
	}
	if !curator && operation.ToolName != "memory_proposal_rebase" {
		return empty, &Error{"forbidden", "proposers may reconcile only their own proposal rebases"}
	}
	return c.operationOutcome(ctx, actor.Policy, *operation, repo)
}
func missingOperation(retry bool) OperationOutcome {
	state, guidance := "in_progress", "Writer is active; reconcile the original key after it finishes."
	if retry {
		state = "not_committed"
		guidance = "No operation exists for this principal and idempotency key; retry the original request with the same key."
	}
	return OperationOutcome{Data: map[string]any{"final_state": state, "operation": nil, "changed_paths": []string{}, "partial": false, "safe_to_retry": retry, "retry_guidance": guidance}}
}
func (c *ProposalControls) operationOutcome(ctx context.Context, policy access.EffectivePolicy, operation control.OperationRecord, repo proposalRepository) (OperationOutcome, error) {
	empty := OperationOutcome{}
	replay, err := operation.ReplayPayload()
	if err != nil {
		return empty, &ChangeValidationError{Message: err.Error()}
	}
	if operation.ToolName == "memory_proposal_rebase" || operation.ToolName == "memory_proposal_review" {
		id, err := proposalPathValue(replay, "proposal_id")
		if err != nil {
			return empty, err
		}
		proposal, err := c.Queue.Proposals.Get(ctx, id)
		if err != nil {
			return empty, err
		}
		if err = RequireProposalAccess(policy, proposal, true); err != nil {
			return empty, err
		}
		if replay == nil {
			replay = map[string]any{}
		}
		return OperationOutcome{Data: map[string]any{"final_state": "committed", "operation": map[string]any{"operation_id": operation.OpID, "tool_name": operation.ToolName, "state": string(operation.State), "idempotency_key": operation.IdempotencyKey, "result_revision": nullableText(operation.ResultRevision)}, "proposal_id": id, "result": replay, "changed_paths": []string{}, "partial": false, "safe_to_retry": false, "retry_guidance": "Control-state change committed; use the recorded result, do not repeat."}, OperationID: &operation.OpID}, nil
	}
	changed := []string{}
	if paths, ok := replay["changed_paths"].([]any); ok {
		for _, value := range paths {
			path, ok := value.(string)
			if !ok {
				continue
			}
			if _, err := access.AuthorizePath(policy, path, "read"); err != nil {
				continue
			}
			if _, err := access.AuthorizePath(policy, path, "write"); err != nil {
				continue
			}
			changed = append(changed, path)
		}
	}
	state, retry, guidance := "", false, ""
	switch operation.State {
	case control.Succeeded:
		state, guidance = "committed", "The commit completed; do not retry the mutation."
	case control.Queued, control.Running, control.Recovering:
		state, guidance = "in_progress", "The outcome is not final; wait and reconcile again."
	case control.Conflict:
		state, guidance = "not_committed", "Nothing was published by this operation. Read current state, then use a fresh expected revision and a new idempotency key."
	default:
		revision, err := repo.main(c.Queue.Paths)
		if err != nil {
			return empty, err
		}
		if operation.BaseRevision != nil && revision != *operation.BaseRevision {
			state, guidance = "indeterminate", "Repository history advanced after this operation began; inspect affected paths before retrying."
		} else {
			state, retry, guidance = "failed_before_mutation", true, "The operation failed before publication; retry the identical request with the same key."
		}
	}
	var message any
	if operation.ErrorMessage != nil {
		text := []rune(*operation.ErrorMessage)
		message = string(text[:min(500, len(text))])
	}
	return OperationOutcome{Data: map[string]any{"final_state": state, "operation": map[string]any{"operation_id": operation.OpID, "idempotency_key": operation.IdempotencyKey, "tool_name": operation.ToolName, "state": string(operation.State), "base_revision": nullableText(operation.BaseRevision), "result_revision": nullableText(operation.ResultRevision), "error_class": nullableText(operation.ErrorClass), "error_message": message}, "changed_paths": changed, "partial": false, "safe_to_retry": retry, "retry_guidance": guidance}, Revision: operation.ResultRevision, OperationID: &operation.OpID}, nil
}
