package umcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func referenceServer(t *testing.T) *Server {
	t.Helper()
	s := NewServer("CompletionOracle")
	if err := s.Prompts.Register(Prompt{Name: "choose", Parameters: []Parameter{{Name: "value", Types: []ParamType{StringParam}, Enum: []any{"alpha", "beta", "alphabet"}}}, Call: func(_ context.Context, args map[string]any) (any, error) { return args["value"], nil }}); err != nil {
		t.Fatal(err)
	}
	if err := s.Resources.Register(Resource{URI: "test:///{path}", Name: "template", Template: true, Read: func(_ context.Context, args map[string]any) (any, error) { return args["path"], nil }}); err != nil {
		t.Fatal(err)
	}
	s.Completions.Register("ref/prompt", "choose", "value", func(_ context.Context, prefix string, _ map[string]any, _ map[string]any, _ map[string]any) (any, error) {
		switch prefix {
		case "invalid":
			return map[string]any{"values": "not list"}, nil
		case "value-error":
			return nil, ArgumentError("provider bad argument")
		case "failure":
			return nil, errors.New("private")
		}
		return map[string]any{"values": []any{"alpha", "alpine", "alpha", "beta", json.Number("1"), true}, "total": json.Number("10"), "hasMore": true}, nil
	})
	return s
}
func TestCompletionSyncAsyncParity(t *testing.T) {
	raw, err := os.ReadFile("testdata/parity/umcp-completion.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Request  string
			Expected json.RawMessage
		}
		LogInput     any `json:"log_input"`
		LogSanitized any `json:"log_sanitized"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	server := referenceServer(t)
	for _, c := range fixture.Cases {
		reply, err := server.Process(context.Background(), []byte(c.Request), RequestContext{})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(reply)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := decode(raw)
		want, _ := decode(c.Expected)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s\ngot %s\nwant %s", c.Request, raw, c.Expected)
		}
	}
	if got := SanitizeLogData(fixture.LogInput); !reflect.DeepEqual(got, fixture.LogSanitized) {
		t.Fatal(got, fixture.LogSanitized)
	}
}

func TestCompletionProviderAndLimits(t *testing.T) {
	s := NewServer("")
	if s.Completions.Available() {
		t.Fatal("empty capabilities")
	}
	if s.Name != "MCPServer" {
		t.Fatal(s.Name)
	}
	_ = s.Prompts.Register(Prompt{Name: "p", Parameters: []Parameter{{Name: "v", Types: []ParamType{StringParam}, Enum: []any{"one", "two"}}}, InputSchema: map[string]any{"type": "object"}, Call: func(context.Context, map[string]any) (any, error) { return nil, nil }})
	if !s.Completions.Available() {
		t.Fatal("prompt completions")
	}
	params := map[string]any{"ref": map[string]any{"type": "ref/prompt", "name": "p"}, "argument": map[string]any{"name": "v"}}
	if out, e, err := s.Completions.Complete(context.Background(), params); err != nil || e != nil || !reflect.DeepEqual(out.(map[string]any)["completion"].(map[string]any)["values"], []string{"one", "two"}) {
		t.Fatal(out, e, err)
	}
	// Provider total/hasMore validation and list coercion.
	for _, bad := range []any{true, []string{"not JSON list"}, map[string]any{"values": []any{}, "total": true}, map[string]any{"values": []any{}, "total": json.Number("-1")}, map[string]any{"values": []any{}, "hasMore": 1}} {
		s.Completions.Register("ref/prompt", "p", "v", func(context.Context, string, map[string]any, map[string]any, map[string]any) (any, error) {
			return bad, nil
		})
		if _, e, _ := s.Completions.Complete(context.Background(), params); e == nil {
			t.Fatal(bad)
		}
	}
	for _, value := range []any{[]any{"one", "three"}, map[string]any{}, map[string]any{"values": []any{}, "total": json.Number("0"), "hasMore": false}} {
		s.Completions.Register("ref/prompt", "p", "v", func(context.Context, string, map[string]any, map[string]any, map[string]any) (any, error) {
			return value, nil
		})
		if _, e, err := s.Completions.Complete(context.Background(), params); e != nil || err != nil {
			t.Fatal(e, err)
		}
	}
	s.Completions.MaxValues = 1
	if out, e, _ := s.Completions.Complete(context.Background(), params); e != nil || len(out.(map[string]any)["completion"].(map[string]any)["values"].([]string)) != 1 {
		t.Fatal(out, e)
	}
	s.Completions.MaxValues = -1
	params["maxValues"] = json.Number("10")
	if _, e, _ := s.Completions.Complete(context.Background(), params); e != nil {
		t.Fatal(e)
	}
	c := &Completions{}
	if c.Available() {
		t.Fatal("empty")
	}
	c.Resources = &ResourceRegistry{}
	if c.Available() {
		t.Fatal("empty resources")
	}
	_ = c.Resources.Register(Resource{URI: "/{x}", Name: "x", Template: true, Parameters: []Parameter{{Name: "x", Enum: []any{"a"}}}, Read: func(context.Context, map[string]any) (any, error) { return nil, nil }})
	if !c.Available() {
		t.Fatal("template")
	}
	result, e, _ := c.Complete(context.Background(), map[string]any{"ref": map[string]any{"type": "ref/resource", "name": "x"}, "argument": map[string]any{"name": "x"}})
	if e != nil || result == nil {
		t.Fatal(e)
	}
	for _, ref := range []map[string]any{{"type": "ref/resource", "uri": "missing"}, {"type": "ref/prompt", "name": "missing"}} {
		c := &Completions{}
		_, _, _, _, err := c.target(ref)
		if err == nil {
			t.Fatal(ref)
		}
	}
	for _, v := range []any{nil, false, "", json.Number("0"), []any{}, map[string]any{}} {
		if truthy(v) {
			t.Fatal(v)
		}
	}
	for _, v := range []any{true, "x", json.Number("1"), json.Number("NaN"), []any{1}, map[string]any{"x": 1}, 1} {
		if !truthy(v) {
			t.Fatal(v)
		}
	}
}

func TestServerLoggingAndNotifications(t *testing.T) {
	s := NewServer("test")
	if err := s.LogMessage("info", "none", "", true); err != nil {
		t.Fatal(err)
	}
	captured := []map[string]any{}
	s.SetNotifier(func(method string, p map[string]any) error {
		if method != "notifications/message" {
			t.Fatal(method)
		}
		captured = append(captured, p)
		return nil
	})
	for _, level := range []string{"debug", "invalid"} {
		if err := s.LogMessage(level, "ignored", "", true); err != nil {
			t.Fatal(err)
		}
	}
	if len(captured) != 0 {
		t.Fatal(captured)
	}
	input := OrderedObject{{"password", "private"}, {"items", []any{"Bearer abc", json.Number("1")}}}
	if err := s.LogMessage("warning", input, "test", true); err != nil {
		t.Fatal(err)
	}
	if len(captured) != 1 || captured[0]["logger"] != "test" {
		t.Fatal(captured)
	}
	if err := s.LogMessage("info", "token=x", "", false); err != nil {
		t.Fatal(err)
	}
	if captured[1]["data"] != "token=x" {
		t.Fatal(captured)
	}
	boom := errors.New("transport")
	s.SetNotifier(func(string, map[string]any) error { return boom })
	if !errors.Is(s.LogMessage("error", "x", "", true), boom) {
		t.Fatal("notify failure lost")
	}
	if _, err := RawResult(json.RawMessage(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := RawResult(json.RawMessage(`{`)); err == nil {
		t.Fatal("bad result")
	}
	called := 0
	deliver := func(ids []string, method string, p map[string]any) error {
		called++
		if method != "notifications/resources/updated" || p["uri"] != "u" {
			t.Fatal(method, p)
		}
		return nil
	}
	if err := s.NotifyResourceUpdated("u", deliver); err != nil || called != 0 {
		t.Fatal(err)
	}
	_, _, _ = s.Resources.Subscribe(context.Background(), map[string]any{"uri": "u"})
	if !reflect.DeepEqual(s.ResourceUpdateRecipients("u"), []string{""}) {
		t.Fatal("global")
	}
	_ = s.NotifyResourceUpdated("u", deliver)
	_, _, _ = s.Resources.Subscribe(context.Background(), map[string]any{"uri": "u", "_session_id": "s1"})
	if !reflect.DeepEqual(s.ResourceUpdateRecipients("u"), []string{"s1"}) {
		t.Fatal("session precedence")
	}
	_ = s.NotifyResourceUpdated("u", deliver)
	if called != 2 {
		t.Fatal(called)
	}
}
