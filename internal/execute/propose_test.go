package execute

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func normalizeExtraRuns(issues []ValidationIssue) []ValidationIssue {
	result, extras := []ValidationIssue{}, map[string][]ValidationIssue{}
	for _, issue := range issues {
		if issue.Message != "Extra inputs are not permitted" {
			result = append(result, issue)
			continue
		}
		branch := issue.Path[:strings.LastIndex(issue.Path, ".")]
		extras[branch] = append(extras[branch], issue)
	}
	branches := make([]string, 0, len(extras))
	for branch := range extras {
		branches = append(branches, branch)
	}
	sort.Strings(branches)
	for _, branch := range branches {
		sort.Slice(extras[branch], func(i, j int) bool { return extras[branch][i].Path < extras[branch][j].Path })
		result = append(result, extras[branch]...)
	}
	return result
}
func TestProposeArgumentReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/execute-propose.json")
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
	if len(fixture.Cases) != 322 {
		t.Fatal(len(fixture.Cases))
	}
	validator, _ := NewArguments()
	for index, tc := range fixture.Cases {
		got, err := validator.Validate("propose", tc.Arguments, tc.Strict)
		if tc.Issues != nil {
			validation, ok := err.(*ValidationError)
			if !ok || !reflect.DeepEqual(normalizeExtraRuns(validation.Issues), normalizeExtraRuns(tc.Issues)) {
				t.Fatal(index, tc.Arguments, validation, tc.Issues)
			}
		} else if err != nil || !reflect.DeepEqual(got, tc.Expected) {
			t.Fatal(index, tc.Arguments, got, err, tc.Expected)
		}
	}
}
