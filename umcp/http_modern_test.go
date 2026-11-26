package umcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModernHTTPMetadataContract(t *testing.T) {
	server := NewServer("modern")
	server.AdditionalProtocolVersions = []string{StatelessHTTPVersion}
	for _, name := range []string{"server/discover", "tools/list", "tools/call"} {
		server.SetHandler(name, func(context.Context, map[string]any) (any, *RPCError, error) {
			return map[string]any{"resultType": "complete"}, nil, nil
		})
	}
	h, err := NewStreamableHTTP(server, DefaultHTTPOptions(), HTTPHooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	for _, c := range []struct {
		method, name, uri, headerName, headerMethod, version, bodyVersion string
		id                                                                any
		status, code                                                      int
	}{
		{"server/discover", "", "", "", "server/discover", StatelessHTTPVersion, StatelessHTTPVersion, 1, 200, 0},
		{"ping", "", "", "", "ping", StatelessHTTPVersion, StatelessHTTPVersion, 1, 200, 0},
		{"tools/list", "", "", "", "tools/list", StatelessHTTPVersion, StatelessHTTPVersion, 1, 200, 0},
		{"tools/call", "draft", "", "draft", "tools/call", StatelessHTTPVersion, StatelessHTTPVersion, 1, 200, 0},
		{"ping", "", "", "", "ping", "2099-01-01", "2099-01-01", 1, 400, -32022},
		{"ping", "", "", "", "ping", StatelessHTTPVersion, "", 1, 400, -32020},
		{"ping", "", "", "", "tools/list", StatelessHTTPVersion, StatelessHTTPVersion, 1, 400, -32020},
		{"ping", "", "", "unneeded", "ping", StatelessHTTPVersion, StatelessHTTPVersion, 1, 400, -32020},
		{"tools/call", "draft", "", "", "tools/call", StatelessHTTPVersion, StatelessHTTPVersion, 1, 400, -32020},
		{"resources/read", "", "memory://status", "memory://status", "resources/read", StatelessHTTPVersion, StatelessHTTPVersion, 1, 404, -32601},
		{"prompts/get", "prompt", "", "prompt", "prompts/get", StatelessHTTPVersion, StatelessHTTPVersion, 1, 404, -32601},
		{"ping", "", "", "", "ping", StatelessHTTPVersion, StatelessHTTPVersion, true, 400, -32600},
		{"unknown", "", "", "", "unknown", StatelessHTTPVersion, StatelessHTTPVersion, true, 404, -32601},
	} {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.id, "method": c.method, "params": map[string]any{"name": c.name, "uri": c.uri, "_meta": map[string]any{protocolVersionMeta: c.bodyVersion}}})
		r := httptest.NewRequest("POST", "/mcp", strings.NewReader(string(raw)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Mcp-Protocol-Version", c.version)
		r.Header.Set("Mcp-Method", c.headerMethod)
		r.Header.Set("Mcp-Session-Id", "legacy-not-required")
		if c.headerName != "" {
			r.Header.Set("Mcp-Name", c.headerName)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatalf("%+v: %d %s", c, w.Code, w.Body.String())
		}
		if c.code != 0 {
			var result struct{ Error RPCError }
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Error.Code != c.code {
				t.Fatalf("%+v: %s (%v)", c, w.Body, err)
			}
		}
	}
	for _, method := range []string{"GET", "DELETE", "OPTIONS"} {
		r := httptest.NewRequest(method, "http://localhost/mcp", nil)
		r.Header.Set("Mcp-Protocol-Version", StatelessHTTPVersion)
		if method == "OPTIONS" {
			r.Header.Set("Origin", "http://localhost")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if method == "OPTIONS" {
			if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Mcp-Method, Mcp-Name") {
				t.Fatal(w.Code, w.Header())
			}
		} else if w.Code != 405 {
			t.Fatal(method, w.Code)
		}
	}
}

func TestHTTPBodyRouteBounds(t *testing.T) {
	h, err := NewStreamableHTTP(NewServer("bounds"), DefaultHTTPOptions(), HTTPHooks{BodyRoute: &HTTPBodyRoute{Match: func(path string) bool { return path == "/files/upload" }, Handle: func(context.Context, string, string, map[string]string, int64, io.Reader) (*HTTPResponse, error) {
		t.Fatal("invalid body reached handler")
		return nil, nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	okRoute := false
	h.hooks.BodyRoute.Handle = func(_ context.Context, _, _ string, _ map[string]string, size int64, body io.Reader) (*HTTPResponse, error) {
		if size != 0 {
			t.Fatal("invalid body reached handler")
		}
		if b, e := io.ReadAll(body); e != nil || len(b) != 0 {
			t.Fatal(b, e)
		}
		okRoute = true
		return &HTTPResponse{Status: 204}, nil
	}
	r := httptest.NewRequest(http.MethodPost, "/mcp/files/upload", nil)
	r.Body = nil
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !okRoute || w.Code != 204 {
		t.Fatal(w.Code)
	}
	if h.deferBody("http://[bad/mcp/files/upload") {
		t.Fatal("bad target deferred")
	}
	for _, c := range []struct {
		size   int64
		status int
	}{{-1, 400}, {(4 << 20) + 1, 413}} {
		r := httptest.NewRequest(http.MethodPost, "/mcp/files/upload", nil)
		r.ContentLength = c.size
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.status || w.Header().Get("Connection") != "close" {
			t.Fatal(c, w.Code, w.Header())
		}
	}
}

func TestDeferredBodyIOFailures(t *testing.T) {
	before := strings.NewReader("x")
	errClosed := io.ErrClosedPipe
	r := &deadlineReader{reader: before, deadline: func() error { return errClosed }}
	if _, err := r.Read(make([]byte, 1)); !errors.Is(err, errClosed) || before.Len() != 1 {
		t.Fatal(err, before.Len())
	}
	cont := continueOnRead(io.NopCloser(before), failureWriter{})
	if _, err := cont.Read(make([]byte, 1)); err == nil || before.Len() != 1 {
		t.Fatal(err, before.Len())
	}
}

func TestParseValueRejectsUnterminatedContainers(t *testing.T) {
	for _, raw := range []string{`{"a":1`, `[1`, `{"a":1]`, `[1}`} {
		if _, err := ParseValue([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
