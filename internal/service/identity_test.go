package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

type identityStore struct {
	policy             *access.NamespacePolicy
	authErr, policyErr error
}

func (s identityStore) Authenticate(_ context.Context, token string) (*access.Principal, error) {
	if s.authErr != nil {
		return nil, s.authErr
	}
	if token != "managed-token" {
		return nil, nil
	}
	return &access.Principal{Name: "dynamic", Roles: []string{"reader"}, Metadata: map[string]string{}}, nil
}
func (s identityStore) Policy(_ context.Context, name string) (*access.NamespacePolicy, error) {
	if s.policyErr != nil {
		return nil, s.policyErr
	}
	if name == "author" || name == "dynamic" {
		return s.policy, nil
	}
	return nil, nil
}
func actorContext(t *testing.T, i *Identity, name, session string) (ProposalActor, error) {
	t.Helper()
	var got ProposalActor
	var failure error
	d := umcp.Dispatcher{Handlers: map[string]umcp.Handler{"context": func(ctx context.Context, _ map[string]any) (any, *umcp.RPCError, error) {
		got, failure = i.Context(ctx)
		return nil, nil, nil
	}}}
	if _, err := d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"context","params":{"principal":"admin","client_instance_id":"spoof","source_chat":"spoof"}}`), umcp.RequestContext{Principal: name, SessionID: session, Headers: map[string]string{"x-principal": "admin"}}); err != nil {
		t.Fatal(err)
	}
	return got, failure
}
func TestIdentityReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/service-identity.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Authorization access.AuthorizationConfig
		ManagedPolicy access.NamespacePolicy `json:"managed_policy"`
		Cases         []struct {
			Kind     string
			Managed  bool
			Headers  map[string]string
			Expected *access.Principal
			Name     *string
			Context  map[string]any
			Policy   access.EffectivePolicy
			Error    string
		}
		Authorize []struct {
			Principal *umcp.Principal
			Tool      *string
			Expected  bool
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	tokens := []BearerPrincipal{{"token", access.Principal{Name: "author", Roles: []string{"proposer", "reader"}, Metadata: map[string]string{"synthetic": "static"}}}, {"admin-token", access.Principal{Name: "admin", Roles: []string{"admin"}}}}
	for _, c := range fixture.Cases {
		var managed ManagedIdentity
		if c.Managed {
			managed = identityStore{policy: &fixture.ManagedPolicy}
		}
		i, err := NewIdentity(tokens, fixture.Authorization, managed, nil)
		if err != nil {
			t.Fatal(err)
		}
		// Python fixture adds an empty token alias after name-map construction.
		i.tokens[""] = i.tokens["token"]
		if c.Kind == "authenticate" {
			got, err := i.AuthenticateHeaders(context.Background(), c.Headers)
			if err != nil || !reflect.DeepEqual(got, c.Expected) {
				t.Fatal(c, got, err)
			}
			continue
		}
		name := ""
		if c.Name != nil {
			name = *c.Name
		}
		got, err := actorContext(t, i, name, "session")
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(got, err, c.Error)
			}
		} else if err != nil || !reflect.DeepEqual(got.Policy, c.Policy) || got.MCPSessionID == nil || *got.MCPSessionID != "session" || got.ClientInstanceID != nil || got.SourceChat != nil {
			t.Fatal(got, err, c.Policy)
		}
	}
	i, err := NewIdentity(nil, fixture.Authorization, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Authorize {
		tool := ""
		if c.Tool != nil {
			tool = *c.Tool
		}
		got, err := i.AuthorizeRequest(context.Background(), c.Principal, "tools/call", tool)
		if err != nil || got != c.Expected {
			t.Fatal(got, err, c)
		}
	}
}
func TestIdentityConstructionAndFailures(t *testing.T) {
	ctx := context.Background()
	p := access.Principal{Name: "author", Roles: []string{"reader"}, Metadata: map[string]string{"label": "original"}}
	config := access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"author": {Roles: []string{"reader"}, ReadPrefixes: []string{"/public/"}}}}
	if _, err := NewIdentity([]BearerPrincipal{{"one", p}, {"two", p}}, config, nil, nil); err == nil {
		t.Fatal("duplicate name")
	}
	touches := 0
	i, err := NewIdentity([]BearerPrincipal{{"one", p}, {"one", p}}, config, nil, func() { touches++ })
	if err != nil {
		t.Fatal(err)
	}
	p.Roles[0] = "admin"
	p.Metadata["label"] = "mutated"
	config.Principals["author"].ReadPrefixes[0] = "/private/"
	principal, err := i.AuthenticateRequest(ctx, "POST", "/mcp", map[string]string{"authorization": "Bearer one"}, "")
	if err != nil || principal.Roles[0] != "reader" || principal.Metadata["label"] != "original" {
		t.Fatal(principal, err)
	}
	principal.Roles[0] = "admin"
	principal.Metadata["label"] = "mutated"
	got, err := actorContext(t, i, "author", "")
	if err != nil || got.MCPSessionID != nil || got.Policy.ReadPrefixes[0] != "/public/" {
		t.Fatal(got, err)
	}
	if touches != 2 {
		t.Fatal(touches)
	}
	for _, store := range []identityStore{{authErr: io.ErrClosedPipe}, {policyErr: io.ErrClosedPipe}} {
		i.managed = store
		if store.authErr != nil {
			if _, err := i.AuthenticateRequest(ctx, "POST", "/mcp", map[string]string{"authorization": "Bearer one"}, ""); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(err)
			}
		} else {
			if _, err := i.resolvePrincipal(ctx, "dynamic"); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(err)
			}
			if _, err := actorContext(t, i, "author", ""); !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(err)
			}
		}
	}
	i.managed = nil
	i.names["unknown-policy"] = access.Principal{Name: "unknown-policy", Roles: []string{"reader"}}
	if _, err := actorContext(t, i, "unknown-policy", ""); err == nil {
		t.Fatal("unknown policy")
	}
	if _, err := i.AuthenticateRequest(ctx, "POST", "/mcp", nil, ""); err != nil {
		t.Fatal(err)
	}
}
