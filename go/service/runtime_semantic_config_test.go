package service

import (
	"encoding/json"
	"testing"
)

func TestSemanticSearchConfig(t *testing.T) {
	c := DefaultSemanticSearchConfig()
	if c.Enabled || c.WorkerMode != "subprocess" || c.WorkerPath != "/usr/local/bin/memento-embed" || c.Dimensions != 384 || c.MaxBatchSize != 16 || !c.RefreshOnStartup {
		t.Fatal(c)
	}
	raw := json.RawMessage(`{"enabled":true,"model_path":" /model ","dimensions":2,"max_batch_size":3}`)
	c, err := DecodeSemanticSearchConfig(raw, nil)
	if err != nil || c.ModelPath == nil || *c.ModelPath != "/model" || c.Dimensions != 2 || c.MaxBatchSize != 3 {
		t.Fatal(c, err)
	}
	raw = json.RawMessage(`{"enabled":true}`)
	c, err = DecodeSemanticSearchConfig(raw, func(name string) (string, bool) { return " /env-model ", name == "MEMENTO_GTE_MODEL" })
	if err != nil || c.ModelPath == nil || *c.ModelPath != "/env-model" {
		t.Fatal(c, err)
	}
}
func TestSemanticSearchConfigFailures(t *testing.T) {
	bad := []string{`{"extra":1}`, `{"worker_mode":"bad"}`, `{"worker_path":""}`, `{"model_id":""}`, `{"dimensions":0}`, `{"default_search_mode":"bad"}`, `{"progressive_nice":20}`, `{"progressive_cpu_busy_limit_percent":101}`, `{"model_path":""}`, `{"enabled":true}`}
	for _, raw := range bad {
		if _, err := DecodeSemanticSearchConfig([]byte(raw), nil); err == nil {
			t.Fatal(raw)
		}
	}
	c := DefaultSemanticSearchConfig()
	for index, mutate := range []func(*SemanticSearchConfig){func(c *SemanticSearchConfig) { c.WorkerTimeoutSeconds = 0 }, func(c *SemanticSearchConfig) { c.MaxInputChars = 0 }, func(c *SemanticSearchConfig) { c.MaxBatchSize = 0 }, func(c *SemanticSearchConfig) { c.MaxCandidates = 0 }, func(c *SemanticSearchConfig) { c.ProgressiveStartupDelaySeconds = -1 }, func(c *SemanticSearchConfig) { c.ProgressiveInteractiveIdleSeconds = -1 }, func(c *SemanticSearchConfig) { c.ProgressiveDelaySeconds = -1 }, func(c *SemanticSearchConfig) { c.ProgressiveLoadAverageLimit = 0 }, func(c *SemanticSearchConfig) { c.ProgressiveCPUSampleSeconds = 0 }} {
		copy := c
		mutate(&copy)
		if copy.Validate() == nil {
			t.Fatal(index)
		}
	}
	if _, err := DecodeSemanticSearchConfig(nil, nil); err != nil {
		t.Fatal(err)
	}
}
