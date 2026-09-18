package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/umcp"
)

func endpointLimits() execute.Limits {
	return execute.Limits{MaxOperations: 12, MaxIntermediates: 12, MaxRecords: 50, MaxOutputBytes: json.Number("65536"), MaxTimeSeconds: 3}
}
func TestExecuteEndpointGuards(t *testing.T) {
	j, _ := jobsTest(t)
	catalog, _ := NewCatalog(CatalogConfig{Surface: "standard"})
	if _, err := NewExecuteEndpoint(nil, catalog, endpointLimits()); err == nil {
		t.Fatal("jobs")
	}
	if _, err := NewExecuteEndpoint(j, nil, endpointLimits()); err == nil {
		t.Fatal("catalog")
	}
	bad := *catalog
	bad.source.ExecuteCapable = nil
	if _, err := NewExecuteEndpoint(j, &bad, endpointLimits()); err == nil {
		t.Fatal("operations")
	}
	oldFactory, oldRaw, oldDefinition := executeFactory, executeRawTemplate, executeDefinition
	defer func() { executeFactory, executeRawTemplate, executeDefinition = oldFactory, oldRaw, oldDefinition }()
	executeFactory = func(execute.Limits) (*execute.Factory, error) { return nil, errors.New("factory") }
	if _, err := NewExecuteEndpoint(j, catalog, endpointLimits()); err == nil {
		t.Fatal("factory")
	}
	executeFactory = oldFactory
	executeRawTemplate = func(*Jobs, []string) (*RawExecuteTemplate, error) { return nil, errors.New("raw") }
	if _, err := NewExecuteEndpoint(j, catalog, endpointLimits()); err == nil {
		t.Fatal("raw")
	}
	executeRawTemplate = oldRaw
	executeDefinition = func([]byte) (proposalToolDefinition, error) {
		return proposalToolDefinition{}, errors.New("definition")
	}
	if _, err := NewExecuteEndpoint(j, catalog, endpointLimits()); err == nil {
		t.Fatal("definition")
	}
	executeDefinition = oldDefinition
	endpoint, err := NewExecuteEndpoint(j, catalog, endpointLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err = endpoint.Register(nil); err == nil {
		t.Fatal("server")
	}
	if _, err = executeToolDefinition([]byte("{")); err == nil {
		t.Fatal("json")
	}
	if _, err = executeToolDefinition([]byte("[]")); err == nil {
		t.Fatal("definition")
	}
}
func TestMarkExecutePending(t *testing.T) {
	markPendingReconciliation(false, true, map[string]any{})
	markPendingReconciliation(true, true, map[string]any{})

	value := map[string]any{"rows": []any{map[string]any{"operation": nil, "safe_to_retry": true}, map[string]any{"operation": nil, "safe_to_retry": false}}}
	markExecutePending(value)
	rows := value["rows"].([]any)
	if rows[0].(map[string]any)["final_state"] != "in_progress" || rows[1].(map[string]any)["final_state"] != nil {
		t.Fatal(value)
	}
}
func TestExecuteEndpointProtocol(t *testing.T) {
	j, _ := jobsTest(t)
	catalog, _ := NewCatalog(CatalogConfig{Surface: "standard"})
	endpoint, _ := NewExecuteEndpoint(j, catalog, endpointLimits())
	server := umcp.NewServer("execute")
	server.SetNotificationOutput(nil)
	if err := endpoint.Register(server); err != nil {
		t.Fatal(err)
	}
	call := func(args map[string]any, principal string) *umcp.Response {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "memory_execute", "arguments": args}})
		response, err := server.Process(context.Background(), raw, umcp.RequestContext{Principal: principal, Transport: "streamable-http"})
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	for _, args := range []map[string]any{{}, {"operations": []any{}}, {"operations": []any{map[string]any{"op": "unknown", "args": map[string]any{}}}}, {"operations": []any{map[string]any{"op": "read", "args": map[string]any{"id_or_path": "/missing"}}}}} {
		if response := call(args, "actor"); response.Error != nil {
			t.Fatal(response.Error)
		}
	}
	if call(map[string]any{"operations": []any{}}, "").Error == nil {
		t.Fatal("identity")
	}
	tiny := endpointLimits()
	tiny.MaxOperations = 1
	limited, _ := NewExecuteEndpoint(j, catalog, tiny)
	server2 := umcp.NewServer("limited")
	_ = limited.Register(server2)
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "memory_execute", "arguments": map[string]any{"operations": []any{map[string]any{"op": "read", "args": map[string]any{"id_or_path": "/a"}}, map[string]any{"op": "read", "args": map[string]any{"id_or_path": "/b"}}}}}})
	response, err := server2.Process(context.Background(), raw, umcp.RequestContext{Principal: "actor"})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	oldDir := j.Controls.Queue.Paths.BareDir
	j.Controls.Queue.Paths.BareDir = "missing"
	response = call(map[string]any{"operations": []any{}}, "actor")
	j.Controls.Queue.Paths.BareDir = oldDir
	if response.Error == nil {
		t.Fatal("revision failure")
	}
}
