package pydatetime

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDiagnosticBranches(t *testing.T) {
	cases := map[string]string{
		"2026-01":                 "Input should be a valid datetime, input is too short",
		"2026-01x01T00:00Z":       "Input should be a valid datetime, invalid date separator, expected `-`",
		"2026-01-01T0":            "Input should be a valid datetime, input is too short",
		"2026-01-01T12:34:5":      "Input should be a valid datetime, input is too short",
		"2026-01-01T12:34Zx":      "Input should be a valid datetime, unexpected extra characters at the end of the input",
		"2026-01-01T12:34:56.??":  "Input should be a valid datetime, invalid timezone sign",
		"2026-01-01T12:34?":       "Input should be a valid datetime, invalid timezone sign",
		"2026-01-01T12:34+x":      "Input should be a valid datetime, invalid timezone hour",
		"2026-01-01T12:34+01:x":   "Input should be a valid datetime, invalid timezone minute",
		"2026-01-01T12:34+01:x2":  "Input should be a valid datetime, invalid timezone minute",
		"2026-01-01T12:34+01:02x": "Input should be a valid datetime, unexpected extra characters at the end of the input",
	}
	if got := pydanticDiagnostic("2026-01-01T12:34Z", false); got != "" {
		t.Fatal(got)
	}
	if got := pydanticDiagnostic("2026-01-01T12:34", true); got != "" {
		t.Fatal(got)
	}
	if got := pydanticDiagnostic("2026-01-01T12:34Z", true); got != "" {
		t.Fatal(got)
	}
	if _, err := ParseJSON("2026-01-01T12:00:00", false); !errors.Is(err, ErrNaive) {
		t.Fatal(err)
	}
	if _, err := ParseJSON("1e9999", false); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if !dayOutOfRange("2026-00-01") {
		t.Fatal("month zero")
	}
	for input, want := range cases {
		if got := pydanticDiagnostic(input, true); got != want {
			t.Fatal(input, got, want)
		}
	}
}

func TestExecutorDiagnosticCorpus(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/execute-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Arguments map[string]any
			Strict    bool
			Issues    []struct{ Path, Message string }
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, tc := range fixture.Cases {
		var want string
		for _, issue := range tc.Issues {
			if issue.Path == "args.items.0.local_updated_at" && !strings.HasPrefix(issue.Message, "Value error") && issue.Message != "Field required" {
				want = issue.Message
				break
			}
		}
		if want == "" {
			continue
		}
		items, _ := tc.Arguments["items"].([]any)
		if len(items) == 0 {
			continue
		}
		item, _ := items[0].(map[string]any)
		value := item["local_updated_at"]
		_, err := ParseJSON(value, tc.Strict)
		if err == nil {
			t.Fatal(value, "accepted")
		}
		reason := Reason(err)
		if reason == "" {
			reason = "Input should be a valid datetime"
		}
		if reason != want {
			t.Fatal(value, tc.Strict, reason, want)
		}
		checked++
	}
	if checked < 100 {
		t.Fatal("small corpus", checked)
	}
}
