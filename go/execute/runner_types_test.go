package execute

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestRunnerTypes(t *testing.T) {
	x := "x"
	if pointerValue(nil) != nil || pointerValue(&x) != "x" {
		t.Fatal("pointer")
	}
	for _, tc := range []struct {
		value json.Number
		want  int
	}{{"0", 0}, {"12", 12}, {"-1", 0}, {"bad", 0}, {"999999999999999999999999", math.MaxInt}} {
		if got := executeLimit(tc.value); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	failure := runFailure("bad", "message")
	if failure.Status != "error" || failure.Warnings == nil {
		t.Fatal(failure)
	}
	success := runSuccess(map[string]any{}, []string{})
	if success.Status != "success" || success.RepoRevision != "final" {
		t.Fatal(success)
	}
	name := "saved"
	limit := json.Number("3")
	plan := Plan{Operations: []PlannedOperation{{Op: "read", SaveAs: &name}}, Returns: []PlannedReturn{{Name: &name, Ref: "$saved", Fields: []string{"path"}, Limit: &limit}}}
	if !reflect.DeepEqual(savedOperations(plan), []SavedOperation{{"read", &name}}) {
		t.Fatal("operations")
	}
	projections := returnProjections(plan)
	if len(projections) != 1 || *projections[0].Limit != 3 {
		t.Fatal(projections)
	}
	if returnProjections(Plan{}) == nil || savedOperations(Plan{}) == nil {
		t.Fatal("empty slices")
	}
}
