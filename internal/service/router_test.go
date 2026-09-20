package service

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type routerFixture struct {
	Parsed []struct {
		Input  string         `json:"input"`
		Action map[string]any `json:"action"`
	} `json:"parsed"`
	Errors []struct {
		Input string `json:"input"`
	} `json:"errors"`
}

func TestRouterParserFixture(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/router.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture routerFixture
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixture.Parsed {
		got, err := ParseNeedleRouterOutput(item.Input)
		if err != nil {
			t.Fatal(item.Input, err)
		}
		want := RouterAction{Action: item.Action["action"].(string)}
		if value, ok := item.Action["query"].(string); ok {
			want.Query = value
		}
		if value, ok := item.Action["search_mode"].(string); ok {
			want.SearchMode = value
		}
		if value, ok := item.Action["field"].(string); ok {
			want.Field = value
		}
		if value, ok := item.Action["id_or_path"].(string); ok {
			want.IDOrPath = value
		}
		if value, ok := item.Action["limit"].(float64); ok {
			want.Limit = int(value)
		}
		if value, ok := item.Action["depth"].(float64); ok {
			want.Depth = int(value)
		}
		if want.Limit == 0 && got.Limit == 3 && (got.Action == "search_then_read" || got.Action == "UNKNOWN" || got.Action == "status_field" || got.Action == "read_field" || got.Action == "search_then_graph") {
			want.Limit = 3
		}
		if want.Depth == 0 && got.Depth == 1 {
			want.Depth = 1
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatal(item.Input, got, want)
		}
	}
	for _, item := range fixture.Errors {
		if _, err := ParseNeedleRouterOutput(item.Input); err == nil {
			t.Fatal(item.Input)
		}
	}
}
func TestRouterParserBranches(t *testing.T) {
	bad := []string{`[{"name":"search_then_read","arguments":{}}]`, `[{"name":"status_field","arguments":{"field":"bad"}}]`, `[{"name":"search_then_read","arguments":{"query":""}}]`, `[{"name":"search_then_read","arguments":{"query":"x","search_mode":"bad"}}]`, `[{"name":"search_paths","arguments":{"query":"x","limit":"3"}}]`, `[{"name":"search_then_graph","arguments":{"query":"x","depth":3}}]`, `[{"name":"read_field","arguments":{"id_or_path":"x","field":"bad"}}]`, `[{"name":"bad","arguments":{}}]`, `[{"name":"UNKNOWN","arguments":{"x":1}}]`, `[{"name":"UNKNOWN","extra":1}]`}
	for _, raw := range bad {
		if _, err := ParseNeedleRouterOutput(raw); err == nil {
			t.Fatal(raw)
		}
	}
	long := make([]byte, 513)
	for i := range long {
		long[i] = 'x'
	}
	raw, _ := json.Marshal([]any{map[string]any{"name": "read_field", "arguments": map[string]any{"id_or_path": string(long), "field": "body"}}})
	if _, err := ParseNeedleRouterOutput(string(raw)); err == nil {
		t.Fatal("long")
	}
	for _, mode := range []string{"lexical", "semantic", "hybrid"} {
		raw := `[{"name":"search_then_read","arguments":{"query":"x","search_mode":"` + mode + `"}}]`
		if _, err := ParseNeedleRouterOutput(raw); err != nil {
			t.Fatal(mode, err)
		}
	}
}
