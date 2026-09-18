package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/umcp"
)

type accessReaderStub struct{ listErr, auditErr, policyErr error }

func (s accessReaderStub) Authenticate(context.Context, string) (*access.Principal, error) {
	return nil, nil
}
func (s accessReaderStub) Policy(_ context.Context, name string) (*access.NamespacePolicy, error) {
	if s.policyErr != nil {
		return nil, s.policyErr
	}
	if name == "admin" {
		return &access.NamespacePolicy{Roles: []string{"admin"}, ReadPrefixes: []string{"/"}}, nil
	}
	return nil, nil
}
func (s accessReaderStub) List(context.Context) ([]access.ManagedPrincipal, error) {
	return nil, s.listErr
}
func (s accessReaderStub) Audit(context.Context, int) ([]access.AuditEntry, error) {
	return nil, s.auditErr
}
func (s accessReaderStub) Create(context.Context, string, string, []string, []string, []string, *string) (access.ManagedPrincipal, string, error) {
	return access.ManagedPrincipal{}, "", errors.New("create")
}
func (s accessReaderStub) Update(context.Context, string, string, []string, []string, []string) (access.ManagedPrincipal, error) {
	return access.ManagedPrincipal{}, errors.New("update")
}
func (s accessReaderStub) Rename(context.Context, string, string, string) (access.ManagedPrincipal, error) {
	return access.ManagedPrincipal{}, errors.New("rename")
}
func (s accessReaderStub) SetEnabled(context.Context, string, string, bool) (access.ManagedPrincipal, error) {
	return access.ManagedPrincipal{}, errors.New("enabled")
}
func (s accessReaderStub) Rotate(context.Context, string, string, *string) (string, error) {
	return "", errors.New("rotate")
}
func (s accessReaderStub) Revoke(context.Context, string, string) (access.ManagedPrincipal, error) {
	return access.ManagedPrincipal{}, errors.New("revoke")
}
func (s accessReaderStub) Delete(context.Context, string, string) (access.ManagedPrincipal, error) {
	return access.ManagedPrincipal{}, errors.New("delete")
}

func TestAccessReadToolBranches(t *testing.T) {
	jobs, _ := jobsTest(t)
	server := umcp.NewServer("access")
	jobs.Identity.managed = accessReaderStub{}
	if err := jobs.registerAccessReadTools(server, []byte("{")); err == nil {
		t.Fatal("invalid metadata")
	}
	if err := jobs.registerAccessTools(server, []byte(`[]`), 1); err == nil {
		t.Fatal("incomplete metadata")
	}
	server.Tools.Visible = func(context.Context, umcp.Tool) bool { return false }
	if err := jobs.RegisterAccessReadTools(server); err != nil {
		t.Fatal(err)
	}
	result, rpcErr, err := server.Tools.List(context.Background(), nil)
	if err != nil || rpcErr != nil || len(result.(map[string]any)["tools"].([]any)) != 0 {
		t.Fatal(result, rpcErr, err)
	}
	jobs, _ = jobsTest(t)
	jobs.Identity.managed = accessReaderStub{}
	server = umcp.NewServer("access")
	_ = server.Tools.Register(umcp.Tool{Name: "public", Call: func(context.Context, map[string]any) (any, error) { return "ok", nil }})
	server.Tools.Visible = func(context.Context, umcp.Tool) bool { return true }
	if err = jobs.RegisterAccessReadTools(server); err != nil {
		t.Fatal(err)
	}
	result, rpcErr, err = server.Tools.List(context.Background(), nil)
	if err != nil || rpcErr != nil || len(result.(map[string]any)["tools"].([]any)) != 1 {
		t.Fatal(result, rpcErr, err)
	}

	for _, tc := range []struct {
		name    string
		managed accessReaderStub
	}{{"access_principal_list", accessReaderStub{listErr: errors.New("list")}}, {"access_audit_list", accessReaderStub{auditErr: errors.New("audit")}}, {"access_principal_list", accessReaderStub{policyErr: errors.New("policy")}}} {
		jobs, _ = jobsTest(t)
		jobs.Identity.managed = tc.managed
		server = umcp.NewServer("access")
		if err = jobs.RegisterAccessReadTools(server); err != nil {
			t.Fatal(err)
		}
		body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tc.name + `"}}`)
		response, processErr := server.Process(context.Background(), body, umcp.RequestContext{Principal: "admin"})
		if processErr != nil || response.Error == nil {
			t.Fatal(tc.name, response, processErr)
		}
	}
	jobs, _ = jobsTest(t)
	jobs.Identity.managed = accessReaderStub{}
	server = umcp.NewServer("access")
	if err = jobs.RegisterAccessReadTools(server); err != nil {
		t.Fatal(err)
	}
	response, processErr := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"access_audit_list","arguments":{"limit":{}}}}`), umcp.RequestContext{Principal: "admin"})
	if processErr != nil || response.Error == nil {
		t.Fatal(response, processErr)
	}
	if _, err = jobs.accessAdmin(context.Background()); err == nil {
		t.Fatal("missing trusted identity")
	}
	response, processErr = server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"access_principal_list"}}`), umcp.RequestContext{Principal: "actor"})
	if processErr != nil || response.Error == nil {
		t.Fatal(response, processErr)
	}

	for _, value := range []any{7, json.Number("8"), json.Number("1.2"), "bad"} {
		n, err := integerArgument(value)
		if (value == 7 && n != 7) || (value == json.Number("8") && n != 8) || ((value == json.Number("1.2") || value == "bad") && err == nil) {
			t.Fatal(value, n, err)
		}
	}
}
