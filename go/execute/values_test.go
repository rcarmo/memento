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

type expected struct {
	Expected any
	Error    string
	Type     string
}

func checkExpected(t *testing.T, value any, err error, want expected) {
	t.Helper()
	if want.Error != "" {
		var cause *Error
		if !errors.As(err, &cause) || cause.Kind != want.Type || err.Error() != want.Error {
			t.Fatal(value, err, want)
		}
		return
	}
	if err != nil || !reflect.DeepEqual(value, want.Expected) {
		t.Fatalf("got %#v / %v want %#v", value, err, want.Expected)
	}
}
func TestExecutorValueReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/execute-values.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Saved                 map[string]any
		Contains, Resolutions []struct {
			Value any
			expected
		}
		References []struct {
			Ref string
			expected
		}
		Extracts []struct {
			Value any
			Path  string
			expected
		}
		Bounds []struct {
			Value    any
			Limit    int
			Expected any
		}
		Projections []struct {
			Operations []struct {
				Operation string
				SaveAs    *string `json:"save_as"`
			}
			Returns    []ReturnProjection
			Failed     []string
			Last       any
			MaxRecords int `json:"max_records"`
			expected
		}
		Outputs []struct {
			Payload     map[string]any
			MaxBytes    int `json:"max_bytes"`
			Committed   bool
			Size        int
			Ensure, Fit expected
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	for i, tc := range fixture.Contains {
		t.Run(fmt.Sprintf("contains-%d", i), func(t *testing.T) { v, err := ContainsReferences(tc.Value); checkExpected(t, v, err, tc.expected) })
	}
	for i, tc := range fixture.Resolutions {
		t.Run(fmt.Sprintf("resolve-%d", i), func(t *testing.T) {
			v, err := ResolveReferences(tc.Value, fixture.Saved)
			checkExpected(t, v, err, tc.expected)
		})
	}
	for i, tc := range fixture.References {
		t.Run(fmt.Sprintf("reference-%d", i), func(t *testing.T) {
			v, err := ResolveReference(tc.Ref, fixture.Saved)
			checkExpected(t, v, err, tc.expected)
		})
	}
	for i, tc := range fixture.Extracts {
		t.Run(fmt.Sprintf("extract-%d", i), func(t *testing.T) { v, err := ExtractField(tc.Value, tc.Path); checkExpected(t, v, err, tc.expected) })
	}
	for _, tc := range fixture.Bounds {
		checkExpected(t, BoundValue(tc.Value, tc.Limit), nil, expected{Expected: tc.Expected})
	}
	for i, tc := range fixture.Projections {
		t.Run(fmt.Sprintf("projection-%d", i), func(t *testing.T) {
			ops := []SavedOperation{}
			for _, op := range tc.Operations {
				ops = append(ops, SavedOperation{op.Operation, op.SaveAs})
			}
			failed := map[string]bool{}
			for _, name := range tc.Failed {
				failed[name] = true
			}
			v, err := ProjectReturns(ops, tc.Returns, fixture.Saved, tc.Last, failed, tc.MaxRecords)
			checkExpected(t, v, err, tc.expected)
		})
	}
	for i, tc := range fixture.Outputs {
		t.Run(fmt.Sprintf("output-%d", i), func(t *testing.T) {
			before, _ := json.Marshal(tc.Payload)
			size, err := PayloadSize(tc.Payload)
			if err != nil || size != tc.Size {
				t.Fatal(size, tc.Size, err)
			}
			checkExpected(t, nil, EnsureOutputSize(tc.Payload, tc.MaxBytes, tc.Committed), tc.Ensure)
			v, err := FitOutputPayload(tc.Payload, tc.MaxBytes, tc.Committed)
			checkExpected(t, v, err, tc.Fit)
			after, _ := json.Marshal(tc.Payload)
			if !bytes.Equal(before, after) {
				t.Fatal("mutated caller input")
			}
		})
	}
}
func TestValueKernelFailuresAndIsolation(t *testing.T) {
	bad := map[string]any{"bad": make(chan int)}
	if _, err := PayloadSize(bad); err == nil {
		t.Fatal("unsupported JSON")
	}
	if err := EnsureOutputSize(bad, 100, true); err == nil {
		t.Fatal("ensure JSON")
	}
	if _, err := FitOutputPayload(bad, 100, true); err == nil {
		t.Fatal("fit JSON")
	}
	name := "same"
	saved := map[string]any{"same": nil}
	trace := []map[string]any{{"status": "success", "save_as": "ok"}, {"status": "error", "save_as": nil}, {"status": "error", "save_as": "same"}, {"status": "error", "save_as": "missing"}}
	if got := FailedSaveNames(trace, saved); !reflect.DeepEqual(got, map[string]bool{"missing": true}) {
		t.Fatal(got)
	}
	value := map[string]any{"rows": []any{map[string]any{"v": json.Number("9007199254740993")}, map[string]any{"v": nil}}}
	original, _ := json.Marshal(value)
	var wg sync.WaitGroup
	fail := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := ProjectReturns([]SavedOperation{{"proposal_asset_get", &name}}, []ReturnProjection{{Ref: "$same.rows", Fields: []string{"v"}}}, map[string]any{"same": value}, nil, nil, 1)
			if err == nil && len(result["same_rows"].([]any)) != 2 {
				err = fmt.Errorf("asset list truncated")
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
	after, _ := json.Marshal(value)
	if !bytes.Equal(original, after) {
		t.Fatal("input mutated")
	}
}
