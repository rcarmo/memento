package execute

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

type runnerCase struct {
	Name    string
	Plan    map[string]any
	Replies []map[string]any
	Limits  map[string]any
	Result  map[string]any
	Calls   []map[string]any
}

func decodeReply(raw map[string]any) DispatchResult {
	result := DispatchResult{Status: raw["status"].(string), Data: map[string]any{}}
	if v, ok := raw["data"].(map[string]any); ok {
		result.Data = v
	}
	if v, ok := raw["error_class"].(string); ok {
		result.ErrorClass = v
	}
	if v, ok := raw["message"].(string); ok {
		result.Message = v
	}
	for key, target := range map[string]**string{"repo_revision": &result.RepoRevision, "index_revision": &result.IndexRevision, "operation_id": &result.OperationID} {
		if v, ok := raw[key].(string); ok {
			value := v
			*target = &value
		}
	}
	if result.Status == "success" {
		if result.RepoRevision == nil {
			v := "r"
			result.RepoRevision = &v
		}
		if result.IndexRevision == nil {
			v := "i"
			result.IndexRevision = &v
		}
	}
	return result
}
func runnerJSON(value any) any {
	raw, _ := json.Marshal(value)
	var out any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	_ = decoder.Decode(&out)
	return out
}
func TestRunnerReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/execute-runner.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []runnerCase
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	planner, _ := NewPlanner()
	arguments, _ := NewArguments()
	for _, tc := range cases {
		plan, err := planner.ParsePlan(tc.Plan)
		if err != nil {
			t.Fatal(tc.Name, err)
		}
		replies := append([]map[string]any{}, tc.Replies...)
		calls := []map[string]any{}
		dispatch := func(_ context.Context, op string, args map[string]any) (DispatchResult, error) {
			calls = append(calls, map[string]any{"method": "memory_" + op, "args": args})
			reply := decodeReply(replies[0])
			replies = replies[1:]
			return reply, nil
		}
		limitsRaw := tc.Limits
		if limitsRaw == nil {
			limitsRaw = map[string]any{}
		}
		limits, _ := ParseLimits(limitsRaw)
		runner := &Runner{Planner: planner, Arguments: arguments, Limits: limits, Dispatch: dispatch}
		clock := []float64{0, 0, 0, 0, 0}
		if tc.Name == "deadline_before_commit" {
			clock = []float64{0, 4}
		} else if tc.Name == "deadline_after_commit" {
			clock = []float64{0, 0, 4}
		}
		index := 0
		runner.Now = func() float64 { value := clock[min(index, len(clock)-1)]; index++; return value }
		got := runner.Run(context.Background(), plan)
		actual := map[string]any{"status": got.Status, "warnings": got.Warnings, "repo_revision": nil, "index_revision": nil, "index_stale": false, "operation_id": nil}
		if got.Status == "success" {
			actual["data"] = got.Data
			actual["next_tools"] = []any{}
			actual["repo_revision"] = got.RepoRevision
			actual["index_revision"] = got.IndexRevision
		} else {
			actual["error_class"] = got.ErrorClass
			actual["message"] = got.Message
		}
		if !reflect.DeepEqual(runnerJSON(actual), runnerJSON(tc.Result)) || !reflect.DeepEqual(runnerJSON(calls), runnerJSON(tc.Calls)) {
			t.Fatal(tc.Name, actual, tc.Result, calls, tc.Calls)
		}
	}
}
func TestRunnerErrors(t *testing.T) {
	limits, _ := ParseLimits(map[string]any{})
	runner, err := NewRunner(limits, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := runner.Run(context.Background(), Plan{}); got.Status != "error" {
		t.Fatal(got)
	}
	runner.Dispatch = func(context.Context, string, map[string]any) (DispatchResult, error) {
		return DispatchResult{}, errors.New("dispatch")
	}
	plan := Plan{Operations: []PlannedOperation{{Op: "read", Args: map[string]any{"id_or_path": "/a"}}}}
	runner.Now = func() float64 { return 0 }
	if got := runner.Run(context.Background(), plan); got.Message != "dispatch" {
		t.Fatal(got)
	}
}
