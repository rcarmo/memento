package execute

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

//go:embed operations.json
var operationDefinitions []byte

type OperationContract struct {
	Name          string   `json:"name"`
	CommitCapable bool     `json:"commit_capable"`
	Fields        []string `json:"fields"`
	Required      []string `json:"required"`
}
type Planner struct{ contracts map[string]OperationContract }

func NewPlanner() (*Planner, error) { return newPlanner(operationDefinitions) }
func newPlanner(raw []byte) (*Planner, error) {
	var definitions []OperationContract
	if err := json.Unmarshal(raw, &definitions); err != nil {
		return nil, err
	}
	p := &Planner{contracts: map[string]OperationContract{}}
	for _, definition := range definitions {
		p.contracts[definition.Name] = definition
	}
	return p, nil
}

type PlannedOperation struct {
	SaveAs *string        `json:"save_as"`
	Op     string         `json:"op"`
	Args   map[string]any `json:"args"`
}

// PlannedReturn keeps Python's unbounded integer limit through plan validation;
// the runner must cap it against actual list length before converting to int.
type PlannedReturn struct {
	Name   *string      `json:"name"`
	Ref    string       `json:"ref"`
	Fields []string     `json:"fields"`
	Limit  *json.Number `json:"limit"`
}
type Plan struct {
	Operations  []PlannedOperation `json:"operations"`
	StopOnError bool               `json:"stop_on_error"`
	Returns     []PlannedReturn    `json:"returns"`
}
type ValidationIssue struct{ Path, Message string }
type ValidationError struct{ Issues []ValidationIssue }

func (e *ValidationError) Error() string { return e.Message("plan") }
func (e *ValidationError) Message(label string) string {
	details := []string{}
	for _, issue := range e.Issues[:min(3, len(e.Issues))] {
		path := truncateRunes(issue.Path, 100)
		message := truncateRunes(strings.Join(strings.FieldsFunc(issue.Message, func(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }), " "), 120)
		details = append(details, path+": "+message)
	}
	suffix := ""
	if len(e.Issues) > 3 {
		suffix = fmt.Sprintf(" (+%d more)", len(e.Issues)-3)
	}
	return truncateRunes(label+": "+strings.Join(details, "; ")+suffix, 512)
}
func truncateRunes(value string, n int) string {
	r := []rune(value)
	if len(r) > n {
		return string(r[:n])
	}
	return value
}
func joinLocation(parent, field string) string {
	if parent == "" {
		return field
	}
	return parent + "." + field
}
func addIssue(issues *[]ValidationIssue, path, message string) {
	*issues = append(*issues, ValidationIssue{path, message})
}
func extraFields(raw map[string]any, known []string, path string, issues *[]ValidationIssue) {
	keys := []string{}
	for key := range raw {
		found := false
		for _, k := range known {
			if key == k {
				found = true
				break
			}
		}
		if !found {
			keys = append(keys, key)
		}
	}
	// Input maps have no insertion order. Multi-extra error ordering is lexical;
	// ordered-object/Pydantic-wide diagnostic parity remains separate work.
	sort.Strings(keys)
	for _, key := range keys {
		addIssue(issues, joinLocation(path, key), "Extra inputs are not permitted")
	}
}
func stringField(raw map[string]any, key, path string, required, nullable bool, issues *[]ValidationIssue) *string {
	value, exists := raw[key]
	location := joinLocation(path, key)
	if !exists {
		if required {
			addIssue(issues, location, "Field required")
		}
		return nil
	}
	if value == nil && nullable {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		addIssue(issues, location, "Input should be a valid string")
		return nil
	}
	return &text
}

// ParsePlan matches the JSON-domain _InputPlan structure. It intentionally does
// not validate save_as syntax, reference existence or per-operation args yet.
func (p *Planner) ParsePlan(value any) (Plan, error) {
	plan := Plan{Operations: []PlannedOperation{}, StopOnError: true, Returns: []PlannedReturn{}}
	issues := []ValidationIssue{}
	raw, ok := value.(map[string]any)
	if !ok {
		return plan, &ValidationError{[]ValidationIssue{{"", "Input should be a valid dictionary or instance of _InputPlan"}}}
	}
	operations, exists := raw["operations"]
	if !exists {
		addIssue(&issues, "operations", "Field required")
	} else if rows, ok := operations.([]any); !ok {
		addIssue(&issues, "operations", "Input should be a valid tuple")
	} else {
		for i, row := range rows {
			path := fmt.Sprintf("operations.%d", i)
			object, ok := row.(map[string]any)
			if !ok {
				addIssue(&issues, path, "Input should be a valid dictionary or instance of _PlannedOperation")
				continue
			}
			op := PlannedOperation{SaveAs: stringField(object, "save_as", path, false, true, &issues), Args: map[string]any{}}
			if name := stringField(object, "op", path, true, false, &issues); name != nil {
				op.Op = *name
				if _, known := p.contracts[*name]; !known {
					addIssue(&issues, path+".op", "Value error, unknown execute operation")
				}
			}
			if args, exists := object["args"]; exists {
				mapping, ok := args.(map[string]any)
				if !ok {
					addIssue(&issues, path+".args", "Input should be a valid dictionary")
				} else {
					op.Args = copyPlanValue(mapping).(map[string]any)
				}
			}
			extraFields(object, []string{"save_as", "op", "args"}, path, &issues)
			plan.Operations = append(plan.Operations, op)
		}
	}
	if value, exists := raw["stop_on_error"]; exists {
		b, message := planBool(value)
		if message != "" {
			addIssue(&issues, "stop_on_error", message)
		} else {
			plan.StopOnError = b
		}
	}
	if value, exists := raw["returns"]; exists {
		rows, ok := value.([]any)
		if !ok {
			addIssue(&issues, "returns", "Input should be a valid tuple")
		} else {
			for i, row := range rows {
				path := fmt.Sprintf("returns.%d", i)
				object, ok := row.(map[string]any)
				if !ok {
					addIssue(&issues, path, "Input should be a valid dictionary or instance of ExecuteReturnProjection")
					continue
				}
				projection := PlannedReturn{Name: stringField(object, "name", path, false, true, &issues), Fields: []string{}}
				if ref := stringField(object, "ref", path, true, false, &issues); ref != nil {
					projection.Ref = *ref
				}
				if fields, exists := object["fields"]; exists {
					items, ok := fields.([]any)
					if !ok {
						addIssue(&issues, path+".fields", "Input should be a valid tuple")
					} else {
						for n, field := range items {
							if text, ok := field.(string); ok {
								projection.Fields = append(projection.Fields, text)
							} else {
								addIssue(&issues, fmt.Sprintf("%s.fields.%d", path, n), "Input should be a valid string")
							}
						}
					}
				}
				if limit := object["limit"]; limit != nil {
					n, message := planInteger(limit)
					if message != "" {
						addIssue(&issues, path+".limit", message)
					} else if strings.HasPrefix(string(n), "-") || n == "0" {
						addIssue(&issues, path+".limit", "Input should be greater than or equal to 1")
					} else {
						projection.Limit = &n
					}
				}
				extraFields(object, []string{"name", "ref", "fields", "limit"}, path, &issues)
				plan.Returns = append(plan.Returns, projection)
			}
		}
	}
	extraFields(raw, []string{"operations", "stop_on_error", "returns"}, "", &issues)
	if len(issues) > 0 {
		return Plan{}, &ValidationError{issues}
	}
	return plan, nil
}
func copyPlanValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range v {
			out[k] = copyPlanValue(x)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = copyPlanValue(x)
		}
		return out
	default:
		return v
	}
}

// ArgumentValidator must implement the selected source operation model, not a
// loose JSON-schema check. No runner or registration is implied by preflight.
type ArgumentValidator func(operation string, args map[string]any, strict bool) (map[string]any, error)

func (p *Planner) Preflight(plan Plan, maxOperations int, validate ArgumentValidator) error {
	if len(plan.Operations) > maxOperations {
		return valueError("plan exceeds configured max_operations")
	}
	commits := 0
	for _, op := range plan.Operations {
		contract, ok := p.contracts[op.Op]
		if !ok {
			return valueError("unknown execute operation")
		}
		if contract.CommitCapable {
			commits++
		}
	}
	if commits > 1 {
		return valueError("plan may contain at most one commit-capable operation")
	}
	if len(plan.Operations) > 0 && validate == nil {
		return errors.New("execute argument validator is required")
	}
	for i, op := range plan.Operations {
		label := fmt.Sprintf("operation %d (%s)", i+1, op.Op)
		refs, err := ContainsReferences(op.Args)
		if err == nil {
			if !refs {
				_, err = validate(op.Op, copyPlanValue(op.Args).(map[string]any), false)
			} else {
				contract := p.contracts[op.Op]
				fields := map[string]bool{}
				for _, key := range contract.Fields {
					fields[key] = true
				}
				for key := range op.Args {
					if !fields[key] {
						err = valueError("unknown argument field")
						break
					}
				}
				if err == nil {
					for _, key := range contract.Required {
						if _, ok := op.Args[key]; !ok {
							err = valueError("missing required argument field")
							break
						}
					}
				}
			}
		}
		if err != nil {
			return operationError(label, err, false)
		}
	}
	return nil
}
func operationError(label string, err error, resolving bool) error {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return valueError(validation.Message(label))
	}
	var source *Error
	if errors.As(err, &source) && (source.Kind == "ValueError" || resolving && (source.Kind == "IndexError" || source.Kind == "TypeError")) {
		return valueError(label + ": " + truncateRunes(source.Message, 300))
	}
	return err
}

// ResolveArguments switches the entire argument model to strict JSON validation
// when any reference occurs, even if other literal fields need lax coercion.
func ResolveArguments(op PlannedOperation, index int, saved map[string]any, validate ArgumentValidator) (map[string]any, error) {
	label := fmt.Sprintf("operation %d (%s)", index, op.Op)
	refs, err := ContainsReferences(op.Args)
	if err != nil {
		return nil, operationError(label, err, true)
	}
	resolved, err := ResolveReferences(op.Args, saved)
	if err != nil {
		return nil, operationError(label, err, true)
	}
	if validate == nil {
		return nil, errors.New("execute argument validator is required")
	}
	args, err := validate(op.Op, resolved.(map[string]any), refs)
	if err != nil {
		return nil, operationError(label, err, true)
	}
	return args, nil
}
func CheckSaveName(name *string, saved map[string]any, maxIntermediates int) error {
	if name == nil {
		return nil
	}
	if !validSavedName(*name) {
		return valueError("invalid save_as identifier: " + *name)
	}
	if _, exists := saved[*name]; len(saved) >= maxIntermediates && !exists {
		return valueError("plan exceeds configured max_intermediates")
	}
	return nil
}

// NormalizeToolArguments preserves Python's identity check for stop_on_error
// when a nested plan is supplied. Explicit null differs from an omitted flag.
func NormalizeToolArguments(args map[string]any) (any, error) {
	stop, exists := args["stop_on_error"]
	if !exists {
		stop = true
	}
	if plan := args["plan"]; plan != nil {
		if args["operations"] != nil || args["returns"] != nil || stop != true {
			return nil, valueError("memory_execute accepts either plan or top-level plan fields, not both")
		}
		return plan, nil
	}
	if args["operations"] == nil {
		return nil, valueError("memory_execute requires plan or operations")
	}
	returns := args["returns"]
	if !planTruthy(returns) {
		returns = []any{}
	}
	return map[string]any{"operations": args["operations"], "stop_on_error": stop, "returns": returns}, nil
}
