package service

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

func TestMCPPingReauthenticatesManagedSession(t *testing.T) {
	ctx, store, _, _ := adminFixture(t)
	policy := access.NamespacePolicy{Roles: []string{"admin"}, ReadPrefixes: []string{"/"}}
	if err := store.Bootstrap(ctx, []access.ConfiguredPrincipal{{Name: "ping-owner", Policy: policy}, {Name: "surviving-admin", Policy: policy}}, map[string]string{"ping-owner": syntheticBearerToken, "surviving-admin": "other-test-token"}); err != nil {
		t.Fatal(err)
	}
	identity, err := NewIdentity([]BearerPrincipal{{Token: syntheticBearerToken, Principal: access.Principal{Name: "static-shadow", Roles: []string{"admin"}}}}, access.AuthorizationConfig{}, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("managed-ping")
	transport, err := umcp.NewStreamableHTTP(server, umcp.DefaultHTTPOptions(), umcp.HTTPHooks{Authenticate: identity.AuthenticateRequest, Authorize: identity.AuthorizeRequest})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	rpc := func(body, session string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "bEaReR "+syntheticBearerToken)
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
			req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		}
		rec := httptest.NewRecorder()
		transport.ServeHTTP(rec, req)
		return rec
	}
	rec := rpc(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`, "")
	session := rec.Header().Get("Mcp-Session-Id")
	if rec.Code != 200 || session == "" {
		t.Fatal(rec.Code, rec.Body.String())
	}
	ping := `{"jsonrpc":"2.0","id":2,"method":"ping"}`
	rec = rpc(ping, session)
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(contractResult(t, response)) != 0 {
		t.Fatal(rec.Code, response)
	}
	if _, err := store.SetEnabled(ctx, "ping-owner", "ping-owner", false); err != nil {
		t.Fatal(err)
	}
	rec = rpc(ping, session)
	if rec.Code != 401 {
		t.Fatal("existing session bypassed disabled credential", rec.Code, rec.Body.String())
	}
}
