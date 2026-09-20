package execute

import (
	"context"
	"fmt"
)

type Dispatcher func(context.Context, string, map[string]any) (DispatchResult, error)

type Runner struct {
	Planner   *Planner
	Arguments *Arguments
	Limits    Limits
	Dispatch  Dispatcher
	Now       func() float64
}

func NewRunner(limits Limits, dispatch Dispatcher) (*Runner, error) {
	factory, err := NewFactory(limits)
	if err != nil {
		return nil, err
	}
	return factory.Runner(dispatch), nil
}

type runState struct {
	trace, revisions []any
	traceMaps        []map[string]any
	saved            map[string]any
	last             any
	committed        bool
	stopped          bool
	stopReason       any
	warnings         []string
}

func (r *Runner) Run(ctx context.Context, plan Plan) RunResult {
	if r.Dispatch == nil {
		return runFailure("validation_error", "execute dispatcher is required")
	}
	if err := r.Planner.Preflight(plan, r.Limits.MaxOperations, r.Arguments.Validate); err != nil {
		return runFailure("validation_error", err.Error())
	}
	state := &runState{trace: []any{}, revisions: []any{}, traceMaps: []map[string]any{}, saved: map[string]any{}, warnings: []string{}}
	started := r.Now()
	for index, operation := range plan.Operations {
		if r.expired(started) {
			return r.processingFailure(state, valueError("plan exceeded configured max_time_seconds"))
		}
		if err := CheckSaveName(operation.SaveAs, state.saved, r.Limits.MaxIntermediates); err != nil {
			return r.processingFailure(state, err)
		}
		args, err := ResolveArguments(operation, index+1, state.saved, r.Arguments.Validate)
		if err != nil {
			return r.processingFailure(state, err)
		}
		result, err := r.Dispatch(ctx, operation.Op, args)
		if err != nil {
			return r.processingFailure(state, err)
		}
		r.record(state, plan.StopOnError, index+1, operation, result)
		if err := EnsureOutputSize(map[string]any{"trace": state.trace, "revisions": state.revisions, "saved": state.saved}, executeLimit(r.Limits.MaxOutputBytes), state.committed); err != nil {
			return r.processingFailure(state, err)
		}
		if r.expired(started) {
			if !state.committed {
				return r.processingFailure(state, valueError("plan exceeded configured max_time_seconds"))
			}
			state.warnings = append(state.warnings, "memory_execute_deadline_exceeded_after_commit")
			state.stopped, state.stopReason = true, "deadline exceeded after committed operation"
			break
		}
		if state.stopped {
			break
		}
	}
	returns, err := ProjectReturns(savedOperations(plan), returnProjections(plan), state.saved, state.last, FailedSaveNames(state.traceMaps, state.saved), r.Limits.MaxRecords)
	if err != nil {
		return r.processingFailure(state, err)
	}
	payload := map[string]any{"trace": state.trace, "revisions": state.revisions, "returns": returns, "stopped": state.stopped, "stop_reason": state.stopReason}
	fitted, err := FitOutputPayload(payload, executeLimit(r.Limits.MaxOutputBytes), state.committed)
	if err != nil {
		return r.processingFailure(state, err)
	}
	if fitted["truncated"] == true {
		state.warnings = append(state.warnings, "memory_execute_output_truncated_after_commit")
	}
	return runSuccess(fitted, state.warnings)
}

func (r *Runner) expired(started float64) bool { return r.Now()-started > r.Limits.MaxTimeSeconds }
func (r *Runner) record(state *runState, stop bool, index int, operation PlannedOperation, result DispatchResult) {
	entry := map[string]any{"index": index, "op": operation.Op, "save_as": pointerValue(operation.SaveAs), "status": result.Status, "repo_revision": pointerValue(result.RepoRevision), "index_revision": pointerValue(result.IndexRevision), "operation_id": pointerValue(result.OperationID)}
	if result.Status == "success" {
		data := any(result.Data)
		if operation.Op != "asset_get" && operation.Op != "proposal_asset_get" {
			data = BoundValue(data, r.Limits.MaxRecords)
		}
		entry["data"] = data
		state.last = data
		if operation.SaveAs != nil {
			state.saved[*operation.SaveAs] = data
		}
		state.revisions = append(state.revisions, map[string]any{"index": index, "op": operation.Op, "repo_revision": pointerValue(result.RepoRevision), "index_revision": pointerValue(result.IndexRevision), "operation_id": pointerValue(result.OperationID)})
		contract := r.Planner.contracts[operation.Op]
		state.committed = state.committed || contract.CommitCapable || operation.Op == "proposal_rebase" || operation.Op == "proposal_review"
	} else {
		entry["error_class"], entry["message"] = result.ErrorClass, result.Message
		if stop {
			state.stopped, state.stopReason = true, fmt.Sprintf("operation %d failed", index)
		}
	}
	state.traceMaps = append(state.traceMaps, entry)
	state.trace = append(state.trace, entry)
}
func (r *Runner) processingFailure(state *runState, err error) RunResult {
	if !state.committed {
		return runFailure("validation_error", err.Error())
	}
	payload := map[string]any{"trace": state.trace, "revisions": state.revisions, "returns": map[string]any{}, "stopped": true, "stop_reason": "post-commit processing failed: " + err.Error()}
	fitted, fitErr := FitOutputPayload(payload, executeLimit(r.Limits.MaxOutputBytes), true)
	if fitErr != nil {
		return runFailure("validation_error", fitErr.Error())
	}
	return runSuccess(fitted, []string{"memory_execute_error_after_commit; reconcile before retrying"})
}
