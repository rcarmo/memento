package umcp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// SSEMode preserves the two pinned legacy transport policies. SyncSSE queues
// responses without a bound; AsyncSSE waits for the event-stream write before
// acknowledging POST. Neither legacy implementation supports resumable streams.
type SSEMode uint8

const (
	SyncSSE SSEMode = iota
	AsyncSSE
)

type SSEOptions struct {
	Mode            SSEMode
	AllowedOrigins  []string
	LocalBind       bool
	MaxRequestBytes int64
	Keepalive       time.Duration
}

func DefaultSSEOptions(mode SSEMode) SSEOptions {
	return SSEOptions{Mode: mode, LocalBind: true, MaxRequestBytes: 4 << 20, Keepalive: 15 * time.Second}
}

type sseDelivery struct {
	payload []byte
	ack     chan error
}
type legacySession struct {
	owner   string
	pending []*sseDelivery
	wake    chan struct{}
	closed  chan struct{}
}

// LegacySSE implements /sse and /message, including principal binding and
// resource subscription cleanup. The network server must enforce timeouts and
// raw HTTP parsing rules separately. It owns the server notification sink.
type LegacySSE struct {
	server   *Server
	common   StreamableHTTP
	options  SSEOptions
	mu       sync.Mutex
	sessions map[string]*legacySession
	closed   bool
	token    func() (string, error)
}

func NewLegacySSE(server *Server, options SSEOptions, hooks HTTPHooks) (*LegacySSE, error) {
	if server == nil || options.Mode > AsyncSSE || options.MaxRequestBytes <= 0 || options.Keepalive <= 0 {
		return nil, errors.New("invalid legacy SSE configuration")
	}
	options.AllowedOrigins = append([]string{}, options.AllowedOrigins...)
	h := &LegacySSE{server: server, common: StreamableHTTP{hooks: hooks}, options: options, sessions: map[string]*legacySession{}, token: randomSessionID}
	server.SetNotifier(func(method string, params map[string]any) error { return h.Notify(nil, method, params) })
	return h, nil
}

func sseEmpty(w http.ResponseWriter, status int, origin string) {
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(status)
}
func (h *LegacySSE) principal(w http.ResponseWriter, r *http.Request, origin string) (*Principal, bool) {
	p, err := h.common.authenticate(r)
	if err != nil {
		sseEmpty(w, 500, origin)
		return nil, false
	}
	if p == nil {
		w.Header().Set("WWW-Authenticate", "Bearer")
		sseEmpty(w, 401, origin)
		return nil, false
	}
	return p, true
}
func (h *LegacySSE) authorized(w http.ResponseWriter, r *http.Request, p *Principal, method, tool any, origin string) bool {
	allowed, err := h.common.authorize(r, p, method, tool)
	if err != nil {
		sseEmpty(w, 500, origin)
		return false
	}
	if !allowed {
		sseEmpty(w, 403, origin)
		return false
	}
	return true
}
func (h *LegacySSE) limits(w http.ResponseWriter, r *http.Request, origin string) bool {
	if r.ContentLength < 0 {
		sseEmpty(w, 400, origin)
		return false
	}
	if r.ContentLength > h.options.MaxRequestBytes {
		sseEmpty(w, 413, origin)
		return false
	}
	return true
}
func (h *LegacySSE) body(w http.ResponseWriter, r *http.Request, origin string) ([]byte, bool) {
	var body []byte
	var err error
	if r.Body != nil {
		body, err = io.ReadAll(io.LimitReader(r.Body, h.options.MaxRequestBytes+1))
	}
	if err != nil || int64(len(body)) != r.ContentLength {
		sseEmpty(w, 400, origin)
		return nil, false
	}
	return body, true
}

func (h *LegacySSE) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The async reference parser consumes and validates the body before routing,
	// origin checks or authentication, even for GET and OPTIONS.
	var body []byte
	var ok bool
	if h.options.Mode == AsyncSSE {
		if badHeaders(r) {
			sseEmpty(w, 400, "")
			return
		}
		if !h.limits(w, r, "") {
			return
		}
		body, ok = h.body(w, r, "")
		if !ok {
			return
		}
	}
	origin := ""
	if value := r.Header.Get("Origin"); value != "" {
		allowed, err := OriginIsAllowed(value, h.options.AllowedOrigins, h.options.LocalBind, r.Host)
		if err == nil && allowed {
			origin = value
		}
	}
	if badHeaders(r) {
		sseEmpty(w, 400, origin)
		return
	}
	if r.Header.Get("Origin") != "" && origin == "" {
		sseEmpty(w, 403, "")
		return
	}
	path, err := RequestTargetPath(requestTarget(r))
	if err != nil {
		sseEmpty(w, 400, origin)
		return
	}
	if r.Method == http.MethodOptions {
		if path != "/sse" && path != "/message" {
			sseEmpty(w, 404, origin)
			return
		}
		if origin == "" {
			w.Header().Set("Allow", "GET, POST, OPTIONS")
			sseEmpty(w, 405, origin)
			return
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization")
		sseEmpty(w, 204, origin)
		return
	}
	if r.Method == http.MethodGet && path == "/sse" {
		h.get(w, r, origin)
		return
	}
	if r.Method == http.MethodPost && path == "/message" {
		h.post(w, r, origin, body)
		return
	}
	status := 404
	if h.options.Mode == SyncSSE && r.Method != http.MethodGet && r.Method != http.MethodPost {
		status = 501
	}
	sseEmpty(w, status, origin)
}

func (h *LegacySSE) get(w http.ResponseWriter, r *http.Request, origin string) {
	if !MediaAcceptsEventStream(r.Header.Get("Accept")) {
		sseEmpty(w, 406, origin)
		return
	}
	p, ok := h.principal(w, r, origin)
	if !ok {
		return
	}
	if !h.authorized(w, r, p, nil, nil, origin) {
		return
	}
	if _, ok := w.(http.Flusher); !ok {
		sseEmpty(w, 500, origin)
		return
	}
	id, err := h.token()
	if err != nil {
		sseEmpty(w, 500, origin)
		return
	}
	session := &legacySession{owner: p.Name, wake: make(chan struct{}, 1), closed: make(chan struct{})}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		sseEmpty(w, 503, origin)
		return
	}
	h.sessions[id] = session
	h.mu.Unlock()
	defer h.remove(id, session)
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	if _, err = io.WriteString(w, "event: endpoint\ndata: /message?sessionId="+id+"\n\n"); err != nil {
		return
	}
	controller := http.NewResponseController(w)
	if err = controller.Flush(); err != nil {
		return
	}
	ticker := time.NewTicker(h.options.Keepalive)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-session.closed:
			return
		case <-ticker.C:
			if _, err = w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			if err = controller.Flush(); err != nil {
				return
			}
		case <-session.wake:
			for {
				h.mu.Lock()
				if len(session.pending) == 0 {
					h.mu.Unlock()
					break
				}
				delivery := session.pending[0]
				session.pending[0] = nil
				session.pending = session.pending[1:]
				h.mu.Unlock()
				_, err = w.Write(delivery.payload)
				if err == nil {
					err = controller.Flush()
				}
				if delivery.ack != nil {
					delivery.ack <- err
				}
				if err != nil {
					return
				}
				if h.options.Mode == SyncSSE {
					// Queue.get(timeout=15) restarts its idle timeout after
					// each sync message; async's keepalive is periodic.
					ticker.Reset(h.options.Keepalive)
				}
			}
		}
	}
}

// Python parse_qs keeps the first nonblank sessionId, accepts semicolons inside
// values, and replaces malformed UTF-8. url.Values.Get alone differs on blanks.
func legacySessionID(query string) string {
	for _, part := range strings.Split(query, "&") {
		name, value, ok := strings.Cut(part, "=")
		if !ok || value == "" {
			continue
		}
		name = replaceInvalidUTF8(unquoteQuery(name))
		if name == "sessionId" {
			return replaceInvalidUTF8(unquoteQuery(value))
		}
	}
	return ""
}
func unquoteQuery(value string) string {
	// QueryUnescape rejects malformed percent escapes. Python preserves those
	// escapes while decoding other valid pairs in the same value.
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '%' && i+2 < len(value) {
			if decoded, err := url.QueryUnescape(value[i : i+3]); err == nil {
				out.WriteString(decoded)
				i += 3
				continue
			}
		}
		if value[i] == '+' {
			out.WriteByte(' ')
		} else {
			out.WriteByte(value[i])
		}
		i++
	}
	return out.String()
}
func (h *LegacySSE) post(w http.ResponseWriter, r *http.Request, origin string, body []byte) {
	if len(r.TransferEncoding) > 0 || r.Header.Get("Transfer-Encoding") != "" {
		sseEmpty(w, 400, origin)
		return
	}
	if !ContentTypeIsJSON(r.Header.Get("Content-Type")) {
		sseEmpty(w, 415, origin)
		return
	}
	if !MediaAcceptsJSON(r.Header.Get("Accept")) {
		sseEmpty(w, 406, origin)
		return
	}
	if h.options.Mode == SyncSSE && !h.limits(w, r, origin) {
		return
	}
	id := legacySessionID(r.URL.RawQuery)
	p, ok := h.principal(w, r, origin)
	if !ok {
		return
	}
	h.mu.Lock()
	session := h.sessions[id]
	h.mu.Unlock()
	if session == nil {
		sseEmpty(w, 404, origin)
		return
	}
	if session.owner != p.Name {
		sseEmpty(w, 403, origin)
		return
	}
	if h.options.Mode == SyncSSE {
		body, ok = h.body(w, r, origin)
		if !ok {
			return
		}
	}
	if !utf8.Valid(body) {
		sseEmpty(w, 400, origin)
		return
	}
	value, _ := decode(body)
	request, _ := value.(map[string]any)
	method := request["method"]
	params, _ := request["params"].(map[string]any)
	var tool any
	if method == "tools/call" {
		tool = params["name"]
	}
	if !h.authorized(w, r, p, method, tool, origin) {
		return
	}
	if method == "resources/subscribe" || method == "resources/unsubscribe" {
		if params == nil {
			params = map[string]any{}
		}
		params["_session_id"] = id
		request["params"] = params
		body, _ = json.Marshal(request)
	}
	if h.options.Mode == AsyncSSE {
		h.mu.Lock()
		active := h.sessions[id] == session
		h.mu.Unlock()
		if !active {
			sseEmpty(w, 404, origin)
			return
		}
	}
	metadata := RequestContext{Transport: "sse", SessionID: id, Principal: p.Name, Peer: peer(r), Headers: headersLower(r)}
	response, err := hookCall(func() (*Response, error) { return h.server.Process(r.Context(), body, metadata) })
	if err != nil {
		sseEmpty(w, 500, origin)
		return
	}
	if response != nil {
		payload, err := ssePayload(response)
		if err != nil {
			sseEmpty(w, 500, origin)
			return
		}
		if err = h.deliver(id, session, payload); err != nil {
			sseEmpty(w, 404, origin)
			return
		}
	} else if h.options.Mode == SyncSSE {
		h.mu.Lock()
		active := h.sessions[id] == session
		h.mu.Unlock()
		if !active {
			sseEmpty(w, 404, origin)
			return
		}
	}
	sseEmpty(w, 202, origin)
}
func ssePayload(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(append([]byte("event: message\ndata: "), raw...), '\n', '\n'), nil
}
func (h *LegacySSE) deliver(id string, session *legacySession, payload []byte) error {
	delivery := &sseDelivery{payload: payload}
	if h.options.Mode == AsyncSSE {
		delivery.ack = make(chan error, 1)
	}
	h.mu.Lock()
	if h.sessions[id] != session {
		h.mu.Unlock()
		return errors.New("SSE session closed")
	}
	session.pending = append(session.pending, delivery)
	select {
	case session.wake <- struct{}{}:
	default:
	}
	h.mu.Unlock()
	if delivery.ack != nil {
		select {
		case err := <-delivery.ack:
			return err
		case <-session.closed:
			return errors.New("SSE session closed")
		}
	}
	return nil
}
func (h *LegacySSE) remove(id string, session *legacySession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[id] != session {
		return
	}
	delete(h.sessions, id)
	session.pending = nil
	close(session.closed)
	h.server.Resources.DropSession(id)
}

// Close terminates streams and releases any waiting asynchronous deliveries.
func (h *LegacySSE) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for id, session := range h.sessions {
		delete(h.sessions, id)
		session.pending = nil
		close(session.closed)
		h.server.Resources.DropSession(id)
	}
}

// Notify sends to every connected legacy stream or an explicit recipient list.
func (h *LegacySSE) Notify(recipients []string, method string, params map[string]any) error {
	message := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		message["params"] = params
	}
	payload, err := ssePayload(message)
	if err != nil {
		return err
	}
	targets := map[string]*legacySession{}
	h.mu.Lock()
	if recipients == nil {
		for id, session := range h.sessions {
			targets[id] = session
		}
	} else {
		for _, id := range recipients {
			if session := h.sessions[id]; session != nil {
				targets[id] = session
			}
		}
	}
	h.mu.Unlock()
	delivered := false
	for id, session := range targets {
		if err := h.deliver(id, session, payload); err == nil {
			delivered = true
		}
	}
	if !delivered {
		return h.server.writeNotification(method, params)
	}
	return nil
}
