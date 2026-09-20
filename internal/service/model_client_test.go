package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func endpoint(raw, format, model string) ModelEndpointConfig {
	return ModelEndpointConfig{BaseURL: raw, APIFormat: format, Model: model, Headers: map[string]string{"X-Test": "yes"}}
}
func TestDecodeModelProviderSlots(t *testing.T) {
	config, err := DecodeModelProviderSlots(nil)
	if err != nil || config.Dream.FallbackEnabled || config.HotQuery.MaxOutputChars != 2000 {
		t.Fatal(config, err)
	}
	raw := json.RawMessage(`{"dream":{"primary":{"base_url":"http://localhost:1/","api_format":"openai","api_key_env":"KEY","model":"m","headers":{" X ":"y"}},"fallbacks":[],"timeout_seconds":4,"max_output_chars":99,"retry_budget":1,"concurrency_limit":2,"allowed_data_classifications":["restricted"],"allow_cross_trust_boundary":false,"fallback_enabled":true,"fallback_on_rate_limit":true,"overload_status_codes":[529]}}`)
	config, err = DecodeModelProviderSlots(raw)
	if err != nil || config.Dream.Primary.Model != "m" || config.Dream.MaxOutputChars != 99 {
		t.Fatal(config, err)
	}
	bad := []string{`{} {}`, `{"x":1}`, `{"dream":{"timeout_seconds":0}}`, `{"dream":{"max_output_chars":1}}`, `{"dream":{"retry_budget":6}}`, `{"dream":{"concurrency_limit":0}}`, `{"dream":{"allowed_data_classifications":[]}}`, `{"dream":{"allowed_data_classifications":[""]}}`, `{"dream":{"allowed_data_classifications":["x","x"]}}`, `{"dream":{"overload_status_codes":[99]}}`, `{"dream":{"primary":{"base_url":"x","api_format":"openai","model":"m"}}}`, `{"dream":{"primary":{"base_url":"http://x","api_format":"bad","model":"m"}}}`, `{"dream":{"primary":{"base_url":"http://x","api_format":"openai","model":""}}}`, `{"dream":{"primary":{"base_url":"http://x","api_format":"openai","api_key_env":" ","model":"m"}}}`, `{"dream":{"primary":{"base_url":"http://x","api_format":"openai","model":"m","headers":{" ":"x"}}}}`}
	for _, item := range bad {
		if _, err := DecodeModelProviderSlots(json.RawMessage(item)); err == nil {
			t.Fatal(item)
		}
	}
}
func TestEndpointModelClients(t *testing.T) {
	key := "KEY"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("X-Test") != "yes" {
			t.Error(r.URL.Path, r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"max_tokens":8192`) {
			t.Error(string(body))
		}
		io.WriteString(w, `{"model":"served","choices":[{"message":{"content":"ok"}}],"usage":{"x":2}}`)
	}))
	defer server.Close()
	client := EndpointModelClient{Endpoint: endpoint(server.URL, "openai", "configured"), LookupEnv: func(string) (string, bool) { return "secret", true }}
	client.Endpoint.APIKeyEnv = &key
	response, err := client.Complete(context.Background(), ModelRequest{Prompt: "p", MaxOutputChars: 9000, Timeout: time.Second})
	if err != nil || response.ModelName != "served" || response.OutputText != "ok" || response.Usage["x"] != 2 {
		t.Fatal(response, err)
	}
	anthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "secret" || r.Header.Get("anthropic-version") == "" {
			t.Error(r.URL.Path, r.Header)
		}
		io.WriteString(w, `{"content":[{"type":"text","text":"a"},{"type":"tool","text":"x"},{"type":"text","text":"b"}]}`)
	}))
	defer anthropic.Close()
	client.Endpoint = endpoint(anthropic.URL, "anthropic", "claude")
	client.Endpoint.APIKeyEnv = &key
	response, err = client.Complete(context.Background(), ModelRequest{Prompt: "p", MaxOutputChars: 10, Timeout: time.Second})
	if err != nil || response.OutputText != "ab" || response.ModelName != "claude" {
		t.Fatal(response, err)
	}
}
func TestEndpointModelFailures(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
	}{{"invalid", "{", 200}, {"extra", `{"choices":[],"extra":1}`, 200}, {"empty", `{"choices":[]}`, 200}, {"http", "no", 500}, {"large", strings.Repeat("x", 1048577), 200}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); io.WriteString(w, tc.body) }))
			defer server.Close()
			_, err := (EndpointModelClient{Endpoint: endpoint(server.URL, "openai", "m")}).Complete(context.Background(), ModelRequest{MaxOutputChars: 32, Timeout: time.Second})
			if err == nil {
				t.Fatal("expected")
			}
			var modelErr *ModelError
			if !errors.As(err, &modelErr) {
				t.Fatal(err)
			}
		})
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(100 * time.Millisecond) }))
	defer slow.Close()
	_, err := (EndpointModelClient{Endpoint: endpoint(slow.URL, "openai", "m")}).Complete(context.Background(), ModelRequest{MaxOutputChars: 32, Timeout: time.Millisecond})
	var modelErr *ModelError
	if !errors.As(err, &modelErr) || modelErr.Kind != "timeout" {
		t.Fatal(err)
	}
}
func slot(primary ModelEndpointConfig) ModelSlotConfig {
	return ModelSlotConfig{Primary: &primary, TimeoutSeconds: 2, MaxOutputChars: 100, ConcurrencyLimit: 1, AllowedDataClassifications: []string{"restricted"}, OverloadStatusCodes: []int{529}}
}
func TestRoutedModelClientPolicyRetryFallback(t *testing.T) {
	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { primaryCalls.Add(1); w.WriteHeader(500) }))
	defer primary.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer fallback.Close()
	dream := slot(endpoint(primary.URL, "openai", "p"))
	dream.RetryBudget = 1
	dream.FallbackEnabled = true
	dream.Fallbacks = []ModelEndpointConfig{endpoint(fallback.URL, "openai", "f")}
	client := RoutedModelClient{Slots: ModelProviderSlotsConfig{Dream: dream}}
	response, err := client.Complete(context.Background(), ModelRequest{Task: "dream_proposal_draft", Prompt: "p", DataClassification: "restricted", MaxOutputChars: 100, Timeout: time.Second})
	if err != nil || response.OutputText != "ok" || primaryCalls.Load() != 2 || len(response.ModelChain) != 3 || response.ModelChain[2].Model != "f" {
		t.Fatal(response, primaryCalls.Load(), err)
	}
	for _, request := range []ModelRequest{{Task: "unknown", DataClassification: "restricted"}, {Task: "dream_proposal_draft", DataClassification: "public"}} {
		if _, err = client.Complete(context.Background(), request); err == nil {
			t.Fatal(request)
		}
	}
	empty := RoutedModelClient{Slots: ModelProviderSlotsConfig{Dream: defaultModelSlot(false)}}
	if _, err = empty.Complete(context.Background(), ModelRequest{Task: "dream_proposal_draft", DataClassification: "internal"}); err == nil {
		t.Fatal("empty")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read") }
func (failingReader) Close() error             { return nil }

func TestModelClientHelpersAndRareErrors(t *testing.T) {
	modelErr := &ModelError{Err: errors.New("wrapped")}
	if modelErr.Error() != "wrapped" || !errors.Is(modelErr, modelErr.Err) {
		t.Fatal(modelErr)
	}
	primary := endpoint("http://localhost", "openai", "p")
	if got := appendEndpoint(&primary, []ModelEndpointConfig{endpoint("http://x", "openai", "f")}); len(got) != 2 || got[0].Model != "p" {
		t.Fatal(got)
	}
	if got := appendEndpoint(nil, nil); len(got) != 0 {
		t.Fatal(got)
	}
	if trustBoundary("http://[::1]") != "local" || trustBoundary("http://example.com") != "remote" || !contains([]string{"x"}, "x") || contains([]string{"x"}, "y") || !containsInt([]int{7}, 7) || containsInt([]int{7}, 8) || minDuration(time.Second, 2*time.Second) != time.Second || minDuration(0, time.Second) != time.Second {
		t.Fatal("helpers")
	}
	client := EndpointModelClient{Endpoint: endpoint(":bad", "openai", "m")}
	if _, err := client.Complete(context.Background(), ModelRequest{}); err == nil {
		t.Fatal("request")
	}
	client = EndpointModelClient{Endpoint: endpoint("http://example.test", "openai", "m"), HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: failingReader{}, Header: make(http.Header)}, nil
	})}}
	if _, err := client.Complete(context.Background(), ModelRequest{}); err == nil {
		t.Fatal("read")
	}
	raw := []byte(`{"choices":[{"message":{"content":"ok"}}]} {}`)
	if _, err := parseModelResponse(endpoint("http://x", "openai", "m"), raw); err == nil {
		t.Fatal("trailing")
	}
	routed := RoutedModelClient{}
	for _, name := range []string{"hot_query", "deep_query", "proposal", "dream"} {
		if _, ok := routed.slot(name); !ok {
			t.Fatal(name)
		}
	}
	blocked := slot(endpoint("http://localhost", "openai", "m"))
	routed = RoutedModelClient{Slots: ModelProviderSlotsConfig{Dream: blocked}, semaphores: map[string]chan struct{}{"dream": make(chan struct{}, 1)}}
	routed.semaphores["dream"] <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := routed.Complete(ctx, ModelRequest{SlotName: "dream", DataClassification: "restricted"}); err == nil {
		t.Fatal("blocked cancellation")
	}
	bad := slot(endpoint(":bad", "openai", "m"))
	routed = RoutedModelClient{Slots: ModelProviderSlotsConfig{Dream: bad}}
	if _, err := routed.Complete(context.Background(), ModelRequest{SlotName: "dream", DataClassification: "restricted"}); err == nil {
		t.Fatal("plain error")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400) }))
	defer server.Close()
	nonretry := slot(endpoint(server.URL, "openai", "m"))
	routed = RoutedModelClient{Slots: ModelProviderSlotsConfig{Dream: nonretry}}
	if _, err := routed.Complete(context.Background(), ModelRequest{SlotName: "dream", DataClassification: "restricted"}); err == nil {
		t.Fatal("nonretryable")
	}
}

func TestRoutedModelClientTerminalAndCrossTrust(t *testing.T) {
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer failed.Close()
	dream := slot(endpoint(failed.URL, "openai", "p"))
	client := RoutedModelClient{Slots: ModelProviderSlotsConfig{Dream: dream}}
	if _, err := client.Complete(context.Background(), ModelRequest{SlotName: "dream", DataClassification: "restricted", MaxOutputChars: 100}); err == nil {
		t.Fatal("terminal")
	}
	success := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer success.Close()
	dream.FallbackEnabled = true
	dream.AllowCrossTrustBoundary = true
	dream.Fallbacks = []ModelEndpointConfig{endpoint(success.URL, "openai", "f")}
	client = RoutedModelClient{Slots: ModelProviderSlotsConfig{Dream: dream}}
	response, err := client.Complete(context.Background(), ModelRequest{SlotName: "dream", DataClassification: "restricted", MaxOutputChars: 100})
	if err != nil || response.OutputText != "ok" {
		t.Fatal(response, err)
	}
}

func TestRoutedModelClientTrustRateAndCancel(t *testing.T) {
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer remote.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"fallback"}}]}`)
	}))
	defer local.Close()
	dream := slot(endpoint(strings.Replace(remote.URL, "127.0.0.1", "example.invalid", 1), "openai", "remote"))
	dream.FallbackEnabled = true
	dream.FallbackOnRateLimit = true
	dream.Fallbacks = []ModelEndpointConfig{endpoint(local.URL, "openai", "local")}
	client := RoutedModelClient{Slots: ModelProviderSlotsConfig{Dream: dream}}
	if _, err := client.Complete(context.Background(), ModelRequest{Task: "dream_proposal_draft", DataClassification: "restricted", Timeout: 20 * time.Millisecond, MaxOutputChars: 100}); err == nil {
		t.Fatal("cross boundary")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Complete(ctx, ModelRequest{Task: "dream_proposal_draft", DataClassification: "restricted"}); err == nil {
		t.Fatal("cancel")
	}
}
