package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/umcp"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func schemaValid(t *testing.T, name string, value any) {
	t.Helper()
	ordered, err := MCPEnvelope(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = umcp.ValidateSchemaSubset(ordered, toolOutputSchema(name), "$"); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	// Independent full JSON Schema validation protects external consumers from
	// mistakes hidden by the deliberately small uMCP runtime validator.
	compiler := jsonschema.NewCompiler()
	url := "https://memento.test/schema/" + name
	if err = compiler.AddResource(url, jsonNormal(toolOutputSchema(name))); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatal(err)
	}
	if err = schema.Validate(jsonNormal(value)); err != nil {
		t.Fatalf("independent schema %s: %v", name, err)
	}
}

func TestStableOutputSchemas(t *testing.T) {
	if len(stableOutputSchemas) != 5 {
		t.Fatal("review stable tool scope")
	}
	for name, schema := range stableOutputSchemas {
		if schema["type"] != "object" || len(schema["oneOf"].([]any)) != 2 {
			t.Fatal(name, schema)
		}
		failure, err := FailureEnvelope(&Error{Class: "validation_error", Message: "invalid request"})
		if err != nil {
			t.Fatal(err)
		}
		schemaValid(t, name, failure)
		if _, err := umcp.FormatToolResult(umcp.OrderedObject{{Name: "status", Value: "success"}}, schema); err == nil {
			t.Fatal("incomplete envelope accepted", name)
		}
	}
	if toolOutputSchema("memory_execute") != nil || toolOutputSchema("memory_route") != nil {
		t.Fatal("dynamic output must remain schema-less")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("malformed embedded schema accepted")
		}
	}()
	loadOutputSchemas([]byte(`{`))
}

func TestStableOutputSchemasLiveRuntime(t *testing.T) {
	for _, surface := range []string{"compact", "standard", "read_only", "curator", "admin"} {
		t.Run(surface, func(t *testing.T) {
			_, server, _ := contractRuntime(t, surface, true, true)
			listed := contractResult(t, contractRPC(t, server, "actor", "tools/list", nil))["tools"].([]any)
			for _, raw := range listed {
				tool := raw.(map[string]any)
				name := tool["name"].(string)
				if s := toolOutputSchema(name); s != nil && !reflect.DeepEqual(tool["outputSchema"], jsonNormal(s)) {
					t.Fatal("discovery schema", name)
				}
			}
			calls := map[string]map[string]any{
				"memory_status": {}, "memory_help": {}, "memory_read": {"id_or_path": "/public/a.md"}, "memory_list": {}, "memory_search": {"query": "mutation", "limit": 2},
			}
			for name, args := range calls {
				result := contractTool(t, server, "actor", name, args)
				schemaValid(t, name, result)
			}
			// Missing read and malformed search remain normal domain errors, not
			// schema-induced transport failures. Model-off status nulls are valid.
			for _, q := range []struct {
				name string
				args map[string]any
			}{{"memory_read", map[string]any{"id_or_path": "/missing.md"}}, {"memory_search", map[string]any{"query": "alpha", "search_mode": "bad"}}} {
				value := contractResult(t, contractRPC(t, server, "actor", "tools/call", map[string]any{"name": q.name, "arguments": q.args}))["structuredContent"].(map[string]any)
				if value["status"] != "error" {
					t.Fatal(value)
				}
				schemaValid(t, q.name, value)
			}
			// Registry metadata is caller-owned; mutating a listed schema cannot
			// change future discovery or relax output validation.
			for _, raw := range listed {
				tool := raw.(map[string]any)
				if tool["name"] == "memory_status" {
					tool["outputSchema"].(map[string]any)["type"] = "string"
				}
			}
			fresh := contractResult(t, contractRPC(t, server, "actor", "tools/list", nil))["tools"].([]any)
			for _, raw := range fresh {
				tool := raw.(map[string]any)
				if tool["name"] == "memory_status" && tool["outputSchema"].(map[string]any)["type"] != "object" {
					t.Fatal("schema alias")
				}
			}
		})
	}
}

func TestOutputSchemaRejectsPayloadDrift(t *testing.T) {
	catalog, _ := NewCatalog(CatalogConfig{Surface: "compact"})
	for _, name := range []string{"memory_help", "memory_list", "memory_search"} {
		var data map[string]any
		switch name {
		case "memory_help":
			data = catalog.Help()
		case "memory_list":
			data = map[string]any{"entries": []any{}}
		default:
			data = map[string]any{"search_mode": "lexical", "query_syntax": "plain", "results": []any{}, "next_cursor": nil}
		}
		value := map[string]any{"status": "success", "data": data, "warnings": []string{}, "next_tools": []string{}, "repo_revision": "r", "index_revision": "r", "index_stale": false, "operation_id": nil}
		schemaValid(t, name, value)
		copy := jsonNormal(value).(map[string]any)
		copy["data"].(map[string]any)["unexpected_field"] = true
		ordered, _ := MCPEnvelope(copy)
		if umcp.ValidateSchemaSubset(ordered, toolOutputSchema(name), "$") == nil {
			t.Fatal("drift accepted", name)
		}
	}
	// Runtime callers returning an invalid output get the existing redacted
	// transport validation failure instead of emitting a misleading success.
	server := umcp.NewServer("output-drift")
	_ = server.Tools.Register(umcp.Tool{Name: "status", OutputSchema: toolOutputSchema("memory_status"), Call: func(context.Context, map[string]any) (any, error) {
		return umcp.OrderedObject{{Name: "status", Value: "success"}}, nil
	}})
	r, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"status","arguments":{}}}`), umcp.RequestContext{})
	if err != nil || r.Error == nil {
		t.Fatal(r, err)
	}
}

func FuzzStableOutputSchemas(f *testing.F) {
	f.Add("memory_status", []byte(`{"status":"error","error_class":"validation_error","message":"bad","warnings":[],"repo_revision":null,"index_revision":null,"index_stale":false,"operation_id":null}`))
	f.Add("memory_list", []byte(`{"status":"success"}`))
	f.Fuzz(func(t *testing.T, name string, raw []byte) {
		if len(raw) > 64<<10 {
			t.Skip()
		}
		s := toolOutputSchema(name)
		if s == nil {
			return
		}
		var v any
		if json.Unmarshal(raw, &v) != nil {
			return
		}
		ordered, err := MCPEnvelope(v)
		if err != nil {
			return
		}
		if err = umcp.ValidateSchemaSubset(ordered, s, "$"); err == nil {
			x, ok := v.(map[string]any)
			if !ok {
				t.Fatal(fmt.Sprint(v))
			}
			if x["status"] != "success" && x["status"] != "error" {
				t.Fatal(x)
			}
			for _, key := range []string{"warnings", "repo_revision", "index_revision", "index_stale", "operation_id"} {
				if _, ok := x[key]; !ok {
					t.Fatal(key)
				}
			}
		}
	})
}

func BenchmarkStableOutputCatalogRegistration(b *testing.B) {
	catalog, err := NewCatalog(CatalogConfig{Surface: "compact"})
	if err != nil {
		b.Fatal(err)
	}
	handlers := map[string]CatalogHandler{}
	for _, op := range catalog.source.Operations {
		handlers[op.Tool] = func(context.Context, map[string]any) (any, error) { return nil, nil }
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server := umcp.NewServer("registration-benchmark")
		server.SetNotificationOutput(nil)
		if err := catalog.Register(server, handlers, nil); err != nil {
			b.Fatal(err)
		}

	}
}
