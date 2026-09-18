package service

import (
	"context"
	"errors"
	"strings"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/umcp"
)

// ManagedIdentity is implemented by access.Store. Database errors propagate;
// they must never cause fallback from managed authentication to static tokens.
type ManagedIdentity interface {
	Authenticate(context.Context, string) (*access.Principal, error)
	Policy(context.Context, string) (*access.NamespacePolicy, error)
}
type BearerPrincipal struct {
	Token     string
	Principal access.Principal
}

// Identity binds trusted transport authentication to service policy. It is not
// an MCP tool, and no tool argument can provide or override principal identity.
// Configuration is copied at construction; managed policy is re-read per call.
type Identity struct {
	tokens        map[string]access.Principal
	names         map[string]access.Principal
	authorization access.AuthorizationConfig
	managed       ManagedIdentity
	touch         func()
}

func NewIdentity(tokens []BearerPrincipal, authorization access.AuthorizationConfig, managed ManagedIdentity, touch func()) (*Identity, error) {
	identity := &Identity{tokens: map[string]access.Principal{}, names: map[string]access.Principal{}, authorization: copyAuthorization(authorization), managed: managed, touch: touch}
	// Mirror dict(bearer_tokens): duplicate token keys keep their final principal
	// but retain their first insertion position for duplicate-name diagnostics.
	order := []string{}
	for _, entry := range tokens {
		if _, exists := identity.tokens[entry.Token]; !exists {
			order = append(order, entry.Token)
		}
		identity.tokens[entry.Token] = copyPrincipal(entry.Principal)
	}
	for _, token := range order {
		principal := identity.tokens[token]
		if _, exists := identity.names[principal.Name]; exists {
			return nil, errors.New("duplicate principal name configured for bearer tokens: " + principal.Name)
		}
		identity.names[principal.Name] = principal
	}
	return identity, nil
}
func copyPrincipal(p access.Principal) access.Principal {
	p.Roles = append([]string{}, p.Roles...)
	metadata := map[string]string{}
	for k, v := range p.Metadata {
		metadata[k] = v
	}
	p.Metadata = metadata
	return p
}
func copyNamespace(p access.NamespacePolicy) access.NamespacePolicy {
	p.Roles = append([]string{}, p.Roles...)
	p.ReadPrefixes = append([]string{}, p.ReadPrefixes...)
	p.WritePrefixes = append([]string{}, p.WritePrefixes...)
	return p
}
func copyAuthorization(config access.AuthorizationConfig) access.AuthorizationConfig {
	principals := map[string]access.NamespacePolicy{}
	for name, policy := range config.Principals {
		principals[name] = copyNamespace(policy)
	}
	config.Principals = principals
	config.ProtectedReadPrefixes = append([]string{}, config.ProtectedReadPrefixes...)
	return config
}

// AuthenticateHeaders expects lowercase header keys supplied by the transport,
// and the case-sensitive, untrimmed "Bearer " prefix used by Python.
func (i *Identity) AuthenticateHeaders(ctx context.Context, headers map[string]string) (*access.Principal, error) {
	authorization := headers["authorization"]
	if !strings.HasPrefix(authorization, "Bearer ") {
		return nil, nil
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	if i.managed != nil {
		return i.managed.Authenticate(ctx, token)
	}
	principal, ok := i.tokens[token]
	if !ok {
		return nil, nil
	}
	copied := copyPrincipal(principal)
	return &copied, nil
}
func (i *Identity) AuthenticateRequest(ctx context.Context, method, path string, headers map[string]string, peer string) (*umcp.Principal, error) {
	if i.touch != nil {
		i.touch()
	}
	p, err := i.AuthenticateHeaders(ctx, headers)
	if err != nil || p == nil {
		return nil, err
	}
	return &umcp.Principal{Name: p.Name, Roles: append([]string{}, p.Roles...), Metadata: copyPrincipal(*p).Metadata}, nil
}
func (i *Identity) AuthorizeRequest(_ context.Context, principal *umcp.Principal, rpcMethod, tool any) (bool, error) {
	if principal == nil {
		return false, nil
	}
	toolName, _ := tool.(string)
	if strings.HasPrefix(toolName, "access_") {
		for _, role := range principal.Roles {
			if role == "admin" {
				return true, nil
			}
		}
		return false, nil
	}
	return true, nil
}
func (i *Identity) HTTPHooks() umcp.HTTPHooks {
	return umcp.HTTPHooks{Authenticate: i.AuthenticateRequest, Authorize: i.AuthorizeRequest}
}
func (i *Identity) resolvePrincipal(ctx context.Context, name string) (access.Principal, error) {
	if name == "" {
		return access.Principal{}, errors.New("missing authenticated principal")
	}
	principal, ok := i.names[name]
	if !ok && i.managed != nil {
		policy, err := i.managed.Policy(ctx, name)
		if err != nil {
			return access.Principal{}, err
		}
		if policy != nil {
			principal = access.Principal{Name: name, Roles: append([]string{}, policy.Roles...), Metadata: map[string]string{}}
			ok = true
		}
	}
	if !ok {
		return access.Principal{}, errors.New("unknown request principal: " + name)
	}
	return copyPrincipal(principal), nil
}
func (i *Identity) ResolvePolicy(ctx context.Context, principal access.Principal) (access.EffectivePolicy, error) {
	if i.managed != nil {
		managed, err := i.managed.Policy(ctx, principal.Name)
		if err != nil {
			return access.EffectivePolicy{}, err
		}
		if managed != nil {
			return access.EffectivePolicy{Principal: principal.Name, Roles: append([]string{}, managed.Roles...), ReadPrefixes: append([]string{}, managed.ReadPrefixes...), WritePrefixes: append([]string{}, managed.WritePrefixes...), ProtectedReadPrefixes: append([]string{}, i.authorization.ProtectedReadPrefixes...)}, nil
		}
	}
	return access.ResolvePolicy(i.authorization, principal)
}

// Context ignores request roles/headers/tool arguments: the principal name must
// already be authenticated by the transport, then mapped through current policy.
// Only session_id is copied, as in MementoMCPServer._context; client/chat remain nil.
func (i *Identity) Context(ctx context.Context) (ProposalActor, error) {
	if i.touch != nil {
		i.touch()
	}
	request := umcp.Context(ctx)
	principal, err := i.resolvePrincipal(ctx, request.Principal)
	if err != nil {
		return ProposalActor{}, err
	}
	policy, err := i.ResolvePolicy(ctx, principal)
	if err != nil {
		return ProposalActor{}, err
	}
	var session *string
	if request.SessionID != "" {
		value := request.SessionID
		session = &value
	}
	return ProposalActor{Policy: policy, MCPSessionID: session}, nil
}
