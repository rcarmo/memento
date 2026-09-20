package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

func TestIdentityRealHTTPManagedRevocation(t *testing.T) {
	ctx := context.Background()
	q, _ := queueTest(t)
	store, err := access.OpenStore(ctx, q.Proposals.DB, "synthetic-test-master")
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := store.Create(ctx, "bootstrap", "dynamic", []string{"proposer"}, []string{"/public/"}, []string{"/public/"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := NewIdentity([]BearerPrincipal{{Token: "static", Principal: access.Principal{Name: "static", Roles: []string{"admin"}}}}, access.AuthorizationConfig{ProtectedReadPrefixes: []string{"/private/"}}, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("identity-test")
	if err = server.Tools.Register(umcp.Tool{Name: "who", Parameters: []umcp.Parameter{{Name: "principal", HasDefault: true, Default: ""}}, Call: func(ctx context.Context, _ map[string]any) (any, error) {
		actor, err := identity.Context(ctx)
		if err != nil {
			return nil, err
		}
		reads := []any{}
		for _, read := range actor.Policy.ReadPrefixes {
			reads = append(reads, read)
		}
		return umcp.OrderedObject{{Name: "principal", Value: actor.Policy.Principal}, {Name: "reads", Value: reads}, {Name: "session", Value: nullableText(actor.MCPSessionID)}}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	options := umcp.DefaultHTTPOptions()
	options.AsyncReference = true
	transport, err := umcp.NewStreamableHTTP(server, options, identity.HTTPHooks())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	host := httptest.NewServer(transport)
	defer host.Close()
	request := func(token, body, session string) (int, string, http.Header) {
		t.Helper()
		req, err := http.NewRequest("POST", host.URL+"/mcp", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		response, err := host.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, string(raw), response.Header
	}
	init := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`
	if status, _, _ := request("static", init, ""); status != 401 {
		t.Fatal("managed fell back to static", status)
	}
	status, _, headers := request(token, init, "")
	if status != 200 {
		t.Fatal(status)
	}
	session := headers.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatal("no session")
	}
	call := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"who","arguments":{"principal":"admin"}}}`
	status, body, _ := request(token, call, session)
	if status != 200 || !strings.Contains(body, "dynamic") || strings.Contains(body, `\"Principal\": \"admin\"`) {
		t.Fatal(status, body)
	}
	// Fresh policy resolution must change grants even with a persistent session.
	if _, err = store.Update(ctx, "bootstrap", "dynamic", []string{"proposer"}, []string{"/changed/"}, nil); err != nil {
		t.Fatal(err)
	}
	status, body, _ = request(token, call, session)
	if status != 200 || !strings.Contains(body, "/changed/") {
		t.Fatal(status, body)
	}
	denied := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"access_principal_list","arguments":{}}}`
	if status, body, _ := request(token, denied, session); status != 403 {
		t.Fatal(status, body)
	}
	// Another authenticated principal cannot borrow the session.
	_, otherToken, err := store.Create(ctx, "bootstrap", "other", []string{"proposer"}, []string{"/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status, body, _ := request(otherToken, call, session); status != 403 {
		t.Fatal(status, body)
	}
	if _, err = store.Revoke(ctx, "bootstrap", "dynamic"); err != nil {
		t.Fatal(err)
	}
	if status, body, _ := request(token, call, session); status != 401 {
		t.Fatal("revoked session accepted", status, body)
	}
}
func TestIdentityConcurrentPrincipalIsolation(t *testing.T) {
	config := access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{}}
	tokens := []BearerPrincipal{}
	for i := range 12 {
		name := fmt.Sprintf("actor-%d", i)
		tokens = append(tokens, BearerPrincipal{name, access.Principal{Name: name, Roles: []string{"reader"}}})
		config.Principals[name] = access.NamespacePolicy{Roles: []string{"reader"}, ReadPrefixes: []string{"/" + name + "/"}}
	}
	identity, err := NewIdentity(tokens, config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := umcp.Dispatcher{Handlers: map[string]umcp.Handler{"who": func(ctx context.Context, _ map[string]any) (any, *umcp.RPCError, error) {
		actor, err := identity.Context(ctx)
		return actor.Policy, nil, err
	}}}
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("actor-%d", i)
			for range 20 {
				response, err := dispatcher.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"who","params":{"principal":"admin"}}`), umcp.RequestContext{Principal: name})
				if err != nil {
					t.Error(err)
					return
				}
				raw, err := json.Marshal(response.Result)
				if err != nil || !strings.Contains(string(raw), name) {
					t.Error(string(raw), err)
				}
			}
		}()
	}
	wg.Wait()
}
