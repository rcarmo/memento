package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/umcp"
)

const syntheticBearerToken = "Synthetic-Token.44"

var bearerSchemes = []string{"Bearer", "bearer", "BEARER", "bEaReR"}

func bearerAuthCases(token string) []struct {
	name, header string
	accepted     bool
} {
	return []struct {
		name, header string
		accepted     bool
	}{
		{"canonical", "Bearer " + token, true},
		{"lowercase", "bearer " + token, true},
		{"uppercase", "BEARER " + token, true},
		{"mixed", "bEaReR " + token, true},
		{"wrong scheme", "Basic " + token, false},
		{"scheme suffix", "BearerX " + token, false},
		{"missing separator", "Bearer" + token, false},
		{"tab separator", "bearer\t" + token, false},
		{"leading header space", " bearer " + token, false},
		{"leading credential space", "bearer  " + token, false},
		{"leading credential tab", "bearer \t" + token, false},
		{"trailing credential space", "bearer " + token + " ", false},
		{"trailing credential tab", "bearer " + token + "\t", false},
		{"wrong token case", "bearer " + strings.ToLower(token), false},
		{"empty credential", "bearer ", false},
		{"scheme only", "Bearer", false},
		{"empty header", "", false},
		{"bad token", "bearer invalid-synthetic-token", false},
		{"non-ASCII scheme", "Bеarer " + token, false}, // Cyrillic е.
	}
}

func TestBearerTokenPreservesCredentialBytes(t *testing.T) {
	// Parsing must not normalize credentials, even when they will fail lookup.
	for _, token := range []string{syntheticBearerToken, "", " token ", "\ttoken\t", "Tökén", "\x00\xff"} {
		for _, scheme := range bearerSchemes {
			got, ok := bearerToken(scheme + " " + token)
			if !ok || got != token {
				t.Fatalf("scheme %q: credential bytes changed", scheme)
			}
		}
	}
}

func TestIdentityBearerSchemes(t *testing.T) {
	ctx, store, _, _ := adminFixture(t)
	if err := store.Bootstrap(ctx, []access.ConfiguredPrincipal{{Name: "scheme-admin", Policy: access.NamespacePolicy{Roles: []string{"admin"}, ReadPrefixes: []string{"/"}}}}, map[string]string{"scheme-admin": syntheticBearerToken}); err != nil {
		t.Fatal(err)
	}
	for _, managed := range []bool{false, true} {
		name := "static"
		var backend ManagedIdentity
		if managed {
			name, backend = "managed", store
		}
		t.Run(name, func(t *testing.T) {
			staticName := "scheme-admin"
			if managed {
				staticName = "static-shadow"
			}
			identity, err := NewIdentity([]BearerPrincipal{
				{Token: syntheticBearerToken, Principal: access.Principal{Name: staticName, Roles: []string{"admin"}}},
				{Token: "Static-Only", Principal: access.Principal{Name: "static-only", Roles: []string{"admin"}}},
			}, access.AuthorizationConfig{}, backend, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range bearerAuthCases(syntheticBearerToken) {
				t.Run(tc.name, func(t *testing.T) {
					headers := map[string]string{"authorization": tc.header}
					principal, err := identity.AuthenticateHeaders(ctx, headers)
					if err != nil || (principal != nil) != tc.accepted {
						t.Fatalf("accepted=%v, error=%v; want %v", principal != nil, err, tc.accepted)
					}
					if principal != nil && principal.Name != "scheme-admin" {
						t.Fatalf("unexpected principal %q", principal.Name)
					}
					transportPrincipal, err := identity.AuthenticateRequest(ctx, "POST", "/mcp", headers, "")
					if err != nil || (transportPrincipal != nil) != tc.accepted {
						t.Fatalf("transport accepted=%v, error=%v; want %v", transportPrincipal != nil, err, tc.accepted)
					}
				})
			}
			if managed {
				for _, scheme := range bearerSchemes {
					principal, err := identity.AuthenticateHeaders(ctx, map[string]string{"authorization": scheme + " Static-Only"})
					if err != nil || principal != nil {
						t.Fatal("managed miss fell back to static credentials", err)
					}
				}
			}
		})
	}
}

func TestAdminHTTPBearerSchemes(t *testing.T) {
	ctx, store, _, handler := adminFixture(t)
	if err := store.Bootstrap(ctx, []access.ConfiguredPrincipal{{Name: "scheme-admin", Policy: access.NamespacePolicy{Roles: []string{"admin"}, ReadPrefixes: []string{"/"}}}}, map[string]string{"scheme-admin": syntheticBearerToken}); err != nil {
		t.Fatal(err)
	}
	transport, err := umcp.NewStreamableHTTP(umcp.NewServer("bearer-auth"), umcp.DefaultHTTPOptions(), umcp.HTTPHooks{Route: handler.Handle})
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	for _, tc := range bearerAuthCases(syntheticBearerToken) {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://localhost/admin/api/principals", nil)
			request.Header.Set("Authorization", tc.header)
			response := httptest.NewRecorder()
			transport.ServeHTTP(response, request)
			want := http.StatusUnauthorized
			if tc.accepted {
				want = http.StatusOK
			}
			if response.Code != want {
				t.Fatalf("status=%d; want %d", response.Code, want)
			}
		})
	}
}

type bearerErrorStore struct {
	*access.Store
}

func (bearerErrorStore) Authenticate(context.Context, string) (*access.Principal, error) {
	return nil, io.ErrClosedPipe
}

func TestBearerManagedFailuresRemainAuthoritative(t *testing.T) {
	ctx, store, token, handler := adminFixture(t)
	// Preserve the store's final-admin safety rule while testing revocation.
	if _, _, err := store.Create(ctx, "bootstrap", "backup-admin", []string{"admin"}, []string{"/"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	identity, err := NewIdentity([]BearerPrincipal{{Token: token, Principal: access.Principal{Name: "static-shadow", Roles: []string{"admin"}}}}, access.AuthorizationConfig{}, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkDenied := func(t *testing.T) {
		t.Helper()
		for _, scheme := range bearerSchemes {
			headers := map[string]string{"authorization": scheme + " " + token}
			principal, err := identity.AuthenticateHeaders(ctx, headers)
			if err != nil || principal != nil {
				t.Fatal("inactive managed credential fell back to static", err)
			}
			response, err := handler.Handle(ctx, "GET", "/admin/api/principals", headers, nil, "")
			if err != nil || response == nil || response.Status != http.StatusUnauthorized {
				t.Fatal("inactive managed credential accepted by admin", err)
			}
		}
	}
	if _, err := store.SetEnabled(ctx, "root", "root", false); err != nil {
		t.Fatal(err)
	}
	t.Run("disabled", checkDenied)
	if _, err := store.SetEnabled(ctx, "bootstrap", "root", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Revoke(ctx, "root", "root"); err != nil {
		t.Fatal(err)
	}
	t.Run("revoked", checkDenied)
	identity.managed = bearerErrorStore{store}
	handler.Store = bearerErrorStore{store}
	for _, scheme := range bearerSchemes {
		headers := map[string]string{"authorization": scheme + " " + token}
		principal, err := identity.AuthenticateHeaders(ctx, headers)
		if principal != nil || !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal("managed error fell back to static", err)
		}
		response, err := handler.Handle(ctx, "GET", "/admin/api/principals", headers, nil, "")
		if response != nil || !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal("admin swallowed managed error", err)
		}
	}
}
