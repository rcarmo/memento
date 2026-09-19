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

func referenceResources(t *testing.T) *ResourceRegistry {
	t.Helper()
	r := &ResourceRegistry{}
	size := 7
	definitions := []Resource{
		{URI: "test:text", Name: "text", Title: "Title", Description: "Description", MIME: "text/markdown", Size: &size, Annotations: map[string]any{"priority": json.Number("1")}, Read: func(context.Context, map[string]any) (any, error) { return "hello ☃", nil }},
		{URI: "test:blob", Read: func(context.Context, map[string]any) (any, error) { return []byte{0, 1, 255}, nil }},
		{URI: "test:mixed", MIME: "text/custom", Read: func(context.Context, map[string]any) (any, error) {
			return []any{OrderedObject{{"text", "entry"}}, OrderedObject{{"uri", "custom:"}, {"mimeType", "custom/type"}, {"text", "override"}}, []any{"nested", nil, true, json.Number("7")}}, nil
		}},
		{URI: "test:failure", Read: func(context.Context, map[string]any) (any, error) { return nil, errors.New("private detail") }},
		{URI: "test:cancelled", Read: func(context.Context, map[string]any) (any, error) { return nil, ErrRequestCancelled }},
		{URI: "test:///{path}", Template: true, Name: "template", Title: "Template", Description: "path", MIME: "text/plain", Annotations: map[string]any{"priority": json.Number("0.5")}, Read: func(_ context.Context, p map[string]any) (any, error) { return "path=" + p["path"].(string), nil }},
	}
	for _, definition := range definitions {
		if err := r.Register(definition); err != nil {
			t.Fatal(err)
		}
	}
	return r
}
func TestResourceSyncAsyncParity(t *testing.T) {
	data, err := os.ReadFile("testdata/parity/umcp-resources.json")
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
	r := referenceResources(t)
	d := Dispatcher{Handlers: map[string]Handler{"resources/list": r.List, "resources/templates/list": r.ListTemplates, "resources/read": r.Read, "resources/subscribe": r.Subscribe, "resources/unsubscribe": r.Unsubscribe}}
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
func TestResourceEdges(t *testing.T) {
	var r ResourceRegistry
	noop := func(context.Context, map[string]any) (any, error) { return nil, nil }
	if err := r.Register(Resource{}); err == nil {
		t.Fatal("nil callback")
	}
	if err := r.Register(Resource{URI: "x/{a}/{a}", Template: true, Read: noop}); err == nil {
		t.Fatal("duplicate captures")
	}
	_, names, err := templatePattern("x/{9bad}/{good}")
	if err != nil || !reflect.DeepEqual(names, []string{"good"}) {
		t.Fatal(names, err)
	}
	for _, uri := range []string{"b", "a"} {
		if err := r.Register(Resource{URI: uri, Name: "same", Read: noop}); err != nil {
			t.Fatal(err)
		}
	}
	value, _, _ := r.List(context.Background(), nil)
	items := value.(map[string]any)["resources"].([]any)
	if items[0].(map[string]any)["uri"] != "a" {
		t.Fatal(items)
	}
	if _, _, err = r.Read(context.Background(), map[string]any{"uri": 1}); err == nil {
		t.Fatal("nonstring")
	}
	if _, _, err = r.Subscribe(context.Background(), map[string]any{"uri": 1}); err == nil {
		t.Fatal("nonstring subscribe")
	}
	if _, err := ResourceContents("x", "", map[string]any{"text": "a"}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{make(chan int), map[string]any{"bad": make(chan int)}, []any{make(chan int)}} {
		if _, err := ResourceContents("x", "", value); err == nil {
			t.Fatal("unsupported content")
		}
	}
	for _, value := range []any{"text", false, json.Number("1"), nil} {
		if _, err := ResourceContents("x", "", value); err != nil {
			t.Fatal(err)
		}
	}
	_ = r.Register(Resource{URI: "bad-result", Read: func(context.Context, map[string]any) (any, error) { return make(chan int), nil }})
	if _, _, err = r.Read(context.Background(), map[string]any{"uri": "bad-result"}); err == nil {
		t.Fatal("bad result")
	}
	_ = r.Register(Resource{URI: "binary", MIME: "custom/binary", Read: func(context.Context, map[string]any) (any, error) { return []byte{1}, nil }})
	if _, rpcErr, err := r.Read(context.Background(), map[string]any{"uri": "binary"}); rpcErr != nil || err != nil {
		t.Fatal(rpcErr, err)
	}
	for _, session := range []string{"", "s2", "s1"} {
		_, _, _ = r.Subscribe(context.Background(), map[string]any{"uri": "u", "_session_id": session})
	}
	if got := r.Subscribers("u"); !reflect.DeepEqual(got, []string{"", "s1", "s2"}) {
		t.Fatal(got)
	}
	_, _, _ = r.Subscribe(context.Background(), map[string]any{"uri": "other", "_session_id": "s1"})
	_, _, _ = r.Unsubscribe(context.Background(), map[string]any{"uri": "u", "_session_id": "s1"})
	_, _, _ = r.Unsubscribe(context.Background(), map[string]any{"uri": "u", "_session_id": "absent"})
	r.DropSession("s2")
	if got := r.Subscribers("u"); !reflect.DeepEqual(got, []string{""}) {
		t.Fatal(got)
	}
	if len(r.Subscribers("none")) != 0 {
		t.Fatal("none")
	}
}
func TestConcurrentResourceRegistry(t *testing.T) {
	r := referenceResources(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = r.List(context.Background(), nil)
			_, _, _ = r.Read(context.Background(), map[string]any{"uri": "test:text"})
			_, _, _ = r.Subscribe(context.Background(), map[string]any{"uri": "u"})
			_ = r.Subscribers("u")
			_, _, _ = r.Unsubscribe(context.Background(), map[string]any{"uri": "u"})
		}()
	}
	wg.Wait()
}
