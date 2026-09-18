package umcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// HTTPHooks are trusted embedding hooks; errors/panics become empty HTTP 500s.
// Authentication runs for each request, even when a session ID is supplied.
type HTTPHooks struct {
	Authenticate func(context.Context, string, string, map[string]string, string) (*Principal, error)
	Authorize    func(context.Context, *Principal, any, any) (bool, error)
	Route        func(context.Context, string, string, map[string]string, []byte, string) (*HTTPResponse, error)
}

// HTTPOptions configures the source Streamable HTTP defaults. The caller must
// configure network read/write timeouts on its net/http.Server as well.
type HTTPOptions struct {
	// AsyncReference preserves aioumcp's auth and routing order. The default
	// false preserves the synchronous MCPServer behaviour.
	AsyncReference  bool
	Endpoint        string
	AllowedOrigins  []string
	LocalBind       bool
	MaxRequestBytes int64
	SessionTTL      time.Duration
	MaxSessions     int
	Keepalive       time.Duration
}

// DefaultHTTPOptions mirrors source HTTP transport configuration.
func DefaultHTTPOptions() HTTPOptions {
	return HTTPOptions{Endpoint: "/mcp", LocalBind: true, MaxRequestBytes: 4 << 20, SessionTTL: 30 * time.Minute, MaxSessions: 1024, Keepalive: 15 * time.Second}
}

// StreamableHTTP is an http.Handler with authenticated persistent sessions.
// net/http handles TCP framing before this handler; raw-parser rejection parity
// (including duplicate Host parsing) is a separate transport-server gate.
type StreamableHTTP struct {
	server   *Server
	hooks    HTTPHooks
	options  HTTPOptions
	sessions *sessionStore
}

// NewStreamableHTTP attaches one notification sink to a server. Do not attach
// multiple transports to the same server without an explicit multiplexing sink.
func NewStreamableHTTP(server *Server, options HTTPOptions, hooks HTTPHooks) (*StreamableHTTP, error) {
	if server == nil || options.Endpoint == "" || !strings.HasPrefix(options.Endpoint, "/") || options.MaxRequestBytes <= 0 || options.SessionTTL <= 0 || options.MaxSessions <= 0 || options.Keepalive <= 0 {
		return nil, fmt.Errorf("invalid Streamable HTTP configuration")
	}
	options.AllowedOrigins = append([]string{}, options.AllowedOrigins...)
	handler := &StreamableHTTP{server: server, hooks: hooks, options: options, sessions: newSessionStore(options.SessionTTL, options.MaxSessions, server.Resources.DropSession)}
	server.SetNotifier(func(method string, params map[string]any) error { return handler.Notify(nil, method, params) })
	return handler, nil
}

// Close terminates event streams and releases subscriptions/session state.
func (h *StreamableHTTP) Close() { h.sessions.close() }

// Notify delivers only to currently open event streams, dropping on full queues.
func (h *StreamableHTTP) Notify(recipients []string, method string, params map[string]any) error {
	message := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		message["params"] = params
	}
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	payload := append([]byte("event: message\ndata: "), data...)
	payload = append(payload, '\n', '\n')
	var targets map[string]bool
	if recipients != nil {
		targets = map[string]bool{}
		for _, id := range recipients {
			targets[id] = true
		}
	}
	h.sessions.broadcast(payload, targets)
	return nil
}
func cors(w http.ResponseWriter, origin string) {
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
		w.Header().Set("Vary", "Origin")
	}
}
func emptyHTTP(w http.ResponseWriter, status int, origin string) {
	cors(w, origin)
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(status)
}
func jsonHTTP(w http.ResponseWriter, status int, value any, origin string) {
	raw, err := json.Marshal(value)
	if err != nil {
		emptyHTTP(w, 500, origin)
		return
	}
	cors(w, origin)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
func protocolSupported(version string) bool {
	for _, v := range ProtocolVersions() {
		if version == v {
			return true
		}
	}
	return false
}
func headersLower(r *http.Request) map[string]string {
	out := map[string]string{}
	if parsed := syncRequest(r); parsed != nil {
		for key, value := range parsed.Last {
			out[key] = value
		}
		return out
	}
	for name := range r.Header {
		out[strings.ToLower(name)] = r.Header.Get(name)
	}
	if r.Host != "" {
		out["host"] = r.Host
	}
	return out
}
func badHeaders(r *http.Request) bool {
	if parsed := syncRequest(r); parsed != nil {
		return HasSingletonHeaderViolations(parsed.Counts, parsed.Version) || HasAmbiguousSingletonValues(parsed.Last)
	}
	counts := map[string]int{}
	for name, values := range r.Header {
		counts[strings.ToLower(name)] = len(values)
	}
	if r.Host != "" {
		counts["host"]++
	}
	return HasSingletonHeaderViolations(counts, r.Proto) || HasAmbiguousSingletonValues(headersLower(r))
}
func peer(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
func hookCall[T any](call func() (T, error)) (value T, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("transport hook panicked")
		}
	}()
	return call()
}
func (h *StreamableHTTP) authenticate(r *http.Request) (*Principal, error) {
	if h.hooks.Authenticate == nil {
		return &Principal{Name: "anonymous"}, nil
	}
	return hookCall(func() (*Principal, error) {
		return h.hooks.Authenticate(r.Context(), r.Method, requestTarget(r), headersLower(r), peer(r))
	})
}
func (h *StreamableHTTP) authorize(r *http.Request, p *Principal, method, tool any) (bool, error) {
	if h.hooks.Authorize == nil {
		return true, nil
	}
	return hookCall(func() (bool, error) { return h.hooks.Authorize(r.Context(), p, method, tool) })
}
func (h *StreamableHTTP) body(w http.ResponseWriter, r *http.Request, origin string) ([]byte, bool) {
	if len(r.TransferEncoding) > 0 || r.Header.Get("Transfer-Encoding") != "" || syncRequest(r) != nil && r.ContentLength < 0 {
		forceSyncClose(w, r)
		emptyHTTP(w, 400, origin)
		return nil, false
	}
	if r.ContentLength > h.options.MaxRequestBytes {
		forceSyncClose(w, r)
		emptyHTTP(w, 413, origin)
		return nil, false
	}
	if r.Body == nil {
		return []byte{}, true
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, h.options.MaxRequestBytes+1))
	if err != nil {
		status := 400
		if e, ok := err.(net.Error); ok && e.Timeout() && syncRequest(r) == nil {
			status = 408
		}
		forceSyncClose(w, r)
		emptyHTTP(w, status, origin)
		return nil, false
	}
	if int64(len(data)) > h.options.MaxRequestBytes {
		forceSyncClose(w, r)
		emptyHTTP(w, 413, origin)
		return nil, false
	}
	if r.ContentLength >= 0 && int64(len(data)) != r.ContentLength {
		forceSyncClose(w, r)
		emptyHTTP(w, 400, origin)
		return nil, false
	}
	return data, true
}
func (h *StreamableHTTP) version(w http.ResponseWriter, r *http.Request, origin string) bool {
	value := r.Header.Get("Mcp-Protocol-Version")
	if protocolSupported(value) {
		return true
	}
	var version *string
	if _, exists := r.Header[http.CanonicalHeaderKey("Mcp-Protocol-Version")]; exists {
		version = &value
	}
	jsonHTTP(w, 400, ProtocolVersionError(version), origin)
	return false
}
func (h *StreamableHTTP) principal(w http.ResponseWriter, r *http.Request, origin string) (*Principal, bool) {
	p, err := h.authenticate(r)
	if err != nil {
		emptyHTTP(w, 500, origin)
		return nil, false
	}
	if p == nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		emptyHTTP(w, 401, origin)
		return nil, false
	}
	return p, true
}
func (h *StreamableHTTP) validated(w http.ResponseWriter, r *http.Request, p *Principal, required bool, origin string) (*httpSession, bool) {
	session, status := h.sessions.validate(r.Header.Get("Mcp-Session-Id"), p.Name, r.Header.Get("Mcp-Protocol-Version"), required)
	if status != 200 {
		if status == 400 && r.Header.Get("Mcp-Session-Id") != "" {
			version := r.Header.Get("Mcp-Protocol-Version")
			jsonHTTP(w, 400, ProtocolVersionError(&version), origin)
		} else {
			emptyHTTP(w, status, origin)
		}
		return nil, false
	}
	return session, true
}
func (h *StreamableHTTP) auxiliary(w http.ResponseWriter, r *http.Request, path, origin string) {
	body, ok := h.body(w, r, origin)
	if !ok {
		return
	}
	// Auxiliary routes own authentication, matching handle_http_request in uMCP.
	var err error
	var response *HTTPResponse
	if h.hooks.Route != nil {
		response, err = hookCall(func() (*HTTPResponse, error) {
			return h.hooks.Route(r.Context(), r.Method, path, headersLower(r), body, peer(r))
		})
		if err != nil {
			emptyHTTP(w, 500, origin)
			return
		}
	}
	if response == nil {
		w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
		emptyHTTP(w, 405, origin)
		return
	}
	if !ValidateHTTPResponse(response, int(h.options.MaxRequestBytes)) {
		emptyHTTP(w, 500, origin)
		return
	}
	cors(w, origin)
	if response.ContentType != nil {
		w.Header().Set("Content-Type", *response.ContentType)
	}
	for _, header := range response.Headers {
		w.Header().Add(header[0], header[1])
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(response.Body)))
	w.WriteHeader(response.Status)
	_, _ = w.Write(response.Body)
}

// ServeHTTP implements source method/status ordering. Authentication is not
// inferred from a session cookie, principal argument or discovery metadata.
func (h *StreamableHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	origin := ""
	if value := r.Header.Get("Origin"); value != "" {
		allowed, err := OriginIsAllowed(value, h.options.AllowedOrigins, h.options.LocalBind, r.Host)
		if err == nil && allowed {
			origin = value
		}
	}
	if badHeaders(r) {
		forceSyncClose(w, r)
		emptyHTTP(w, 400, origin)
		return
	}
	if r.Header.Get("Origin") != "" && origin == "" {
		forceSyncClose(w, r)
		emptyHTTP(w, 403, "")
		return
	}
	path, err := RequestTargetPath(requestTarget(r))
	if err != nil {
		emptyHTTP(w, 400, origin)
		return
	}
	if h.options.AsyncReference {
		body, ok := h.body(w, r, origin)
		if !ok {
			return
		}
		r = r.Clone(r.Context())
		r.Body = io.NopCloser(bytes.NewReader(body))
		if path != h.options.Endpoint {
			h.auxiliary(w, r, path, origin)
			return
		}
	}
	if r.Method == http.MethodOptions {
		if path != h.options.Endpoint {
			h.auxiliary(w, r, path, origin)
			return
		}
		if r.Header.Get("Origin") == "" {
			w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
			emptyHTTP(w, 405, origin)
			return
		}
		cors(w, origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, MCP-Protocol-Version, Mcp-Session-Id, Last-Event-ID, Authorization")
		emptyHTTP(w, 204, origin)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete {
		status := 501
		if h.options.AsyncReference {
			status = 405
			w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
		}
		emptyHTTP(w, status, origin)
		return
	}
	if path != h.options.Endpoint {
		h.auxiliary(w, r, path, origin)
		return
	}
	var principal *Principal
	if h.options.AsyncReference {
		var ok bool
		principal, ok = h.principal(w, r, origin)
		if !ok {
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		h.get(w, r, origin, principal)
	case http.MethodDelete:
		h.delete(w, r, origin, principal)
	case http.MethodPost:
		h.post(w, r, origin, principal)
	}
}
func (h *StreamableHTTP) post(w http.ResponseWriter, r *http.Request, origin string, principal *Principal) {
	body, ok := h.body(w, r, origin)
	if !ok {
		return
	}
	if !ContentTypeIsJSON(r.Header.Get("Content-Type")) {
		emptyHTTP(w, 415, origin)
		return
	}
	if !MediaAcceptsJSON(r.Header.Get("Accept")) {
		emptyHTTP(w, 406, origin)
		return
	}
	if principal == nil {
		principal, ok = h.principal(w, r, origin)
		if !ok {
			return
		}
	}
	value, valid := decode(body)
	if !valid || !utf8.Valid(body) {
		jsonHTTP(w, 200, failure(nil, -32700, "Parse error"), origin)
		return
	}
	request, ok := value.(map[string]any)
	if !ok {
		jsonHTTP(w, 200, failure(nil, -32600, "Invalid Request"), origin)
		return
	}
	method := request["method"]
	initialize := method == "initialize"
	if !initialize && !h.version(w, r, origin) {
		return
	}
	id := r.Header.Get("Mcp-Session-Id")
	if initialize && id != "" {
		emptyHTTP(w, 400, origin)
		return
	}
	if !initialize {
		if _, ok = h.validated(w, r, principal, false, origin); !ok {
			return
		}
	}
	if method == nil && ValidJSONRPCResponse(body) {
		emptyHTTP(w, 202, origin)
		return
	}
	params, _ := request["params"].(map[string]any)
	var tool any
	if method == "tools/call" {
		tool = params["name"]
	}
	allowed, err := h.authorize(r, principal, method, tool)
	if err != nil {
		emptyHTTP(w, 500, origin)
		return
	}
	if !allowed {
		emptyHTTP(w, 403, origin)
		return
	}
	if id != "" && (method == "resources/subscribe" || method == "resources/unsubscribe") {
		if params == nil {
			params = map[string]any{}
		}
		params["_session_id"] = id
		request["params"] = params
		body, _ = json.Marshal(request)
	}
	metadata := RequestContext{Transport: "streamable-http", ProtocolVersion: r.Header.Get("Mcp-Protocol-Version"), SessionID: id, Principal: principal.Name, Peer: peer(r), Headers: headersLower(r)}
	response, err := hookCall(func() (*Response, error) { return h.server.Process(r.Context(), body, metadata) })
	if err != nil {
		emptyHTTP(w, 500, origin)
		return
	}
	_, hasID := request["id"]
	if method != nil && !hasID || response == nil {
		emptyHTTP(w, 202, origin)
		return
	}
	if initialize && response.Error == nil {
		if result, ok := response.Result.(map[string]any); ok {
			if version, ok := result["protocolVersion"].(string); ok && protocolSupported(version) {
				created, status, err := h.sessions.create(principal.Name, version)
				if err != nil || status != 200 {
					emptyHTTP(w, status, origin)
					return
				}
				w.Header().Set("Mcp-Session-Id", created)
			}
		}
	}
	jsonHTTP(w, 200, response, origin)
}
func (h *StreamableHTTP) delete(w http.ResponseWriter, r *http.Request, origin string, principal *Principal) {
	if _, ok := h.body(w, r, origin); !ok {
		return
	}
	if !h.version(w, r, origin) {
		return
	}
	if principal == nil {
		var ok bool
		principal, ok = h.principal(w, r, origin)
		if !ok {
			return
		}
	}
	session, ok := h.validated(w, r, principal, true, origin)
	if !ok {
		return
	}
	emptyHTTP(w, h.sessions.remove(r.Header.Get("Mcp-Session-Id"), session), origin)
}
func (h *StreamableHTTP) get(w http.ResponseWriter, r *http.Request, origin string, principal *Principal) {
	if !MediaAcceptsEventStream(r.Header.Get("Accept")) {
		emptyHTTP(w, 406, origin)
		return
	}
	if !h.version(w, r, origin) {
		return
	}
	if principal == nil {
		var ok bool
		principal, ok = h.principal(w, r, origin)
		if !ok {
			return
		}
	}
	session, ok := h.validated(w, r, principal, true, origin)
	if !ok {
		return
	}
	id := r.Header.Get("Mcp-Session-Id")
	disconnect, status := h.sessions.attach(id, session)
	if status != 200 {
		emptyHTTP(w, status, origin)
		return
	}
	defer h.sessions.detach(id, session)
	flusher, ok := w.(http.Flusher)
	if !ok {
		emptyHTTP(w, 500, origin)
		return
	}
	cors(w, origin)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	if _, err := io.WriteString(w, ": connected\n\n"); err != nil {
		return
	}
	if err := flushHTTP(flusher); err != nil {
		return
	}
	ticker := time.NewTicker(h.options.Keepalive)
	defer ticker.Stop()
	for {
		var payload []byte
		select {
		case <-r.Context().Done():
			return
		case <-disconnect:
			return
		case payload = <-session.queue:
		case <-ticker.C:
			payload = []byte(": keepalive\n\n")
		}
		if !h.writeEvent(w, flusher, disconnect, payload, id, session) {
			return
		}
	}
}

func (h *StreamableHTTP) writeEvent(w io.Writer, flusher http.Flusher, disconnect <-chan struct{}, payload []byte, id string, session *httpSession) bool {
	select {
	case <-disconnect:
		return false
	default:
	}
	if _, err := w.Write(payload); err != nil {
		return false
	}
	if err := flushHTTP(flusher); err != nil {
		return false
	}
	return h.sessions.touch(id, session)
}

func flushHTTP(flusher http.Flusher) error {
	if f, ok := flusher.(interface{ FlushError() error }); ok {
		return f.FlushError()
	}
	flusher.Flush()
	return nil
}
