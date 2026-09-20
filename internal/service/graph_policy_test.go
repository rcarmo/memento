package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rcarmo/memento/internal/access"
)

type graphManagedFailure struct{}

func (graphManagedFailure) Policy(context.Context, string) (*access.NamespacePolicy, error) {
	return nil, errors.New("policy")
}
func (graphManagedFailure) List(context.Context) ([]access.ManagedPrincipal, error) {
	return nil, errors.New("list")
}
func TestGraphPolicyFailures(t *testing.T) {
	d := GraphPolicyDirectory{Managed: graphManagedFailure{}}
	if _, err := d.Resolve(context.Background(), map[string]string{"x-memento-simulated-principal": "x"}); err == nil {
		t.Fatal("policy")
	}
	if _, err := d.List(context.Background()); err == nil {
		t.Fatal("list")
	}
}
func TestGraphStaticPolicies(t *testing.T) {
	config := access.AuthorizationConfig{ProtectedReadPrefixes: []string{"/private/"}, Principals: map[string]access.NamespacePolicy{"z": {Roles: []string{"reader"}, ReadPrefixes: []string{"/z/"}}, "a": {Roles: []string{"admin"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}}}
	d := GraphPolicyDirectory{Static: config}
	items, err := d.List(context.Background())
	if err != nil || len(items) != 2 || items[0].Name != "a" || items[1].Name != "z" {
		t.Fatal(items, err)
	}
	policy, err := d.Resolve(context.Background(), map[string]string{"x-memento-simulated-principal": " a "})
	if err != nil || policy.Principal != "a" || policy.ProtectedReadPrefixes[0] != "/private/" {
		t.Fatal(policy, err)
	}
	config.Principals["a"] = access.NamespacePolicy{}
	if policy.ReadPrefixes[0] != "/" {
		t.Fatal("aliased")
	}
	if value, err := d.Resolve(context.Background(), nil); err != nil || value != nil {
		t.Fatal(value, err)
	}
	if _, err = d.Resolve(context.Background(), map[string]string{"x-memento-simulated-principal": "missing"}); err == nil {
		t.Fatal("unknown")
	}
}
func TestGraphManagedPolicies(t *testing.T) {
	j, _ := jobsTest(t)
	store, err := access.OpenStore(context.Background(), j.Controls.Queue.Proposals.DB, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"active", "disabled", "revoked"} {
		_, _, err = store.Create(context.Background(), "bootstrap", name, []string{"reader"}, []string{"/"}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, _ = store.SetEnabled(context.Background(), "bootstrap", "disabled", false)
	_, _ = store.Revoke(context.Background(), "bootstrap", "revoked")
	d := GraphPolicyDirectory{Managed: store, Static: access.AuthorizationConfig{ProtectedReadPrefixes: []string{"/private/"}}}
	items, err := d.List(context.Background())
	if err != nil || len(items) != 1 || items[0].Name != "active" {
		t.Fatal(items, err)
	}
	policy, err := d.Resolve(context.Background(), map[string]string{"x-memento-simulated-principal": "active"})
	if err != nil || policy.Principal != "active" {
		t.Fatal(policy, err)
	}
}
