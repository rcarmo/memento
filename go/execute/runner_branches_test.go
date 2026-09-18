package execute

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunnerConstructorFailures(t *testing.T) {
	r, _ := NewRunner(runnerLimitsForTest(), nil)
	if r.Now() <= 0 {
		t.Fatal("clock")
	}

	oldOps, oldArgs := operationDefinitions, argumentDefinitions
	defer func() { operationDefinitions, argumentDefinitions = oldOps, oldArgs }()
	operationDefinitions = []byte("{")
	if _, err := NewRunner(runnerLimitsForTest(), nil); err == nil {
		t.Fatal("planner")
	}
	operationDefinitions = oldOps
	argumentDefinitions = []byte("{")
	if _, err := NewRunner(runnerLimitsForTest(), nil); err == nil {
		t.Fatal("arguments")
	}
}
func runnerLimitsForTest() Limits {
	return Limits{MaxOperations: 12, MaxIntermediates: 12, MaxRecords: 50, MaxOutputBytes: json.Number("65536"), MaxTimeSeconds: 3}
}
func TestRunnerFailureBranches(t *testing.T) {
	good := func(context.Context, string, map[string]any) (DispatchResult, error) {
		r := "r"
		i := "i"
		return DispatchResult{Status: "success", Data: map[string]any{}, RepoRevision: &r, IndexRevision: &i}, nil
	}
	runner, _ := NewRunner(runnerLimitsForTest(), good)
	runner.Now = func() float64 { return 0 }
	if got := runner.Run(context.Background(), Plan{Operations: []PlannedOperation{{Op: "unknown", Args: map[string]any{}}}}); got.Status != "error" {
		t.Fatal(got)
	}
	badName := "bad name"
	if got := runner.Run(context.Background(), Plan{Operations: []PlannedOperation{{Op: "read", Args: map[string]any{"id_or_path": "/a"}, SaveAs: &badName}}}); got.Status != "error" {
		t.Fatal(got)
	}
	if got := runner.Run(context.Background(), Plan{Operations: []PlannedOperation{{Op: "read", Args: map[string]any{"id_or_path": "$missing"}}}}); got.Status != "error" {
		t.Fatal(got)
	}
	calls := 0
	runner.Now = func() float64 {
		calls++
		if calls > 2 {
			return 4
		}
		return 0
	}
	if got := runner.Run(context.Background(), Plan{Operations: []PlannedOperation{{Op: "read", Args: map[string]any{"id_or_path": "/a"}}}}); got.Status != "error" {
		t.Fatal(got)
	}
	limits := runnerLimitsForTest()
	limits.MaxOutputBytes = "512"
	runner, _ = NewRunner(limits, func(context.Context, string, map[string]any) (DispatchResult, error) {
		r := "r"
		i := "i"
		return DispatchResult{Status: "success", Data: map[string]any{"blob": strings.Repeat("x", 2000)}, RepoRevision: &r, IndexRevision: &i}, nil
	})
	runner.Now = func() float64 { return 0 }
	if got := runner.Run(context.Background(), Plan{Operations: []PlannedOperation{{Op: "read", Args: map[string]any{"id_or_path": "/a"}}}}); got.Status != "error" {
		t.Fatal(got)
	}
	limits.MaxOutputBytes = "1"
	runner, _ = NewRunner(limits, good)
	runner.Now = func() float64 { return 0 }
	if got := runner.Run(context.Background(), Plan{Operations: []PlannedOperation{{Op: "create", Args: map[string]any{"path": "/a", "concept_type": "note", "title": "t", "body": "b", "expected_revision": "r", "idempotency_key": "k"}}}}); got.Status != "error" {
		t.Fatal(got)
	}
}
