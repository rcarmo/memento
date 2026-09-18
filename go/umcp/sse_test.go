package umcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testSSE(t *testing.T, mode SSEMode) (*Server, *LegacySSE) {
	t.Helper()
	s := NewServer("SSEOracle")
	options := DefaultSSEOptions(mode)
	options.MaxRequestBytes = 256
	options.Keepalive = 5 * time.Millisecond
	hooks := HTTPHooks{
		Authenticate: func(_ context.Context, _, _ string, headers map[string]string, _ string) (*Principal, error) {
			token := headers["authorization"]
			if token == "Bearer crash" {
				return nil, errors.New("private")
			}
			if !strings.HasPrefix(token, "Bearer ") {
				return nil, nil
			}
			return &Principal{Name: token[7:]}, nil
		},
		Authorize: func(_ context.Context, p *Principal, method, tool any) (bool, error) {
			if method == "crash" {
				return false, errors.New("private")
			}
			return p.Name != "denied" && method != "forbidden", nil
		},
	}
	s.dispatcher.Handlers["initialize"] = func(ctx context.Context, params map[string]any) (any, *RPCError, error) {
		c := Context(ctx)
		return map[string]any{"transport": c.Transport, "principal": c.Principal, "session": c.SessionID != "", "params": params}, nil, nil
	}
	h, err := NewLegacySSE(s, options, hooks)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return s, h
}
func readSSE(reader *bufio.Reader) (string, string, error) {
	kind, data := "", ""
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", "", err
		}
		if line == "\n" && kind != "" {
			return kind, data, nil
		}
		if strings.HasPrefix(line, "event: ") {
			kind = strings.TrimSpace(line[7:])
		}
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimSpace(line[6:])
		}
	}
}
func connectSSE(t *testing.T, h *LegacySSE) (*httptest.Server, *http.Response, *bufio.Reader, string) {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	// Close active sessions before server.Close waits for handlers.
	t.Cleanup(h.Close)
	req, _ := http.NewRequest("GET", server.URL+"/sse", nil)
	req.Header.Set("Authorization", "Bearer reader")
	req.Header.Set("Origin", "http://localhost")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	reader := bufio.NewReader(response.Body)
	kind, endpoint, err := readSSE(reader)
	if err != nil || kind != "endpoint" {
		t.Fatal(kind, endpoint, err)
	}
	id := strings.TrimPrefix(endpoint, "/message?sessionId=")
	if len(id) != 36 || id[14] != '4' {
		t.Fatal("UUID4", id)
	}
	return server, response, reader, id
}
func TestLegacySSEPythonWire(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-sse.json")
	if err != nil {
		t.Fatal(err)
	}
	type expected struct {
		Status  int
		Headers map[string]string
		Event   json.RawMessage
	}
	var modes []struct {
		Mode      string
		Handshake expected
		Cases     []struct {
			Input struct {
				Method, Path, Body string
				Event              bool
				Headers            map[string]string
			}
			Expected expected
		}
	}
	if err = json.Unmarshal(raw, &modes); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range modes {
		t.Run(fixture.Mode, func(t *testing.T) {
			mode := SyncSSE
			if fixture.Mode == "async" {
				mode = AsyncSSE
			}
			_, h := testSSE(t, mode)
			server, response, reader, id := connectSSE(t, h)
			if response.StatusCode != fixture.Handshake.Status {
				t.Fatal(response.StatusCode)
			}
			for k, v := range fixture.Handshake.Headers {
				if response.Header.Get(k) != v {
					t.Fatal(k, response.Header, v)
				}
			}
			for i, c := range fixture.Cases {
				t.Run(fmt.Sprint(i), func(t *testing.T) {
					req, _ := http.NewRequest(c.Input.Method, server.URL+strings.ReplaceAll(c.Input.Path, "{{SESSION}}", id), strings.NewReader(c.Input.Body))
					for k, v := range c.Input.Headers {
						req.Header.Set(k, v)
					}
					reply, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
					if err != nil {
						t.Fatal(err)
					}
					defer reply.Body.Close()
					body, err := io.ReadAll(reply.Body)
					if err != nil {
						t.Fatal(err)
					}
					if reply.StatusCode != c.Expected.Status {
						t.Fatalf("%s %s: %d != %d body %s", req.Method, req.URL, reply.StatusCode, c.Expected.Status, body)
					}
					for k, v := range c.Expected.Headers {
						// BaseHTTPRequestHandler produces an HTML error page for unsupported
						// methods. That raw-server response is outside this handler fixture.
						if mode == SyncSSE && c.Input.Method == "PUT" && k == "content-type" {
							continue
						}
						if reply.Header.Get(k) != v {
							t.Errorf("%s %q != %q", k, reply.Header.Get(k), v)
						}
					}
					if reply.Header.Get("Access-Control-Expose-Headers") != "" {
						t.Fatal("legacy SSE exposed Streamable HTTP headers")
					}
					if c.Input.Event {
						kind, data, err := readSSE(reader)
						if err != nil || kind != "message" {
							t.Fatal(kind, data, err)
						}
						actual, ok := decode([]byte(data))
						want, _ := decode(c.Expected.Event)
						if !ok || !reflect.DeepEqual(actual, want) {
							t.Fatalf("event %s != %s", data, c.Expected.Event)
						}
					}
				})
			}
		})
	}
}

func TestLegacySSEValidationAndHelpers(t *testing.T) {
	for _, o := range []SSEOptions{{Mode: 2}, {Mode: SyncSSE}, {Mode: SyncSSE, MaxRequestBytes: 1}, {Mode: SyncSSE, MaxRequestBytes: 1, Keepalive: -1}} {
		if _, err := NewLegacySSE(NewServer("test"), o, HTTPHooks{}); err == nil {
			t.Fatal(o)
		}
	}
	if _, err := NewLegacySSE(nil, DefaultSSEOptions(SyncSSE), HTTPHooks{}); err == nil {
		t.Fatal("nil server")
	}
	for _, c := range []struct{ Query, Want string }{{"sessionId=&sessionId=one", "one"}, {"other&nope=x", ""}, {"sessionId=a;b", "a;b"}, {"sessionId=%ff%zz+%61", "�%zz a"}, {"session%49d=a%2Bb", "a+b"}} {
		if got := legacySessionID(c.Query); got != c.Want {
			t.Fatal(c, got)
		}
	}
	if _, err := ssePayload(make(chan int)); err == nil {
		t.Fatal("invalid JSON")
	}
	for _, mode := range []SSEMode{SyncSSE, AsyncSSE} {
		_, h := testSSE(t, mode)
		for _, c := range []struct {
			Method, Path, Body string
			Headers            map[string]string
			Status             int
		}{
			{"POST", "/message", "{}", map[string]string{"Transfer-Encoding": "chunked"}, 400},
			{"POST", "/message", "{}", map[string]string{"Origin": "https://evil.example"}, 403},
			{"GET", "/sse", "", map[string]string{"Authorization": "a,b"}, 400},
		} {
			w := httpRequest(h, c.Method, c.Path, c.Body, c.Headers)
			if w.Code != c.Status {
				t.Fatal(c, w.Code)
			}
		}
		r := httptest.NewRequest("POST", "/message", nil)
		r.ContentLength = -1
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
		w = httptest.NewRecorder()
		r.ContentLength = 1
		r.Body = io.NopCloser(failureReader{})
		if _, ok := h.body(w, r, ""); ok || w.Code != 400 {
			t.Fatal("bad read")
		}
		h.token = func() (string, error) { return "", io.ErrClosedPipe }
		if w = httpRequest(h, "GET", "/sse", "", nil); w.Code != 500 {
			t.Fatal(w.Code)
		}
		h.token = randomSessionID
		h.Close()
		if w = httpRequest(h, "GET", "/sse", "", nil); w.Code != 503 {
			t.Fatal(w.Code)
		}
	}
}

// A writer with controllable failures, without requiring fragile socket timing.
type sseWriter struct {
	header               http.Header
	code, writes, failAt int
	onWrite              func(string)
}

func (w *sseWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}
func (w *sseWriter) WriteHeader(code int) { w.code = code }
func (w *sseWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.onWrite != nil {
		w.onWrite(string(p))
	}
	if w.writes == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}
func (w *sseWriter) Flush() {}

type noFlush struct{ http.ResponseWriter }

func TestLegacySSEStreamFailuresAndKeepalive(t *testing.T) {
	_, h := testSSE(t, SyncSSE)
	r := httptest.NewRequest("GET", "/sse", nil)
	r.Header.Set("Authorization", "Bearer reader")
	w := httptest.NewRecorder()
	h.ServeHTTP(noFlush{w}, r)
	if w.Code != 500 {
		t.Fatal(w.Code)
	}
	sw := &sseWriter{failAt: 1}
	h.ServeHTTP(sw, r)
	if len(h.sessions) != 0 {
		t.Fatal("endpoint failure leaked")
	}
	sw = &sseWriter{failAt: 2}
	h.ServeHTTP(sw, r)
	if sw.writes != 2 || len(h.sessions) != 0 {
		t.Fatal("keepalive failure leaked")
	}
	ctx, cancel := context.WithCancel(context.Background())
	sw = &sseWriter{onWrite: func(string) { cancel() }}
	h.ServeHTTP(sw, r.WithContext(ctx))
	if len(h.sessions) != 0 {
		t.Fatal("context failure leaked")
	}
}

func addLegacySession(h *LegacySSE, id, owner string) *legacySession {
	session := &legacySession{owner: owner, wake: make(chan struct{}, 1), closed: make(chan struct{})}
	h.mu.Lock()
	h.sessions[id] = session
	h.mu.Unlock()
	return session
}
func TestLegacySSENotificationsAndDelivery(t *testing.T) {
	for _, mode := range []SSEMode{SyncSSE, AsyncSSE} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			s, h := testSSE(t, mode)
			_, response, reader, id := connectSSE(t, h)
			if err := s.send("notifications/test", nil); err != nil {
				t.Fatal(err)
			}
			kind, data, err := readSSE(reader)
			if err != nil || kind != "message" || !strings.Contains(data, `"method":"notifications/test"`) || strings.Contains(data, "params") {
				t.Fatal(kind, data, err)
			}
			if err = h.Notify([]string{id}, "notifications/test", map[string]any{"value": 1}); err != nil {
				t.Fatal(err)
			}
			_, data, err = readSSE(reader)
			if err != nil || !strings.Contains(data, `"value":1`) {
				t.Fatal(data, err)
			}
			if err = h.Notify([]string{"missing"}, "ignored", nil); err != nil {
				t.Fatal(err)
			}
			if err = h.Notify(nil, "bad", map[string]any{"value": make(chan int)}); err == nil {
				t.Fatal("invalid data")
			}
			response.Body.Close()
			h.Close()
			if len(h.sessions) != 0 {
				t.Fatal("sessions leaked")
			}
		})
	}
	_, h := testSSE(t, SyncSSE)
	session := addLegacySession(h, "synthetic", "reader")
	for range 102 {
		if err := h.deliver("synthetic", session, []byte("test")); err != nil {
			t.Fatal(err)
		}
	}
	if len(session.pending) != 102 {
		t.Fatal("legacy queue must be unbounded")
	}
	h.remove("synthetic", session)
	h.remove("synthetic", session)
	if err := h.deliver("synthetic", session, nil); err == nil {
		t.Fatal("closed session accepted delivery")
	}
	_, h = testSSE(t, AsyncSSE)
	session = addLegacySession(h, "synthetic", "reader")
	done := make(chan error, 1)
	go func() { done <- h.deliver("synthetic", session, nil) }()
	<-session.wake
	h.Close()
	if err := <-done; err == nil {
		t.Fatal("closed stream delivery succeeded")
	}
	// Async POST sees failed event write; cleanup remains owned by GET.
	_, h = testSSE(t, AsyncSSE)
	session = addLegacySession(h, "synthetic", "reader")
	go func() { done <- h.deliver("synthetic", session, nil) }()
	<-session.wake
	h.mu.Lock()
	session.pending[0].ack <- io.ErrClosedPipe
	h.mu.Unlock()
	if err := <-done; !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

func TestLegacySSEPostErrorPaths(t *testing.T) {
	for _, mode := range []SSEMode{SyncSSE, AsyncSSE} {
		s, h := testSSE(t, mode)
		session := addLegacySession(h, "id", "reader")
		path := "/message?sessionId=id"
		for _, body := range []string{"\xff"} {
			if w := httpRequest(h, "POST", path, body, nil); w.Code != 400 {
				t.Fatal(w.Code)
			}
		}
		r := httptest.NewRequest("POST", path, strings.NewReader("{}"))
		r.Header.Set("Authorization", "Bearer reader")
		r.Header.Set("Content-Type", "application/json")
		r.ContentLength = 3
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
		s.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, io.ErrClosedPipe }
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
		if w = httpRequest(h, "POST", path, body, nil); w.Code != 500 {
			t.Fatal(w.Code)
		}
		s.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) { return make(chan int), nil, nil }
		if w = httpRequest(h, "POST", path, body, nil); w.Code != 500 {
			t.Fatal(w.Code)
		}
		s.dispatcher.Handlers["initialize"] = func(context.Context, map[string]any) (any, *RPCError, error) {
			h.remove("id", session)
			return "result", nil, nil
		}
		if w = httpRequest(h, "POST", path, body, nil); w.Code != 404 {
			t.Fatal(w.Code)
		}
		session = addLegacySession(h, "id", "reader")
		h.common.hooks.Authorize = func(context.Context, *Principal, any, any) (bool, error) { h.remove("id", session); return true, nil }
		if w = httpRequest(h, "POST", path, body, nil); w.Code != 404 {
			t.Fatal(w.Code)
		}
	}
}

func TestLegacySSERemainingBranches(t *testing.T) {
	_, h := testSSE(t, SyncSSE)
	r := httptest.NewRequest("GET", "/sse", nil)
	r.URL.Opaque = "http://[bad]"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	// A synthetic context-free writer exits because Close terminates its session.
	r = httptest.NewRequest("GET", "/sse", nil)
	r.Header.Set("Authorization", "Bearer reader")
	sw := &sseWriter{onWrite: func(string) { h.Close() }}
	h.ServeHTTP(sw, r)
	// Both source modes terminate when a queued event cannot be written.
	for _, mode := range []SSEMode{SyncSSE, AsyncSSE} {
		_, h := testSSE(t, mode)
		var ack chan error
		sw := &sseWriter{failAt: 2, onWrite: func(data string) {
			if !strings.HasPrefix(data, "event: endpoint") {
				return
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			for _, session := range h.sessions {
				item := &sseDelivery{payload: []byte("event: message\ndata: {}\n\n")}
				if mode == AsyncSSE {
					ack = make(chan error, 1)
					item.ack = ack
				}
				session.pending = append(session.pending, item)
				session.wake <- struct{}{}
			}
		}}
		h.ServeHTTP(sw, r)
		if sw.writes != 2 || len(h.sessions) != 0 {
			t.Fatal("failed event not cleaned up")
		}
		if ack != nil {
			if err := <-ack; !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal(err)
			}
		}
	}
	// Missing params become a trusted session-only object before validation.
	_, h = testSSE(t, SyncSSE)
	session := addLegacySession(h, "id", "reader")
	w = httpRequest(h, "POST", "/message?sessionId=id", `{"jsonrpc":"2.0","id":1,"method":"resources/subscribe"}`, nil)
	if w.Code != 202 {
		t.Fatal(w.Code)
	}
	// Sync notification handlers can invalidate a session during authorisation.
	h.common.hooks.Authorize = func(context.Context, *Principal, any, any) (bool, error) { h.remove("id", session); return true, nil }
	w = httpRequest(h, "POST", "/message?sessionId=id", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	// Successful keepalive uses the same writer and does not produce an event.
	_, h = testSSE(t, SyncSSE)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sw = &sseWriter{onWrite: func(data string) {
		if data == ": keepalive\n\n" {
			cancel()
		}
	}}
	h.ServeHTTP(sw, r.WithContext(ctx))
	if sw.writes < 2 {
		t.Fatal("no keepalive")
	}
}

func FuzzLegacySSE(f *testing.F) {
	for _, body := range []string{"", "{}", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, "\xff"} {
		f.Add(body, "")
	}
	f.Fuzz(func(t *testing.T, body, query string) {
		s := NewServer("fuzz")
		h, err := NewLegacySSE(s, DefaultSSEOptions(SyncSSE), HTTPHooks{})
		if err != nil {
			t.Fatal(err)
		}
		defer h.Close()
		_ = legacySessionID(query)
		addLegacySession(h, "fuzz", "anonymous")
		_ = httpRequest(h, "POST", "/message?sessionId=fuzz", body, nil)
	})
}

func TestLegacySSESubscriptionsAndReconnection(t *testing.T) {
	for _, mode := range []SSEMode{SyncSSE, AsyncSSE} {
		s, h := testSSE(t, mode)
		server, _, reader, id := connectSSE(t, h)
		body := `{"jsonrpc":"2.0","id":1,"method":"resources/subscribe","params":{"uri":"test://one","_session_id":"spoof"}}`
		req, _ := http.NewRequest("POST", server.URL+"/message?sessionId="+id, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer reader")
		req.Header.Set("Content-Type", "application/json")
		reply, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		reply.Body.Close()
		if reply.StatusCode != 202 {
			t.Fatal(reply.StatusCode)
		}
		if _, _, err = readSSE(reader); err != nil {
			t.Fatal(err)
		}
		if got := s.Resources.Subscribers("test://one"); !reflect.DeepEqual(got, []string{id}) {
			t.Fatal(got)
		}
		h.mu.Lock()
		session := h.sessions[id]
		h.mu.Unlock()
		h.remove(id, session)
		if got := s.Resources.Subscribers("test://one"); len(got) != 0 {
			t.Fatal("subscription leaked", got)
		}
		w := httpRequest(h, "POST", "/message?sessionId="+id, body, nil)
		if w.Code != 404 {
			t.Fatal("disconnected session accepted", w.Code)
		}
		_, _, _, next := connectSSE(t, h)
		if next == id {
			t.Fatal("legacy reconnect reused session")
		}
	}
}

type failingSSEFlush struct {
	sseWriter
	flushes, failFlush int
}

func (w *failingSSEFlush) FlushError() error {
	w.flushes++
	if w.flushes == w.failFlush {
		return io.ErrClosedPipe
	}
	return nil
}
func TestLegacySSEFlushFailures(t *testing.T) {
	for _, fail := range []int{1, 2} {
		_, h := testSSE(t, SyncSSE)
		r := httptest.NewRequest("GET", "/sse", nil)
		r.Header.Set("Authorization", "Bearer reader")
		w := &failingSSEFlush{failFlush: fail}
		h.ServeHTTP(w, r)
		if len(h.sessions) != 0 || w.flushes != fail {
			t.Fatal("flush error leaked", w.flushes)
		}
	}
	_, h := testSSE(t, AsyncSSE)
	r := httptest.NewRequest("GET", "/sse", nil)
	r.Header.Set("Authorization", "Bearer reader")
	var ack chan error
	w := &failingSSEFlush{failFlush: 2}
	w.onWrite = func(data string) {
		if !strings.HasPrefix(data, "event: endpoint") {
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		for _, session := range h.sessions {
			ack = make(chan error, 1)
			session.pending = append(session.pending, &sseDelivery{payload: []byte("event: message\ndata: {}\n\n"), ack: ack})
			session.wake <- struct{}{}
		}
	}
	h.ServeHTTP(w, r)
	if err := <-ack; !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("flush failure not acknowledged", err)
	}
}

func TestAsyncSSETransferEncodingRouteOrder(t *testing.T) {
	// _sse_read_http_request does not reject Transfer-Encoding itself. POST
	// /message rejects it; unrelated paths still produce 404 after parsing.
	_, h := testSSE(t, AsyncSSE)
	if w := httpRequest(h, "GET", "/missing", "", map[string]string{"Transfer-Encoding": "chunked"}); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
