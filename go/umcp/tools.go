package umcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// ParamType is an explicit source-signature type, independent of JSON Schema.
// uMCP coerces by callable signature, not by a registered schema override.
type ParamType string

const (
	AnyParam     ParamType = "any"
	StringParam  ParamType = "string"
	IntegerParam ParamType = "integer"
	NumberParam  ParamType = "number"
	BooleanParam ParamType = "boolean"
	ObjectParam  ParamType = "object"
	ArrayParam   ParamType = "array"
)

// Parameter preserves declaration order, required/default distinction and union order.
type Parameter struct {
	Name       string
	Types      []ParamType
	Enum       []any
	Default    any
	HasDefault bool
}

// Tool describes a registered handler. Go APIs require explicit signature/schema
// metadata rather than pretending reflection can reproduce Python annotations.
type Tool struct {
	Name, Description string
	InputSchema       map[string]any
	OutputSchema      map[string]any
	Annotations       map[string]any
	Parameters        []Parameter
	Call              func(context.Context, map[string]any) (any, error)
}

// ArgumentError matches ValueError from the reference tool handler.
type ArgumentError string

func (e ArgumentError) Error() string { return string(e) }

// ExecutionError allows a compatibility adapter to preserve source exception type
// and message for stdio; network transports redact these details.
type ExecutionError struct{ Type, Message string }

func (e ExecutionError) Error() string { return e.Message }

// ToolRegistry is concurrent-safe for registration/list/call. A handler never
// runs under the registry lock. Registered definitions are defensively copied.
type ToolRegistry struct {
	mu              sync.RWMutex
	tools           map[string]Tool
	Notify          func(string, map[string]any) error
	DefaultPageSize int
	// Visible optionally filters discovery using trusted request context. It
	// never authorises calls; handlers and transport hooks retain that boundary.
	Visible func(context.Context, Tool) bool
}

// InferToolAnnotations mirrors name-based source hints, including read precedence.
func InferToolAnnotations(name string) map[string]any {
	read, destructive := false, false
	for _, prefix := range []string{"list_", "get_", "read_", "inspect_", "extract_", "query_", "search_", "check_", "audit_", "fetch_", "calculate_", "recommend_"} {
		if strings.HasPrefix(name, prefix) || strings.Contains(name, "_"+strings.TrimSuffix(prefix, "_")) {
			read = true
		}
	}
	for _, prefix := range []string{"delete_", "clear_", "cleanup_", "restart_"} {
		if strings.HasPrefix(name, prefix) || strings.Contains(name, "_"+strings.TrimSuffix(prefix, "_")) {
			destructive = true
		}
	}
	result := map[string]any{"readOnlyHint": read, "destructiveHint": !read && destructive}
	if strings.HasPrefix(name, "web_") || strings.HasPrefix(name, "azure_") {
		result["openWorldHint"] = true
	}
	return result
}

func cloneJSON(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, x := range v {
			out[key] = cloneJSON(x)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = cloneJSON(x)
		}
		return out
	case OrderedObject:
		out := make(OrderedObject, len(v))
		for i, m := range v {
			out[i] = Member{m.Name, cloneJSON(m.Value)}
		}
		return out
	default:
		return v
	}
}
func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	return cloneJSON(value).(map[string]any)
}
func cloneTool(tool Tool) Tool {
	tool.InputSchema = cloneMap(tool.InputSchema)
	tool.OutputSchema = cloneMap(tool.OutputSchema)
	tool.Annotations = cloneMap(tool.Annotations)
	params := make([]Parameter, len(tool.Parameters))
	for i, p := range tool.Parameters {
		p.Types = append([]ParamType{}, p.Types...)
		if p.Enum != nil {
			p.Enum = cloneJSON(p.Enum).([]any)
		}
		p.Default = cloneJSON(p.Default)
		params[i] = p
	}
	tool.Parameters = params
	return tool
}

// Register replaces the dynamic tool by name. Caller-supplied empty description
// falls back to the source's default. Invalid Go handlers are rejected explicitly.
func (r *ToolRegistry) Register(tool Tool) error {
	if tool.Call == nil {
		return errors.New("tool handler is required")
	}
	tool = cloneTool(tool)
	if tool.Description == "" {
		tool.Description = "Execute " + tool.Name + " tool"
	}
	if tool.InputSchema == nil {
		tool.InputSchema = map[string]any{}
	}
	if tool.Annotations == nil {
		tool.Annotations = InferToolAnnotations(tool.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tools == nil {
		r.tools = make(map[string]Tool)
	}
	r.tools[tool.Name] = tool
	return nil
}

// Unregister removes only a dynamic registration, reporting whether it existed.
func (r *ToolRegistry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.tools[name]
	delete(r.tools, name)
	return exists
}

// RegisterAndNotify emits after successful registration; notify failure does not roll back.
func (r *ToolRegistry) RegisterAndNotify(tool Tool) error {
	if err := r.Register(tool); err != nil {
		return err
	}
	if r.Notify != nil {
		return r.Notify("notifications/tools/list_changed", nil)
	}
	return nil
}

// UnregisterAndNotify emits only when a registration was removed.
func (r *ToolRegistry) UnregisterAndNotify(name string) (bool, error) {
	removed := r.Unregister(name)
	if removed && r.Notify != nil {
		return true, r.Notify("notifications/tools/list_changed", nil)
	}
	return removed, nil
}

// List returns sorted source-compatible metadata and principal-bound pagination.
func (r *ToolRegistry) List(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	r.mu.RLock()
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, cloneTool(tool))
	}
	pageSize := r.DefaultPageSize
	visible := r.Visible
	r.mu.RUnlock()
	if visible != nil {
		filtered := tools[:0]
		for _, tool := range tools {
			if visible(ctx, cloneTool(tool)) {
				filtered = append(filtered, tool)
			}
		}
		tools = filtered
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	items := make([]DiscoveryItem, 0, len(tools))
	for _, tool := range tools {
		meta := map[string]any{"name": tool.Name, "description": tool.Description, "inputSchema": tool.InputSchema}
		if tool.OutputSchema != nil {
			meta["outputSchema"] = tool.OutputSchema
		}
		if len(tool.Annotations) > 0 {
			meta["annotations"] = tool.Annotations
		}
		items = append(items, DiscoveryItem{Identity: []string{tool.Name}, Value: meta})
	}
	result, err := ListPage(ctx, "tools", "tools", items, params, pageSize)
	return result, err, nil
}

// Call reproduces parameter selection/coercion/defaults and output formatting.
// Invalid non-object argument containers expose a handler error, not a fabricated
// successful reply; transport hardening remains responsible for remote failures.
func (r *ToolRegistry) Call(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	nameValue := params["name"]
	if nameValue == nil || nameValue == "" || nameValue == false {
		return nil, &RPCError{Code: -32602, Message: "Missing 'name' parameter"}, nil
	}
	name, ok := nameValue.(string)
	if !ok {
		return nil, nil, errors.New("tool name must be a string")
	}
	arguments := map[string]any{}
	if value, exists := params["arguments"]; exists {
		var ok bool
		arguments, ok = value.(map[string]any)
		if !ok {
			return nil, nil, errors.New("tool arguments must be an object")
		}
	}
	r.mu.RLock()
	tool, exists := r.tools[name]
	r.mu.RUnlock()
	if !exists {
		return nil, &RPCError{Code: -32601, Message: "Tool not found: " + name}, nil
	}
	declared := map[string]bool{}
	for _, p := range tool.Parameters {
		declared[p.Name] = true
	}
	unknown := []string{}
	for key := range arguments {
		if !declared[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return nil, &RPCError{Code: -32602, Message: "Unrecognized parameter(s): " + strings.Join(unknown, ", ")}, nil
	}
	kwargs := make(map[string]any, len(tool.Parameters))
	for _, p := range tool.Parameters {
		if value, ok := arguments[p.Name]; ok {
			kwargs[p.Name] = coerce(value, p.Types)
		} else if p.HasDefault {
			kwargs[p.Name] = cloneJSON(p.Default)
		} else {
			return nil, &RPCError{Code: -32602, Message: fmt.Sprintf("Required parameter '%s' is missing", p.Name)}, nil
		}
	}
	output, err := invokeTool(ctx, tool.Call, kwargs)
	if errors.Is(err, ErrRequestCancelled) {
		return nil, nil, err
	}
	if err != nil {
		var argument ArgumentError
		if errors.As(err, &argument) {
			return nil, &RPCError{Code: -32602, Message: string(argument)}, nil
		}
		var source ExecutionError
		kind := "error"
		if errors.As(err, &source) {
			kind = source.Type
		}
		detail := fmt.Sprintf("Tool execution error for %s: %s: %s", name, kind, err.Error())
		return nil, &RPCError{Code: -32603, Message: RemoteSafeFailure(ctx, "Tool execution failed", detail)}, nil
	}
	result, err := FormatToolResult(output, tool.OutputSchema)
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: RemoteSafeFailure(ctx, "Tool output validation failed", fmt.Sprintf("Tool output validation failed for %s: %s", name, err))}, nil
	}
	return result, nil, nil
}

func invokeTool(ctx context.Context, call func(context.Context, map[string]any) (any, error), args map[string]any) (result any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = ExecutionError{Type: "GoPanic", Message: fmt.Sprint(recovered)}
		}
	}()
	return call(ctx, args)
}

var pythonInt = regexp.MustCompile(`^[+-]?[0-9](?:_?[0-9])*$`)

func coercedNumber(text string, kind ParamType) (any, bool) {
	clean := normalizeDecimalDigits(strings.TrimSpace(text))
	if kind == IntegerParam {
		if !pythonInt.MatchString(clean) {
			return nil, false
		}
		n, _ := new(big.Int).SetString(strings.ReplaceAll(clean, "_", ""), 10)
		return json.Number(n.String()), true
	}
	if !qualityNumber.MatchString(clean) {
		return nil, false
	}
	value := pythonQuality(clean)
	// Source nonfinite values survive coercion. json.Number retains their text in
	// formatted content; structured JSON nonfinite replies remain an explicit gap.
	return json.Number(pythonFloat(value)), true
}
func matchesType(value any, kind ParamType) bool {
	switch kind {
	case AnyParam:
		return false // isinstance(value, typing.Any) is not used as a scalar match.
	case StringParam:
		_, ok := value.(string)
		return ok
	case IntegerParam:
		if _, ok := value.(bool); ok {
			return true
		}
		n, ok := value.(json.Number)
		return ok && integer.MatchString(n.String())
	case NumberParam:
		n, ok := value.(json.Number)
		return ok && !integer.MatchString(n.String())
	case BooleanParam:
		_, ok := value.(bool)
		return ok
	case ObjectParam:
		_, ok := value.(map[string]any)
		return ok
	case ArrayParam:
		_, ok := value.([]any)
		return ok
	default:
		return false
	}
}
func coerce(value any, types []ParamType) any {
	if value == nil {
		return nil
	}
	if len(types) > 1 {
		for _, kind := range types {
			if matchesType(value, kind) {
				return value
			}
		}
		if text, ok := value.(string); ok {
			for _, kind := range types {
				if kind == IntegerParam || kind == NumberParam {
					if number, ok := coercedNumber(text, kind); ok {
						return number
					}
				}
			}
		}
		return value
	}
	if len(types) == 0 {
		return value
	}
	if text, ok := value.(string); ok {
		switch types[0] {
		case IntegerParam, NumberParam:
			if number, ok := coercedNumber(text, types[0]); ok {
				return number
			}
		case BooleanParam:
			lower := strings.ToLower(text)
			return lower == "true" || lower == "1" || lower == "yes"
		}
	}
	return value
}
