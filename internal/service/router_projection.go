package service

import (
	"errors"
	"strconv"
	"strings"
)

type RouteProjection struct {
	Ref    string
	Fields []string
	Limit  *int
}

func ResolveRouteProjection(payload any, ref string) (any, error) {
	current := payload
	for _, part := range strings.Split(ref, ".") {
		switch value := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = value[part]
			if !ok {
				return nil, errors.New("routed projection field not found: " + ref)
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value) {
				return nil, errors.New("routed projection field not found: " + ref)
			}
			current = value[index]
		default:
			return nil, errors.New("routed projection field not found: " + ref)
		}
	}
	return current, nil
}
func ProjectRouteResult(payload map[string]any, projection RouteProjection) (map[string]any, error) {
	value, err := ResolveRouteProjection(payload, projection.Ref)
	if err != nil {
		return nil, err
	}
	if len(projection.Fields) > 0 {
		if rows, ok := value.([]any); ok {
			limit := len(rows)
			if projection.Limit != nil && *projection.Limit < limit {
				limit = *projection.Limit
			}
			selected := []any{}
			for _, raw := range rows[:limit] {
				row, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				item := map[string]any{}
				for _, field := range projection.Fields {
					if fieldValue, exists := row[field]; exists {
						item[field] = fieldValue
					}
				}
				selected = append(selected, item)
			}
			value = selected
		}
	} else if projection.Limit != nil {
		if rows, ok := value.([]any); ok {
			limit := min(len(rows), *projection.Limit)
			value = rows[:limit]
		}
	}
	return map[string]any{"value": value}, nil
}
func routeProjection(expansion map[string]any) *RouteProjection {
	raw, ok := expansion["projection"].(map[string]any)
	if !ok {
		return nil
	}
	projection := &RouteProjection{}
	projection.Ref, _ = raw["ref"].(string)
	if fields, ok := raw["fields"].([]any); ok {
		for _, field := range fields {
			if text, ok := field.(string); ok {
				projection.Fields = append(projection.Fields, text)
			}
		}
	}
	if limit, ok := raw["limit"].(int); ok {
		projection.Limit = &limit
	}
	return projection
}
