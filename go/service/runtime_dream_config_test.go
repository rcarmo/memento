package service

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDreamConfig(t *testing.T) {
	defaults := DefaultDreamConfig()
	decoded, err := DecodeDreamConfig(nil)
	if err != nil || !reflect.DeepEqual(decoded, defaults) {
		t.Fatal(decoded, err)
	}
	decoded, err = DecodeDreamConfig(json.RawMessage(`{"mode":"report_only","scanner":{"oversized_body_chars":256,"oversized_top_level_sections":1,"max_oversized_candidates":1,"duplicate_similarity_threshold":0.5},"budgets":{"max_signals_per_run":1,"max_model_proposals_per_run":0,"max_runtime_seconds":0.1,"daily_proposal_limit":0}}`))
	if err != nil || decoded.Mode != "report_only" || decoded.Scanner.OversizedBodyChars != 256 {
		t.Fatal(decoded, err)
	}
	var tiers IntelligentTiersConfig
	tiers.Dream = json.RawMessage(`{"mode":"report_only"}`)
	if err = tiers.ValidateModelsOff(); err != nil {
		t.Fatal(err)
	}
	tiers.Dream = json.RawMessage(`{"mode":"propose"}`)
	if err = tiers.ValidateModelsOff(); err != nil {
		t.Fatal(err)
	}
}
func TestDreamConfigFailures(t *testing.T) {
	cases := []string{`{"extra":1}`, `{} {}`, `{"mode":"bad"}`, `{"model_policy_revision":""}`, `{"prompt_version":""}`, `{"tool_version":""}`, `{"interval_seconds":299}`, `{"quiet_period_seconds":-1}`, `{"scanner":{"oversized_body_chars":255}}`, `{"scanner":{"oversized_top_level_sections":0}}`, `{"scanner":{"max_oversized_candidates":0}}`, `{"scanner":{"max_oversized_candidates":4}}`, `{"scanner":{"duplicate_similarity_threshold":0.4}}`, `{"scanner":{"duplicate_similarity_threshold":1.1}}`, `{"budgets":{"max_signals_per_run":0}}`, `{"budgets":{"max_signals_per_run":101}}`, `{"budgets":{"max_model_proposals_per_run":-1}}`, `{"budgets":{"max_model_proposals_per_run":21}}`, `{"budgets":{"max_runtime_seconds":0}}`, `{"budgets":{"daily_proposal_limit":-1}}`, `{"budgets":{"daily_proposal_limit":201}}`}
	for _, raw := range cases {
		if _, err := DecodeDreamConfig(json.RawMessage(raw)); err == nil {
			t.Fatal(raw)
		}
	}
}
