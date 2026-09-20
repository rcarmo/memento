package umcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
)

func referencePrompts(t *testing.T) *PromptRegistry {
	t.Helper()
	r := &PromptRegistry{}
	prompts := []Prompt{
		{Name: "hello", CallableDescription: "Hello prompt.\n\nCategories: Work, work; AI / bad category | good_tag", Description: "Discovery override", Categories: []string{"explicit"}, Parameters: []Parameter{{Name: "name", Types: []ParamType{StringParam}, HasDefault: true, Default: "world"}}, Call: func(_ context.Context, a map[string]any) (any, error) { return "Hello " + a["name"].(string), nil }},
		{Name: "count", Parameters: []Parameter{{Name: "value", Types: []ParamType{IntegerParam}}}, Call: func(_ context.Context, a map[string]any) (any, error) { return a["value"], nil }},
		{Name: "messages", Call: func(context.Context, map[string]any) (any, error) {
			return []any{OrderedObject{{"role", "assistant"}, {"content", OrderedObject{{"type", "text"}, {"text", "message"}}}}}, nil
		}},
		{Name: "body", CallableDescription: "Body prompt.\n\nCategories: original", Call: func(context.Context, map[string]any) (any, error) {
			return OrderedObject{{"messages", []any{}}, {"description", "override"}, {"custom", true}}, nil
		}},
		{Name: "scalar", Call: func(context.Context, map[string]any) (any, error) { return []any{json.Number("1"), nil, true}, nil }},
		{Name: "mapping", Call: func(context.Context, map[string]any) (any, error) {
			return OrderedObject{{"z", json.Number("1")}, {"a", json.Number("2")}}, nil
		}},
		{Name: "fail", Call: func(context.Context, map[string]any) (any, error) {
			return nil, ArgumentError("private argument failure")
		}},
		{Name: "cancelled", Call: func(context.Context, map[string]any) (any, error) { return nil, ErrRequestCancelled }},
	}
	for _, p := range prompts {
		p.InputSchema = map[string]any{}
		if err := r.Register(p); err != nil {
			t.Fatal(err)
		}
	}
	return r
}
func TestPromptSyncAsyncParity(t *testing.T) {
	data, err := os.ReadFile("testdata/parity/umcp-prompts.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Request, Transport string
			Expected           json.RawMessage
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	r := referencePrompts(t)
	d := Dispatcher{Handlers: map[string]Handler{"prompts/list": r.List, "prompts/get": r.Get}}
	for _, c := range fixture.Cases {
		response, err := d.Process(context.Background(), []byte(c.Request), RequestContext{Transport: c.Transport, Principal: "reader"})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := decode(raw)
		want, _ := decode(c.Expected)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s %s\ngot %s\nwant %s", c.Transport, c.Request, raw, c.Expected)
		}
	}
}
func TestPromptLifecycleAndEdges(t *testing.T) {
	var r PromptRegistry
	noop := func(context.Context, map[string]any) (any, error) { return nil, nil }
	if err := r.Register(Prompt{}); err == nil {
		t.Fatal("no callback")
	}
	notifications := 0
	r.Notify = func(method string, p map[string]any) error {
		if method != "notifications/prompts/list_changed" || len(p) != 0 {
			t.Fatal(method, p)
		}
		notifications++
		return nil
	}
	p := Prompt{Name: "x", InputSchema: map[string]any{"type": "object"}, Parameters: []Parameter{{Name: "a", Default: "default", HasDefault: true}}, Call: noop}
	if err := r.RegisterAndNotify(p); err != nil {
		t.Fatal(err)
	}
	p.InputSchema["type"] = "mutated"
	result, _, _ := r.List(context.Background(), nil)
	if result.(map[string]any)["prompts"].([]any)[0].(map[string]any)["inputSchema"].(map[string]any)["type"] != "object" {
		t.Fatal("alias")
	}
	if ok, err := r.UnregisterAndNotify("x"); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, err := r.UnregisterAndNotify("missing"); ok || err != nil {
		t.Fatal(ok, err)
	}
	if notifications != 2 {
		t.Fatal(notifications)
	}
	if err := r.RegisterAndNotify(Prompt{}); err == nil {
		t.Fatal("bad prompt")
	}
	r.Notify = nil
	_ = r.RegisterAndNotify(Prompt{Name: "empty", Call: noop})
	_, _ = r.UnregisterAndNotify("empty")
	boom := errors.New("notify")
	r.Notify = func(string, map[string]any) error { return boom }
	if err := r.RegisterAndNotify(Prompt{Name: "x", Call: noop}); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if ok, err := r.UnregisterAndNotify("x"); !ok || !errors.Is(err, boom) {
		t.Fatal(ok, err)
	}
	_ = r.Register(Prompt{Name: "bad", Call: func(context.Context, map[string]any) (any, error) { return make(chan int), nil }})
	for _, params := range []map[string]any{{"name": 1}, {"name": "bad", "arguments": []any{}}, {"name": "bad"}} {
		if _, _, err := r.Get(context.Background(), params); err == nil {
			t.Fatal(params)
		}
	}
	for _, value := range []any{[]any{map[string]any{"role": "x"}}, map[string]any{"messages": []any{}, "categories": []any{"override"}}, OrderedObject{{"messages", []any{}}}} {
		if _, err := PromptResult("doc", []string{"cat"}, value); err != nil {
			if _, ok := value.([]any); !ok {
				t.Fatal(err)
			}
		}
	}
	if got := PromptCategories("Categories: x, x; good-tag / bad token | _bad"); !reflect.DeepEqual(got, []string{"x", "x; good-tag / bad token | _bad"}) {
		t.Fatal(got)
	}
}
func TestConcurrentPromptRegistry(t *testing.T) {
	r := referencePrompts(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = r.List(context.Background(), nil)
			_, _, _ = r.Get(context.Background(), map[string]any{"name": "hello"})
		}()
	}
	wg.Wait()
}

func TestPromptMetadataEdges(t *testing.T) {
	r := &PromptRegistry{}
	if err := r.Register(Prompt{Name: "types", Categories: []string{}, Parameters: []Parameter{{Name: "untyped"}, {Name: "any", Types: []ParamType{AnyParam}}, {Name: "union", Types: []ParamType{StringParam, IntegerParam}}}, Call: func(context.Context, map[string]any) (any, error) { return nil, nil }}); err != nil {
		t.Fatal(err)
	}
	if got := PromptCategories("Category: Foo, ,Bar\nText [categories: foo, baz]\r[category:extra]"); !reflect.DeepEqual(got, []string{"foo", "bar", "baz", "extra"}) {
		t.Fatal(got)
	}
	if len(PromptCategories("")) != 0 {
		t.Fatal("empty categories")
	}
}
