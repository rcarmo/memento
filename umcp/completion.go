package umcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
)

// CompletionProvider receives copies of source prefix/context/ref/argument data.
// A result is a list or object containing values and optional total/hasMore.
type CompletionProvider func(context.Context, string, map[string]any, map[string]any, map[string]any) (any, error)
type completionKey struct{ kind, name, argument string }

// Completions resolves registered prompt/template signatures and enum/provider values.
type Completions struct {
	Prompts   *PromptRegistry
	Resources *ResourceRegistry
	MaxValues int
	mu        sync.RWMutex
	providers map[completionKey]CompletionProvider
}

// Register replaces a provider keyed by canonical reference identity/argument.
func (c *Completions) Register(kind, name, argument string, provider CompletionProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.providers == nil {
		c.providers = make(map[completionKey]CompletionProvider)
	}
	c.providers[completionKey{kind, name, argument}] = provider
}

// Available implements get_config's dynamic completions capability decision.
func (c *Completions) Available() bool {
	c.mu.RLock()
	providers := len(c.providers) > 0
	c.mu.RUnlock()
	if providers {
		return true
	}
	if c.Prompts != nil {
		c.Prompts.mu.RLock()
		has := len(c.Prompts.prompts) > 0
		c.Prompts.mu.RUnlock()
		if has {
			return true
		}
	}
	if c.Resources != nil {
		c.Resources.mu.RLock()
		has := len(c.Resources.templates) > 0
		c.Resources.mu.RUnlock()
		return has
	}
	return false
}

func (c *Completions) target(ref map[string]any) (string, string, []Parameter, map[string]any, error) {
	kind, _ := ref["type"].(string)
	switch kind {
	case "ref/prompt":
		name, ok := ref["name"].(string)
		if !ok || name == "" {
			return "", "", nil, nil, fmt.Errorf("Invalid completion ref: prompt name is required")
		}
		if c.Prompts != nil {
			c.Prompts.mu.RLock()
			p, ok := c.Prompts.prompts[name]
			c.Prompts.mu.RUnlock()
			if ok {
				return kind, name, p.Parameters, p.InputSchema, nil
			}
		}
		return "", "", nil, nil, fmt.Errorf("Unknown prompt ref: %s", name)
	case "ref/resource":
		raw := ref["uri"]
		if !truthy(raw) {
			raw = ref["uriTemplate"]
		}
		if !truthy(raw) {
			raw = ref["name"]
		}
		name, ok := raw.(string)
		if !ok || name == "" {
			return "", "", nil, nil, fmt.Errorf("Invalid completion ref: resource template identifier is required")
		}
		if c.Resources != nil {
			c.Resources.mu.RLock()
			defer c.Resources.mu.RUnlock()
			for _, entry := range c.Resources.templates {
				r := entry.resource
				if r.Name == name || r.URI == name {
					params := r.Parameters
					if len(params) == 0 {
						params = make([]Parameter, len(entry.names))
						for i, name := range entry.names {
							params[i] = Parameter{Name: name, Types: []ParamType{StringParam}}
						}
					}
					return kind, r.URI, params, r.InputSchema, nil
				}
			}
		}
		return "", "", nil, nil, fmt.Errorf("Unknown resource template ref: %s", name)
	default:
		return "", "", nil, nil, fmt.Errorf("Invalid completion ref: unsupported ref type")
	}
}
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case json.Number:
		r, ok := numberRat(v)
		return !ok || r.Sign() != 0
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}
func pythonString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return pythonValueRepr(orderedSchemaValue(value))
}
func positiveInteger(value any) (*big.Int, bool) {
	number, ok := value.(json.Number)
	if !ok || !integer.MatchString(number.String()) {
		return nil, false
	}
	n, _ := new(big.Int).SetString(number.String(), 10)
	return n, n.Sign() > 0
}
func completionResult(raw any) ([]string, *big.Int, *bool, error) {
	values := raw
	var total, more any
	if object, ok := mapObject(raw); ok {
		values = object["values"]
		if _, exists := object["values"]; !exists {
			values = []any{}
		}
		total = object["total"]
		more = object["hasMore"]
	}
	list, ok := values.([]any)
	if !ok {
		return nil, nil, nil, ArgumentError("Completion provider must return a list or {'values': [...]} result")
	}
	out := []string{}
	seen := map[string]bool{}
	for _, value := range list {
		text := pythonString(value)
		if !seen[text] {
			seen[text] = true
			out = append(out, text)
		}
	}
	var t *big.Int
	if total != nil {
		n, ok := total.(json.Number)
		if !ok || !integer.MatchString(n.String()) {
			return nil, nil, nil, ArgumentError("Completion provider returned invalid total")
		}
		t, _ = new(big.Int).SetString(n.String(), 10)
		if t.Sign() < 0 {
			return nil, nil, nil, ArgumentError("Completion provider returned invalid total")
		}
	}
	var m *bool
	if more != nil {
		b, ok := more.(bool)
		if !ok {
			return nil, nil, nil, ArgumentError("Completion provider returned invalid hasMore")
		}
		m = &b
	}
	return out, t, m, nil
}

// Complete handles completion/complete with filtering, stable deduplication and caps.
func (c *Completions) Complete(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	invalid := func(message string) (any, *RPCError, error) {
		return nil, &RPCError{Code: -32602, Message: message}, nil
	}
	ref, ok := params["ref"].(map[string]any)
	if !ok {
		return invalid("Invalid params: 'ref' must be an object")
	}
	argument, ok := params["argument"].(map[string]any)
	if !ok {
		return invalid("Invalid params: 'argument' must be an object")
	}
	contextValue := params["context"]
	if !truthy(contextValue) {
		contextValue = map[string]any{}
	}
	contextArgs, ok := contextValue.(map[string]any)
	if !ok {
		return invalid("Invalid params: 'context' must be an object")
	}
	maximum := c.MaxValues
	if maximum == 0 {
		maximum = 100
	}
	requested := params["maxValues"]
	if _, exists := params["maxValues"]; !exists {
		requested = json.Number(fmt.Sprint(maximum))
	}
	limit, ok := positiveInteger(requested)
	if !ok {
		return invalid("Invalid params: 'maxValues' must be a positive integer")
	}
	name, ok := argument["name"].(string)
	if !ok || name == "" {
		return invalid("Invalid params: 'argument.name' must be a non-empty string")
	}
	prefix := ""
	if raw := argument["value"]; raw != nil {
		prefix = pythonString(raw)
	}
	rawArgs := contextArgs["arguments"]
	if !truthy(rawArgs) {
		rawArgs = map[string]any{}
	}
	args, ok := rawArgs.(map[string]any)
	if !ok {
		return invalid("Invalid params: 'context.arguments' must be an object")
	}
	kind, reference, parameters, schema, err := c.target(ref)
	if err != nil {
		return invalid(err.Error())
	}
	var parameter *Parameter
	for i := range parameters {
		if parameters[i].Name == name {
			parameter = &parameters[i]
			break
		}
	}
	if parameter == nil {
		return invalid("Unknown completion argument: " + name)
	}
	var values []any
	if props, ok := schema["properties"].(map[string]any); ok {
		if property, ok := props[name].(map[string]any); ok {
			values, _ = property["enum"].([]any)
		}
	}
	if len(values) == 0 {
		values = parameter.Enum
	}
	stringsOut := []string{}
	for _, value := range values {
		stringsOut = append(stringsOut, pythonString(value))
	}
	c.mu.RLock()
	provider := c.providers[completionKey{kind, reference, name}]
	c.mu.RUnlock()
	var total *big.Int
	var hasMore *bool
	if provider != nil {
		output, err := invokeTool(ctx, func(ctx context.Context, _ map[string]any) (any, error) {
			return provider(ctx, prefix, cloneMap(args), cloneMap(ref), cloneMap(argument))
		}, nil)
		if err != nil {
			if e, ok := err.(ArgumentError); ok {
				return invalid(e.Error())
			}
			return nil, &RPCError{Code: -32603, Message: "Completion provider failed"}, nil
		}
		provided, t, m, err := completionResult(output)
		if err != nil {
			return invalid(err.Error())
		}
		stringsOut = append(stringsOut, provided...)
		total = t
		hasMore = m
	}
	filtered := []string{}
	seen := map[string]bool{}
	for _, value := range stringsOut {
		if prefix != "" && !strings.HasPrefix(value, prefix) {
			continue
		}
		if !seen[value] {
			seen[value] = true
			filtered = append(filtered, value)
		}
	}
	count := len(filtered)
	if total == nil || total.Cmp(big.NewInt(int64(count))) < 0 {
		total = big.NewInt(int64(count))
	}
	n := min(count, maximum)
	if limit.Cmp(big.NewInt(int64(n))) < 0 {
		n = int(limit.Int64())
	}
	if n < 0 {
		n = max(0, count+n)
	}
	more := total.Cmp(big.NewInt(int64(n))) > 0
	if hasMore != nil {
		more = *hasMore
	}
	completion := map[string]any{"values": filtered[:n], "hasMore": more}
	if more || total.Cmp(big.NewInt(int64(n))) != 0 {
		completion["total"] = json.Number(total.String())
	}
	return map[string]any{"completion": completion}, nil, nil
}
