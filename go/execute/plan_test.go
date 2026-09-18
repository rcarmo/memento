package execute

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func normalJSON(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var result any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	_ = d.Decode(&result)
	return result
}
func TestPlanModelReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/execute-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	type modelCase struct {
		Value       any
		Expected    any
		Error, Type string
	}
	var f struct {
		Plans, Limits []modelCase
		Normalization []struct {
			Arguments map[string]any
			expected
		}
		Preflights []struct {
			Plan          map[string]any
			MaxOperations int `json:"max_operations"`
			Calls         []string
			expected
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = d.Decode(&f); err != nil {
		t.Fatal(err)
	}
	p, err := NewPlanner()
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, got any, err error, want modelCase, label string) {
		t.Helper()
		if want.Error != "" {
			var validation *ValidationError
			if !errors.As(err, &validation) || validation.Message(label) != want.Error {
				t.Fatal(got, err, want.Error)
			}
			return
		}
		actual := normalJSON(got)
		// JSON distinguishes numeric spelling but the decoded float limit is
		// identical for 3/3.0. Normalise only this known float field; all integer
		// fields (including unbounded output-byte limits) remain exact.
		if label == "limits" && err == nil {
			a := actual.(map[string]any)
			e := want.Expected.(map[string]any)
			an, ae := a["max_time_seconds"].(json.Number).Float64()
			en, ee := e["max_time_seconds"].(json.Number).Float64()
			if ae != nil || ee != nil || an != en {
				t.Fatal(an, en, ae, ee)
			}
			a["max_time_seconds"] = e["max_time_seconds"]
		}
		if err != nil || !reflect.DeepEqual(actual, want.Expected) {
			t.Fatal(got, err, want.Expected)
		}
	}
	for i, tc := range f.Plans {
		t.Run(fmt.Sprintf("plan-%d", i), func(t *testing.T) { got, err := p.ParsePlan(tc.Value); check(t, got, err, tc, "plan") })
	}
	for i, tc := range f.Limits {
		t.Run(fmt.Sprintf("limits-%d", i), func(t *testing.T) { got, err := ParseLimits(tc.Value); check(t, got, err, tc, "limits") })
	}
	for i, tc := range f.Normalization {
		t.Run(fmt.Sprintf("normal-%d", i), func(t *testing.T) {
			got, err := NormalizeToolArguments(tc.Arguments)
			checkExpected(t, got, err, tc.expected)
		})
	}
	for i, tc := range f.Preflights {
		t.Run(fmt.Sprintf("preflight-%d", i), func(t *testing.T) {
			plan, err := p.ParsePlan(tc.Plan)
			if err != nil {
				t.Fatal(err)
			}
			calls := []string{}
			err = p.Preflight(plan, tc.MaxOperations, func(name string, args map[string]any, strict bool) (map[string]any, error) {
				calls = append(calls, name)
				if strict {
					t.Error("static preflight cannot be strict")
				}
				return args, nil
			})
			checkExpected(t, nil, err, tc.expected)
			if !reflect.DeepEqual(calls, tc.Calls) {
				t.Fatal(calls, tc.Calls)
			}
		})
	}
}
func TestPlanPreflightAndResolveBoundaries(t *testing.T) {
	p, err := NewPlanner()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newPlanner([]byte("{")); err == nil {
		t.Fatal("bad contracts")
	}
	if err := p.Preflight(Plan{Operations: []PlannedOperation{{Op: "unknown"}}}, 12, nil); err == nil {
		t.Fatal("unknown op")
	}
	static := Plan{Operations: []PlannedOperation{{Op: "read", Args: map[string]any{"id_or_path": "a"}}}}
	if err := p.Preflight(static, 12, nil); err == nil {
		t.Fatal("missing validator")
	}
	for _, cause := range []error{&ValidationError{[]ValidationIssue{{"args.query", "Field required"}}}, valueError("bad"), &Error{"TypeError", "wrong type"}, &Error{"IndexError", "bad index"}, io.ErrClosedPipe} {
		err := p.Preflight(static, 12, func(string, map[string]any, bool) (map[string]any, error) { return nil, cause })
		if err == nil {
			t.Fatal("failure lost")
		}
		if source, ok := cause.(*Error); cause == io.ErrClosedPipe || ok && source.Kind != "ValueError" {
			if !errors.Is(err, cause) {
				t.Fatal("preflight unexpectedly caught error", err, cause)
			}
		}
	}
	ref := PlannedOperation{Op: "search", Args: map[string]any{"query": "$value", "limit": "2"}}
	if err := p.Preflight(Plan{Operations: []PlannedOperation{ref}}, 12, nil); err == nil {
		t.Fatal("reference preflight silently accepted missing validator")
	}
	got, err := ResolveArguments(ref, 1, map[string]any{"value": "literal"}, func(name string, args map[string]any, strict bool) (map[string]any, error) {
		if !strict || args["limit"] != "2" || args["query"] != "literal" {
			t.Fatal(args, strict)
		}
		return args, nil
	})
	if err != nil || got["query"] != "literal" {
		t.Fatal(got, err)
	}
	for _, args := range []map[string]any{{"query": "$bad..path"}, {"query": "$unknown"}, {"query": "literal"}} {
		op := PlannedOperation{Op: "search", Args: args}
		if _, err := ResolveArguments(op, 2, nil, nil); err == nil {
			t.Fatal(args)
		}
	}
	if _, err := ResolveArguments(ref, 1, map[string]any{"value": "x"}, func(string, map[string]any, bool) (map[string]any, error) {
		return nil, &ValidationError{[]ValidationIssue{{"args.limit", "Input should be a valid integer"}}}
	}); err == nil || !strings.Contains(err.Error(), "operation 1 (search): args.limit") {
		t.Fatal(err)
	}
	existing, bad, newName := "existing", "invalid name", "new"
	saved := map[string]any{"existing": nil}
	for _, name := range []*string{nil, &existing} {
		if err := CheckSaveName(name, saved, 1); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []*string{&bad, &newName} {
		if err := CheckSaveName(name, saved, 1); err == nil {
			t.Fatal("save limit")
		}
	}
	for _, kind := range []string{"TypeError", "IndexError", "ValueError", "RuntimeError"} {
		cause := &Error{kind, "synthetic"}
		_, err := ResolveArguments(ref, 1, map[string]any{"value": "x"}, func(string, map[string]any, bool) (map[string]any, error) { return nil, cause })
		if kind == "RuntimeError" {
			if err != cause {
				t.Fatal(err)
			}
		} else if err == nil || err.Error() != "operation 1 (search): synthetic" {
			t.Fatal(err)
		}
	}
	// Truncate diagnostic fields and final text by Unicode code points.
	v := &ValidationError{}
	for range 5 {
		v.Issues = append(v.Issues, ValidationIssue{strings.Repeat("é", 120), strings.Repeat(" x\n", 90)})
	}
	if len([]rune(v.Error())) > 512 {
		t.Fatal(v.Error())
	}
}
func TestPlanDefensiveCopiesAndConcurrency(t *testing.T) {
	p, err := NewPlanner()
	if err != nil {
		t.Fatal(err)
	}
	raw := map[string]any{"operations": []any{map[string]any{"op": "search", "args": map[string]any{"query": "x", "nested": []any{map[string]any{"value": "a"}}}}}}
	var wg sync.WaitGroup
	fail := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			plan, err := p.ParsePlan(raw)
			if err == nil {
				plan.Operations[0].Args["nested"].([]any)[0].(map[string]any)["value"] = "mutated"
				err = p.Preflight(Plan{Operations: []PlannedOperation{{Op: "search", Args: map[string]any{"query": "x"}}}}, 12, func(_ string, args map[string]any, _ bool) (map[string]any, error) {
					args["query"] = "new"
					return args, nil
				})
			}
			fail <- err
		}()
	}
	wg.Wait()
	close(fail)
	for err := range fail {
		if err != nil {
			t.Fatal(err)
		}
	}
	if raw["operations"].([]any)[0].(map[string]any)["args"].(map[string]any)["nested"].([]any)[0].(map[string]any)["value"] != "a" {
		t.Fatal(raw)
	}
}

func TestPlanScalarsMalformedInternalValues(t *testing.T) {
	// Invalid json.Number tokens cannot cross a JSON decoder; fail them safely
	// when these kernels are called directly by another Go component.
	if _, message := planInteger(json.Number("invalid")); message != "Input should be a valid integer" {
		t.Fatal(message)
	}
	if _, message := planInteger(json.Number("1e9999")); message != "Input should be a finite number" {
		t.Fatal(message)
	}
	if _, message := planFloat(json.Number("bad")); message != "Input should be a valid number" {
		t.Fatal(message)
	}
	if !planTruthy(struct{}{}) {
		t.Fatal("native truthiness")
	}
}
