package execute

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestManifestArgumentFixtureContract(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/execute-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Arguments map[string]any
			Strict    bool
			Expected  map[string]any
			Issues    []ValidationIssue
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 312 {
		t.Fatal("review compare_manifest source corpus change", len(fixture.Cases))
	}
	strict, lax, successes, failures := 0, 0, 0, 0
	for _, test := range fixture.Cases {
		if test.Strict {
			strict++
		} else {
			lax++
		}
		if test.Issues != nil {
			failures++
		} else if test.Expected != nil {
			successes++
		} else {
			t.Fatal("fixture has neither expected value nor issues")
		}
	}
	if strict != 156 || lax != 156 || successes == 0 || failures == 0 {
		t.Fatal(strict, lax, successes, failures)
	}
	if got := normalizeManifestArgument([]any{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}); got.([]any)[0] != "2026-01-01 00:00:00+00:00" {
		t.Fatal(got)
	}
	validator, err := NewArguments()
	if err != nil {
		t.Fatal(err)
	}
	for index, test := range fixture.Cases {
		got, err := validator.Validate("compare_manifest", test.Arguments, test.Strict)
		if test.Issues != nil {
			validation, ok := err.(*ValidationError)
			if !ok || !reflect.DeepEqual(validation.Issues, test.Issues) {
				t.Fatal(index, test.Arguments, validation, test.Issues)
			}
		} else if err != nil || !reflect.DeepEqual(normalizeManifestArgument(got), test.Expected) {
			t.Fatal(index, test.Arguments, normalizeManifestArgument(got), err, test.Expected)
		}
	}
	planner, _ := NewPlanner()
	referenced := PlannedOperation{Op: "compare_manifest", Args: map[string]any{"items": "$saved"}}
	if err := planner.Preflight(Plan{Operations: []PlannedOperation{referenced}}, 12, validator.Validate); err != nil {
		t.Fatal(err)
	}
	item := map[string]any{"name": "n", "local_path": "p", "memento_path": "/abc", "local_updated_at": "2026-01-01T00:00Z", "local_body_sha256": string(bytes.Repeat([]byte{'a'}, 64)), "local_bytes": json.Number("0")}
	if got, err := ResolveArguments(referenced, 1, map[string]any{"saved": []any{item}}, validator.Validate); err != nil || len(got["items"].([]any)) != 1 {
		t.Fatal(got, err)
	}
	issues := []ValidationIssue{}
	if validateManifestDatetime(json.Number("999999999999999999"), "x", false, &issues) != nil || len(issues) != 1 {
		t.Fatal(issues)
	}
	issues = nil
	if validateManifestMatch(map[string]any{"aliases": map[string]any{"x": 1}}, false, &issues) == nil || len(issues) != 1 {
		t.Fatal(issues)
	}
	issues = nil
	if validateManifestMatch(map[string]any{"aliases": []any{}}, false, &issues) == nil || len(issues) != 1 {
		t.Fatal(issues)
	}
	issues = nil
	if validateManifestMatch(map[string]any{"aliases": map[string]any{"x": "y"}}, false, &issues) == nil || len(issues) != 0 {
		t.Fatal(issues)
	}
}
