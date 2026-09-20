package execute

import "fmt"

type proposeVariant struct {
	path, model string
	fields      []argumentField
	asset       bool
}

var proposeVariants = []proposeVariant{
	{"ProposeCreateChange", "ProposeCreateChange", []argumentField{{Name: "kind", Type: "string", Required: true, Enum: []string{"create"}}, {Name: "path", Type: "string", Required: true}, {Name: "concept_type", Type: "string", Required: true}, {Name: "title", Type: "string", Required: true}, {Name: "body", Type: "string", Required: true}, {Name: "description", Type: "string", Nullable: true}, {Name: "tags", Type: "array", Default: []any{}, Items: &argumentField{Type: "string"}}, {Name: "aliases", Type: "array", Default: []any{}, Items: &argumentField{Type: "string"}}}, false},
	{"ProposePatchChange", "ProposePatchChange", []argumentField{{Name: "kind", Type: "string", Required: true, Enum: []string{"patch"}}, {Name: "path", Type: "string", Required: true}, {Name: "title", Type: "string", Nullable: true}, {Name: "description", Type: "string", Nullable: true}, {Name: "body", Type: "string", Nullable: true}, {Name: "status", Type: "string", Nullable: true}, {Name: "tags", Type: "array", Nullable: true, Items: &argumentField{Type: "string"}}, {Name: "aliases", Type: "array", Nullable: true, Items: &argumentField{Type: "string"}}}, false},
	{"ProposeRenameChange", "ProposeRenameChange", []argumentField{{Name: "kind", Type: "string", Required: true, Enum: []string{"rename"}}, {Name: "path", Type: "string", Required: true}, {Name: "new_path", Type: "string", Required: true}}, false},
	{"function-after[validate_transport(), ProposeAssetChange]", "ProposeAssetChange", []argumentField{{Name: "kind", Type: "string", Required: true, Enum: []string{"attach_asset_pack"}}, {Name: "path", Type: "string", Required: true}, {Name: "asset_kind", Type: "string", Required: true}, {Name: "version", Type: "string", Required: true}, {Name: "zip_base64", Type: "string", Nullable: true}, {Name: "staged_asset_id", Type: "string", Nullable: true}}, true},
	{"ProposeTrashChange", "ProposeTrashChange", []argumentField{{Name: "kind", Type: "string", Required: true, Enum: []string{"trash"}}, {Name: "path", Type: "string", Required: true}}, false},
}

func (a *Arguments) validatePropose(args map[string]any, strict bool) (map[string]any, error) {
	issues := []ValidationIssue{}
	result := map[string]any{"rationale": nil}
	for _, field := range []argumentField{{Name: "intent", Type: "string", Required: true}, {Name: "base_revision", Type: "string", Required: true}} {
		value, exists := args[field.Name]
		if !exists {
			addIssue(&issues, "args."+field.Name, "Field required")
		} else {
			result[field.Name] = validateArgumentField(field, value, "args."+field.Name, strict, &issues)
		}
	}
	value, exists := args["changes"]
	if !exists {
		addIssue(&issues, "args.changes", "Field required")
	} else if rows, ok := value.([]any); !ok {
		message := "Input should be a valid list"
		if strict {
			message = "Input should be a valid array"
		}
		addIssue(&issues, "args.changes", message)
	} else {
		changes := []any{}
		for index, row := range rows {
			change, nested := validateProposeChange(row, index, strict)
			if len(nested) > 0 {
				issues = append(issues, nested...)
			} else {
				changes = append(changes, change)
			}
		}
		result["changes"] = changes
	}
	if value, exists := args["rationale"]; exists {
		result["rationale"] = validateArgumentField(argumentField{Type: "string", Nullable: true}, value, "args.rationale", strict, &issues)
	}
	extraFields(args, []string{"intent", "base_revision", "changes", "rationale"}, "args", &issues)
	if len(issues) > 0 {
		return nil, &ValidationError{issues}
	}
	return result, nil
}
func validateProposeChange(value any, index int, strict bool) (map[string]any, []ValidationIssue) {
	raw, ok := value.(map[string]any)
	if !ok {
		issues := []ValidationIssue{}
		for _, variant := range proposeVariants {
			message := "Input should be a valid dictionary or instance of " + variant.model
			if strict {
				message = "Input should be an object"
			}
			addIssue(&issues, fmt.Sprintf("args.changes.%d.%s", index, variant.path), message)
		}
		return nil, issues
	}
	all := []ValidationIssue{}
	for _, variant := range proposeVariants {
		path := fmt.Sprintf("args.changes.%d.%s", index, variant.path)
		issues := []ValidationIssue{}
		result := map[string]any{}
		names := []string{}
		for _, field := range variant.fields {
			names = append(names, field.Name)
			item, exists := raw[field.Name]
			if !exists {
				if field.Required {
					addIssue(&issues, path+"."+field.Name, "Field required")
				} else if field.Nullable {
					result[field.Name] = nil
				} else {
					result[field.Name] = copyPlanValue(field.Default)
				}
			} else {
				result[field.Name] = validateArgumentField(field, item, path+"."+field.Name, strict, &issues)
			}
		}
		extraFields(raw, names, path, &issues)
		if len(issues) == 0 && variant.asset && (result["zip_base64"] == nil) == (result["staged_asset_id"] == nil) {
			addIssue(&issues, path, "Value error, exactly one of zip_base64 or staged_asset_id is required")
		}
		if len(issues) == 0 {
			return result, nil
		}
		all = append(all, issues...)
	}
	return nil, all
}
