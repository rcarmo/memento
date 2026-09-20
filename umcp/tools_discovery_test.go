package umcp

import (
	"context"
	"testing"
)

func discoveryTool(name string) Tool {
	return Tool{Name: name, Call: func(context.Context, map[string]any) (any, error) { return name, nil }}
}
func TestExplicitToolDiscoveryOrder(t *testing.T) {
	r := ToolRegistry{}
	for _, name := range []string{"hidden", "second", "first"} {
		_ = r.Register(discoveryTool(name))
	}
	if err := r.SetDiscoveryOrder([]string{"first", "second"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDiscoveryOrder([]string{"first", "first"}); err == nil {
		t.Fatal("duplicate")
	}
	if err := r.SetDiscoveryOrder([]string{"missing"}); err == nil {
		t.Fatal("missing")
	}
	if err := r.SetDiscoveryOrder([]string{"first", "second"}); err != nil {
		t.Fatal(err)
	}
	_ = r.Register(discoveryTool("later"))
	_ = r.Register(discoveryTool("first"))
	r.Visible = func(_ context.Context, tool Tool) bool { return tool.Name != "second" }
	result, rpcErr, err := r.List(context.Background(), nil)
	if err != nil || rpcErr != nil {
		t.Fatal(result, rpcErr, err)
	}
	tools := result.(map[string]any)["tools"].([]any)
	if len(tools) != 2 || tools[0].(map[string]any)["name"] != "first" || tools[1].(map[string]any)["name"] != "later" {
		t.Fatal(tools)
	}
	if output, rpcErr, err := r.Call(context.Background(), map[string]any{"name": "hidden"}); err != nil || rpcErr != nil || output == nil {
		t.Fatal(output, rpcErr, err)
	}
	if !r.Unregister("first") || r.Unregister("first") {
		t.Fatal("unregister")
	}
	result, _, _ = r.List(context.Background(), nil)
	tools = result.(map[string]any)["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "later" {
		t.Fatal(tools)
	}
}
