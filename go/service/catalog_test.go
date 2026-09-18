package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rcarmo/memento/go/umcp"
)

type catalogFixture struct {
	Cases []struct {
		Surface       string
		Answer, Route bool
		Catalog, Help map[string]any
		Tools         []any
	}
	Resources, Templates, Prompts []any
	PromptCases                   []struct {
		Arguments map[string]any
		Expected  string
	} `json:"prompt_cases"`
	Hidden map[string]any
}

func loadCatalogFixture(t *testing.T) catalogFixture {
	t.Helper()
	raw, err := os.ReadFile("../testdata/parity/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var f catalogFixture
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}
func TestCatalogReference(t *testing.T) {
	f := loadCatalogFixture(t)
	for _, tc := range f.Cases {
		t.Run(fmt.Sprintf("%s-%t-%t", tc.Surface, tc.Answer, tc.Route), func(t *testing.T) {
			c, err := NewCatalog(CatalogConfig{Surface: tc.Surface, AnswerEnabled: tc.Answer, RouteEnabled: tc.Route})
			if err != nil {
				t.Fatal(err)
			}
			if got := jsonNormal(c.Payload()); !reflect.DeepEqual(got, tc.Catalog) {
				t.Fatal("catalog mismatch", got, tc.Catalog)
			}
			if got := jsonNormal(c.Help()); !reflect.DeepEqual(got, tc.Help["data"]) {
				t.Fatal("help mismatch", got, tc.Help["data"])
			}
			// Test-only echo callbacks prove registration and protocol discovery, not
			// implementation of the remaining service methods or production readiness.
			handlers := map[string]CatalogHandler{}
			for _, op := range c.source.Operations {
				handlers[op.Tool] = func(context.Context, map[string]any) (any, error) { return map[string]any{"test_only": true}, nil }
			}
			server := umcp.NewServer("catalog-test")
			server.SetNotificationOutput(nil)
			if err = c.Register(server, handlers, nil); err != nil {
				t.Fatal(err)
			}
			result, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "reader"})
			if err != nil || !reflect.DeepEqual(jsonNormal(result.Result).(map[string]any)["tools"], tc.Tools) {
				t.Fatal(result, tc.Tools, err)
			}
		})
	}
	c, _ := NewCatalog(CatalogConfig{Surface: "read_only", AnswerEnabled: true, RouteEnabled: true})
	hidden, err := c.Operation("propose")
	if err != nil || !reflect.DeepEqual(jsonNormal(hidden), f.Hidden) {
		t.Fatal(hidden, f.Hidden, err)
	}
	for _, tc := range f.PromptCases {
		args := map[string]any{"target_path": "/skills/example.md", "asset_kind": "skill", "version": "1.0.0"}
		for k, v := range tc.Arguments {
			args[k] = v
		}
		got, err := assetPublicationPrompt(args)
		if err != nil || got != tc.Expected {
			t.Fatal(got, tc.Expected, err)
		}
	}
}
func TestCatalogGuardsAndIsolation(t *testing.T) {
	if _, err := NewCatalog(CatalogConfig{Surface: "bad"}); err == nil {
		t.Fatal("surface")
	}
	if _, err := newCatalog(CatalogConfig{Surface: "compact"}, []byte("{")); err == nil {
		t.Fatal("bad data")
	}
	c, _ := NewCatalog(CatalogConfig{Surface: "compact"})
	server := umcp.NewServer("guard")
	server.SetNotificationOutput(nil)
	if err := c.Register(server, nil, nil); err == nil || !strings.Contains(err.Error(), "missing handlers") {
		t.Fatal(err)
	}
	response, _ := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{})
	if len(jsonNormal(response.Result).(map[string]any)["tools"].([]any)) != 0 {
		t.Fatal("partial registration")
	}
	// Even read_only must supply hidden dispatch handlers, not merely visible
	// tools. No pretend full surface over the implemented development subset.
	readOnly, _ := NewCatalog(CatalogConfig{Surface: "read_only"})
	partial := testCatalogHandlers(readOnly)
	delete(partial, "memory_create")
	if err := readOnly.Register(server, partial, nil); err == nil {
		t.Fatal("missing hidden handler")
	}
	if _, err := c.Operation("bad"); err == nil {
		t.Fatal("operation")
	}
	if _, err := c.Workflow("bad"); err == nil {
		t.Fatal("workflow")
	}
	if err := c.register(server, nil, nil, []byte("{")); err == nil {
		t.Fatal("bad definitions")
	}
	// Generated JSON decoding itself fails closed; invalid limits do not leak
	// into other catalog instances via caller-owned maps.
	limits := map[string]any{"max_records": json.Number("7")}
	configured, _ := NewCatalog(CatalogConfig{Surface: "read_only", ExecuteLimits: limits})
	limits["max_records"] = 0
	if jsonNormal(configured.Help()).(map[string]any)["mcp"].(map[string]any)["execute_limits"].(map[string]any)["max_records"] != float64(7) {
		t.Fatal("aliased limits")
	}
	contract, _ := c.Operation("read")
	contract["input_schema"].(map[string]any)["broken"] = true
	again, _ := c.Operation("read")
	if again["input_schema"].(map[string]any)["broken"] != nil {
		t.Fatal("aliased schema")
	}
	if _, err := assetPublicationPrompt(map[string]any{"target_path": 1}); err == nil {
		t.Fatal("type")
	}
	if _, err := assetPublicationPrompt(map[string]any{"target_path": string([]byte{255}), "asset_kind": "docs", "version": "1.0.0"}); err == nil {
		t.Fatal("invalid UTF-8")
	}
	if _, err := catalogResource(map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("JSON")
	}
	// More deeply nested values can marshal but exceed the pyjson depth cap.
	var nested any = "leaf"
	for range 110 {
		nested = map[string]any{"x": nested}
	}
	if _, err := catalogResource(nested); err == nil {
		t.Fatal("depth")
	}
	var wg sync.WaitGroup
	fail := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := c.Payload()
			delete(p, "workflows")
			help := c.Help()
			if help["mcp"] == nil {
				fail <- io.ErrUnexpectedEOF
			}
		}()
	}
	wg.Wait()
	close(fail)
	for err := range fail {
		t.Fatal(err)
	}
}
