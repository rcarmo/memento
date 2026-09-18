package umcp

import (
	"encoding/json"
	"fmt"
)

// ValidateSchemaSubset reproduces uMCP's small output-schema validator, not the
// whole JSON Schema standard. In particular oneOf accepts the first successful
// branch, just like anyOf in the source. Values come from ParseValue; schemas
// come from ordinary JSON maps. No additional keywords are silently enforced.
func ValidateSchemaSubset(value any, schema map[string]any, path string) error {
	if len(schema) == 0 {
		return nil
	}
	if choices, exists := schema["enum"]; exists {
		found := false
		if list, ok := choices.([]any); ok {
			for _, choice := range list {
				if pythonEqual(value, orderedSchemaValue(choice)) {
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("%s must be one of %s", path, pythonValueRepr(orderedSchemaValue(choices)))
		}
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		if branches, ok := schema[key].([]any); ok && len(branches) > 0 {
			var first error
			for _, branch := range branches {
				object, ok := branch.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid schema branch")
				}
				err := ValidateSchemaSubset(value, object, path)
				if err == nil {
					return nil
				}
				if first == nil {
					first = err
				}
			}
			return first
		}
	}
	expected := schema["type"]
	if kinds, ok := expected.([]any); ok {
		var first error
		for _, kind := range kinds {
			copy := make(map[string]any, len(schema))
			for k, v := range schema {
				copy[k] = v
			}
			copy["type"] = kind
			err := ValidateSchemaSubset(value, copy, path)
			if err == nil {
				return nil
			}
			if first == nil {
				first = err
			}
		}
		if first == nil {
			return fmt.Errorf("empty schema type list")
		}
		return first
	}
	kind, _ := expected.(string)
	switch kind {
	case "null":
		if value != nil {
			return fmt.Errorf("%s must be null", path)
		}
		return nil
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s must be a string", path)
		}
		return nil
	case "integer":
		if n, ok := value.(json.Number); !ok || !integer.MatchString(n.String()) {
			return fmt.Errorf("%s must be an integer", path)
		}
		return nil
	case "number":
		if _, ok := value.(json.Number); !ok {
			return fmt.Errorf("%s must be a number", path)
		}
		return nil
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", path)
		}
		return nil
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		if itemSchema, ok := schema["items"].(map[string]any); ok {
			for i, item := range items {
				if err := ValidateSchemaSubset(item, itemSchema, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	_, propertiesPresent := schema["properties"]
	_, requiredPresent := schema["required"]
	if kind == "object" || propertiesPresent || requiredPresent {
		object, ok := value.(OrderedObject)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		properties, _ := schema["properties"].(map[string]any)
		required, _ := schema["required"].([]any)
		for _, raw := range required {
			key, ok := raw.(string)
			if !ok {
				return fmt.Errorf("invalid required property")
			}
			if _, found := lookup(object, key); !found {
				return fmt.Errorf("%s.%s is required", path, key)
			}
		}
		additional := schema["additionalProperties"]
		for _, member := range object {
			if child, found := properties[member.Name]; found {
				childSchema, ok := child.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid property schema")
				}
				if err := ValidateSchemaSubset(member.Value, childSchema, path+"."+member.Name); err != nil {
					return err
				}
			} else if additional == false {
				return fmt.Errorf("%s.%s is not allowed", path, member.Name)
			} else if child, ok := additional.(map[string]any); ok {
				if err := ValidateSchemaSubset(member.Value, child, path+"."+member.Name); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ParseSchema decodes schema keywords as maps while retaining ordered enum
// values whose Python repr appears in validation errors.
func ParseSchema(raw []byte) (map[string]any, error) {
	value, err := ParseValue(raw)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	object, ok := value.(OrderedObject)
	if !ok {
		return nil, fmt.Errorf("schema must be an object")
	}
	return schemaObject(object), nil
}
func schemaObject(object OrderedObject) map[string]any {
	out := make(map[string]any, len(object))
	for _, m := range object {
		if m.Name == "enum" {
			out[m.Name] = m.Value
		} else {
			out[m.Name] = schemaTree(m.Value)
		}
	}
	return out
}
func schemaTree(value any) any {
	switch v := value.(type) {
	case OrderedObject:
		return schemaObject(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = schemaTree(item)
		}
		return out
	default:
		return value
	}
}

// orderedSchemaValue adapts JSON map/number values to the structured value domain.
// Schema enum object order does not affect Python equality; sort via JSON marshal.
func orderedSchemaValue(value any) any {
	switch value.(type) {
	case OrderedObject, []any, json.Number:
		return value
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	// json.Marshal already produced valid JSON with supported scalar numbers.
	result, _ := ParseValue(raw)
	return result
}

// FormatToolResult returns the Python content/structuredContent shape for JSON
// values. Return OrderedObject for insertion-sensitive mappings; arbitrary Go
// structs/native objects must be normalised by method adapters, not guessed.
func FormatToolResult(value any, outputSchema map[string]any) (map[string]any, error) {
	if outputSchema != nil {
		if err := ValidateSchemaSubset(value, outputSchema, "$"); err != nil {
			return nil, err
		}
	}
	var text string
	if s, ok := value.(string); ok {
		text = s
	} else {
		raw, err := pythonJSON(value)
		if err != nil {
			return nil, err
		}
		text = string(raw)
	}
	result := map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
	_, object := value.(OrderedObject)
	_, isString := value.(string)
	if object || outputSchema != nil && !isString {
		result["structuredContent"] = value
	}
	return result, nil
}
