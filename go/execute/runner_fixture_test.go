package execute

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestRunnerFixtureContract(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/execute-runner.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name    string
		Plan    map[string]any
		Replies []map[string]any
		Result  map[string]any
		Calls   []map[string]any
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 11 {
		t.Fatal("review runner corpus", len(cases))
	}
	names := map[string]bool{}
	for _, tc := range cases {
		if tc.Name == "" || tc.Plan == nil || tc.Result["status"] == nil || names[tc.Name] {
			t.Fatal(tc.Name)
		}
		names[tc.Name] = true
	}
	for _, required := range []string{"saved_reference", "stop_on_error", "continue_on_error", "deadline_before_commit", "deadline_after_commit", "post_commit_projection_error", "failed_save_return_skipped", "output_limit_after_commit"} {
		if !names[required] {
			t.Fatal(required)
		}
	}
}
