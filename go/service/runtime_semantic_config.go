package service

import (
	"encoding/json"
	"errors"
	"strings"
)

type SemanticSearchConfig struct {
	Enabled                           bool    `json:"enabled"`
	WorkerMode                        string  `json:"worker_mode"`
	WorkerPath                        string  `json:"worker_path"`
	WorkerTimeoutSeconds              float64 `json:"worker_timeout_seconds"`
	FFILibraryPath                    *string `json:"ffi_library_path"`
	SQLiteExtensionPath               *string `json:"sqlite_extension_path"`
	ModelPath                         *string `json:"model_path"`
	ModelID                           string  `json:"model_id"`
	Dimensions                        int     `json:"dimensions"`
	MaxInputChars                     int     `json:"max_input_chars"`
	MaxBatchSize                      int     `json:"max_batch_size"`
	MaxCandidates                     int     `json:"max_candidates"`
	DefaultSearchMode                 string  `json:"default_search_mode"`
	RefreshOnStartup                  bool    `json:"refresh_on_startup"`
	ProgressiveEnabled                bool    `json:"progressive_enabled"`
	ProgressiveStartupDelaySeconds    float64 `json:"progressive_startup_delay_seconds"`
	ProgressiveInteractiveIdleSeconds float64 `json:"progressive_interactive_idle_seconds"`
	ProgressiveDelaySeconds           float64 `json:"progressive_delay_seconds"`
	ProgressiveLoadAverageLimit       float64 `json:"progressive_load_average_limit"`
	ProgressiveCPUBusyLimitPercent    float64 `json:"progressive_cpu_busy_limit_percent"`
	ProgressiveCPUSampleSeconds       float64 `json:"progressive_cpu_sample_seconds"`
	ProgressiveNice                   int     `json:"progressive_nice"`
}

func DefaultSemanticSearchConfig() SemanticSearchConfig {
	return SemanticSearchConfig{WorkerMode: "subprocess", WorkerPath: "/usr/local/bin/memento-embed", WorkerTimeoutSeconds: 300, ModelID: "rust-gte", Dimensions: 384, MaxInputChars: 4096, MaxBatchSize: 16, MaxCandidates: 200, DefaultSearchMode: "lexical", RefreshOnStartup: true, ProgressiveStartupDelaySeconds: 120, ProgressiveInteractiveIdleSeconds: 15, ProgressiveDelaySeconds: 30, ProgressiveLoadAverageLimit: 1.5, ProgressiveCPUBusyLimitPercent: 75, ProgressiveCPUSampleSeconds: 15, ProgressiveNice: 15}
}
func (c SemanticSearchConfig) Validate() error {
	if c.WorkerMode != "subprocess" && c.WorkerMode != "in_process" {
		return errors.New("invalid semantic worker mode")
	}
	if strings.TrimSpace(c.WorkerPath) == "" || strings.TrimSpace(c.ModelID) == "" {
		return errors.New("semantic path/model_id must not be empty")
	}
	if c.WorkerTimeoutSeconds <= 0 || c.Dimensions < 1 || c.MaxInputChars < 1 || c.MaxBatchSize < 1 || c.MaxCandidates < 1 {
		return errors.New("semantic setting out of range")
	}
	if c.DefaultSearchMode != "lexical" && c.DefaultSearchMode != "semantic" && c.DefaultSearchMode != "hybrid" {
		return errors.New("invalid default search mode")
	}
	if c.ProgressiveStartupDelaySeconds < 0 || c.ProgressiveInteractiveIdleSeconds < 0 || c.ProgressiveDelaySeconds < 0 || c.ProgressiveLoadAverageLimit <= 0 || c.ProgressiveCPUBusyLimitPercent <= 0 || c.ProgressiveCPUBusyLimitPercent > 100 || c.ProgressiveCPUSampleSeconds <= 0 || c.ProgressiveNice < 0 || c.ProgressiveNice > 19 {
		return errors.New("semantic progressive setting out of range")
	}
	for _, path := range []*string{c.FFILibraryPath, c.SQLiteExtensionPath, c.ModelPath} {
		if path != nil && strings.TrimSpace(*path) == "" {
			return errors.New("path values must not be empty")
		}
	}
	if c.Enabled && c.ModelPath == nil {
		return errors.New("semantic search requires model_path/MEMENTO_GTE_MODEL")
	}
	return nil
}
func (c SemanticSearchConfig) Resolved(lookup func(string) (string, bool)) SemanticSearchConfig {
	if lookup != nil {
		if value, ok := lookup("MEMENTO_GTE_MODEL"); ok && strings.TrimSpace(value) != "" {
			text := strings.TrimSpace(value)
			c.ModelPath = &text
		}
	}
	if c.ModelPath != nil {
		text := strings.TrimSpace(*c.ModelPath)
		c.ModelPath = &text
	}
	c.WorkerPath = strings.TrimSpace(c.WorkerPath)
	c.ModelID = strings.TrimSpace(c.ModelID)
	return c
}
func DecodeSemanticSearchConfig(raw json.RawMessage, lookup func(string) (string, bool)) (SemanticSearchConfig, error) {
	c := DefaultSemanticSearchConfig()
	if len(raw) > 0 {
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&c); err != nil {
			return SemanticSearchConfig{}, err
		}
	}
	c = c.Resolved(lookup)
	if err := c.Validate(); err != nil {
		return SemanticSearchConfig{}, err
	}
	return c, nil
}
