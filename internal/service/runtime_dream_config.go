package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type DreamScannerConfig struct {
	OversizedBodyChars           int     `json:"oversized_body_chars"`
	OversizedTopLevelSections    int     `json:"oversized_top_level_sections"`
	MaxOversizedCandidates       int     `json:"max_oversized_candidates"`
	DuplicateSimilarityThreshold float64 `json:"duplicate_similarity_threshold"`
}
type DreamBudgetsConfig struct {
	MaxSignalsPerRun        int     `json:"max_signals_per_run"`
	MaxModelProposalsPerRun int     `json:"max_model_proposals_per_run"`
	MaxRuntimeSeconds       float64 `json:"max_runtime_seconds"`
	DailyProposalLimit      int     `json:"daily_proposal_limit"`
}
type DreamConfig struct {
	Mode, ModelPolicyRevision, PromptVersion, ToolVersion string
	IntervalSeconds, QuietPeriodSeconds                   int
	Scanner                                               DreamScannerConfig
	Budgets                                               DreamBudgetsConfig
}
type dreamWire struct {
	Mode                string             `json:"mode"`
	ModelPolicyRevision string             `json:"model_policy_revision"`
	PromptVersion       string             `json:"prompt_version"`
	ToolVersion         string             `json:"tool_version"`
	IntervalSeconds     int                `json:"interval_seconds"`
	QuietPeriodSeconds  int                `json:"quiet_period_seconds"`
	Scanner             DreamScannerConfig `json:"scanner"`
	Budgets             DreamBudgetsConfig `json:"budgets"`
}

func DefaultDreamConfig() DreamConfig {
	return DreamConfig{"disabled", "disabled", "v1", "v1", 21600, 300, DreamScannerConfig{6000, 6, 3, .86}, DreamBudgetsConfig{25, 5, 30, 20}}
}
func DecodeDreamConfig(raw json.RawMessage) (DreamConfig, error) {
	defaults := DefaultDreamConfig()
	if len(bytes.TrimSpace(raw)) == 0 {
		return defaults, nil
	}
	wire := dreamWire(defaults)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return DreamConfig{}, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return DreamConfig{}, errors.New("trailing JSON")
	}
	if wire.Mode != "disabled" && wire.Mode != "report_only" && wire.Mode != "propose" {
		return DreamConfig{}, errors.New("dream.mode must be disabled, report_only, or propose")
	}
	if wire.ModelPolicyRevision == "" || wire.PromptVersion == "" || wire.ToolVersion == "" || wire.IntervalSeconds < 300 || wire.QuietPeriodSeconds < 0 || wire.Scanner.OversizedBodyChars < 256 || wire.Scanner.OversizedTopLevelSections < 1 || wire.Scanner.MaxOversizedCandidates < 1 || wire.Scanner.MaxOversizedCandidates > 3 || wire.Scanner.DuplicateSimilarityThreshold < .5 || wire.Scanner.DuplicateSimilarityThreshold > 1 || wire.Budgets.MaxSignalsPerRun < 1 || wire.Budgets.MaxSignalsPerRun > 100 || wire.Budgets.MaxModelProposalsPerRun < 0 || wire.Budgets.MaxModelProposalsPerRun > 20 || wire.Budgets.MaxRuntimeSeconds <= 0 || wire.Budgets.DailyProposalLimit < 0 || wire.Budgets.DailyProposalLimit > 200 {
		return DreamConfig{}, errors.New("dream configuration out of range")
	}
	return DreamConfig(wire), nil
}
