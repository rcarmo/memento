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

func FuzzExecutorPlan(f *testing.F) {
	for _, raw := range []string{`{"operations":[]}`, `{"operations":[{"op":"read","args":{"id_or_path":"$saved.path"}}]}`, `{"returns":[{"ref":"$x","limit":"1_000"}]}`, `{"operations":[{"op":"patch","save_as":"bad name"}],"stop_on_error":"yes"}`, `{"operations":null,"returns":[{}]}`, `{"max_time_seconds":"1e9999","max_output_bytes":99999999999999999999}`} {
		f.Add(raw)
	}
	planner, err := NewPlanner()
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 8192 {
			return
		}
		value, err := pyjson.Parse(raw)
		if err != nil {
			return
		}
		before, err := pyjson.Dumps(value)
		if err != nil {
			return
		}
		_, _ = ParseLimits(value)
		if args, ok := value.(map[string]any); ok {
			_, _ = NormalizeToolArguments(args)
		}
		plan, err := planner.ParsePlan(value)
		if err != nil {
			if len([]rune(err.Error())) > 512 {
				t.Fatal("unbounded validation message")
			}
		} else {
			// Round-trip structure only; no no-op argument validator is allowed to
			// disguise the unimplemented per-operation validation boundary.
			encoded, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := pyjson.Parse(string(encoded))
			if err != nil {
				t.Fatal(err)
			}
			again, err := planner.ParsePlan(decoded)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(again)
			if string(encoded) != string(b) {
				t.Fatal("non-idempotent plan")
			}
			_ = planner.Preflight(plan, 12, nil)
		}
		after, err := pyjson.Dumps(value)
		if err != nil || after != before {
			t.Fatal("input mutated", err)
		}
	})
}

func FuzzExecutorArguments(f *testing.F) {
	for _, raw := range []string{`{}`, `{"limit":"1_000"}`, `{"query":"x","limit":2.0}`, `{"fields":["path",null,true]}`, `{"path":"x","expected_revision":"r","idempotency_key":"k","tags":["a"]}`, `{"id_or_path":"x","cursor":"cccc"}`, `{"confirm":"yes"}`, `{"selected_change_indexes":[1,"2",true]}`} {
		f.Add(raw, uint8(0), false)
	}
	validator, err := NewArguments()
	if err != nil {
		f.Fatal(err)
	}
	planner, err := NewPlanner()
	if err != nil {
		f.Fatal(err)
	}
	operations := []string{"inventory", "search", "asset_metadata", "patch", "purge", "proposal_revise", "asset_get", "proposal_review", "propose"}
	f.Fuzz(func(t *testing.T, raw string, index uint8, strict bool) {
		if len(raw) > 8192 {
			return
		}
		value, err := pyjson.Parse(raw)
		if err != nil {
			return
		}
		args, ok := value.(map[string]any)
		if !ok {
			return
		}
		before, err := pyjson.Dumps(args)
		if err != nil {
			return
		}
		op := operations[int(index)%len(operations)]
		result, err := validator.Validate(op, args, strict)
		if err == nil {
			// Successfully normalised wire values must pass strict validation,
			// preserve every field and keep integers exact on a second pass.
			again, err := validator.Validate(op, result, true)
			if err != nil {
				t.Fatal("normalisation was not strict-valid", err)
			}
			a, err := pyjson.Dumps(result)
			if err != nil {
				t.Fatal(err)
			}
			b, err := pyjson.Dumps(again)
			if err != nil || a != b {
				t.Fatal("non-idempotent argument model", err)
			}
		} else if v, ok := err.(*ValidationError); ok {
			if len([]rune(v.Message("operation 1 ("+op+")"))) > 512 {
				t.Fatal("unbounded diagnostic")
			}
		}
		plan := Plan{Operations: []PlannedOperation{{Op: op, Args: args}}}
		_ = planner.Preflight(plan, 12, validator.Validate)
		_, _ = ResolveArguments(plan.Operations[0], 1, map[string]any{"saved": args}, validator.Validate)
		after, err := pyjson.Dumps(args)
		if err != nil || before != after {
			t.Fatal("input mutated", err)
		}
	})
}
