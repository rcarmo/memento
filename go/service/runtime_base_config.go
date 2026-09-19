package service

import (
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/rcarmo/memento/go/execute"
)

type LimitsConfig struct {
	MaxConceptBytes  int `json:"max_concept_bytes"`
	MaxSearchResults int `json:"max_search_results"`
}
type MCPExecuteConfig struct {
	MaxOperations    int     `json:"max_operations"`
	MaxIntermediates int     `json:"max_intermediates"`
	MaxRecords       int     `json:"max_records"`
	MaxOutputBytes   int     `json:"max_output_bytes"`
	MaxTimeSeconds   float64 `json:"max_time_seconds"`
}
type MCPConfig struct {
	ToolSurface          string           `json:"tool_surface"`
	CompactAnswerEnabled bool             `json:"compact_answer_enabled"`
	MaxRequestBytes      int64            `json:"max_request_bytes"`
	AllowedOrigins       []string         `json:"allowed_origins"`
	Execute              MCPExecuteConfig `json:"execute"`
}
type IntelligentTiersConfig struct {
	DeepAnswers        json.RawMessage `json:"deep_answers"`
	ExactAnswerCache   json.RawMessage `json:"exact_answer_cache"`
	HotWorkingMemory   json.RawMessage `json:"hot_working_memory"`
	ModelProposals     json.RawMessage `json:"model_proposals"`
	Dream              json.RawMessage `json:"dream"`
	ModelProviderSlots json.RawMessage `json:"model_provider_slots"`
	SemanticSearch     json.RawMessage `json:"semantic_search"`
	NeedleRouter       json.RawMessage `json:"needle_router"`
}

func defaultLimitsConfig() LimitsConfig { return LimitsConfig{262144, 100} }
func defaultMCPConfig() MCPConfig {
	return MCPConfig{ToolSurface: "compact", CompactAnswerEnabled: true, MaxRequestBytes: 72 * 1024 * 1024, Execute: MCPExecuteConfig{12, 12, 50, 65536, 3}}
}
func (c MCPConfig) Validate() error {
	valid := map[string]bool{"compact": true, "standard": true, "read_only": true, "curator": true, "admin": true}
	if !valid[c.ToolSurface] {
		return errors.New("invalid MCP tool surface")
	}
	if c.MaxRequestBytes < 4*1024*1024 {
		return errors.New("max_request_bytes must be at least 4194304")
	}
	e := c.Execute
	if e.MaxOperations < 1 || e.MaxOperations > 32 || e.MaxIntermediates < 1 || e.MaxIntermediates > 64 || e.MaxRecords < 1 || e.MaxRecords > 500 || e.MaxOutputBytes < 512 || e.MaxTimeSeconds <= 0 || e.MaxTimeSeconds > 30 {
		return errors.New("MCP execute setting out of range")
	}
	for _, item := range c.AllowedOrigins {
		item = strings.TrimSpace(item)
		parsed, err := url.Parse(item)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("allowed origins must be HTTP(S) origins without paths")
		}
	}
	return nil
}
func (c MCPConfig) NormalizedOrigins() []string {
	out := make([]string, 0, len(c.AllowedOrigins))
	seen := map[string]bool{}
	for _, item := range c.AllowedOrigins {
		item = strings.TrimSpace(item)
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}
func (c MCPConfig) ExecuteLimits() execute.Limits {
	return execute.Limits{MaxOperations: c.Execute.MaxOperations, MaxIntermediates: c.Execute.MaxIntermediates, MaxRecords: c.Execute.MaxRecords, MaxOutputBytes: json.Number(strconv.Itoa(c.Execute.MaxOutputBytes)), MaxTimeSeconds: c.Execute.MaxTimeSeconds}
}
func activeField(raw json.RawMessage, field string) (bool, error) {
	if len(raw) == 0 {
		return false, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return false, err
	}
	value := object[field]
	if len(value) == 0 {
		return false, nil
	}
	if field == "mode" {
		var mode string
		if err := json.Unmarshal(value, &mode); err != nil {
			return false, err
		}
		return mode != "" && mode != "disabled", nil
	}
	var enabled bool
	if err := json.Unmarshal(value, &enabled); err != nil {
		return false, err
	}
	return enabled, nil
}
func (c IntelligentTiersConfig) ValidateModelsOff() error {
	if _, err := decodeNeedleConfig(c.NeedleRouter); err != nil {
		return err
	}
	for _, item := range []struct {
		raw   json.RawMessage
		field string
	}{{c.DeepAnswers, "enabled"}, {c.ExactAnswerCache, "enabled"}, {c.HotWorkingMemory, "enabled"}, {c.ModelProposals, "enabled"}, {c.Dream, "mode"}, {c.SemanticSearch, "enabled"}, {c.NeedleRouter, "enabled"}} {
		active, err := activeField(item.raw, item.field)
		if err != nil {
			return err
		}
		if active {
			return errors.New("enabled intelligent tiers are not supported by the models-off runtime")
		}
	}
	return nil
}
