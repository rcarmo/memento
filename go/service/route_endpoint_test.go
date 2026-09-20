package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/umcp"
)

type routeInferenceStub struct {
	output       string
	err          error
	query, tools string
}

func (s *routeInferenceStub) Generate(ctx context.Context, query, tools string, _ needle.GenerationOptions) (string, error) {
	s.query, s.tools = query, tools
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return s.output, s.err
}
func routeEndpointTest(t *testing.T, output string) (RouteEndpoint, *routeInferenceStub) {
	t.Helper()
	jobs, _ := jobsTest(t)
	jobs.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	stub := &routeInferenceStub{output: output}
	return RouteEndpoint{Jobs: jobs, Router: stub}, stub
}
func routeCall(t *testing.T, endpoint RouteEndpoint, args map[string]any) (*umcp.Response, error) {
	t.Helper()
	server := umcp.NewServer("route")
	if err := endpoint.Register(server); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "memory_route", "arguments": args}})
	return server.Process(context.Background(), raw, umcp.RequestContext{Principal: "actor"})
}
func TestRouteEndpointPreview(t *testing.T) {
	endpoint, stub := routeEndpointTest(t, `[{"name":"search_paths","arguments":{"query":"Piclaw","limit":3}}]`)
	response, err := routeCall(t, endpoint, map[string]any{"request": " Find Piclaw ", "execute": false})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	if stub.query != "Find Piclaw" || stub.tools != CanonicalShallowToolsJSON {
		t.Fatal(stub.query, stub.tools)
	}
	if err = endpoint.Register(nil); err == nil {
		t.Fatal("nil")
	}
	if err = endpoint.register(umcp.NewServer("bad"), []byte("bad")); err == nil {
		t.Fatal("decode")
	}
	if err = endpoint.register(umcp.NewServer("missing"), []byte("[]")); err == nil {
		t.Fatal("missing")
	}
}
func TestRouteEndpointDirectExecution(t *testing.T) {
	cases := []struct{ output, request string }{{`[{"name":"search_paths","arguments":{"query":"Synthetic","limit":1}}]`, "find Synthetic"}, {`[{"name":"status_field","arguments":{"field":"principal"}}]`, "status"}, {`[{"name":"read_field","arguments":{"id_or_path":"/public/a.md","field":"title"}}]`, "show /public/a.md title"}}
	for _, tc := range cases {
		endpoint, _ := routeEndpointTest(t, tc.output)
		response, err := routeCall(t, endpoint, map[string]any{"request": tc.request, "execute": true})
		if err != nil || response.Error != nil {
			t.Fatal(tc, response.Error, err)
		}
	}
}
func TestRouteEndpointExecutePlan(t *testing.T) {
	endpoint, _ := routeEndpointTest(t, `[{"name":"search_then_read","arguments":{"query":"Synthetic"}}]`)
	catalog, err := NewCatalog(CatalogConfig{Surface: "standard"})
	if err != nil {
		t.Fatal(err)
	}
	execute, err := NewExecuteEndpoint(endpoint.Jobs, catalog, endpointLimits())
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Execute = execute
	response, err := routeCall(t, endpoint, map[string]any{"request": "find Synthetic", "execute": true})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	endpoint.Execute = nil
	response, err = routeCall(t, endpoint, map[string]any{"request": "find Synthetic", "execute": true})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
}
func TestRouteEndpointFailures(t *testing.T) {
	endpoint, _ := routeEndpointTest(t, `[{"name":"UNKNOWN","arguments":{}}]`)
	if response, err := routeCall(t, endpoint, map[string]any{"request": "unknown", "execute": false}); err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	for _, args := range []map[string]any{{}, {"request": 1}, {"request": "x", "execute": 1}, {"request": "x"}, {"request": " ", "execute": false}, {"request": strings.Repeat("x", 201), "execute": false}} {
		response, err := routeCall(t, endpoint, args)
		if err != nil || response.Error != nil {
			continue
		}
		if response.Result == nil {
			t.Fatal(args, response)
		}
	}
	endpoint.Router = nil
	if response, err := routeCall(t, endpoint, map[string]any{"request": "x", "execute": false}); err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	endpoint, stub := routeEndpointTest(t, "bad")
	if response, err := routeCall(t, endpoint, map[string]any{"request": "x", "execute": false}); err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	stub.output = strings.Repeat("x", 2001)
	if got := boundedRouteOutput(stub.output); len([]rune(got)) != 2003 {
		t.Fatal(len(got))
	}
	if boundedRouteOutput("x") != "x" {
		t.Fatal("short")
	}
	stub.err = errors.New("boom")
	response, err := routeCall(t, endpoint, map[string]any{"request": "x", "execute": false})
	if err == nil && (response == nil || response.Error == nil) {
		t.Fatal("inference", response)
	}
}
func TestRouteEndpointPostInferenceFailures(t *testing.T) {
	boom := errors.New("boom")
	for _, kind := range []string{"dispatch", "adapt"} {
		endpoint, _ := routeEndpointTest(t, `[{"name":"search_paths","arguments":{"query":"x"}}]`)
		if kind == "dispatch" {
			endpoint.dispatchFn = func(context.Context, map[string]any) (any, error) { return nil, boom }
		} else {
			endpoint.dispatchFn = func(context.Context, map[string]any) (any, error) { return map[string]any{}, nil }
			endpoint.adaptFn = func(any, *RouteProjection) (map[string]any, SuccessOptions, error) {
				return nil, SuccessOptions{}, boom
			}
		}
		response, err := routeCall(t, endpoint, map[string]any{"request": "x", "execute": true})
		if kind == "dispatch" {
			if err == nil && (response == nil || response.Error == nil) {
				t.Fatal(kind, response)
			}
		} else if err != nil || response.Error != nil {
			t.Fatal(kind, response, err)
		}
	}
}

func TestRouteEndpointInternalBranches(t *testing.T) {
	endpoint, _ := routeEndpointTest(t, `[{"name":"UNKNOWN","arguments":{}}]`)
	if _, err := endpoint.Call(context.Background(), map[string]any{"request": "x", "execute": false}); err == nil {
		t.Fatal("identity")
	}
	if value, err := endpoint.dispatch(context.Background(), map[string]any{"tool": "bad", "args": map[string]any{}}); err != nil || value == nil {
		t.Fatal(value, err)
	}
	if value, err := endpoint.dispatch(context.Background(), map[string]any{"tool": "memory_execute", "args": map[string]any{}}); err != nil || value == nil {
		t.Fatal(value, err)
	}
	if _, _, err := projectRoutedEnvelope(map[string]any{"status": "success", "data": map[string]any{}}, &RouteProjection{Ref: "missing"}); err == nil {
		t.Fatal("projection")
	}
	endpoint.Jobs.Workers.SetExecuteBusy(true)
	defer endpoint.Jobs.Workers.SetExecuteBusy(false)
	server := umcp.NewServer("busy")
	if err := endpoint.Register(server); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_route","arguments":{"request":"x","execute":false}}}`)
	response, err := server.Process(context.Background(), raw, umcp.RequestContext{Principal: "actor"})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
}

func TestRouterActionPayloads(t *testing.T) {
	for _, raw := range []string{`[{"name":"search_then_read","arguments":{"query":"x"}}]`, `[{"name":"search_then_read","arguments":{"query":"x","search_mode":"semantic"}}]`, `[{"name":"search_paths","arguments":{"query":"x"}}]`, `[{"name":"status_field","arguments":{"field":"principal"}}]`, `[{"name":"search_then_graph","arguments":{"query":"x"}}]`, `[{"name":"read_field","arguments":{"id_or_path":"x","field":"body"}}]`, `[{"name":"UNKNOWN","arguments":{}}]`} {
		action, err := ParseNeedleRouterOutput(raw)
		if err != nil {
			t.Fatal(err)
		}
		if routerActionPayload(action)["action"] != action.Action {
			t.Fatal(action)
		}
	}
}
