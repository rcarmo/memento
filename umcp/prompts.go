package umcp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Prompt separates registered discovery metadata from the callable's original
// description/schema. The Python source uses callable docs again on prompts/get,
// even if register_prompt overrides discovery description/categories.
type Prompt struct {
	Name, Description, CallableDescription string
	InputSchema                            map[string]any
	Categories                             []string
	Parameters                             []Parameter
	Call                                   func(context.Context, map[string]any) (any, error)
}

// PromptRegistry mirrors dynamic prompts, using explicit callable metadata.
type PromptRegistry struct {
	mu              sync.RWMutex
	prompts         map[string]Prompt
	Notify          func(string, map[string]any) error
	DefaultPageSize int
}

var categoryLine = regexp.MustCompile(`(?i)^\s*categor(?:y|ies):\s*(.+)$`)
var categoryBracket = regexp.MustCompile(`(?i)\[(?:categor(?:y|ies)):\s*([^\]]+)\]`)

// PromptCategories parses the source's optional docstring metadata line.
func PromptCategories(doc string) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			token := strings.ToLower(strings.TrimSpace(part))
			if token != "" && !seen[token] {
				out = append(out, token)
				seen[token] = true
			}
		}
	}
	for _, line := range strings.FieldsFunc(doc, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\v' || r == '\f' || r == 0x85 || r == 0x2028 || r == 0x2029 || r >= 0x1c && r <= 0x1e
	}) {
		if matches := categoryLine.FindStringSubmatch(line); matches != nil {
			add(matches[1])
		}
		for _, matches := range categoryBracket.FindAllStringSubmatch(line, -1) {
			add(matches[1])
		}
	}
	return out
}
func parameterSchema(types []ParamType) map[string]any {
	if len(types) == 0 {
		return map[string]any{"type": "string"}
	}
	if len(types) == 1 {
		if types[0] == AnyParam {
			return map[string]any{}
		}
		return map[string]any{"type": string(types[0])}
	}
	branches := make([]any, 0, len(types))
	for _, kind := range types {
		branches = append(branches, parameterSchema([]ParamType{kind}))
	}
	return map[string]any{"oneOf": branches}
}

func clonePrompt(p Prompt) Prompt {
	p.InputSchema = cloneMap(p.InputSchema)
	p.Categories = append([]string{}, p.Categories...)
	tool := cloneTool(Tool{Parameters: p.Parameters})
	p.Parameters = tool.Parameters
	return p
}

// Register stores a prompt without executing it.
func (r *PromptRegistry) Register(prompt Prompt) error {
	if prompt.Call == nil {
		return errors.New("prompt handler is required")
	}
	categoriesExplicit := prompt.Categories != nil
	prompt = clonePrompt(prompt)
	if prompt.CallableDescription == "" {
		prompt.CallableDescription = "Prompt template " + prompt.Name
	}
	if prompt.Description == "" {
		prompt.Description = prompt.CallableDescription
	}
	if !categoriesExplicit {
		prompt.Categories = PromptCategories(prompt.Description)
	}
	if len(prompt.InputSchema) == 0 {
		properties := map[string]any{}
		required := []any{}
		for _, p := range prompt.Parameters {
			schema := parameterSchema(p.Types)
			if p.Enum != nil {
				schema["enum"] = cloneJSON(p.Enum)
			}
			properties[p.Name] = schema
			if !p.HasDefault {
				required = append(required, p.Name)
			}
		}
		prompt.InputSchema = map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			prompt.InputSchema["required"] = required
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.prompts == nil {
		r.prompts = make(map[string]Prompt)
	}
	r.prompts[prompt.Name] = prompt
	return nil
}

// Unregister removes a dynamic prompt, returning whether it existed.
func (r *PromptRegistry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.prompts[name]
	delete(r.prompts, name)
	return ok
}

// RegisterAndNotify emits the corresponding list_changed notification.
func (r *PromptRegistry) RegisterAndNotify(p Prompt) error {
	if err := r.Register(p); err != nil {
		return err
	}
	if r.Notify != nil {
		return r.Notify("notifications/prompts/list_changed", nil)
	}
	return nil
}

// UnregisterAndNotify emits only when a prompt was actually removed.
func (r *PromptRegistry) UnregisterAndNotify(name string) (bool, error) {
	removed := r.Unregister(name)
	if removed && r.Notify != nil {
		return true, r.Notify("notifications/prompts/list_changed", nil)
	}
	return removed, nil
}

// List returns source-compatible sorted prompt descriptions and cursors.
func (r *PromptRegistry) List(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	r.mu.RLock()
	prompts := make([]Prompt, 0, len(r.prompts))
	for _, p := range r.prompts {
		prompts = append(prompts, clonePrompt(p))
	}
	pageSize := r.DefaultPageSize
	r.mu.RUnlock()
	sort.Slice(prompts, func(i, j int) bool { return prompts[i].Name < prompts[j].Name })
	items := make([]DiscoveryItem, 0, len(prompts))
	for _, p := range prompts {
		meta := map[string]any{"name": p.Name, "description": p.Description, "inputSchema": p.InputSchema}
		meta["categories"] = p.Categories
		items = append(items, DiscoveryItem{Identity: []string{p.Name}, Value: meta})
	}
	result, err := ListPage(ctx, "prompts", "prompts", items, params, pageSize)
	return result, err, nil
}
func mapObject(value any) (map[string]any, bool) {
	if m, ok := value.(map[string]any); ok {
		return m, true
	}
	if m, ok := value.(OrderedObject); ok {
		out := make(map[string]any, len(m))
		for _, member := range m {
			out[member.Name] = member.Value
		}
		return out, true
	}
	return nil, false
}
func messagesValid(list []any) bool {
	for _, v := range list {
		object, ok := mapObject(v)
		if !ok {
			return false
		}
		_, role := object["role"]
		_, content := object["content"]
		if !role || !content {
			return false
		}
	}
	return true
}

// PromptResult normalises supported JSON return shapes without inventing messages.
func PromptResult(description string, categories []string, value any) (map[string]any, error) {
	result := map[string]any{"description": description}
	if len(categories) > 0 {
		result["categories"] = categories
	}
	text, plain := value.(string)
	if plain {
		result["messages"] = []any{map[string]any{"role": "user", "content": map[string]any{"type": "text", "text": text}}}
		return result, nil
	}
	if list, ok := value.([]any); ok && messagesValid(list) {
		result["messages"] = list
		return result, nil
	}
	if object, ok := mapObject(value); ok {
		if _, ok := object["messages"].([]any); ok {
			merged := cloneMap(object)
			if _, ok := merged["description"]; !ok {
				merged["description"] = description
			}
			if _, ok := merged["categories"]; !ok && len(categories) > 0 {
				merged["categories"] = categories
			}
			return merged, nil
		}
	}
	raw, err := pythonJSON(value)
	if err != nil {
		return nil, err
	}
	result["messages"] = []any{map[string]any{"role": "user", "content": map[string]any{"type": "text", "text": string(raw)}}}
	return result, nil
}

// Get invokes a prompt, preserving the distinct required-argument/error messages.
func (r *PromptRegistry) Get(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	value := params["name"]
	if value == nil || value == "" || value == false {
		return nil, &RPCError{Code: -32602, Message: "Missing required parameter 'name'"}, nil
	}
	name, ok := value.(string)
	if !ok {
		return nil, nil, errors.New("prompt name must be a string")
	}
	r.mu.RLock()
	prompt, ok := r.prompts[name]
	r.mu.RUnlock()
	if !ok {
		return nil, &RPCError{Code: -32601, Message: "Prompt not found: " + name}, nil
	}
	args := map[string]any{}
	if raw := params["arguments"]; raw != nil {
		var ok bool
		args, ok = raw.(map[string]any)
		if !ok {
			return nil, nil, errors.New("prompt arguments must be an object")
		}
	}
	allowed := map[string]bool{}
	for _, p := range prompt.Parameters {
		allowed[p.Name] = true
	}
	unknown := []string{}
	for name := range args {
		if !allowed[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return nil, &RPCError{Code: -32602, Message: "Unrecognized parameter(s): " + strings.Join(unknown, ", ")}, nil
	}
	kwargs := map[string]any{}
	for _, p := range prompt.Parameters {
		if value, ok := args[p.Name]; ok {
			kwargs[p.Name] = coerce(value, p.Types)
		} else if p.HasDefault {
			kwargs[p.Name] = cloneJSON(p.Default)
		} else {
			return nil, &RPCError{Code: -32602, Message: fmt.Sprintf("Missing required argument '%s' for prompt %s", p.Name, name)}, nil
		}
	}
	output, err := invokeTool(ctx, prompt.Call, kwargs)
	if errors.Is(err, ErrRequestCancelled) {
		return nil, nil, err
	}
	if err != nil {
		return nil, &RPCError{Code: -32603, Message: RemoteSafeFailure(ctx, "Prompt execution failed", "Prompt execution error: "+err.Error())}, nil
	}
	result, err := PromptResult(prompt.CallableDescription, PromptCategories(prompt.CallableDescription), output)
	if err != nil {
		return nil, nil, err
	}
	return result, nil, nil
}
