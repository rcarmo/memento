package service

import (
	"context"
	"encoding/json"
	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/umcp"
	"testing"
)

func TestAccessMutationLifecycle(t *testing.T) {
	ctx := context.Background()
	j, _ := jobsTest(t)
	store, err := access.OpenStore(ctx, j.Controls.Queue.Proposals.DB, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Create(ctx, "bootstrap", "admin", []string{"admin"}, []string{"/"}, []string{"/"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	j.Identity.managed = store
	j.Identity.names["admin"] = access.Principal{Name: "admin", Roles: []string{"admin"}}
	s := umcp.NewServer("access")
	if err = j.RegisterAccessTools(s); err != nil {
		t.Fatal(err)
	}
	listed, processErr := s.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "admin"})
	if processErr != nil || listed.Error != nil || len(listed.Result.(map[string]any)["tools"].([]any)) != 10 {
		t.Fatal(listed, processErr)
	}
	call := func(name string, args map[string]any) map[string]any {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		r, e := s.Process(ctx, raw, umcp.RequestContext{Principal: "admin"})
		if e != nil || r.Error != nil {
			t.Fatal(name, r.Error, e)
		}
		return r.Result.(map[string]any)
	}
	base := map[string]any{"name": "agent", "roles": []any{"reader"}, "read_prefixes": []any{"/skills/"}, "write_prefixes": []any{}, "idempotency_key": "create-1"}
	call("access_principal_create", base)
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "access_principal_create", "arguments": base}})
	replay, processErr := s.Process(ctx, raw, umcp.RequestContext{Principal: "admin"})
	if processErr != nil || replay.Error == nil {
		t.Fatal("create credential replayed", replay, processErr)
	}
	call("access_principal_update", map[string]any{"name": "agent", "roles": []any{"proposer"}, "read_prefixes": []any{"/skills/"}, "write_prefixes": []any{"/skills/"}})
	call("access_principal_rename", map[string]any{"name": "agent", "new_name": "worker"})
	call("access_principal_disable", map[string]any{"name": "worker"})
	call("access_principal_enable", map[string]any{"name": "worker"})
	rot := map[string]any{"name": "worker", "idempotency_key": "rotate-1"}
	call("access_credential_rotate", rot)
	raw, _ = json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "access_credential_rotate", "arguments": rot}})
	replay, processErr = s.Process(ctx, raw, umcp.RequestContext{Principal: "admin"})
	if processErr != nil || replay.Error == nil {
		t.Fatal("rotate credential replayed", replay, processErr)
	}
	call("access_principal_revoke", map[string]any{"name": "worker"})
	call("access_principal_disable", map[string]any{"name": "worker"})
	call("access_principal_delete", map[string]any{"name": "worker"})
	events, err := store.Audit(ctx, 100)
	if err != nil || len(events) < 8 {
		t.Fatal(len(events), err)
	}
	for _, event := range events {
		if event.Target == "admin" {
			continue
		}
		if event.Actor != "admin" {
			t.Fatal(event)
		}
	}
}
func TestAccessCreateMissingFields(t *testing.T) {
	j, _ := jobsTest(t)
	store, _ := access.OpenStore(context.Background(), j.Controls.Queue.Proposals.DB, "synthetic")
	j.Identity.managed = store
	j.Identity.names["admin"] = access.Principal{Name: "admin", Roles: []string{"admin"}}
	s := umcp.NewServer("access")
	_ = j.RegisterAccessTools(s)
	r, e := s.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"access_principal_create","arguments":{}}}`), umcp.RequestContext{Principal: "admin"})
	if e != nil || r.Error == nil || r.Error.Code != -32602 {
		t.Fatal(r, e)
	}
}
