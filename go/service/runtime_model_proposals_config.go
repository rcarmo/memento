package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type ModelProposalLimitsConfig struct {
	MaxSearchResults      int `json:"max_search_results"`
	MaxConsultedConcepts  int `json:"max_consulted_concepts"`
	MaxContextChars       int `json:"max_context_chars"`
	MaxOutputChars        int `json:"max_output_chars"`
	MaxDiffChars          int `json:"max_diff_chars"`
	MaxChanges            int `json:"max_changes"`
	MaxBodyChars          int `json:"max_body_chars"`
	MaxRationaleChars     int `json:"max_rationale_chars"`
	MaxSecretEntropyChars int `json:"max_secret_entropy_chars"`
}
type ModelProposalsConfig struct {
	Enabled             bool                      `json:"enabled"`
	ModelPolicyRevision string                    `json:"model_policy_revision"`
	PromptVersion       string                    `json:"prompt_version"`
	Limits              ModelProposalLimitsConfig `json:"limits"`
}

func DefaultModelProposalsConfig() ModelProposalsConfig {
	return ModelProposalsConfig{false, "disabled", "v1", ModelProposalLimitsConfig{5, 6, 12000, 8000, 2000000, 20, 32000, 4000, 32}}
}
func DecodeModelProposalsConfig(raw json.RawMessage) (ModelProposalsConfig, error) {
	defaults := DefaultModelProposalsConfig()
	if len(bytes.TrimSpace(raw)) == 0 {
		return defaults, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&defaults); err != nil {
		return ModelProposalsConfig{}, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return ModelProposalsConfig{}, errors.New("trailing JSON")
	}
	l := defaults.Limits
	if defaults.ModelPolicyRevision == "" || defaults.PromptVersion == "" || l.MaxSearchResults < 1 || l.MaxSearchResults > 10 || l.MaxConsultedConcepts < 1 || l.MaxConsultedConcepts > 10 || l.MaxContextChars < 512 || l.MaxOutputChars < 256 || l.MaxDiffChars < 1 || l.MaxChanges < 1 || l.MaxChanges > 100 || l.MaxBodyChars < 1 || l.MaxRationaleChars < 1 || l.MaxSecretEntropyChars < 8 {
		return ModelProposalsConfig{}, errors.New("model proposal configuration out of range")
	}
	return defaults, nil
}
