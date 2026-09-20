package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/access"
)

type managedAccessStoreStub struct {
	managedAccessStore
	authenticate func(context.Context, string) (*access.Principal, error)
	list         func(context.Context) ([]access.ManagedPrincipal, error)
	audit        func(context.Context, int) ([]access.AuditEntry, error)
	create       func(context.Context, string, string, []string, []string, []string, *string) (access.ManagedPrincipal, string, error)
	update       func(context.Context, string, string, []string, []string, []string) (access.ManagedPrincipal, error)
	rename       func(context.Context, string, string, string) (access.ManagedPrincipal, error)
	setEnabled   func(context.Context, string, string, bool) (access.ManagedPrincipal, error)
	rotate       func(context.Context, string, string, *string) (string, error)
	revoke       func(context.Context, string, string) (access.ManagedPrincipal, error)
	delete       func(context.Context, string, string) (access.ManagedPrincipal, error)
}

func (s managedAccessStoreStub) Authenticate(ctx context.Context, token string) (*access.Principal, error) {
	if s.authenticate != nil {
		return s.authenticate(ctx, token)
	}
	return s.managedAccessStore.Authenticate(ctx, token)
}
func (s managedAccessStoreStub) Policy(ctx context.Context, name string) (*access.NamespacePolicy, error) {
	return s.managedAccessStore.Policy(ctx, name)
}
func (s managedAccessStoreStub) List(ctx context.Context) ([]access.ManagedPrincipal, error) {
	if s.list != nil {
		return s.list(ctx)
	}
	return s.managedAccessStore.List(ctx)
}
func (s managedAccessStoreStub) Audit(ctx context.Context, limit int) ([]access.AuditEntry, error) {
	if s.audit != nil {
		return s.audit(ctx, limit)
	}
	return s.managedAccessStore.Audit(ctx, limit)
}
func (s managedAccessStoreStub) Create(ctx context.Context, actor, name string, roles, reads, writes []string, key *string) (access.ManagedPrincipal, string, error) {
	if s.create != nil {
		return s.create(ctx, actor, name, roles, reads, writes, key)
	}
	return s.managedAccessStore.Create(ctx, actor, name, roles, reads, writes, key)
}
func (s managedAccessStoreStub) Update(ctx context.Context, actor, name string, roles, reads, writes []string) (access.ManagedPrincipal, error) {
	if s.update != nil {
		return s.update(ctx, actor, name, roles, reads, writes)
	}
	return s.managedAccessStore.Update(ctx, actor, name, roles, reads, writes)
}
func (s managedAccessStoreStub) Rename(ctx context.Context, actor, name, newName string) (access.ManagedPrincipal, error) {
	if s.rename != nil {
		return s.rename(ctx, actor, name, newName)
	}
	return s.managedAccessStore.Rename(ctx, actor, name, newName)
}
func (s managedAccessStoreStub) SetEnabled(ctx context.Context, actor, name string, enabled bool) (access.ManagedPrincipal, error) {
	if s.setEnabled != nil {
		return s.setEnabled(ctx, actor, name, enabled)
	}
	return s.managedAccessStore.SetEnabled(ctx, actor, name, enabled)
}
func (s managedAccessStoreStub) Rotate(ctx context.Context, actor, name string, key *string) (string, error) {
	if s.rotate != nil {
		return s.rotate(ctx, actor, name, key)
	}
	return s.managedAccessStore.Rotate(ctx, actor, name, key)
}
func (s managedAccessStoreStub) Revoke(ctx context.Context, actor, name string) (access.ManagedPrincipal, error) {
	if s.revoke != nil {
		return s.revoke(ctx, actor, name)
	}
	return s.managedAccessStore.Revoke(ctx, actor, name)
}
func (s managedAccessStoreStub) Delete(ctx context.Context, actor, name string) (access.ManagedPrincipal, error) {
	if s.delete != nil {
		return s.delete(ctx, actor, name)
	}
	return s.managedAccessStore.Delete(ctx, actor, name)
}

func TestAdminHelpersAndStaticBranches(t *testing.T) {
	if _, err := adminJSON(map[string]any{"bad": make(chan int)}, 200); err == nil {
		t.Fatal("adminJSON")
	}
	if response, err := adminCallError(nil); err != nil || response != nil {
		t.Fatal(response, err)
	}
	for _, err := range []error{adminAccessError("bad request"), &json.SyntaxError{Offset: 1}, &json.UnmarshalTypeError{Value: "object", Type: reflect.TypeOf(0)}} {
		response, callErr := adminCallError(err)
		if callErr != nil || response.Status != 400 {
			t.Fatal(err, response, callErr)
		}
	}
	if response, err := adminCallError(io.ErrClosedPipe); !errors.Is(err, io.ErrClosedPipe) || response != nil {
		t.Fatal(response, err)
	}
	if got := adminString(map[string]any{"name": 42}, "name"); got != "42" {
		t.Fatal(got)
	}
	if got := adminString(map[string]any{}, "missing"); got != "" {
		t.Fatal(got)
	}
	if values, err := adminStrings(map[string]any{}, "roles"); err != nil || values != nil {
		t.Fatal(values, err)
	}
	if _, err := adminStrings(map[string]any{"roles": "reader"}, "roles"); err == nil {
		t.Fatal("roles")
	}
	if _, err := adminStrings(map[string]any{"roles": []any{"reader", 1}}, "roles"); err == nil {
		t.Fatal("roles item")
	}
	if values, err := adminStrings(map[string]any{"roles": []any{"reader", "admin"}}, "roles"); err != nil || !reflect.DeepEqual(values, []string{"reader", "admin"}) {
		t.Fatal(values, err)
	}
	if object, err := adminObject(nil); err != nil || len(object) != 0 {
		t.Fatal(object, err)
	}
	if _, err := adminObject([]byte("{")); err == nil {
		t.Fatal("syntax")
	}
	if _, err := adminObject([]byte("[]")); err == nil {
		t.Fatal("object")
	}
	if principal, err := (AdminHTTP{}).authenticate(context.Background(), map[string]string{"authorization": "Bearer token"}); err != nil || principal != nil {
		t.Fatal(principal, err)
	}
	stub := managedAccessStoreStub{authenticate: func(context.Context, string) (*access.Principal, error) {
		return &access.Principal{Name: "root", Roles: []string{"admin"}}, nil
	}}
	if principal, err := (AdminHTTP{Store: stub}).authenticate(context.Background(), map[string]string{"authorization": "Bearer token"}); err != nil || principal == nil || principal.Name != "root" {
		t.Fatal(principal, err)
	}
	if response := adminStaticResponse(""); response.Status != 404 || !reflect.DeepEqual(response.Headers, adminHeaders) {
		t.Fatal(response)
	}
	for _, relative := range []string{"/index.html", "bad", "nested/../index.html", "nested//index.html", "./index.html"} {
		if response := adminStaticResponse(relative); response.Status != 404 {
			t.Fatal(relative, response)
		}
	}
	old := adminReadFile
	adminReadFile = func(fs.FS, string) ([]byte, error) { return nil, io.ErrClosedPipe }
	t.Cleanup(func() { adminReadFile = old })
	if response := adminStaticResponse("index.html"); response.Status != 404 {
		t.Fatal(response)
	}
}

func TestAdminHTTPFailures(t *testing.T) {
	_, store, adminToken, handler := adminFixture(t)
	response, payload, err := adminCall(t, AdminHTTP{}, "GET", "/admin/api/principals", "", nil)
	if err != nil || response.Status != 503 || payload["error"] != "access management is not configured" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "GET", "/admin/missing", "", nil)
	if err != nil || response.Status != 404 || payload["error"] != "not found" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "DELETE", "/admin/api/principals", adminToken, nil)
	if err != nil || response.Status != 404 || payload["error"] != "not found" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "GET", "/admin/api/principals", "", nil)
	if err != nil || response.Status != 401 || payload["error"] != "admin bearer credential required" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals", adminToken, []byte("{"))
	if err != nil || response.Status != 400 {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals", adminToken, []byte("[]"))
	if err != nil || response.Status != 400 || payload["error"] != "request body must be an object" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals", adminToken, map[string]any{"roles": "reader"})
	if err != nil || response.Status != 400 || payload["error"] != "roles must be an array of strings" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals", adminToken, map[string]any{"read_prefixes": "bad"})
	if err != nil || response.Status != 400 || payload["error"] != "read_prefixes must be an array of strings" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals", adminToken, map[string]any{"write_prefixes": "bad"})
	if err != nil || response.Status != 400 || payload["error"] != "write_prefixes must be an array of strings" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/missing/unknown", adminToken, map[string]any{})
	if err != nil || response.Status != 404 || payload["error"] != "not found" {
		t.Fatal(response, payload, err)
	}
	response, payload, err = adminCall(t, handler, "POST", "/admin/api/principals/root/rename", adminToken, map[string]any{})
	if err != nil || response.Status != 400 || payload["error"] != "principal name must use lowercase letters, digits, and hyphens" {
		t.Fatal(response, payload, err)
	}
	boomed := AdminHTTP{Store: managedAccessStoreStub{managedAccessStore: store, authenticate: func(context.Context, string) (*access.Principal, error) {
		return nil, io.ErrClosedPipe
	}}}
	if response, _, err = adminCall(t, boomed, "GET", "/admin/api/principals", adminToken, nil); !errors.Is(err, io.ErrClosedPipe) || response != nil {
		t.Fatal(response, err)
	}
	boomed = AdminHTTP{Store: managedAccessStoreStub{managedAccessStore: store, list: func(context.Context) ([]access.ManagedPrincipal, error) {
		return nil, adminAccessError("list denied")
	}}}
	response, payload, err = adminCall(t, boomed, "GET", "/admin/api/principals", adminToken, nil)
	if err != nil || response.Status != 400 || payload["error"] != "list denied" {
		t.Fatal(response, payload, err)
	}
	boomed = AdminHTTP{Store: managedAccessStoreStub{managedAccessStore: store, audit: func(context.Context, int) ([]access.AuditEntry, error) {
		return nil, io.ErrClosedPipe
	}}}
	if response, _, err = adminCall(t, boomed, "GET", "/admin/api/activity", adminToken, nil); !errors.Is(err, io.ErrClosedPipe) || response != nil {
		t.Fatal(response, err)
	}
	boomed = AdminHTTP{Store: managedAccessStoreStub{managedAccessStore: store, create: func(context.Context, string, string, []string, []string, []string, *string) (access.ManagedPrincipal, string, error) {
		return access.ManagedPrincipal{}, "", adminAccessError("create denied")
	}}}
	response, payload, err = adminCall(t, boomed, "POST", "/admin/api/principals", adminToken, map[string]any{"name": "x", "roles": []string{"reader"}, "read_prefixes": []string{"/public/"}, "write_prefixes": []string{}})
	if err != nil || response.Status != 400 || payload["error"] != "create denied" {
		t.Fatal(response, payload, err)
	}
	boomed = AdminHTTP{Store: managedAccessStoreStub{managedAccessStore: store, update: func(context.Context, string, string, []string, []string, []string) (access.ManagedPrincipal, error) {
		return access.ManagedPrincipal{}, io.ErrClosedPipe
	}}}
	if response, _, err = adminCall(t, boomed, "POST", "/admin/api/principals/root/update", adminToken, map[string]any{"roles": []string{"admin"}, "read_prefixes": []string{"/"}, "write_prefixes": []string{"/"}}); !errors.Is(err, io.ErrClosedPipe) || response != nil {
		t.Fatal(response, err)
	}
	if _, err = handler.principalAction(context.Background(), "root", "/admin/api/principals/root/update", map[string]any{"roles": "bad"}); err == nil {
		t.Fatal("update roles")
	}
	if _, err = handler.principalAction(context.Background(), "root", "/admin/api/principals/root/update", map[string]any{"read_prefixes": "bad"}); err == nil {
		t.Fatal("update reads")
	}
	if _, err = handler.principalAction(context.Background(), "root", "/admin/api/principals/root/update", map[string]any{"write_prefixes": "bad"}); err == nil {
		t.Fatal("update writes")
	}
	for name, boomed := range map[string]AdminHTTP{
		"disable": {Store: managedAccessStoreStub{managedAccessStore: store, setEnabled: func(context.Context, string, string, bool) (access.ManagedPrincipal, error) {
			return access.ManagedPrincipal{}, io.ErrClosedPipe
		}}},
		"rotate": {Store: managedAccessStoreStub{managedAccessStore: store, rotate: func(context.Context, string, string, *string) (string, error) { return "", io.ErrClosedPipe }}},
		"revoke": {Store: managedAccessStoreStub{managedAccessStore: store, revoke: func(context.Context, string, string) (access.ManagedPrincipal, error) {
			return access.ManagedPrincipal{}, io.ErrClosedPipe
		}}},
		"delete": {Store: managedAccessStoreStub{managedAccessStore: store, delete: func(context.Context, string, string) (access.ManagedPrincipal, error) {
			return access.ManagedPrincipal{}, io.ErrClosedPipe
		}}},
	} {
		if _, err = boomed.principalAction(context.Background(), "root", "/admin/api/principals/root/"+name, map[string]any{}); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(name, err)
		}
	}
}
