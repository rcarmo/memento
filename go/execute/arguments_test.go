package execute

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
)

func TestArgumentReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/execute-arguments.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Definitions []argumentContract
		Cases       []struct {
			Operation string
			Arguments map[string]any
			Strict    bool
			Expected  map[string]any
			Error     string
			Issues    []ValidationIssue
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	validator, err := NewArguments()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.Definitions) != 27 || len(validator.contracts) != 27 {
		t.Fatal("argument model count changed")
	}
	for _, definition := range fixture.Definitions {
		if !reflect.DeepEqual(validator.contracts[definition.Operation], definition.Fields) {
			t.Fatal("runtime definition differs from source", definition.Operation)
		}
	}
	for i, tc := range fixture.Cases {
		t.Run(fmt.Sprintf("%d-%s-%t", i, tc.Operation, tc.Strict), func(t *testing.T) {
			got, err := validator.Validate(tc.Operation, tc.Arguments, tc.Strict)
			if tc.Error != "" {
				var validation *ValidationError
				if !errors.As(err, &validation) || validation.Message("operation 1 ("+tc.Operation+")") != tc.Error {
					t.Fatal(tc.Arguments, got, err, tc.Error)
				}
				if !reflect.DeepEqual(validation.Issues, tc.Issues) {
					t.Fatal(tc.Arguments, validation.Issues, tc.Issues)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.Expected) {
				t.Fatal(tc.Arguments, got, err, tc.Expected)
			}
		})
	}
}
func TestArgumentValidationIntegration(t *testing.T) {
	validator, err := NewArguments()
	if err != nil {
		t.Fatal(err)
	}
	planner, err := NewPlanner()
	if err != nil {
		t.Fatal(err)
	}
	if validator.Supports("propose") || validator.Supports("compare_manifest") || !validator.Supports("inventory") {
		t.Fatal("support set")
	}
	if _, err := validator.Validate("propose", nil, false); err == nil {
		t.Fatal("unported operation")
	}
	for _, raw := range []string{"{", `[{"operation":"x","fields":[{"type":"object"}]}]`, `[{"operation":"x","fields":[{"type":"array"}]}]`, `[{"operation":"x","fields":[{"type":"string","pattern":"bad"}]}]`, `[{"operation":"x","fields":[{"type":"array","items":{"type":"string"},"maxItems":2}]}]`} {
		if _, err := newArguments([]byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
	operations := []PlannedOperation{{Op: "search", Args: map[string]any{"query": "literal", "limit": "2"}}, {Op: "proposal_review", Args: map[string]any{"proposal_id": "p", "decision": "approve"}}}
	if err = planner.Preflight(Plan{Operations: operations}, 12, validator.Validate); err != nil {
		t.Fatal(err)
	}
	args, err := ResolveArguments(operations[0], 1, nil, validator.Validate)
	if err != nil || args["limit"] != json.Number("2") {
		t.Fatal(args, err)
	}
	referenced := PlannedOperation{Op: "search", Args: map[string]any{"query": "$saved", "limit": "2"}}
	if err = planner.Preflight(Plan{Operations: []PlannedOperation{referenced}}, 12, validator.Validate); err != nil {
		t.Fatal(err)
	}
	if _, err = ResolveArguments(referenced, 1, map[string]any{"saved": "query"}, validator.Validate); err == nil {
		t.Fatal("literal string integer coerced inside referenced model")
	}
	referenced.Args["limit"] = json.Number("2")
	if got, err := ResolveArguments(referenced, 1, map[string]any{"saved": "query"}, validator.Validate); err != nil || got["query"] != "query" {
		t.Fatal(got, err)
	}
	// Unsupported models also fail closed after referenced values resolve.
	unsupported := PlannedOperation{Op: "propose", Args: map[string]any{"intent": "i", "base_revision": "r", "changes": "$saved"}}
	if _, err := ResolveArguments(unsupported, 1, map[string]any{"saved": []any{}}, validator.Validate); err == nil {
		t.Fatal("unsupported referenced model accepted")
	}
	if err := planner.Preflight(Plan{Operations: []PlannedOperation{{Op: "compare_manifest", Args: map[string]any{"items": []any{}}}}}, 12, validator.Validate); err == nil {
		t.Fatal("unsupported static model accepted")
	}
	// Provided arrays must be copied as well as the default arrays.
	input := []any{"path", "title"}
	copy, err := validator.Validate("inventory", map[string]any{"fields": input}, true)
	if err != nil {
		t.Fatal(err)
	}
	copy["fields"].([]any)[0] = "changed"
	if input[0] != "path" {
		t.Fatal("caller list aliased")
	}
	// Static invalid arguments must fail before a mutation runner can start.
	operations = append(operations, PlannedOperation{Op: "purge", Args: map[string]any{"path": "/trash/a.md", "expected_revision": "r", "idempotency_key": "k", "confirm": nil}})
	if err = planner.Preflight(Plan{Operations: operations}, 12, validator.Validate); err == nil {
		t.Fatal("invalid static mutation")
	}
	// Defaults and caller lists are copied, including fields of aliased arguments.
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := validator.Validate("inventory", map[string]any{}, false)
			if err == nil {
				value["fields"].([]any)[0] = "mutated"
			}
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := validator.Validate("inventory", nil, false)
	if err != nil || got["fields"].([]any)[0] != "path" {
		t.Fatal(got, err)
	}
}
