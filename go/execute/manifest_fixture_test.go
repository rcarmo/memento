package execute

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestManifestArgumentFixtureContract(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/execute-manifest.json")
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
}
