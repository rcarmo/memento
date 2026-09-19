package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcarmo/memento/go/control"
)

type ModelEndpointConfig struct {
	BaseURL   string            `json:"base_url"`
	APIFormat string            `json:"api_format"`
	APIKeyEnv *string           `json:"api_key_env"`
	Model     string            `json:"model"`
	Headers   map[string]string `json:"headers"`
}
type ModelSlotConfig struct {
	Primary                    *ModelEndpointConfig  `json:"primary"`
	Fallbacks                  []ModelEndpointConfig `json:"fallbacks"`
	TimeoutSeconds             float64               `json:"timeout_seconds"`
	MaxOutputChars             int                   `json:"max_output_chars"`
	RetryBudget                int                   `json:"retry_budget"`
	ConcurrencyLimit           int                   `json:"concurrency_limit"`
	AllowedDataClassifications []string              `json:"allowed_data_classifications"`
	AllowCrossTrustBoundary    bool                  `json:"allow_cross_trust_boundary"`
	FallbackEnabled            bool                  `json:"fallback_enabled"`
	FallbackOnRateLimit        bool                  `json:"fallback_on_rate_limit"`
	OverloadStatusCodes        []int                 `json:"overload_status_codes"`
}
type ModelProviderSlotsConfig struct {
	HotQuery  ModelSlotConfig `json:"hot_query"`
	DeepQuery ModelSlotConfig `json:"deep_query"`
	Proposal  ModelSlotConfig `json:"proposal"`
	Dream     ModelSlotConfig `json:"dream"`
}
type modelSlotsWire struct {
	HotQuery  ModelSlotConfig `json:"hot_query"`
	DeepQuery ModelSlotConfig `json:"deep_query"`
	Proposal  ModelSlotConfig `json:"proposal"`
	Dream     ModelSlotConfig `json:"dream"`
}
type ModelRequest struct {
	Task, Prompt, SlotName, DataClassification string
	MaxOutputChars                             int
	Timeout                                    time.Duration
	Metadata                                   map[string]string
}
type ModelResponse struct {
	ModelName, OutputText string
	Usage                 map[string]int
	ModelChain            []control.ModelAttempt
}
type ModelClient interface {
	Complete(context.Context, ModelRequest) (ModelResponse, error)
}
type ModelError struct {
	Kind      string
	Status    int
	Retryable bool
	Err       error
}

func (e *ModelError) Error() string { return e.Err.Error() }
func (e *ModelError) Unwrap() error { return e.Err }

func defaultModelSlot(fallback bool) ModelSlotConfig {
	return ModelSlotConfig{TimeoutSeconds: 3, MaxOutputChars: 2000, ConcurrencyLimit: 1, AllowedDataClassifications: []string{"internal"}, FallbackEnabled: fallback, OverloadStatusCodes: []int{529}}
}
func DecodeModelProviderSlots(raw json.RawMessage) (ModelProviderSlotsConfig, error) {
	defaults := modelSlotsWire{defaultModelSlot(true), defaultModelSlot(true), defaultModelSlot(false), defaultModelSlot(false)}
	if len(bytes.TrimSpace(raw)) == 0 {
		return ModelProviderSlotsConfig(defaults), nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&defaults); err != nil {
		return ModelProviderSlotsConfig{}, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ModelProviderSlotsConfig{}, errors.New("trailing JSON")
	}
	slots := []*ModelSlotConfig{&defaults.HotQuery, &defaults.DeepQuery, &defaults.Proposal, &defaults.Dream}
	for _, slot := range slots {
		if slot.TimeoutSeconds <= 0 || slot.TimeoutSeconds > 120 || slot.MaxOutputChars < 32 || slot.RetryBudget < 0 || slot.RetryBudget > 5 || slot.ConcurrencyLimit < 1 || slot.ConcurrencyLimit > 32 || len(slot.AllowedDataClassifications) == 0 || len(slot.OverloadStatusCodes) > 16 {
			return ModelProviderSlotsConfig{}, errors.New("model slot configuration out of range")
		}
		seen := map[string]bool{}
		for _, c := range slot.AllowedDataClassifications {
			if strings.TrimSpace(c) == "" {
				return ModelProviderSlotsConfig{}, errors.New("allowed data classifications must not be empty")
			}
			if seen[c] {
				return ModelProviderSlotsConfig{}, errors.New("allowed data classifications must be unique")
			}
			seen[c] = true
		}
		for _, status := range slot.OverloadStatusCodes {
			if status < 100 || status > 599 {
				return ModelProviderSlotsConfig{}, errors.New("overload status codes must be valid HTTP status codes")
			}
		}
		for _, endpoint := range appendEndpoint(slot.Primary, slot.Fallbacks) {
			if err := endpoint.validate(); err != nil {
				return ModelProviderSlotsConfig{}, err
			}
		}
	}
	return ModelProviderSlotsConfig(defaults), nil
}
func appendEndpoint(primary *ModelEndpointConfig, fallbacks []ModelEndpointConfig) []ModelEndpointConfig {
	out := append([]ModelEndpointConfig{}, fallbacks...)
	if primary != nil {
		out = append([]ModelEndpointConfig{*primary}, out...)
	}
	return out
}
func (e ModelEndpointConfig) validate() error {
	parsed, err := url.Parse(e.BaseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("model base_url must be an absolute HTTP(S) URL")
	}
	if e.APIFormat != "openai" && e.APIFormat != "anthropic" {
		return errors.New("model api_format must be openai or anthropic")
	}
	if e.Model == "" {
		return errors.New("model name must not be empty")
	}
	if e.APIKeyEnv != nil && strings.TrimSpace(*e.APIKeyEnv) == "" {
		return errors.New("api_key_env must not be empty")
	}
	for key := range e.Headers {
		if strings.TrimSpace(key) == "" {
			return errors.New("header names must not be empty")
		}
	}
	return nil
}

type EndpointModelClient struct {
	Endpoint  ModelEndpointConfig
	HTTP      *http.Client
	LookupEnv func(string) (string, bool)
}

func (c EndpointModelClient) Complete(ctx context.Context, request ModelRequest) (ModelResponse, error) {
	client := c.HTTP
	if client == nil {
		client = &http.Client{}
	}
	lookup := c.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	endpoint := strings.TrimRight(c.Endpoint.BaseURL, "/")
	path := "/v1/chat/completions"
	payload := map[string]any{"model": c.Endpoint.Model, "stream": false, "messages": []any{map[string]any{"role": "user", "content": request.Prompt}}, "max_tokens": max(1, min(request.MaxOutputChars, 8192))}
	if c.Endpoint.APIFormat == "anthropic" {
		path = "/v1/messages"
		delete(payload, "stream")
	}
	raw, _ := json.Marshal(payload)
	callCtx := ctx
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint+path, bytes.NewReader(raw))
	if err != nil {
		return ModelResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	for key, value := range c.Endpoint.Headers {
		req.Header.Set(key, value)
	}
	if c.Endpoint.APIKeyEnv != nil {
		if secret, ok := lookup(*c.Endpoint.APIKeyEnv); ok {
			if c.Endpoint.APIFormat == "anthropic" {
				req.Header.Set("x-api-key", secret)
				req.Header.Set("anthropic-version", "2023-06-01")
			} else {
				req.Header.Set("Authorization", "Bearer "+secret)
			}
		}
	}
	response, err := client.Do(req)
	if err != nil {
		kind := "connection_failed"
		if errors.Is(err, context.DeadlineExceeded) {
			kind = "timeout"
		}
		return ModelResponse{}, &ModelError{Kind: kind, Retryable: true, Err: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1048577))
	if err != nil {
		return ModelResponse{}, err
	}
	if len(body) > 1048576 {
		return ModelResponse{}, &ModelError{Kind: "invalid_output", Err: errors.New("model response exceeds 1 MiB limit")}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ModelResponse{}, &ModelError{Kind: fmt.Sprintf("http_%d", response.StatusCode), Status: response.StatusCode, Err: errors.New(string(body))}
	}
	return parseModelResponse(c.Endpoint, body)
}
func parseModelResponse(endpoint ModelEndpointConfig, body []byte) (ModelResponse, error) {
	var raw struct {
		Model   *string `json:"model"`
		Choices []struct {
			Message map[string]any `json:"message"`
		} `json:"choices"`
		Content []map[string]any `json:"content"`
		Usage   map[string]int   `json:"usage"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return ModelResponse{}, &ModelError{Kind: "invalid_output", Err: err}
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ModelResponse{}, &ModelError{Kind: "invalid_output", Err: errors.New("model response contains trailing JSON")}
	}
	name := endpoint.Model
	if raw.Model != nil {
		name = *raw.Model
	}
	text := ""
	if endpoint.APIFormat == "anthropic" {
		for _, item := range raw.Content {
			if item["type"] == "text" {
				value, ok := item["text"].(string)
				if ok {
					text += value
				}
			}
		}
	} else if len(raw.Choices) > 0 {
		value, _ := raw.Choices[0].Message["content"].(string)
		text = value
	}
	if text == "" {
		return ModelResponse{}, &ModelError{Kind: "invalid_output", Err: errors.New("model response contained no text content")}
	}
	return ModelResponse{name, text, raw.Usage, nil}, nil
}

type RoutedModelClient struct {
	Slots      ModelProviderSlotsConfig
	HTTP       *http.Client
	LookupEnv  func(string) (string, bool)
	mu         sync.Mutex
	semaphores map[string]chan struct{}
}

func (c *RoutedModelClient) Complete(ctx context.Context, request ModelRequest) (ModelResponse, error) {
	slotName := request.SlotName
	if slotName == "" {
		slotName = map[string]string{"dream_proposal_draft": "dream", "memory_proposal_draft": "proposal", "memory_answer_hot": "hot_query", "memory_answer_deep": "deep_query"}[request.Task]
	}
	slot, ok := c.slot(slotName)
	if !ok {
		return ModelResponse{}, &ModelError{Kind: "policy_denied", Err: fmt.Errorf("no model slot for task %s", request.Task)}
	}
	if !contains(slot.AllowedDataClassifications, request.DataClassification) {
		return ModelResponse{}, &ModelError{Kind: "policy_denied", Err: fmt.Errorf("slot %s disallows data classification %s", slotName, request.DataClassification)}
	}
	chain := appendEndpoint(slot.Primary, slot.Fallbacks)
	if len(chain) == 0 {
		return ModelResponse{}, &ModelError{Kind: "policy_denied", Err: fmt.Errorf("model slot %s is not configured", slotName)}
	}
	allowed := chain[:1]
	if slot.FallbackEnabled {
		primary := trustBoundary(chain[0].BaseURL)
		for _, candidate := range chain[1:] {
			if slot.AllowCrossTrustBoundary || trustBoundary(candidate.BaseURL) == primary {
				allowed = append(allowed, candidate)
			}
		}
	}
	lease := c.semaphore(slotName, slot.ConcurrencyLimit)
	select {
	case lease <- struct{}{}:
		defer func() { <-lease }()
	case <-ctx.Done():
		return ModelResponse{}, &ModelError{Kind: "cancelled", Err: ctx.Err()}
	}
	attempts := []control.ModelAttempt{}
	for index := 0; ; index++ {
		endpoint := allowed[index]
		retries := slot.RetryBudget
		for {
			limit := request
			limit.Timeout = minDuration(request.Timeout, time.Duration(slot.TimeoutSeconds*float64(time.Second)))
			limit.MaxOutputChars = min(request.MaxOutputChars, slot.MaxOutputChars)
			response, err := (EndpointModelClient{endpoint, c.HTTP, c.LookupEnv}).Complete(ctx, limit)
			if err == nil {
				response.ModelChain = append(attempts, control.ModelAttempt{Model: response.ModelName, Outcome: "success"})
				return response, nil
			}
			var modelErr *ModelError
			if !errors.As(err, &modelErr) {
				return ModelResponse{}, err
			}
			attempts = append(attempts, control.ModelAttempt{Model: endpoint.Model, Outcome: modelErr.Kind})
			retryable := modelErr.Retryable || modelErr.Status >= 500 || containsInt(slot.OverloadStatusCodes, modelErr.Status) || (modelErr.Status == 429 && slot.FallbackOnRateLimit)
			if !retryable {
				return ModelResponse{}, err
			}
			if retries > 0 {
				retries--
				continue
			}
			if index+1 < len(allowed) {
				break
			}
			return ModelResponse{}, err
		}
	}
}
func (c *RoutedModelClient) slot(name string) (ModelSlotConfig, bool) {
	switch name {
	case "hot_query":
		return c.Slots.HotQuery, true
	case "deep_query":
		return c.Slots.DeepQuery, true
	case "proposal":
		return c.Slots.Proposal, true
	case "dream":
		return c.Slots.Dream, true
	}
	return ModelSlotConfig{}, false
}
func (c *RoutedModelClient) semaphore(name string, n int) chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.semaphores == nil {
		c.semaphores = map[string]chan struct{}{}
	}
	if c.semaphores[name] == nil {
		c.semaphores[name] = make(chan struct{}, n)
	}
	return c.semaphores[name]
}
func trustBoundary(raw string) string {
	host, _ := url.Parse(raw)
	switch host.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return "local"
	}
	return "remote"
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func containsInt(values []int, want int) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func minDuration(a, b time.Duration) time.Duration {
	if a <= 0 || b < a {
		return b
	}
	return a
}
