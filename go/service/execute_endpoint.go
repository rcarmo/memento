package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/internal/envelope"
	"github.com/rcarmo/memento/go/umcp"
)

var executeFactory = execute.NewFactory
var executeRawTemplate = NewRawExecuteTemplate
var executeDefinition = executeToolDefinition

type ExecuteEndpoint struct {
	jobs    *Jobs
	factory *execute.Factory
	raw     *RawExecuteTemplate
	tool    proposalToolDefinition
}

func NewExecuteEndpoint(jobs *Jobs, catalog *Catalog, limits execute.Limits) (*ExecuteEndpoint, error) {
	if jobs == nil || catalog == nil {
		return nil, errors.New("execute endpoint requires jobs and catalog")
	}
	operations := append([]string{}, catalog.source.ExecuteCapable...)
	operations = append(operations, "asset_get", "proposal_asset_get")
	if len(operations) != 29 {
		return nil, errors.New("execute endpoint requires 29 operations")
	}
	factory, err := executeFactory(limits)
	if err != nil {
		return nil, err
	}
	raw, err := executeRawTemplate(jobs, operations)
	if err != nil {
		return nil, err
	}
	tool, err := executeDefinition(catalogToolDefinitions)
	if err != nil {
		return nil, err
	}
	return &ExecuteEndpoint{jobs, factory, raw, tool}, nil
}
func (e *ExecuteEndpoint) Call(ctx context.Context, args map[string]any) (any, error) {
	principal, session, err := e.jobs.Identity.requestPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	value, err := execute.NormalizeToolArguments(args)
	if err != nil {
		return executeFailure(err), nil
	}
	plan, err := e.factory.Parse(value)
	if err != nil {
		return executeFailure(err), nil
	}
	dispatcher, _ := e.raw.Bind(principal, session)
	reconciliation := IsReconciliationPlan(plan)
	return e.jobs.Workers.ExecutePlan(ctx, reconciliation, func(work context.Context) (any, error) {
		result := e.factory.Runner(dispatcher.Dispatch).Run(work, plan)
		markPendingReconciliation(reconciliation, e.jobs.Workers.ExecuteBusy(), result.Data)
		if result.Status == "error" {
			return envelope.NewFailure(result.ErrorClass, result.Message)
		}
		return e.jobs.Controls.Queue.SuccessEnvelope(result.Data, SuccessOptions{Warnings: result.Warnings})
	})
}
func markPendingReconciliation(reconciliation, busy bool, value any) {
	if reconciliation && busy {
		markExecutePending(value)
	}
}
func markExecutePending(value any) {
	switch item := value.(type) {
	case map[string]any:
		if item["operation"] == nil && item["safe_to_retry"] == true {
			item["final_state"] = "in_progress"
			item["safe_to_retry"] = false
			item["retry_guidance"] = "A worker is active; reconcile again after it finishes."
		}
		for _, child := range item {
			markExecutePending(child)
		}
	case []any:
		for _, child := range item {
			markExecutePending(child)
		}
	}
}
func executeFailure(err error) envelope.Failure {
	failure, _ := envelope.NewFailure("validation_error", err.Error())
	return failure
}
func (e *ExecuteEndpoint) Register(server *umcp.Server) error {
	if server == nil {
		return errors.New("execute registration requires a server")
	}
	raw, _ := json.Marshal([]proposalToolDefinition{e.tool})
	return registerProposalTools(server, func(ctx context.Context, _ string, args map[string]any) (any, error) { return e.Call(ctx, args) }, nil, raw)
}
func executeToolDefinition(raw []byte) (proposalToolDefinition, error) {
	var definitions []proposalToolDefinition
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&definitions); err != nil {
		return proposalToolDefinition{}, err
	}
	for _, definition := range definitions {
		if definition.Name == "memory_execute" {
			return definition, nil
		}
	}
	return proposalToolDefinition{}, errors.New("memory_execute definition is unavailable")
}
