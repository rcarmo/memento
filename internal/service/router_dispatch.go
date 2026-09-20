package service

import (
	"bytes"
	"encoding/json"
	"errors"
)

func routeResultMap(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var result map[string]any
	_ = decoder.Decode(&result)
	if result == nil {
		return nil, errors.New("routed result must be an object")
	}
	return result, nil
}
func projectRoutedEnvelope(value any, projection *RouteProjection) (map[string]any, SuccessOptions, error) {
	result, err := routeResultMap(value)
	if err != nil {
		return nil, SuccessOptions{}, err
	}
	if result["status"] == "success" && projection != nil {
		data, ok := result["data"].(map[string]any)
		if !ok {
			return nil, SuccessOptions{}, errors.New("routed result data must be an object")
		}
		projected, projectErr := ProjectRouteResult(data, *projection)
		if projectErr != nil {
			return nil, SuccessOptions{}, &ChangeValidationError{projectErr.Error()}
		}
		result["data"] = projected
	}
	options := SuccessOptions{}
	if text, ok := result["repo_revision"].(string); ok {
		options.RepoRevision = &text
	}
	if text, ok := result["index_revision"].(string); ok {
		options.IndexRevision = &text
	}
	options.IndexStale, _ = result["index_stale"].(bool)
	if text, ok := result["operation_id"].(string); ok {
		options.OperationID = &text
	}
	if values, ok := result["warnings"].([]any); ok {
		for _, raw := range values {
			if text, ok := raw.(string); ok {
				options.Warnings = append(options.Warnings, text)
			}
		}
	}
	return result, options, nil
}
