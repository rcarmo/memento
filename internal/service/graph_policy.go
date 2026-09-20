package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/rcarmo/memento/internal/access"
)

type GraphManagedPolicies interface {
	Policy(context.Context, string) (*access.NamespacePolicy, error)
	List(context.Context) ([]access.ManagedPrincipal, error)
}
type GraphPrincipal struct {
	Name                  string   `json:"name"`
	Roles                 []string `json:"roles"`
	ReadPrefixes          []string `json:"read_prefixes"`
	WritePrefixes         []string `json:"write_prefixes"`
	ProtectedReadPrefixes []string `json:"protected_read_prefixes"`
}
type GraphPolicyDirectory struct {
	Static  access.AuthorizationConfig
	Managed GraphManagedPolicies
}

func (d GraphPolicyDirectory) Resolve(ctx context.Context, headers map[string]string) (*access.EffectivePolicy, error) {
	name := strings.TrimSpace(headers["x-memento-simulated-principal"])
	if name == "" {
		return nil, nil
	}
	var policy *access.NamespacePolicy
	var err error
	if d.Managed != nil {
		policy, err = d.Managed.Policy(ctx, name)
		if err != nil {
			return nil, err
		}
	} else if value, ok := d.Static.Principals[name]; ok {
		copy := value
		policy = &copy
	}
	if policy == nil {
		return nil, errors.New("unknown simulated principal")
	}
	return &access.EffectivePolicy{Principal: name, Roles: append([]string{}, policy.Roles...), ReadPrefixes: append([]string{}, policy.ReadPrefixes...), WritePrefixes: append([]string{}, policy.WritePrefixes...), ProtectedReadPrefixes: append([]string{}, d.Static.ProtectedReadPrefixes...)}, nil
}
func (d GraphPolicyDirectory) List(ctx context.Context) ([]GraphPrincipal, error) {
	out := []GraphPrincipal{}
	if d.Managed != nil {
		items, err := d.Managed.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !item.Enabled || item.Revoked || item.Deleted {
				continue
			}
			out = append(out, GraphPrincipal{item.Name, append([]string{}, item.Roles...), append([]string{}, item.ReadPrefixes...), append([]string{}, item.WritePrefixes...), append([]string{}, d.Static.ProtectedReadPrefixes...)})
		}
		return out, nil
	}
	names := make([]string, 0, len(d.Static.Principals))
	for name := range d.Static.Principals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		item := d.Static.Principals[name]
		out = append(out, GraphPrincipal{name, append([]string{}, item.Roles...), append([]string{}, item.ReadPrefixes...), append([]string{}, item.WritePrefixes...), append([]string{}, d.Static.ProtectedReadPrefixes...)})
	}
	return out, nil
}
