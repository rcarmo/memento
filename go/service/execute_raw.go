package service

import (
	"context"
	"errors"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/execute"
)

// RawExecuteDispatcher invokes only executor operations after outer admission.
// The caller captures trusted identity/session before constructing it.
type RawExecuteDispatcher struct {
	jobs      *Jobs
	principal access.Principal
	session   *string
	allowed   map[string]bool
}

func NewRawExecuteDispatcher(jobs *Jobs, principal access.Principal, session *string, operations []string) (*RawExecuteDispatcher, error) {
	if jobs == nil || jobs.Identity == nil || jobs.Controls == nil || principal.Name == "" {
		return nil, errors.New("raw execute dispatcher requires composed jobs and principal")
	}
	allowed := map[string]bool{}
	for _, operation := range operations {
		if operation == "" {
			return nil, errors.New("raw execute dispatcher has an empty operation")
		}
		allowed[operation] = true
	}
	if len(allowed) == 0 {
		return nil, errors.New("raw execute dispatcher requires operations")
	}
	return &RawExecuteDispatcher{jobs: jobs, principal: principal, session: session, allowed: allowed}, nil
}

func (d *RawExecuteDispatcher) Dispatch(ctx context.Context, operation string, args map[string]any) (execute.DispatchResult, error) {
	value, err := d.Call(ctx, operation, args)
	if err != nil {
		return execute.DispatchResult{}, err
	}
	adapter, _ := NewExecuteAdapter(map[string]CatalogHandler{"memory_" + operation: func(context.Context, map[string]any) (any, error) { return value, nil }})
	return adapter.Dispatch(ctx, operation, args)
}

func (d *RawExecuteDispatcher) Call(ctx context.Context, operation string, args map[string]any) (any, error) {
	if !d.allowed[operation] {
		return nil, errors.New("execute operation is unavailable")
	}
	name := "memory_" + operation
	resolve := name != "memory_purge" || args["confirm"] == true
	return d.jobs.runWithPolicy(ctx, d.principal, d.session, name, resolve, func(ctx context.Context, controls *ProposalControls, actor ProposalActor) (map[string]any, SuccessOptions, error) {
		return runProposalTool(ctx, controls, actor, name, args)
	}, control.Connect)
}
