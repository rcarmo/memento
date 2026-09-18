package execute

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rcarmo/memento/go/internal/pyjson"
)

func FuzzExecutorValues(f *testing.F) {
	for _, s := range []string{`{"key":"$saved.rows.0.v"}`, `["$saved","$invalid..x"]`, `"$saved.rows.999999999999999999999999"`, `{"k":[1,2,{"deep":[3,4]}]}`, `null`, `"$saved.rows.name"`} {
		f.Add(s, uint8(2), uint16(512))
	}
	f.Fuzz(func(t *testing.T, raw string, limit uint8, budget uint16) {
		if len(raw) > 4096 {
			return
		}
		value, err := pyjson.Parse(raw)
		if err != nil {
			return
		}
		// The port's JSON writer intentionally caps recursion. Inputs rejected by
		// that runtime boundary are not admitted to an execute runner.
		before, err := pyjson.Dumps(value)
		if err != nil {
			return
		}
		saved := map[string]any{"saved": value}
		_, _ = ContainsReferences(value)
		_, _ = ResolveReferences(value, saved)
		if text, ok := value.(string); ok {
			_, _ = ResolveReference(text, saved)
			_, _ = ExtractField(value, text)
		}
		n := int(limit % 8)
		bounded := BoundValue(value, n)
		var check func(any)
		check = func(v any) {
			switch x := v.(type) {
			case []any:
				if len(x) > n {
					t.Fatal("bound exceeded")
				}
				for _, item := range x {
					check(item)
				}
			case map[string]any:
				for _, item := range x {
					check(item)
				}
			}
		}
		check(bounded)
		payload := map[string]any{"trace": []any{map[string]any{"data": value}}, "returns": map[string]any{"result": value}, "revisions": []any{map[string]any{"operation_id": "synthetic"}}}
		fitted, err := FitOutputPayload(payload, int(budget), true)
		if err == nil {
			size, err := PayloadSize(fitted)
			if err != nil || size > int(budget) {
				t.Fatal(size, err)
			}
		}
		after, err := pyjson.Dumps(value)
		if err != nil || after != before {
			t.Fatal("input changed", err)
		}
	})
}
func TestReferenceLimitsAndHugeIndices(t *testing.T) {
	saved := map[string]any{"rows": []any{1}}
	for _, ref := range []string{"$rows." + strings.Repeat("9", 500), "$missing", "$rows.name"} {
		if _, err := ResolveReference(ref, saved); err == nil {
			t.Fatal(ref)
		}
	}
	long := "$a." + strings.Repeat("b", 253)
	if len(long) != 256 {
		t.Fatal(len(long))
	}
	if ok, err := ContainsReferences(long); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if _, err := ContainsReferences(long + "b"); err == nil {
		t.Fatal("reference length")
	}
	// Asset source operation exemptions survive a later non-asset save_as reuse.
	name := "rows"
	zero := ""
	ops := []SavedOperation{{"asset_get", &name}, {"list", &name}}
	got, err := ProjectReturns(ops, []ReturnProjection{{Ref: "$rows", Name: &zero}}, map[string]any{"rows": []any{1, 2, 3}}, nil, nil, 1)
	if err != nil || len(got["rows"].([]any)) != 3 {
		t.Fatal(got, err)
	}
	// Index arithmetic must never round through float64.
	value := json.Number("9007199254740993")
	got, err = ProjectReturns(nil, []ReturnProjection{{Ref: "$value"}}, map[string]any{"value": value}, nil, nil, 1)
	if err != nil || fmt.Sprint(got["value"]) != string(value) {
		t.Fatal(got, err)
	}
}
