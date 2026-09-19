package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/umcp"
)

const CanonicalShallowToolsJSON = `[{"name":"search_then_read","description":"Search for the best matching concept, then read it.","parameters":{"query":{"type":"string","required":true},"search_mode":{"type":"string","required":false}}},{"name":"search_paths","description":"Search concepts and return matching paths.","parameters":{"query":{"type":"string","required":true},"limit":{"type":"number","required":false},"search_mode":{"type":"string","required":false}}},{"name":"status_field","description":"Return one service status field.","parameters":{"field":{"type":"string","required":true}}},{"name":"search_then_graph","description":"Search for a concept, then inspect its graph neighborhood.","parameters":{"query":{"type":"string","required":true},"depth":{"type":"number","required":false},"search_mode":{"type":"string","required":false}}},{"name":"read_field","description":"Read an exact path or concept id and return one field.","parameters":{"id_or_path":{"type":"string","required":true},"field":{"type":"string","required":true}}},{"name":"UNKNOWN","description":"Use for unsupported, unsafe, ambiguous, external or insufficiently identified requests.","parameters":{}}]`

type RouteInference interface {
	Generate(*needle.Tokenizer, string, string, needle.GenerationOptions, needle.Checkpoint) (string, error)
}
type RouteEndpoint struct {
	Jobs       *Jobs
	Router     RouteInference
	Tokenizer  *needle.Tokenizer
	Execute    *ExecuteEndpoint
	dispatchFn func(context.Context, map[string]any) (any, error)
	adaptFn    func(any, *RouteProjection) (map[string]any, SuccessOptions, error)
}

func (e RouteEndpoint) Call(ctx context.Context, args map[string]any) (any, error) {
	request, ok := args["request"].(string)
	if !ok {
		return nil, errors.New("request must be a string")
	}
	execute := true
	if value, exists := args["execute"]; exists {
		var valid bool
		execute, valid = value.(bool)
		if !valid {
			return nil, errors.New("execute must be a boolean")
		}
	}
	request = strings.TrimSpace(request)
	if request == "" {
		return e.failure(errors.New("request must not be empty"))
	}
	if utf8.RuneCountInString(request) > 200 {
		return e.failure(errors.New("request must be at most 200 characters"))
	}
	if e.Router == nil || e.Tokenizer == nil {
		return e.failure(errors.New("needle router is not loaded"))
	}
	if _, err := e.Jobs.Identity.Context(ctx); err != nil {
		return nil, err
	}
	generated, err := e.Jobs.Workers.Call(ctx, "memory_route", func(work context.Context) (any, error) {
		return e.Router.Generate(e.Tokenizer, request, CanonicalShallowToolsJSON, needle.DefaultGenerationOptions(), func(string) error { return work.Err() })
	})
	if err != nil {
		return nil, err
	}
	raw, ok := generated.(string)
	if !ok {
		return generated, nil
	}
	action, err := ParseNeedleRouterOutput(raw)
	if err != nil {
		return e.failure(err)
	}
	payload := map[string]any{"request": request, "router_output": boundedRouteOutput(raw), "action": routerActionPayload(action), "executed": false}
	expansion := ExpandRouterAction(action, request)
	if expansion == nil {
		payload["abstained"] = true
		return e.routeSuccess(payload, SuccessOptions{})
	}
	payload["expansion"] = expansion
	if !execute {
		return e.routeSuccess(payload, SuccessOptions{})
	}
	dispatch := e.dispatch
	if e.dispatchFn != nil {
		dispatch = e.dispatchFn
	}
	routed, dispatchErr := dispatch(ctx, expansion)
	if dispatchErr != nil {
		return nil, dispatchErr
	}
	adapt := projectRoutedEnvelope
	if e.adaptFn != nil {
		adapt = e.adaptFn
	}
	result, options, adaptErr := adapt(routed, routeProjection(expansion))
	if adaptErr != nil {
		return e.failure(adaptErr)
	}
	payload["executed"] = true
	payload["result"] = result
	return e.routeSuccess(payload, options)
}
func (e RouteEndpoint) routeSuccess(payload map[string]any, options SuccessOptions) (any, error) {
	return e.Jobs.Controls.Queue.SuccessEnvelope(payload, options)
}
func (e RouteEndpoint) dispatch(ctx context.Context, expansion map[string]any) (any, error) {
	tool, _ := expansion["tool"].(string)
	args, _ := expansion["args"].(map[string]any)
	switch tool {
	case "memory_search":
		if args["query_syntax"] == nil {
			args["query_syntax"] = "plain"
		}
		if limit, ok := args["limit"].(int); ok {
			args["limit"] = json.Number(strconv.Itoa(limit))
		}
		return e.Jobs.callStatusOrTool(ctx, tool, args)
	case "memory_status", "memory_read":
		return e.Jobs.callStatusOrTool(ctx, tool, args)
	case "memory_execute":
		if e.Execute == nil {
			return e.failure(errors.New("memory execute is not configured"))
		}
		return e.Execute.Call(ctx, args)
	default:
		return e.failure(errors.New("unsupported direct routed tool: " + tool))
	}
}
func (e RouteEndpoint) failure(err error) (any, error) {
	failure, _ := FailureEnvelope(&ChangeValidationError{err.Error()})
	return failure, nil
}
func boundedRouteOutput(value string) string {
	runes := []rune(value)
	if len(runes) <= 2000 {
		return value
	}
	return string(runes[:2000]) + "..."
}
func routerActionPayload(a RouterAction) map[string]any {
	out := map[string]any{"action": a.Action}
	switch a.Action {
	case "search_then_read":
		out["query"] = a.Query
		if a.SearchMode == "" {
			out["search_mode"] = nil
		} else {
			out["search_mode"] = a.SearchMode
		}
	case "search_paths":
		out["query"], out["limit"], out["search_mode"] = a.Query, a.Limit, a.SearchMode
	case "status_field":
		out["field"] = a.Field
	case "search_then_graph":
		out["query"], out["depth"], out["search_mode"] = a.Query, a.Depth, a.SearchMode
	case "read_field":
		out["id_or_path"], out["field"] = a.IDOrPath, a.Field
	}
	return out
}
func (e RouteEndpoint) Register(server *umcp.Server) error {
	return e.register(server, catalogToolDefinitions)
}
func (e RouteEndpoint) register(server *umcp.Server, source []byte) error {
	if server == nil {
		return errors.New("route endpoint requires a uMCP server")
	}
	var definitions []proposalToolDefinition
	if err := json.Unmarshal(source, &definitions); err != nil {
		return err
	}
	for _, definition := range definitions {
		if definition.Name == "memory_route" {
			raw, _ := json.Marshal([]proposalToolDefinition{definition})
			return registerProposalTools(server, func(ctx context.Context, _ string, args map[string]any) (any, error) { return e.Call(ctx, args) }, nil, raw)
		}
	}
	return errors.New("route tool definition is unavailable")
}
