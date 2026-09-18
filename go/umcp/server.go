package umcp

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
)

// Server composes implemented uMCP method registries with request dispatch.
// It is protocol infrastructure, not the Memento service. Transports provide
// trusted contexts and an appropriate authenticated notification sink.
type Server struct {
	Name, Version, Instructions string
	Tools                       ToolRegistry
	Prompts                     PromptRegistry
	Resources                   ResourceRegistry
	Completions                 Completions
	dispatcher                  Dispatcher
	mu                          sync.RWMutex
	loggingLevel                string
	notify                      func(string, map[string]any) error
	notificationMu              sync.Mutex
	notificationOutput          io.Writer
}

// NewServer uses the pinned uMCP defaults and wires only implemented methods.
func NewServer(name string) *Server {
	if name == "" {
		name = "MCPServer"
	}
	s := &Server{Name: name, Version: "0.2.2", Instructions: "This server provides tool functionality via the Model Context Protocol.", loggingLevel: "info", notificationOutput: os.Stdout}
	s.Completions.Prompts = &s.Prompts
	s.Completions.Resources = &s.Resources
	s.dispatcher = Dispatcher{Handlers: map[string]Handler{
		"initialize": s.initialize, "tools/list": s.Tools.List, "tools/call": s.Tools.Call,
		"prompts/list": s.Prompts.List, "prompts/get": s.Prompts.Get,
		"resources/list": s.Resources.List, "resources/templates/list": s.Resources.ListTemplates, "resources/read": s.Resources.Read,
		"resources/subscribe": s.Resources.Subscribe, "resources/unsubscribe": s.Resources.Unsubscribe,
		"completion/complete": s.Completions.Complete, "logging/setLevel": s.setLoggingLevel,
	}, Notify: s.send}
	s.Tools.Notify = s.send
	s.Prompts.Notify = s.send
	return s
}

// SetNotifier changes the delivery sink under a lock. The sink decides transport
// framing, buffering and session recipients; no principal is accepted from args.
func (s *Server) SetNotifier(notify func(string, map[string]any) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notify = notify
}
func (s *Server) send(method string, params map[string]any) error {
	s.mu.RLock()
	notify := s.notify
	s.mu.RUnlock()
	if notify != nil {
		return notify(method, params)
	}
	return s.writeNotification(method, params)
}

// SetNotificationOutput sets the source stdout fallback for file/TCP/stdio and
// legacy SSE without a successful recipient. Nil deliberately disables output.
// Active Streamable HTTP suppresses fallback even with no connected streams.
func (s *Server) SetNotificationOutput(output io.Writer) {
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()
	s.notificationOutput = output
}
func (s *Server) writeNotification(method string, params map[string]any) error {
	message := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		message["params"] = params
	}
	s.notificationMu.Lock()
	defer s.notificationMu.Unlock()
	if s.notificationOutput == nil {
		return nil
	}
	return encodeLine(s.notificationOutput, message)
}

// Process applies the common reference validation/context rules.
func (s *Server) Process(ctx context.Context, input []byte, trusted RequestContext) (*Response, error) {
	return s.dispatcher.Process(ctx, input, trusted)
}

// Config reports source-compatible initialisation metadata and dynamic capability state.
func (s *Server) Config() map[string]any {
	capabilities := map[string]any{"tools": map[string]any{"listChanged": true}, "prompts": map[string]any{"get": true, "listChanged": true}, "resources": map[string]any{"subscribe": true, "listChanged": true}, "logging": map[string]any{}}
	if s.Completions.Available() {
		capabilities["completions"] = map[string]any{}
	}
	return map[string]any{"protocolVersion": ProtocolVersions()[0], "serverInfo": map[string]any{"name": s.Name, "version": s.Version}, "capabilities": capabilities, "instructions": s.Instructions}
}
func (s *Server) initialize(_ context.Context, params map[string]any) (any, *RPCError, error) {
	config := s.Config()
	var accepted *string
	if v, ok := params["protocolVersion"].(string); ok {
		accepted = &v
	}
	config["protocolVersion"] = ExactOrFallback(accepted, ProtocolVersions()[0])
	return config, nil, nil
}

var logLevels = []string{"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency"}

func levelIndex(level string) int {
	for i, value := range logLevels {
		if value == level {
			return i
		}
	}
	return -1
}
func (s *Server) setLoggingLevel(_ context.Context, params map[string]any) (any, *RPCError, error) {
	level, _ := params["level"].(string)
	if levelIndex(level) < 0 {
		return nil, &RPCError{Code: -32602, Message: "Invalid params: 'level' must be one of debug, info, notice, warning, error, critical, alert, emergency"}, nil
	}
	s.mu.Lock()
	s.loggingLevel = level
	s.mu.Unlock()
	return map[string]any{}, nil, nil
}

var secretKey = regexp.MustCompile(`(?i)(?:pass(word)?|secret|token|api[_-]?key|auth(orization)?|cookie|session|credential)`)
var secretValue = regexp.MustCompile(`(?i)(bearer\s+[A-Za-z0-9._~+/=-]+|sk-[A-Za-z0-9]+|api[_-]?key\s*[:=]\s*\S+|token\s*[:=]\s*\S+)`)

// SanitizeLogData recursively redacts the same key/value patterns as uMCP.
func SanitizeLogData(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			if secretKey.MatchString(key) {
				out[key] = "[redacted]"
			} else {
				out[key] = SanitizeLogData(item)
			}
		}
		return out
	case OrderedObject:
		out := make(OrderedObject, len(v))
		for i, m := range v {
			out[i] = Member{Name: m.Name}
			if secretKey.MatchString(m.Name) {
				out[i].Value = "[redacted]"
			} else {
				out[i].Value = SanitizeLogData(m.Value)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = SanitizeLogData(item)
		}
		return out
	case string:
		return secretValue.ReplaceAllString(v, "[redacted]")
	default:
		return value
	}
}

// LogMessage honours logging/setLevel and source sanitisation. It propagates
// notification write failures; callers must not recursively log those failures.
func (s *Server) LogMessage(level string, data any, logger string, sanitize bool) error {
	s.mu.RLock()
	threshold := s.loggingLevel
	s.mu.RUnlock()
	index := levelIndex(level)
	if index < 0 || index < levelIndex(threshold) {
		return nil
	}
	if sanitize {
		data = SanitizeLogData(data)
	}
	params := map[string]any{"level": level, "data": data}
	if logger != "" {
		params["logger"] = logger
	}
	return s.send("notifications/message", params)
}

// RawResult normalises source JSON data into insertion-preserving values for
// registered method adapters (unlike decoding into Go maps and losing order).
func RawResult(value json.RawMessage) (any, error) { return ParseValue(value) }

// ResourceUpdateRecipients follows the reference's preference for session
// subscriptions over legacy-global subscriptions. Delivery belongs to transport.
func (s *Server) ResourceUpdateRecipients(uri string) []string {
	all := s.Resources.Subscribers(uri)
	sessions := []string{}
	global := false
	for _, id := range all {
		if id == "" {
			global = true
		} else {
			sessions = append(sessions, id)
		}
	}
	if len(sessions) > 0 {
		return sessions
	}
	if global {
		return []string{""}
	}
	return []string{}
}

// NotifyResourceUpdated delivers to an explicitly selected recipient set.
func (s *Server) NotifyResourceUpdated(uri string, deliver func([]string, string, map[string]any) error) error {
	ids := s.ResourceUpdateRecipients(uri)
	if len(ids) == 0 {
		return nil
	}
	return deliver(ids, "notifications/resources/updated", map[string]any{"uri": strings.Clone(uri)})
}
