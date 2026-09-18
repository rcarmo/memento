package umcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"sync"
	"testing"
)

func TestNotificationFallbackReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/umcp-notifications.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Mode           string
		Active         bool `json:"active_http"`
		Session        bool
		Targets        []string
		Params         map[string]any
		Events, Stdout []any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		s := NewServer("test")
		var output bytes.Buffer
		s.SetNotificationOutput(&output)
		events := []any{}
		if c.Active {
			h, err := NewStreamableHTTP(s, DefaultHTTPOptions(), HTTPHooks{})
			if err != nil {
				t.Fatal(err)
			}
			err = h.Notify(c.Targets, "notifications/test", c.Params)
			h.Close()
			if err != nil {
				t.Fatal(err)
			}
		} else {
			mode := SyncSSE
			if c.Mode == "async" {
				mode = AsyncSSE
			}
			h, err := NewLegacySSE(s, DefaultSSEOptions(mode), HTTPHooks{})
			if err != nil {
				t.Fatal(err)
			}
			var session *legacySession
			if c.Session {
				session = addLegacySession(h, "one", "reader")
			}
			if mode == AsyncSSE && c.Session && (c.Targets == nil || len(c.Targets) > 0 && c.Targets[0] == "one") {
				done := make(chan error, 1)
				go func() { done <- h.Notify(c.Targets, "notifications/test", c.Params) }()
				<-session.wake
				h.mu.Lock()
				item := session.pending[0]
				h.mu.Unlock()
				kind, data, err := readSSE(bufio.NewReader(bytes.NewReader(item.payload)))
				if err != nil || kind != "message" {
					t.Fatal(err)
				}
				var v any
				_ = json.Unmarshal([]byte(data), &v)
				events = append(events, v)
				item.ack <- nil
				if err = <-done; err != nil {
					t.Fatal(err)
				}
			} else {
				if err = h.Notify(c.Targets, "notifications/test", c.Params); err != nil {
					t.Fatal(err)
				}
				if session != nil {
					for _, item := range session.pending {
						_, data, err := readSSE(bufio.NewReader(bytes.NewReader(item.payload)))
						if err != nil {
							t.Fatal(err)
						}
						var v any
						_ = json.Unmarshal([]byte(data), &v)
						events = append(events, v)
					}
				}
			}
			h.Close()
		}
		stdout := []any{}
		decoder := json.NewDecoder(&output)
		for {
			var v any
			err = decoder.Decode(&v)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			stdout = append(stdout, v)
		}
		if !reflect.DeepEqual(events, c.Events) || !reflect.DeepEqual(stdout, c.Stdout) {
			t.Fatalf("case %d: events %v != %v; stdout %v != %v", i, events, c.Events, stdout, c.Stdout)
		}
	}
}

func TestServerNotificationOutput(t *testing.T) {
	s := NewServer("test")
	var output bytes.Buffer
	s.SetNotificationOutput(&output)
	var workers sync.WaitGroup
	for range 10 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 10 {
				if err := s.send("notifications/test", nil); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	workers.Wait()
	if len(bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))) != 100 {
		t.Fatal("interleaved lines")
	}
	s.SetNotificationOutput(nil)
	if err := s.send("notifications/test", nil); err != nil {
		t.Fatal(err)
	}
	s.SetNotificationOutput(failureWriter{})
	if err := s.send("notifications/test", nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	s.SetNotificationOutput(&output)
	if err := s.send("notifications/test", map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("invalid payload accepted")
	}
	// Failed async SSE write falls back to stdout, unlike active Streamable HTTP.
	h, err := NewLegacySSE(s, DefaultSSEOptions(AsyncSSE), HTTPHooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	session := addLegacySession(h, "one", "reader")
	output.Reset()
	done := make(chan error, 1)
	go func() { done <- s.send("notifications/test", nil) }()
	<-session.wake
	h.mu.Lock()
	session.pending[0].ack <- io.ErrClosedPipe
	h.mu.Unlock()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if output.Len() == 0 {
		t.Fatal("missing stdout fallback")
	}
}
