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

func testHandler(ctx context.Context, params map[string]any) (any, *RPCError, error) {
	if params["cancel"] == true {
		return nil, nil, ErrRequestCancelled
	}
	c := Context(ctx)
	return map[string]any{"params": params, "context": map[string]any{"request_id": c.RequestID, "progress_token": c.ProgressToken, "principal": c.Principal, "transport": c.Transport, "session_id": c.SessionID, "headers": c.Headers}}, nil, nil
}

func TestSyncAsyncDispatcherParity(t *testing.T) {
	raw, err := os.ReadFile("testdata/parity/umcp-dispatch.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Input    string
		Expected json.RawMessage
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	d := Dispatcher{Handlers: map[string]Handler{"initialize": testHandler}}
	trusted := RequestContext{Transport: "streamable-http", Principal: "oracle-reader", SessionID: "session-1", Headers: map[string]string{"x-test": "one"}}
	for _, c := range fixtures {
		r, err := d.Process(context.Background(), []byte(c.Input), trusted)
		if err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := decode(got)
		b, _ := decode(c.Expected)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("input %s\ngot %s\nwant %s", c.Input, got, c.Expected)
		}
	}
}

func TestDispatcherContextIsolation(t *testing.T) {
	trusted := RequestContext{Transport: "tcp", RequestID: json.RawMessage(`42`), ProgressToken: json.RawMessage(`"old"`), Principal: "reader", Peer: "peer", Headers: map[string]string{"one": "original"}}
	var wg sync.WaitGroup
	handler := func(ctx context.Context, p map[string]any) (any, *RPCError, error) {
		a := Context(ctx)
		a.Headers["one"] = "changed"
		a.RequestID[0] = '9'
		if b := Context(ctx); b.Headers["one"] != "original" || string(b.RequestID) != "1" {
			t.Error("handler mutated trusted context", b)
		}
		return "ok", nil, nil
	}
	d := Dispatcher{Handlers: map[string]Handler{"test": handler}}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"test","params":{"principal":"attacker"}}`), trusted)
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if trusted.Headers["one"] != "original" || string(trusted.RequestID) != "42" {
		t.Fatal("caller context mutated")
	}
	if Context(context.Background()).Principal != "" {
		t.Fatal("request leaked into parent context")
	}
}

func TestDispatcherFailuresAndCancellation(t *testing.T) {
	generic := errors.New("failed")
	d := Dispatcher{Handlers: map[string]Handler{
		"error": func(context.Context, map[string]any) (any, *RPCError, error) {
			return "ignored", &RPCError{Code: -32602, Message: "bad", Data: map[string]any{"field": "a"}}, nil
		},
		"failure":       func(context.Context, map[string]any) (any, *RPCError, error) { return nil, nil, generic },
		"unmarshalable": func(context.Context, map[string]any) (any, *RPCError, error) { return make(chan int), nil, nil },
	}}
	cancelled := ""
	d.Cancel = func(id json.RawMessage) { cancelled = string(id) }
	r, err := d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"error"}`), RequestContext{})
	if err != nil || r.Error.Code != -32602 {
		t.Fatal(r, err)
	}
	if _, err = json.Marshal(r); err != nil {
		t.Fatal(err)
	}
	r, err = d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","method":"failure"}`), RequestContext{})
	if r != nil || !errors.Is(err, generic) {
		t.Fatal(r, err)
	}
	r, err = d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","method":"unmarshalable"}`), RequestContext{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = json.Marshal(r); err == nil {
		t.Fatal("bad result encoded")
	}
	for _, input := range []string{`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":99}}`, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":null}}`} {
		r, err = d.Process(context.Background(), []byte(input), RequestContext{})
		if r != nil || err != nil {
			t.Fatal(r, err)
		}
	}
	if cancelled != "99" {
		t.Fatal(cancelled)
	}
	for _, transport := range []string{"tcp", "sse", "streamable-http", "stdio", "file", ""} {
		ctx := context.WithValue(context.Background(), requestContextKey{}, RequestContext{Transport: transport})
		want := "private"
		if transport == "tcp" || transport == "sse" || transport == "streamable-http" {
			want = "public"
		}
		if RemoteSafeFailure(ctx, "public", "private") != want {
			t.Fatal(transport)
		}
	}
}

func FuzzDispatch(f *testing.F) {
	for _, s := range []string{"", "null", "{}", `{"jsonrpc":"2.0","id":1,"method":"notifications/initialized"}`, `{"jsonrpc":"2.0","id":1,"method":"unknown","params":{}}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		d := Dispatcher{}
		_, _ = d.Process(context.Background(), []byte(raw), RequestContext{})
	})
}
