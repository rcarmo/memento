package umcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testHTTP(t *testing.T) (*Server, *StreamableHTTP) {
	t.Helper()
	s := NewServer("test")
	options := DefaultHTTPOptions()
	options.Keepalive = 5 * time.Millisecond
	h, err := NewStreamableHTTP(s, options, HTTPHooks{Authenticate: func(_ context.Context, _, _ string, headers map[string]string, _ string) (*Principal, error) {
		auth := headers["authorization"]
		if !strings.HasPrefix(auth, "Bearer ") {
			return nil, nil
		}
		return &Principal{Name: strings.TrimPrefix(auth, "Bearer ")}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return s, h
}
func httpRequest(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer reader")
	r.Header.Set("Mcp-Protocol-Version", "2025-03-26")
	if method == "POST" {
		r.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func initialiseHTTP(t *testing.T, h *StreamableHTTP) string {
	t.Helper()
	w := httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, nil)
	if w.Code != 200 || w.Header().Get("Mcp-Session-Id") == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	return w.Header().Get("Mcp-Session-Id")
}

func TestHTTPInitialiseSessionAndStateless(t *testing.T) {
	s, h := testHTTP(t)
	id := initialiseHTTP(t, h)
	for _, c := range []struct {
		Method, Body, ID, Principal, Version string
		Status                               int
	}{
		{"POST", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, id, "reader", "2025-03-26", 200},
		{"POST", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, "", "reader", "2025-03-26", 200},
		{"POST", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, id, "reader", "2025-03-26", 202},
		{"POST", `{"jsonrpc":"2.0","method":"unknown"}`, id, "reader", "2025-03-26", 202},
		{"POST", `{"jsonrpc":"2.0","id":2,"result":{}}`, id, "reader", "2025-03-26", 202},
		{"POST", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, id, "other", "2025-03-26", 403},
		{"POST", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, id, "reader", "2024-11-05", 400},
		{"POST", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, "unknown", "reader", "2025-03-26", 404},
		{"POST", `{"jsonrpc":"2.0","id":2,"method":"initialize"}`, id, "reader", "2025-03-26", 400},
		{"DELETE", "", "", "reader", "2025-03-26", 400},
	} {
		w := httpRequest(h, c.Method, "/mcp", c.Body, map[string]string{"Mcp-Session-Id": c.ID, "Authorization": "Bearer " + c.Principal, "Mcp-Protocol-Version": c.Version})
		if w.Code != c.Status {
			t.Errorf("%+v: %d %s", c, w.Code, w.Body.String())
		}
	}
	w := httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":3,"method":"resources/subscribe","params":{"uri":"u","_session_id":"spoof"}}`, map[string]string{"Mcp-Session-Id": id})
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if got := s.Resources.Subscribers("u"); len(got) != 1 || got[0] != id {
		t.Fatal(got)
	}
	w = httpRequest(h, "DELETE", "/mcp", "", map[string]string{"Mcp-Session-Id": id})
	if w.Code != 200 || len(s.Resources.Subscribers("u")) != 0 {
		t.Fatal(w.Code)
	}
	if httpRequest(h, "DELETE", "/mcp", "", map[string]string{"Mcp-Session-Id": id}).Code != 404 {
		t.Fatal("deleted session accepted")
	}
	h.options.MaxSessions = 1
	h.sessions.limit = 1
	_ = initialiseHTTP(t, h)
	if httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, nil).Code != 503 {
		t.Fatal("session cap")
	}
}

func TestHTTPValidationAndHooks(t *testing.T) {
	s, h := testHTTP(t)
	valid := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	for _, c := range []struct {
		Method, Path, Body string
		Headers            map[string]string
		Status             int
	}{
		{"POST", "/mcp", valid, map[string]string{"Content-Type": "text/plain"}, 415}, {"POST", "/mcp", valid, map[string]string{"Accept": "text/event-stream"}, 406}, {"POST", "/mcp", valid, map[string]string{"Authorization": ""}, 401},
		{"POST", "/mcp", valid, map[string]string{"Mcp-Protocol-Version": ""}, 400}, {"POST", "/mcp", valid, map[string]string{"Origin": "https://bad.example"}, 403},
		{"GET", "/mcp", "", map[string]string{"Accept": "application/json"}, 406}, {"GET", "/mcp", "", map[string]string{"Mcp-Protocol-Version": "invalid"}, 400}, {"GET", "/mcp", "", map[string]string{"Authorization": ""}, 401}, {"GET", "/mcp", "", nil, 400},
		{"DELETE", "/mcp", "", map[string]string{"Mcp-Protocol-Version": ""}, 400}, {"DELETE", "/mcp", "", map[string]string{"Authorization": ""}, 401},
		{"OPTIONS", "/mcp", "", map[string]string{"Origin": "http://localhost"}, 204}, {"OPTIONS", "/mcp", "", nil, 405}, {"OPTIONS", "/anything", "", nil, 405}, {"PUT", "/mcp", "", nil, 501},
		{"POST", "/mcp", "{", nil, 200}, {"POST", "/mcp", "[]", nil, 200}, {"POST", "/mcp", "\xff", nil, 200},
	} {
		w := httpRequest(h, c.Method, c.Path, c.Body, c.Headers)
		if w.Code != c.Status {
			t.Errorf("%+v: %d %s", c, w.Code, w.Body.String())
		}
	}
	h.options.MaxRequestBytes = 2
	if httpRequest(h, "POST", "/mcp", valid, nil).Code != 413 {
		t.Fatal("oversize")
	}
	h.options.MaxRequestBytes = 4 << 20
	for _, header := range []string{"Authorization", "Content-Type", "Origin"} {
		r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(valid))
		r.Header[header] = []string{"a", "b"}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(header, w.Code)
		}
	}
	for _, headers := range []map[string]string{{"Authorization": "a,b"}, {"Transfer-Encoding": "chunked"}} {
		if w := httpRequest(h, "POST", "/mcp", valid, headers); w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(valid))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer reader")
	r.ContentLength = int64(len(valid) + 1)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("short body", w.Code)
	}
	r = httptest.NewRequest("POST", "http://localhost/mcp", nil)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer reader")
	r.Header.Del("Mcp-Protocol-Version")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	h.hooks.Authenticate = func(context.Context, string, string, map[string]string, string) (*Principal, error) {
		return nil, errors.New("private")
	}
	if httpRequest(h, "POST", "/mcp", valid, nil).Code != 500 {
		t.Fatal("auth error")
	}
	h.hooks.Authenticate = func(context.Context, string, string, map[string]string, string) (*Principal, error) { panic("private") }
	if httpRequest(h, "POST", "/mcp", valid, nil).Code != 500 {
		t.Fatal("auth panic")
	}
	h.hooks.Authenticate = nil
	h.hooks.Authorize = func(context.Context, *Principal, any, any) (bool, error) { return false, nil }
	if httpRequest(h, "POST", "/mcp", valid, nil).Code != 403 {
		t.Fatal("denied")
	}
	h.hooks.Authorize = func(context.Context, *Principal, any, any) (bool, error) { return false, errors.New("private") }
	if httpRequest(h, "POST", "/mcp", valid, nil).Code != 500 {
		t.Fatal("authorize error")
	}
	h.hooks.Authorize = nil
	s.dispatcher.Handlers["broken"] = func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, errors.New("private") }
	if httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"broken"}`, nil).Code != 500 {
		t.Fatal("handler failure")
	}
	s.dispatcher.Handlers["bad-json"] = func(context.Context, map[string]any) (any, *RPCError, error) { return make(chan int), nil, nil }
	if httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"bad-json"}`, nil).Code != 500 {
		t.Fatal("response encoding")
	}
	if _, err := NewStreamableHTTP(nil, DefaultHTTPOptions(), HTTPHooks{}); err == nil {
		t.Fatal("nil server")
	}
	if peer(&http.Request{RemoteAddr: "local"}) != "local" {
		t.Fatal("peer")
	}
}

func TestHTTPAuxiliaryRoutes(t *testing.T) {
	_, h := testHTTP(t)
	if w := httpRequest(h, "GET", "/missing", "", nil); w.Code != 405 || w.Header().Get("Allow") == "" {
		t.Fatal(w.Code)
	}
	if w := httpRequest(h, "GET", "/missing", "", map[string]string{"Authorization": ""}); w.Code != 405 {
		t.Fatal(w.Code)
	}
	h.hooks.Authorize = func(context.Context, *Principal, any, any) (bool, error) {
		t.Fatal("MCP auth called for auxiliary route")
		return false, nil
	}
	h.hooks.Route = func(_ context.Context, method, path string, _ map[string]string, body []byte, _ string) (*HTTPResponse, error) {
		mime := "text/plain"
		if path != "/asset" || method != "POST" || string(body) != "data" {
			t.Error(method, path, string(body))
		}
		return &HTTPResponse{Status: 201, Body: []byte("accepted"), ContentType: &mime, Headers: [][2]string{{"X-Result", "yes"}}}, nil
	}
	w := httpRequest(h, "POST", "/asset", "data", map[string]string{"Origin": "http://localhost"})
	if w.Code != 201 || w.Body.String() != "accepted" || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost" {
		t.Fatal(w.Code, w.Body.String())
	}
	h.hooks.Route = func(context.Context, string, string, map[string]string, []byte, string) (*HTTPResponse, error) {
		return &HTTPResponse{Status: 99}, nil
	}
	if httpRequest(h, "GET", "/asset", "", nil).Code != 500 {
		t.Fatal("bad route response")
	}
	h.hooks.Route = func(context.Context, string, string, map[string]string, []byte, string) (*HTTPResponse, error) {
		return nil, errors.New("private")
	}
	if httpRequest(h, "GET", "/asset", "", nil).Code != 500 {
		t.Fatal("route error")
	}
}

func TestHTTPLiveEventStreamAndDelete(t *testing.T) {
	server, handler := testHTTP(t)
	testServer := httptest.NewServer(handler)
	defer testServer.Close()
	id := initialiseHTTP(t, handler)
	request, _ := http.NewRequest("GET", testServer.URL+"/mcp", nil)
	request.Header.Set("Authorization", "Bearer reader")
	request.Header.Set("Mcp-Protocol-Version", "2025-03-26")
	request.Header.Set("Mcp-Session-Id", id)
	request.Header.Set("Accept", "text/event-stream")
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	if err != nil || line != ": connected\n" {
		t.Fatal(line, err)
	}
	_, _ = reader.ReadString('\n')
	duplicate, err := client.Do(request.Clone(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	_ = duplicate.Body.Close()
	if duplicate.StatusCode != 409 {
		t.Fatal(duplicate.StatusCode)
	}
	if err = server.Tools.RegisterAndNotify(Tool{Name: "one", Call: func(context.Context, map[string]any) (any, error) { return nil, nil }}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		line, err = reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "data: ") {
			var notification map[string]any
			if err = json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data: "))), &notification); err != nil {
				t.Fatal(err)
			}
			if notification["method"] != "notifications/tools/list_changed" {
				t.Fatal(notification)
			}
			break
		}
		if i == 19 {
			t.Fatal("notification missing")
		}
	}
	for i := 0; i < 20; i++ {
		line, err = reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == ": keepalive\n" {
			break
		}
		if i == 19 {
			t.Fatal("no heartbeat")
		}
	}
	deleted := httpRequest(handler, "DELETE", "/mcp", "", map[string]string{"Mcp-Session-Id": id})
	if deleted.Code != 200 {
		t.Fatal(deleted.Code)
	}
	_, err = io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err = handler.Notify(nil, "bad", map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("invalid notification")
	}
	if err = handler.Notify([]string{id}, "test", nil); err != nil {
		t.Fatal(err)
	}
}

type timeoutBody struct{}

func (timeoutBody) Read([]byte) (int, error) { return 0, timeoutError{} }
func (timeoutBody) Close() error             { return nil }

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

type plainResponseWriter struct {
	header http.Header
	status int
}

func (w *plainResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}
func (w *plainResponseWriter) WriteHeader(status int)      { w.status = status }
func (w *plainResponseWriter) Write(b []byte) (int, error) { return len(b), nil }

type streamWriter struct {
	plainResponseWriter
	writes  int
	failAt  int
	onFlush func(int)
}

func (w *streamWriter) Write(b []byte) (int, error) {
	w.writes++
	if w.writes == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return len(b), nil
}
func (w *streamWriter) Flush() {
	if w.onFlush != nil {
		w.onFlush(w.writes)
	}
}

func TestHTTPBodyFailureBranches(t *testing.T) {
	_, h := testHTTP(t)
	for _, c := range []struct {
		body   io.ReadCloser
		length int64
		max    int64
		status int
	}{{nil, 0, 10, 0}, {timeoutBody{}, -1, 10, 408}, {io.NopCloser(failureReader{}), -1, 10, 400}, {io.NopCloser(strings.NewReader("abc")), -1, 2, 413}} {
		r := httptest.NewRequest("POST", "http://localhost/mcp", nil)
		r.Body = c.body
		r.ContentLength = c.length
		h.options.MaxRequestBytes = c.max
		w := httptest.NewRecorder()
		_, ok := h.body(w, r, "")
		if c.status == 0 {
			if !ok {
				t.Fatal("nil body")
			}
		} else if ok || w.Code != c.status {
			t.Fatal(ok, w.Code, c.status)
		}
	}
	h.options.MaxRequestBytes = 1
	if httpRequest(h, "POST", "/aux", "long", nil).Code != 413 {
		t.Fatal("aux bounds")
	}
	if httpRequest(h, "DELETE", "/mcp", "long", nil).Code != 413 {
		t.Fatal("delete bounds")
	}
	r := httptest.NewRequest("GET", "http://localhost/mcp", nil)
	r.URL.Opaque = "http://[bad]"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("bad path", w.Code)
	}
}

func TestHTTPProcessingFailureBranches(t *testing.T) {
	server, h := testHTTP(t)
	server.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, nil }
	if httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, nil).Code != 200 {
		t.Fatal("nonobject init")
	}
	server.dispatcher.Handlers["initialize"] = server.initialize
	id := initialiseHTTP(t, h)
	if w := httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"resources/subscribe"}`, map[string]string{"Mcp-Session-Id": id}); w.Code != 200 {
		t.Fatal(w.Code)
	}
	// Dispatcher returns nil for an acknowledged notification even if it has an ID.
	if w := httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"notifications/initialized"}`, nil); w.Code != 202 {
		t.Fatal(w.Code)
	}
	server.dispatcher.Handlers["tools/list"] = func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, errors.New("failed") }
	if w := httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, nil); w.Code != 500 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer reader")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("missing version")
	}
}

func TestHTTPStreamFailureBranches(t *testing.T) {
	_, h := testHTTP(t)
	get := func(id string, ctx context.Context) *http.Request {
		r := httptest.NewRequest("GET", "http://localhost/mcp", nil).WithContext(ctx)
		r.Header.Set("Authorization", "Bearer reader")
		r.Header.Set("Mcp-Protocol-Version", "2025-03-26")
		r.Header.Set("Mcp-Session-Id", id)
		return r
	}
	id := initialiseHTTP(t, h)
	plain := &plainResponseWriter{}
	h.ServeHTTP(plain, get(id, context.Background()))
	if plain.status != 500 {
		t.Fatal(plain.status)
	}
	first := &streamWriter{failAt: 1}
	h.ServeHTTP(first, get(id, context.Background()))
	if first.writes != 1 {
		t.Fatal(first.writes)
	}
	later := &streamWriter{failAt: 2}
	h.ServeHTTP(later, get(id, context.Background()))
	if later.writes != 2 {
		t.Fatal(later.writes)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h.ServeHTTP(&streamWriter{}, get(id, ctx))
	// Remove while writing a payload so touch observes the vanished session.
	vanish := &streamWriter{onFlush: func(n int) {
		if n == 2 {
			h.sessions.mu.Lock()
			delete(h.sessions.sessions, id)
			h.sessions.mu.Unlock()
		}
	}}
	h.ServeHTTP(vanish, get(id, context.Background()))
	id = initialiseHTTP(t, h)
	// Queue a notification and a disconnect concurrently; either branch terminates.
	writer := &streamWriter{onFlush: func(n int) {
		if n == 1 {
			_ = h.Notify(nil, "queued", nil)
			session, _ := h.sessions.validate(id, "reader", "2025-03-26", true)
			h.sessions.remove(id, session)
		}
	}}
	h.ServeHTTP(writer, get(id, context.Background()))
}

func TestHTTPToolAuthorisationReceivesName(t *testing.T) {
	_, h := testHTTP(t)
	called := false
	h.hooks.Authorize = func(_ context.Context, _ *Principal, method, tool any) (bool, error) {
		called = true
		if method != "tools/call" || tool != "missing" {
			t.Fatal(method, tool)
		}
		return true, nil
	}
	w := httpRequest(h, "POST", "/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"missing"}}`, nil)
	if w.Code != 200 || !called {
		t.Fatal(w.Code, called)
	}
}

func TestEventDisconnectWinsBeforeWrite(t *testing.T) {
	_, h := testHTTP(t)
	done := make(chan struct{})
	close(done)
	writer := &streamWriter{}
	if h.writeEvent(writer, writer, done, []byte("event"), "absent", nil) || writer.writes != 0 {
		t.Fatal("wrote after disconnect")
	}
}

func TestPythonHTTPWireScenarios(t *testing.T) {
	data, err := os.ReadFile("../testdata/parity/umcp-http.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input struct {
			Method, Path, Body string
			Headers            map[string]string
		}
		Expected struct {
			Status  int
			Headers map[string]string
			Body    any
		}
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	s := NewServer("HTTPOracle")
	options := DefaultHTTPOptions()
	options.MaxRequestBytes = 256
	h, err := NewStreamableHTTP(s, options, HTTPHooks{Authenticate: func(_ context.Context, _, _ string, headers map[string]string, _ string) (*Principal, error) {
		token := headers["authorization"]
		if token == "Bearer crash" {
			return nil, errors.New("private auth error")
		}
		if !strings.HasPrefix(token, "Bearer ") {
			return nil, nil
		}
		return &Principal{Name: strings.TrimPrefix(token, "Bearer ")}, nil
	}, Authorize: func(_ context.Context, p *Principal, _, _ any) (bool, error) { return p.Name != "denied", nil }, Route: func(_ context.Context, _, path string, _ map[string]string, _ []byte, _ string) (*HTTPResponse, error) {
		if path == "/ok" {
			mime := "text/plain"
			return &HTTPResponse{Status: 201, Body: []byte("ok"), ContentType: &mime, Headers: [][2]string{{"X-Extra", "yes"}}}, nil
		}
		if path == "/bad-route" {
			return nil, errors.New("private route error")
		}
		return nil, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	httpServer := httptest.NewServer(h)
	defer httpServer.Close()
	session := ""
	for i, c := range cases {
		request, err := http.NewRequest(c.Input.Method, httpServer.URL+c.Input.Path, strings.NewReader(c.Input.Body))
		if err != nil {
			t.Fatal(err)
		}
		for key, value := range c.Input.Headers {
			value = strings.ReplaceAll(value, "{{SESSION}}", session)
			if strings.EqualFold(key, "host") {
				request.Host = value
			} else {
				request.Header.Set(key, value)
			}
		}
		response, err := httpServer.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != c.Expected.Status {
			t.Errorf("case %d %s %s status %d != %d", i, c.Input.Method, c.Input.Path, response.StatusCode, c.Expected.Status)
		}
		selected := map[string]string{}
		for _, name := range []string{"content-type", "www-authenticate", "allow", "access-control-allow-origin", "access-control-allow-methods", "access-control-allow-headers", "access-control-expose-headers", "vary", "x-extra"} {
			if v := response.Header.Get(name); v != "" {
				selected[name] = v
			}
		}
		if id := response.Header.Get("Mcp-Session-Id"); id != "" {
			session = id
			selected["mcp-session-id"] = "{{SESSION}}"
		}
		if !reflect.DeepEqual(selected, c.Expected.Headers) {
			t.Errorf("case %d headers %v != %v", i, selected, c.Expected.Headers)
		}
		var body any = string(raw)
		if response.Header.Get("Content-Type") == "application/json" && len(raw) > 0 {
			if err = json.Unmarshal(raw, &body); err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(body, c.Expected.Body) {
			t.Errorf("case %d body %#v != %#v", i, body, c.Expected.Body)
		}
	}
}

func FuzzHTTPPost(f *testing.F) {
	for _, body := range []string{"", `{}`, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, body string) {
		s := NewServer("fuzz")
		options := DefaultHTTPOptions()
		options.MaxRequestBytes = 4096
		h, err := NewStreamableHTTP(s, options, HTTPHooks{})
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		_ = httpRequest(h, "POST", "/mcp", body, nil)
	})
}
