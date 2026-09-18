package service

import (
	"context"
	"errors"

	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/umcp"
)

// ExecuteAdapter invokes the same immutable callbacks used by direct catalog
// dispatch. It does not perform worker admission or register memory_execute.
type ExecuteAdapter struct{ handlers map[string]CatalogHandler }

func NewExecuteAdapter(handlers map[string]CatalogHandler) (*ExecuteAdapter, error) {
	if len(handlers) == 0 {
		return nil, errors.New("execute adapter requires operation handlers")
	}
	owned := make(map[string]CatalogHandler, len(handlers))
	for name, handler := range handlers {
		if handler != nil {
			owned[name] = handler
		}
	}
	return &ExecuteAdapter{handlers: owned}, nil
}
func (a *ExecuteAdapter) Dispatch(ctx context.Context, operation string, args map[string]any) (execute.DispatchResult, error) {
	handler := a.handlers["memory_"+operation]
	if handler == nil {
		return execute.DispatchResult{}, errors.New("execute operation handler is unavailable")
	}
	value, err := handler(ctx, args)
	if err != nil {
		return execute.DispatchResult{}, err
	}
	normal, err := MCPEnvelope(value)
	if err != nil {
		return execute.DispatchResult{}, err
	}
	raw, ok := executeObject(normal)
	if !ok {
		return execute.DispatchResult{}, errors.New("execute operation returned a non-object envelope")
	}
	status, _ := raw["status"].(string)
	result := execute.DispatchResult{Status: status}
	result.RepoRevision = optionalEnvelopeString(raw["repo_revision"])
	result.IndexRevision = optionalEnvelopeString(raw["index_revision"])
	result.OperationID = optionalEnvelopeString(raw["operation_id"])
	switch status {
	case "success":
		data, ok := executeObject(raw["data"])
		if !ok {
			return execute.DispatchResult{}, errors.New("execute success envelope requires object data")
		}
		result.Data = data
	case "error":
		result.ErrorClass, _ = raw["error_class"].(string)
		result.Message, _ = raw["message"].(string)
		if result.ErrorClass == "" || result.Message == "" {
			return execute.DispatchResult{}, errors.New("execute error envelope is malformed")
		}
	default:
		return execute.DispatchResult{}, errors.New("execute operation returned an invalid status")
	}
	return result, nil
}
func executeObject(value any) (map[string]any, bool) {
	if object, ok := value.(map[string]any); ok {
		return object, true
	}
	object, ok := value.(umcp.OrderedObject)
	if !ok {
		return nil, false
	}
	result := make(map[string]any, len(object))
	for _, member := range object {
		result[member.Name] = executeValue(member.Value)
	}
	return result, true
}
func executeValue(value any) any {
	if object, ok := executeObject(value); ok {
		return object
	}
	if rows, ok := value.([]any); ok {
		result := make([]any, len(rows))
		for i, row := range rows {
			result[i] = executeValue(row)
		}
		return result
	}
	return value
}
func optionalEnvelopeString(value any) *string {
	text, ok := value.(string)
	if !ok {
		return nil
	}
	return &text
}
