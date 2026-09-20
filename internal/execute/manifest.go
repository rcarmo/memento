package execute

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/rcarmo/memento/internal/pydatetime"
)

var manifestDigest = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func intPointer(value int) *int { return &value }

func (a *Arguments) validateManifest(args map[string]any, strict bool) (map[string]any, error) {
	issues := []ValidationIssue{}
	result := map[string]any{"path_prefix": "/", "match": nil, "include_asset_metadata": false}
	if value, ok := args["path_prefix"]; ok {
		result["path_prefix"] = validateArgumentField(argumentField{Type: "string", MinLength: intPointer(1), MaxLength: intPointer(1024)}, value, "args.path_prefix", strict, &issues)
	}
	if value, ok := args["items"]; ok {
		result["items"] = validateManifestItems(value, strict, &issues)
	} else {
		addIssue(&issues, "args.items", "Field required")
	}
	if value, ok := args["match"]; ok {
		result["match"] = validateManifestMatch(value, strict, &issues)
	}
	if value, ok := args["include_asset_metadata"]; ok {
		result["include_asset_metadata"] = validateArgumentField(argumentField{Type: "boolean"}, value, "args.include_asset_metadata", strict, &issues)
	}
	extraFields(args, []string{"path_prefix", "items", "match", "include_asset_metadata"}, "args", &issues)
	if len(issues) == 0 {
		missing := false
		for _, value := range result["items"].([]any) {
			missing = missing || value.(map[string]any)["memento_path"] == nil
		}
		match, _ := result["match"].(map[string]any)
		if missing && (match == nil || match["path_template"] == nil) {
			addIssue(&issues, "args", "Value error, items without memento_path require match.path_template")
		}
	}
	if len(issues) > 0 {
		return nil, &ValidationError{issues}
	}
	return result, nil
}
func validateManifestItems(value any, strict bool, issues *[]ValidationIssue) any {
	rows, ok := value.([]any)
	if !ok {
		message := "Input should be a valid list"
		if strict {
			message = "Input should be a valid array"
		}
		addIssue(issues, "args.items", message)
		return nil
	}
	result := []any{}
	for index, value := range rows {
		before := len(*issues)
		item := validateManifestItem(value, index, strict, issues)
		if len(*issues) == before {
			result = append(result, item)
		}
	}
	if len(rows) < 1 {
		addIssue(issues, "args.items", "List should have at least 1 item after validation, not 0")
	} else if len(rows) > 50 {
		addIssue(issues, "args.items", fmt.Sprintf("List should have at most 50 items after validation, not %d", len(rows)))
	}
	return result
}
func validateManifestItem(value any, index int, strict bool, issues *[]ValidationIssue) map[string]any {
	path := fmt.Sprintf("args.items.%d", index)
	raw, ok := value.(map[string]any)
	if !ok {
		message := "Input should be a valid dictionary or instance of ManifestItemArgs"
		if strict {
			message = "Input should be an object"
		}
		addIssue(issues, path, message)
		return nil
	}
	result := map[string]any{"memento_path": nil}
	fields := []argumentField{{Name: "name", Type: "string", Required: true, MinLength: intPointer(1), MaxLength: intPointer(128)}, {Name: "local_path", Type: "string", Required: true, MinLength: intPointer(1), MaxLength: intPointer(512)}, {Name: "memento_path", Type: "string", Nullable: true, MinLength: intPointer(4), MaxLength: intPointer(1024)}}
	for _, field := range fields {
		item, exists := raw[field.Name]
		if !exists {
			if field.Required {
				addIssue(issues, path+"."+field.Name, "Field required")
			}
			continue
		}
		result[field.Name] = validateArgumentField(field, item, path+"."+field.Name, strict, issues)
	}
	if item, exists := raw["local_updated_at"]; exists {
		result["local_updated_at"] = validateManifestDatetime(item, path+".local_updated_at", strict, issues)
	} else {
		addIssue(issues, path+".local_updated_at", "Field required")
	}
	if item, exists := raw["local_body_sha256"]; exists {
		text, valid := item.(string)
		if !valid {
			addIssue(issues, path+".local_body_sha256", "Input should be a valid string")
		} else if !manifestDigest.MatchString(text) {
			addIssue(issues, path+".local_body_sha256", "String should match pattern '^[0-9a-fA-F]{64}$'")
		} else {
			result["local_body_sha256"] = text
		}
	} else {
		addIssue(issues, path+".local_body_sha256", "Field required")
	}
	if item, exists := raw["local_bytes"]; exists {
		result["local_bytes"] = validateArgumentField(argumentField{Type: "integer", Minimum: intPointer(0)}, item, path+".local_bytes", strict, issues)
	} else {
		addIssue(issues, path+".local_bytes", "Field required")
	}
	extraFields(raw, []string{"name", "local_path", "memento_path", "local_updated_at", "local_body_sha256", "local_bytes"}, path, issues)
	return result
}
func validateManifestDatetime(value any, path string, strict bool, issues *[]ValidationIssue) any {
	stamp, err := pydatetime.ParseJSON(value, strict)
	if err == nil {
		return stamp
	}
	if errors.Is(err, pydatetime.ErrNaive) {
		addIssue(issues, path, "Value error, local_updated_at must be timezone-aware")
		return nil
	}
	message := pydatetime.Reason(err)
	if message == "" {
		message = "Input should be a valid datetime"
	}
	addIssue(issues, path, message)
	return nil
}
func validateManifestMatch(value any, strict bool, issues *[]ValidationIssue) any {
	if value == nil {
		return nil
	}
	raw, ok := value.(map[string]any)
	if !ok {
		message := "Input should be a valid dictionary or instance of ManifestMatchArgs"
		if strict {
			message = "Input should be an object"
		}
		addIssue(issues, "args.match", message)
		return nil
	}
	result := map[string]any{"path_template": nil, "aliases": map[string]any{}}
	if item, exists := raw["path_template"]; exists {
		result["path_template"] = validateArgumentField(argumentField{Type: "string", Nullable: true, MinLength: intPointer(5), MaxLength: intPointer(1024)}, item, "args.match.path_template", strict, issues)
	}
	before := len(*issues)
	if item, exists := raw["aliases"]; exists {
		aliases, valid := item.(map[string]any)
		if !valid {
			addIssue(issues, "args.match.aliases", "Input should be a valid dictionary")
		} else {
			copied := map[string]any{}
			for key, value := range aliases {
				text, valid := value.(string)
				if !valid {
					addIssue(issues, "args.match.aliases."+key, "Input should be a valid string")
				} else {
					copied[key] = text
				}
			}
			result["aliases"] = copied
			if len(*issues) == before && len(aliases) > 50 {
				addIssue(issues, "args.match", "Value error, manifest aliases exceed 50 entries")
			}
		}
	}
	extraFields(raw, []string{"path_template", "aliases"}, "args.match", issues)
	return result
}
func normalizeManifestArgument(value any) any {
	switch v := value.(type) {
	case time.Time:
		return v.Format("2006-01-02 15:04:05.999999-07:00")
	case []any:
		r := make([]any, len(v))
		for i, item := range v {
			r[i] = normalizeManifestArgument(item)
		}
		return r
	case map[string]any:
		r := map[string]any{}
		for key, item := range v {
			r[key] = normalizeManifestArgument(item)
		}
		return r
	default:
		return value
	}
}
