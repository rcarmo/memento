package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/execute"
	"github.com/rcarmo/memento/umcp"
)

func modelHandlers() map[string]CatalogHandler {
	h := map[string]CatalogHandler{}
	for _, n := range []string{"memory_answer", "memory_route", "memory_propose_freeform", "memory_propose_update"} {
		n := n
		h[n] = func(context.Context, map[string]any) (any, error) {
			return map[string]any{"status": "success", "data": map[string]any{"handler": n}, "warnings": []any{}, "next_tools": []any{}}, nil
		}
	}
	return h
}
func TestConfiguredServerComposition(t *testing.T) {
	j, _ := jobsTest(t)
	j.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	for _, surface := range []string{"compact", "standard", "read_only", "curator", "admin"} {
		catalog, _ := NewCatalog(CatalogConfig{Surface: surface, AnswerEnabled: true, RouteEnabled: true})
		server := umcp.NewServer("configured")
		err := j.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: CatalogConfig{Surface: surface, AnswerEnabled: true, RouteEnabled: true}, Limits: endpointLimits(), ModelHandlers: modelHandlers()})
		if err != nil {
			t.Fatal(surface, err)
		}
		r, e := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "actor"})
		if e != nil || r.Error != nil || len(r.Result.(map[string]any)["tools"].([]any)) != len(catalog.ToolNames()) {
			t.Fatal(surface, r, e)
		}
	}
}
func TestConfiguredServerAccessComposition(t *testing.T) {
	ctx := context.Background()
	j, _ := jobsTest(t)
	j.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	store, _ := access.OpenStore(ctx, j.Controls.Queue.Proposals.DB, "synthetic")
	_, _, _ = store.Create(ctx, "bootstrap", "admin", []string{"admin"}, []string{"/"}, []string{"/"}, nil)
	j.Identity.managed = store
	j.Identity.names["admin"] = access.Principal{Name: "admin", Roles: []string{"admin"}}
	server := umcp.NewServer("configured")
	if err := j.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard"}, Limits: endpointLimits(), ModelHandlers: modelHandlers(), ManagedAccess: true}); err != nil {
		t.Fatal(err)
	}
	r, e := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "admin"})
	if e != nil || r.Error != nil {
		t.Fatal(r, e)
	}
	accessTools := 0
	for _, v := range r.Result.(map[string]any)["tools"].([]any) {
		if strings.HasPrefix(v.(map[string]any)["name"].(string), "access_") {
			accessTools++
		}
	}
	if accessTools != 10 {
		t.Fatal(accessTools)
	}
}
func TestConfiguredServerDispatch(t *testing.T) {
	j, _ := jobsTest(t)
	j.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	called := ""
	handlers := modelHandlers()
	handlers["memory_answer"] = func(context.Context, map[string]any) (any, error) {
		called = "answer"
		return map[string]any{"status": "success", "data": map[string]any{}, "warnings": []any{}, "next_tools": []any{}}, nil
	}
	server := umcp.NewServer("configured")
	if err := j.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard", AnswerEnabled: true}, Limits: endpointLimits(), ModelHandlers: handlers}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memory_answer", "memory_execute", "memory_help", "memory_status", "memory_list"} {
		body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `","arguments":`)
		switch name {
		case "memory_execute":
			body = append(body, []byte(`{"operations":[]}}}`)...)
		case "memory_answer":
			body = append(body, []byte(`{"question":"test"}}}`)...)
		default:
			body = append(body, []byte(`{}}}`)...)
		}
		response, err := server.Process(context.Background(), body, umcp.RequestContext{Principal: "actor"})
		if err != nil {
			t.Fatal(name, response, err)
		}
		if name != "memory_list" && response.Error != nil {
			t.Fatal(name, response.Error)
		}
	}
	if called != "answer" {
		t.Fatal(called)
	}
}

func TestConfiguredServerGuards(t *testing.T) {
	j, _ := jobsTest(t)
	if j.RegisterConfiguredServer(nil, ConfiguredServerOptions{}) == nil {
		t.Fatal("server")
	}
	if j.RegisterConfiguredServer(umcp.NewServer("x"), ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "bad"}}) == nil {
		t.Fatal("catalog")
	}
	if j.RegisterConfiguredServer(umcp.NewServer("x"), ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard"}, Limits: endpointLimits()}) == nil {
		t.Fatal("models")
	}
	oldFactory := executeFactory
	executeFactory = func(execute.Limits) (*execute.Factory, error) { return nil, errors.New("endpoint") }
	if j.RegisterConfiguredServer(umcp.NewServer("x"), ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard"}, Limits: endpointLimits()}) == nil {
		t.Fatal("endpoint")
	}
	executeFactory = oldFactory
	if j.RegisterConfiguredServer(umcp.NewServer("x"), ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard"}, Limits: endpointLimits(), ModelHandlers: modelHandlers(), ManagedAccess: true}) == nil {
		t.Fatal("access")
	}
}
