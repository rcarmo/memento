package service

import (
	"context"
	"encoding/json"
	"errors"
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
	Jobs      *Jobs
	Router    RouteInference
	Tokenizer *needle.Tokenizer
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
	if execute {
		return e.failure(errors.New("needle route execution is not implemented"))
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
	return e.Jobs.Call(ctx, "memory_route", func(ctx context.Context, c *ProposalControls, _ ProposalActor) (map[string]any, SuccessOptions, error) {
		raw, err := e.Router.Generate(e.Tokenizer, request, CanonicalShallowToolsJSON, needle.DefaultGenerationOptions(), func(string) error { return ctx.Err() })
		if err != nil {
			return nil, SuccessOptions{}, err
		}
		action, err := ParseNeedleRouterOutput(raw)
		if err != nil {
			return nil, SuccessOptions{}, &ChangeValidationError{err.Error()}
		}
		payload := map[string]any{"request": request, "router_output": boundedRouteOutput(raw), "action": routerActionPayload(action), "executed": false}
		expansion := ExpandRouterAction(action, request)
		if expansion == nil {
			payload["abstained"] = true
		} else {
			payload["expansion"] = expansion
		}
		return payload, SuccessOptions{}, nil
	})
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
