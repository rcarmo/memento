package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type DeepAnswerLimitsConfig struct {
	MaxSteps       int     `json:"max_steps"`
	MaxTimeSeconds float64 `json:"max_time_seconds"`
	MaxConcepts    int     `json:"max_concepts"`
	MaxChars       int     `json:"max_chars"`
	MaxAnswerChars int     `json:"max_answer_chars"`
}
type DeepAnswersConfig struct {
	Enabled             bool                   `json:"enabled"`
	ModelPolicyRevision string                 `json:"model_policy_revision"`
	PromptVersion       string                 `json:"prompt_version"`
	ToolVersion         string                 `json:"tool_version"`
	TraceMaxEntries     int                    `json:"trace_max_entries"`
	TraceMaxAgeDays     int                    `json:"trace_max_age_days"`
	Limits              DeepAnswerLimitsConfig `json:"limits"`
}
type ExactAnswerCacheConfig struct {
	Enabled    bool `json:"enabled"`
	TTLSeconds int  `json:"ttl_seconds"`
	MaxEntries int  `json:"max_entries"`
}
type HotWorkingMemoryConfig struct {
	Enabled            bool `json:"enabled"`
	TTLSeconds         int  `json:"ttl_seconds"`
	MaxChangedConcepts int  `json:"max_changed_concepts"`
	MaxAnswers         int  `json:"max_answers"`
	MaxExcerptChars    int  `json:"max_excerpt_chars"`
}

func DefaultDeepAnswersConfig() DeepAnswersConfig {
	return DeepAnswersConfig{false, "disabled", "v1", "v1", 50, 30, DeepAnswerLimitsConfig{8, 3, 6, 12000, 2000}}
}
func DefaultExactAnswerCacheConfig() ExactAnswerCacheConfig {
	return ExactAnswerCacheConfig{false, 86400, 200}
}
func DefaultHotWorkingMemoryConfig() HotWorkingMemoryConfig {
	return HotWorkingMemoryConfig{false, 3600, 10, 10, 4000}
}
func decodeAnswerConfig(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func DecodeDeepAnswersConfig(raw json.RawMessage) (DeepAnswersConfig, error) {
	v := DefaultDeepAnswersConfig()
	if err := decodeAnswerConfig(raw, &v); err != nil {
		return v, err
	}
	l := v.Limits
	if v.ModelPolicyRevision == "" || v.PromptVersion == "" || v.ToolVersion == "" || v.TraceMaxEntries < 1 || v.TraceMaxAgeDays < 1 || l.MaxSteps < 1 || l.MaxSteps > 12 || l.MaxTimeSeconds <= 0 || l.MaxConcepts < 1 || l.MaxChars < 256 || l.MaxAnswerChars < 32 {
		return v, errors.New("deep answer configuration out of range")
	}
	return v, nil
}
func DecodeExactAnswerCacheConfig(raw json.RawMessage) (ExactAnswerCacheConfig, error) {
	v := DefaultExactAnswerCacheConfig()
	if err := decodeAnswerConfig(raw, &v); err != nil {
		return v, err
	}
	if v.TTLSeconds < 1 || v.MaxEntries < 1 {
		return v, errors.New("exact answer cache configuration out of range")
	}
	return v, nil
}
func DecodeHotWorkingMemoryConfig(raw json.RawMessage) (HotWorkingMemoryConfig, error) {
	v := DefaultHotWorkingMemoryConfig()
	if err := decodeAnswerConfig(raw, &v); err != nil {
		return v, err
	}
	if v.TTLSeconds < 1 || v.MaxChangedConcepts < 1 || v.MaxChangedConcepts > 10 || v.MaxAnswers < 1 || v.MaxAnswers > 10 || v.MaxExcerptChars < 128 {
		return v, errors.New("hot working memory configuration out of range")
	}
	return v, nil
}
