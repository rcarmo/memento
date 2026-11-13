package umcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestServerPing(t *testing.T) {
	s := NewServer("ping-test")
	calls := 0
	s.SetNotifier(func(string, map[string]any) error { calls++; return nil })
	for _, params := range []string{"", `,"params":{}`, `,"params":{"ignored":"no-side-effect"}`} {
		raw := []byte(`{"jsonrpc":"2.0","id":9007199254740993,"method":"ping"` + params + `}`)
		out, err := s.Process(context.Background(), raw, RequestContext{})
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			ID     json.Number    `json:"id"`
			Result map[string]any `json:"result"`
			Error  any            `json:"error"`
		}
		if err = json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != "9007199254740993" || got.Error != nil || got.Result == nil || len(got.Result) != 0 {
			t.Fatal(string(b))
		}
	}
	// The transport owns notification-response suppression. The dispatcher
	// preserves its existing response semantics for ordinary notifications.
	out, err := s.Process(context.Background(), []byte(`{"jsonrpc":"2.0","method":"ping"}`), RequestContext{})
	if err != nil || out == nil || out.Error != nil || out.ID != nil {
		t.Fatal(out, err)
	}
	for _, raw := range []string{`{"jsonrpc":"2.0","method":"notifications/initialized"}`} {
		out, err := s.Process(context.Background(), []byte(raw), RequestContext{})
		if err != nil || out != nil {
			t.Fatal(out, err)
		}
	}
	if calls != 0 {
		t.Fatal("ping emitted notifications", calls)
	}
	out, err = s.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"still-unknown"}`), RequestContext{})
	if err != nil || out == nil || out.Error == nil || out.Error.Code != -32601 {
		t.Fatal(out, err)
	}
}

func TestServerPingStdio(t *testing.T) {
	server := NewServer("stdio-ping")
	var out bytes.Buffer
	in := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-03-26\"}}\n{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"ping\"}\n")
	if err := server.serveStdio(context.Background(), in, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatal(out.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &result); err != nil {
		t.Fatal(err)
	}
	empty, ok := result["result"].(map[string]any)
	if !ok || len(empty) != 0 || result["id"] != float64(2) || result["error"] != nil {
		t.Fatal(result)
	}
}
