package execute

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"
)

//go:embed arguments.json
var argumentDefinitions []byte

type argumentField struct {
	Name                 string `json:"name"`
	Type                 string `json:"type"`
	Required, Nullable   bool
	Default              any
	Enum                 []string
	Minimum, Maximum     *int
	MinLength, MaxLength *int
	MinItems             *int
	Pattern              string
	Items                *argumentField
}
type argumentContract struct {
	Operation string
	Fields    []argumentField
}

// Arguments implements only the explicitly exported simple argument models.
// Propose/compare_manifest nested models fail closed until separately ported.
type Arguments struct{ contracts map[string][]argumentField }

func NewArguments() (*Arguments, error) { return newArguments(argumentDefinitions) }
func newArguments(raw []byte) (*Arguments, error) {
	var definitions []argumentContract
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	// This is a narrow, source-derived contract, not a permissive JSON Schema
	// reader: new constraints must fail closed until implemented explicitly.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definitions); err != nil {
		return nil, err
	}
	result := &Arguments{contracts: map[string][]argumentField{}}
	for _, definition := range definitions {
		for _, field := range definition.Fields {
			if err := validateArgumentContract(field); err != nil {
				return nil, err
			}
		}
		result.contracts[definition.Operation] = definition.Fields
	}
	return result, nil
}
func validateArgumentContract(field argumentField) error {
	switch field.Type {
	case "string", "integer", "boolean":
	case "array":
		if field.Items == nil {
			return errors.New("array argument contract has no items")
		}
		return validateArgumentContract(*field.Items)
	default:
		return fmt.Errorf("unsupported argument contract type: %s", field.Type)
	}
	if field.Pattern != "" && argumentPatterns[field.Pattern] == nil {
		return fmt.Errorf("unsupported argument pattern: %s", field.Pattern)
	}
	return nil
}
func (a *Arguments) Supports(operation string) bool {
	if operation == "compare_manifest" || operation == "propose" {
		return true
	}
	_, ok := a.contracts[operation]
	return ok
}

// Validate is a real ArgumentValidator for the supported operation models. The
// strict flag applies to the whole model, not only fields containing references.
func (a *Arguments) Validate(operation string, args map[string]any, strict bool) (map[string]any, error) {
	if operation == "compare_manifest" {
		return a.validateManifest(args, strict)
	}
	if operation == "propose" {
		return a.validatePropose(args, strict)
	}
	fields, ok := a.contracts[operation]
	if !ok {
		return nil, fmt.Errorf("execute argument validation not implemented for %s", operation)
	}
	result := map[string]any{}
	issues := []ValidationIssue{}
	names := []string{}
	for _, field := range fields {
		names = append(names, field.Name)
		value, exists := args[field.Name]
		if !exists {
			if field.Required {
				addIssue(&issues, "args."+field.Name, "Field required")
			} else {
				result[field.Name] = copyPlanValue(field.Default)
			}
			continue
		}
		result[field.Name] = validateArgumentField(field, value, "args."+field.Name, strict, &issues)
	}
	extraFields(args, names, "args", &issues)
	// Pydantic after-model validators run only when every field is valid.
	if len(issues) == 0 && operation == "asset_metadata" {
		message := ""
		if result["id_or_path"] != nil && result["path_prefix"] != nil {
			message = "asset metadata accepts id_or_path or path_prefix, not both"
		} else if result["id_or_path"] != nil && result["cursor"] != nil {
			message = "asset metadata cursor requires path_prefix scope"
		} else if result["version"] != nil && result["asset_kind"] == nil {
			message = "asset metadata version requires asset_kind"
		}
		if message != "" {
			addIssue(&issues, "args", "Value error, "+message)
		}
	}
	if len(issues) > 0 {
		return nil, &ValidationError{issues}
	}
	return result, nil
}

var argumentPatterns = map[string]*regexp.Regexp{
	`^[0-9a-f]{64}$`:                             regexp.MustCompile(`^[0-9a-f]{64}$`),
	`^[a-z0-9]+(?:-[a-z0-9]+)*$`:                 regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`),
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`: regexp.MustCompile(`^(0|[1-9]\p{Nd}*)\.(0|[1-9]\p{Nd}*)\.(0|[1-9]\p{Nd}*)$`),
}

func validateArgumentField(field argumentField, value any, path string, strict bool, issues *[]ValidationIssue) any {
	if value == nil && field.Nullable {
		return nil
	}
	if len(field.Enum) > 0 {
		text, ok := value.(string)
		if ok {
			for _, allowed := range field.Enum {
				if text == allowed {
					return text
				}
			}
		}
		choices := []string{}
		for _, v := range field.Enum {
			choices = append(choices, "'"+v+"'")
		}
		message := strings.Join(choices, ", ")
		if len(choices) > 1 {
			message = strings.Join(choices[:len(choices)-1], ", ") + " or " + choices[len(choices)-1]
		}
		addIssue(issues, path, "Input should be "+message)
		return nil
	}
	switch field.Type {
	case "string":
		text, ok := value.(string)
		if !ok {
			addIssue(issues, path, "Input should be a valid string")
			return nil
		}
		length := utf8.RuneCountInString(text)
		if field.MinLength != nil && length < *field.MinLength {
			addIssue(issues, path, fmt.Sprintf("String should have at least %d character%s", *field.MinLength, plural(*field.MinLength)))
			return nil
		}
		if field.MaxLength != nil && length > *field.MaxLength {
			addIssue(issues, path, fmt.Sprintf("String should have at most %d character%s", *field.MaxLength, plural(*field.MaxLength)))
			return nil
		}
		if field.Pattern != "" && !argumentPatterns[field.Pattern].MatchString(text) {
			addIssue(issues, path, "String should match pattern '"+field.Pattern+"'")
			return nil
		}
		return text
	case "integer":
		if strict {
			n, ok := value.(json.Number)
			if !ok || strings.ContainsAny(string(n), ".eE") {
				addIssue(issues, path, "Input should be a valid integer")
				return nil
			}
		}
		n, message := planInteger(value)
		if message != "" {
			addIssue(issues, path, message)
			return nil
		}
		number, _ := new(big.Int).SetString(string(n), 10)
		if field.Maximum != nil && number.Cmp(big.NewInt(int64(*field.Maximum))) > 0 {
			addIssue(issues, path, fmt.Sprintf("Input should be less than or equal to %d", *field.Maximum))
			return nil
		}
		if field.Minimum != nil && number.Cmp(big.NewInt(int64(*field.Minimum))) < 0 {
			addIssue(issues, path, fmt.Sprintf("Input should be greater than or equal to %d", *field.Minimum))
			return nil
		}
		return n
	case "boolean":
		if strict {
			if _, ok := value.(bool); !ok {
				addIssue(issues, path, "Input should be a valid boolean")
				return nil
			}
		}
		b, message := planBool(value)
		if message != "" {
			addIssue(issues, path, message)
			return nil
		}
		return b
	default: // Only arrays remain; contracts reject unknown types at construction.
		rows, ok := value.([]any)
		if !ok {
			addIssue(issues, path, "Input should be a valid array")
			if !strict {
				(*issues)[len(*issues)-1].Message = "Input should be a valid tuple"
			}
			return nil
		}
		result := []any{}
		for i, item := range rows {
			before := len(*issues)
			validated := validateArgumentField(*field.Items, item, fmt.Sprintf("%s.%d", path, i), strict, issues)
			if len(*issues) == before {
				result = append(result, validated)
			}
		}
		// Pydantic checks tuple length after dropping invalid items, even if
		// element validation already failed.
		if field.MinItems != nil && len(result) < *field.MinItems {
			addIssue(issues, path, fmt.Sprintf("Tuple should have at least %d item%s after validation, not %d", *field.MinItems, plural(*field.MinItems), len(result)))
		}
		return result
	}
}
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
