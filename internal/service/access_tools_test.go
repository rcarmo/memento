package service

import (
	"context"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

func TestAccessReadToolsDiscoveryAndCalls(t *testing.T) {
	ctx := context.Background()
	jobs, _ := jobsTest(t)
	store, err := access.OpenStore(ctx, jobs.Controls.Queue.Proposals.DB, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := store.Create(ctx, "bootstrap", "managed-admin", []string{"admin"}, []string{"/"}, []string{"/"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobs.Identity, err = NewIdentity([]BearerPrincipal{{Token: token, Principal: access.Principal{Name: "managed-admin", Roles: []string{"admin"}}}}, access.AuthorizationConfig{}, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("access")
	if err = jobs.RegisterAccessReadTools(server); err != nil {
		t.Fatal(err)
	}
	call := func(principal, body string) *umcp.Response {
		response, processErr := server.Process(ctx, []byte(body), umcp.RequestContext{Principal: principal})
		if processErr != nil {
			t.Fatal(processErr)
		}
		return response
	}
	reader := call("reader", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	admin := call("managed-admin", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if len(reader.Result.(map[string]any)["tools"].([]any)) != 0 || len(admin.Result.(map[string]any)["tools"].([]any)) != 2 {
		t.Fatal(reader, admin)
	}
	for _, body := range []string{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"access_principal_list","arguments":{}}}`, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"access_audit_list","arguments":{}}}`} {
		if response := call("managed-admin", body); response.Error != nil {
			t.Fatal(response.Error)
		}
		if response := call("reader", body); response.Error == nil {
			t.Fatal("non-admin call succeeded")
		}
	}
}

func TestAccessReadToolsRequireConfiguration(t *testing.T) {
	jobs, _ := jobsTest(t)
	if err := jobs.RegisterAccessReadTools(nil); err == nil {
		t.Fatal("nil server")
	}
	if err := jobs.RegisterAccessReadTools(umcp.NewServer("access")); err == nil {
		t.Fatal("missing store")
	}
}
