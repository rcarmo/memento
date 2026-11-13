package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcarmo/memento/umcp"
)

// Exercise the daemon's normal startup, identity hooks and Streamable HTTP
// session ownership. Ping must not bypass authentication or become a tool.
func TestMCPPingHTTPAuthenticationAndSessions(t *testing.T) {
	rt, server, _ := contractRuntime(t, "compact", true, false)
	transport, err := umcp.NewStreamableHTTP(server, umcp.DefaultHTTPOptions(), rt.HTTPHooks)
	if err != nil {
		t.Fatal(err)
	}
	defer transport.Close()
	rpc := func(auth, session, version, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		if version != "" {
			req.Header.Set("Mcp-Protocol-Version", version)
		}
		rec := httptest.NewRecorder()
		transport.ServeHTTP(rec, req)
		return rec
	}
	const init = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"regression","version":"1"}}}`
	const ping = `{"jsonrpc":"2.0","id":2,"method":"ping"}`
	for _, scheme := range []string{"Bearer", "bearer", "BEARER", "bEaReR"} {
		t.Run(scheme, func(t *testing.T) {
			auth := scheme + " reader-token"
			rec := rpc(auth, "", "", init)
			if rec.Code != 200 {
				t.Fatal(rec.Code, rec.Body.String())
			}
			var initialized map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &initialized); err != nil {
				t.Fatal(err)
			}
			result := contractResult(t, initialized)
			if result["protocolVersion"] != "2025-03-26" {
				t.Fatal(result)
			}
			session := rec.Header().Get("Mcp-Session-Id")
			if session == "" {
				t.Fatal("missing session")
			}
			rec = rpc(auth, session, "2025-03-26", `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
			if rec.Code != 202 || rec.Body.Len() != 0 {
				t.Fatal(rec.Code, rec.Body.String())
			}
			for i := 0; i < 2; i++ {
				rec = rpc(auth, session, "2025-03-26", ping)
				if rec.Code != 200 {
					t.Fatal(rec.Code, rec.Body.String())
				}
				var response map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				empty := contractResult(t, response)
				if len(empty) != 0 || response["id"] != float64(2) {
					t.Fatal(response)
				}
			}
			for _, tc := range []struct {
				name, auth, session, version string
				status                       int
			}{{"missing-auth", "", session, "2025-03-26", 401}, {"wrong-token", "Bearer wrong", session, "2025-03-26", 401}, {"token-case", "bearer READER-TOKEN", session, "2025-03-26", 401}, {"wrong-owner", "Bearer token", session, "2025-03-26", 403}, {"unknown-session", auth, "unknown-session", "2025-03-26", 404}, {"unsupported-version", auth, session, "1900-01-01", 400}, {"session-version-mismatch", auth, session, "2024-11-05", 400}} {
				t.Run(tc.name, func(t *testing.T) {
					got := rpc(tc.auth, tc.session, tc.version, ping)
					if got.Code != tc.status {
						t.Fatal(got.Code, got.Body.String())
					}
				})
			}
			rec = rpc(auth, session, "2025-03-26", `{"jsonrpc":"2.0","method":"ping"}`)
			if rec.Code != 202 || rec.Body.Len() != 0 {
				t.Fatal(rec.Code, rec.Body.String())
			}
		})
	}
}
