package umcp

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"reflect"
	"sync"
	"testing"
)

func registeredTools(t *testing.T) *ToolRegistry {
	t.Helper()
	r := &ToolRegistry{}
	input := map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
	value := func(_ context.Context, args map[string]any) (any, error) { return args["value"], nil }
	tools := []Tool{
		{Name: "echo", OutputSchema: map[string]any{"type": "string"}, Parameters: []Parameter{{Name: "text", Types: []ParamType{StringParam}}}, Call: func(_ context.Context, args map[string]any) (any, error) { return args["text"], nil }},
		{Name: "add", OutputSchema: map[string]any{"type": "integer"}, Parameters: []Parameter{{Name: "a", Types: []ParamType{IntegerParam}}, {Name: "b", Types: []ParamType{IntegerParam}, HasDefault: true, Default: json.Number("1")}}, Call: func(_ context.Context, args map[string]any) (any, error) {
			a, ok := args["a"].(json.Number)
			if !ok {
				return nil, ExecutionError{"TypeError", "can only concatenate str (not \"int\") to str"}
			}
			b := args["b"].(json.Number)
			x, _ := new(big.Int).SetString(a.String(), 10)
			y, _ := new(big.Int).SetString(b.String(), 10)
			return json.Number(x.Add(x, y).String()), nil
		}},
		{Name: "boolean", OutputSchema: map[string]any{"type": "boolean"}, Parameters: []Parameter{{Name: "flag", Types: []ParamType{BooleanParam}, HasDefault: true, Default: false}}, Call: func(_ context.Context, args map[string]any) (any, error) { return args["flag"], nil }},
		{Name: "union", Parameters: []Parameter{{Name: "value", Types: []ParamType{IntegerParam, StringParam}}}, Call: value},
		{Name: "numeric", Parameters: []Parameter{{Name: "value", Types: []ParamType{IntegerParam, NumberParam}}}, Call: value},
		{Name: "ping", OutputSchema: map[string]any{"type": "string"}, Call: func(context.Context, map[string]any) (any, error) { return "pong", nil }},
		{Name: "structured", OutputSchema: map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer"}}, Call: func(context.Context, map[string]any) (any, error) {
			return OrderedObject{{"z", json.Number("1")}, {"a", json.Number("2")}}, nil
		}},
		{Name: "invalid", OutputSchema: map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer"}}, Call: func(context.Context, map[string]any) (any, error) { return OrderedObject{{"z", "wrong"}}, nil }},
		{Name: "argument", Call: func(context.Context, map[string]any) (any, error) { return nil, ArgumentError("bad argument") }},
		{Name: "failure", Call: func(context.Context, map[string]any) (any, error) {
			return nil, ExecutionError{"RuntimeError", "private details"}
		}},
		{Name: "cancelled", Call: func(context.Context, map[string]any) (any, error) { return nil, ErrRequestCancelled }},
	}
	for _, tool := range tools {
		tool.InputSchema = input
		if err := r.Register(tool); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestSyncAsyncToolParity(t *testing.T) {
	data, err := os.ReadFile("../testdata/parity/umcp-tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Request, Transport string
			Expected           json.RawMessage
		}
		Tools       json.RawMessage
		Annotations []struct {
			Name     string
			Expected map[string]any
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	r := registeredTools(t)
	d := Dispatcher{Handlers: map[string]Handler{"tools/call": r.Call, "tools/list": r.List}}
	for _, c := range fixture.Cases {
		response, err := d.Process(context.Background(), []byte(c.Request), RequestContext{Transport: c.Transport, Principal: "reader"})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := decode(raw)
		want, _ := decode(c.Expected)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s %s\ngot %s\nwant %s", c.Transport, c.Request, raw, c.Expected)
		}
	}
	listed, rpcErr, err := r.List(context.Background(), nil)
	if err != nil || rpcErr != nil {
		t.Fatal(err, rpcErr)
	}
	raw, _ := json.Marshal(listed)
	got, _ := decode(raw)
	want, _ := decode(fixture.Tools)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("list got %s\nwant %s", raw, fixture.Tools)
	}
	for _, c := range fixture.Annotations {
		if !reflect.DeepEqual(InferToolAnnotations(c.Name), c.Expected) {
			t.Fatal(c)
		}
	}
}

func TestRegistryLifecycleAndIsolation(t *testing.T) {
	var r ToolRegistry
	noop := func(context.Context, map[string]any) (any, error) { return nil, nil }
	if err := r.Register(Tool{}); err == nil {
		t.Fatal("nil handler")
	}
	notifications := 0
	r.Notify = func(method string, p map[string]any) error {
		if method != "notifications/tools/list_changed" || len(p) != 0 {
			t.Fatal(method, p)
		}
		notifications++
		return nil
	}
	tool := Tool{Name: "x", Description: "description", Annotations: map[string]any{}, Parameters: []Parameter{{Name: "a", HasDefault: true, Default: OrderedObject{{"x", []any{"value"}}}}}, InputSchema: map[string]any{"type": "object", "required": []any{"a"}}, Call: noop}
	if err := r.RegisterAndNotify(tool); err != nil {
		t.Fatal(err)
	}
	tool.InputSchema["type"] = "changed"
	tool.Parameters[0].Name = "changed"
	result, _, _ := r.List(context.Background(), nil)
	first := result.(map[string]any)["tools"].([]any)[0].(map[string]any)
	if first["inputSchema"].(map[string]any)["type"] != "object" {
		t.Fatal("metadata aliased")
	}
	first["inputSchema"].(map[string]any)["type"] = "modified"
	result, _, _ = r.List(context.Background(), nil)
	if result.(map[string]any)["tools"].([]any)[0].(map[string]any)["inputSchema"].(map[string]any)["type"] != "object" {
		t.Fatal("list aliases stored metadata")
	}
	if ok, err := r.UnregisterAndNotify("x"); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, err := r.UnregisterAndNotify("missing"); ok || err != nil {
		t.Fatal(ok, err)
	}
	if notifications != 2 {
		t.Fatal(notifications)
	}
	boom := errors.New("notify failed")
	r.Notify = func(string, map[string]any) error { return boom }
	if err := r.RegisterAndNotify(Tool{Name: "x", Call: noop}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if ok, err := r.UnregisterAndNotify("x"); !ok || !errors.Is(err, boom) {
		t.Fatal(ok, err)
	}
	if err := r.RegisterAndNotify(Tool{}); err == nil {
		t.Fatal("registered missing handler")
	}
	r.Notify = nil
	if err := r.RegisterAndNotify(Tool{Name: "x", Call: noop}); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.UnregisterAndNotify("x"); !ok || err != nil {
		t.Fatal(ok, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Register(Tool{Name: "concurrent", Call: noop})
			_, _, _ = r.List(context.Background(), nil)
			r.Unregister("concurrent")
		}()
	}
	wg.Wait()
}

func TestCoercionAndCallErrors(t *testing.T) {
	r := registeredTools(t)
	for _, params := range []map[string]any{{"name": 1}, {"name": "ping", "arguments": nil}, {"name": "ping", "arguments": []any{}}} {
		if _, _, err := r.Call(context.Background(), params); err == nil {
			t.Fatal(params)
		}
	}
	_ = r.Register(Tool{Name: "generic-error", Call: func(context.Context, map[string]any) (any, error) { return nil, errors.New("detail") }})
	_, failure, _ := r.Call(context.Background(), map[string]any{"name": "generic-error"})
	if failure == nil || failure.Code != -32603 {
		t.Fatal(failure)
	}
	for _, c := range []struct {
		V     any
		Types []ParamType
		Want  any
	}{
		{nil, []ParamType{IntegerParam}, nil}, {"yes", []ParamType{BooleanParam}, true}, {"false", []ParamType{BooleanParam}, false}, {"٤٢", []ParamType{IntegerParam}, json.Number("42")}, {"1_000", []ParamType{IntegerParam}, json.Number("1000")}, {"+1.5", []ParamType{NumberParam}, json.Number("1.5")}, {"bad", []ParamType{IntegerParam}, "bad"}, {"bad", []ParamType{NumberParam}, "bad"}, {"x", nil, "x"}, {true, []ParamType{IntegerParam, StringParam}, true}, {json.Number("1.5"), []ParamType{NumberParam, StringParam}, json.Number("1.5")}, {true, []ParamType{BooleanParam, StringParam}, true}, {map[string]any{}, []ParamType{ObjectParam, StringParam}, map[string]any{}}, {[]any{}, []ParamType{ArrayParam, StringParam}, []any{}}, {"no", []ParamType{AnyParam, BooleanParam}, "no"}, {"x", []ParamType{"unknown"}, "x"},
	} {
		if got := coerce(c.V, c.Types); !reflect.DeepEqual(got, c.Want) {
			t.Fatal(c, got)
		}
	}
	if matchesType(1, "unknown") || matchesType(1, AnyParam) {
		t.Fatal("bad type match")
	}
}

func TestArgumentErrorText(t *testing.T) {
	if ArgumentError("bad").Error() != "bad" {
		t.Fatal("error message changed")
	}
}

func TestHandlerPanicIsRedacted(t *testing.T) {
	r := &ToolRegistry{}
	_ = r.Register(Tool{Name: "panic", Call: func(context.Context, map[string]any) (any, error) { panic("private secret") }})
	d := Dispatcher{Handlers: map[string]Handler{"tools/call": r.Call}}
	response, err := d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"panic"}}`), RequestContext{Transport: "streamable-http"})
	if err != nil || response.Error == nil || response.Error.Code != -32603 || response.Error.Message != "Tool execution failed" {
		t.Fatal(response, err)
	}
}

func TestToolVisibility(t *testing.T) {
	r := ToolRegistry{}
	for _, name := range []string{"public", "admin"} {
		name := name
		_ = r.Register(Tool{Name: name, Call: func(context.Context, map[string]any) (any, error) { return name, nil }})
	}
	r.Visible = func(ctx context.Context, tool Tool) bool {
		name := tool.Name
		tool.Name = "mutated"
		if tool.InputSchema != nil {
			tool.InputSchema["mutated"] = true
		}
		// Visibility callbacks execute outside the registry lock.
		if err := r.Register(Tool{Name: name, Call: func(context.Context, map[string]any) (any, error) { return name, nil }}); err != nil {
			t.Fatal(err)
		}
		return name != "admin" || Context(ctx).Principal == "admin"
	}
	d := Dispatcher{Handlers: map[string]Handler{"tools/list": r.List}}
	for _, tc := range []struct {
		principal string
		count     int
	}{{"reader", 1}, {"admin", 2}} {
		response, err := d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), RequestContext{Principal: tc.principal})
		tools := response.Result.(map[string]any)["tools"].([]any)
		if err != nil || response.Error != nil || len(tools) != tc.count {
			t.Fatal(tc, response, err)
		}
		for _, raw := range tools {
			metadata := raw.(map[string]any)
			if metadata["name"] == "mutated" || metadata["inputSchema"].(map[string]any)["mutated"] != nil {
				t.Fatal("visibility callback mutated stored discovery metadata", metadata)
			}
		}
	}
}
