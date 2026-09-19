package service

import (
	"github.com/rcarmo/memento/go/control"
	"strings"
	"testing"
)

func TestProfileQuestionAndEvidence(t *testing.T) {
	p := ProfileQuestion("Who owns the current work API token?")
	if !p.SecretIntent || !p.Relational || p.TemporalIntent != "current" || p.NamespaceHint == nil || *p.NamespaceHint != "work" {
		t.Fatalf("profile=%#v", p)
	}
	if !NamespaceMatches(p, "/work/a.md") || NamespaceMatches(p, "/personal/a.md") {
		t.Fatal("namespace mismatch")
	}
	if !SensitiveEvidence([]string{"Secret"}) || !CurrentlyIneligible("active", []string{"stale"}) {
		t.Fatal("sensitivity filters")
	}
	normal := ProfileQuestion("Which service powers the rack?")
	concepts := []AnswerReadConcept{{Title: "Rack service", Body: "The service powers this rack", Path: "/a.md", Status: "active"}}
	if !EvidenceSufficient(normal, concepts) {
		t.Fatal("expected sufficient evidence")
	}
	historical := ProfileQuestion("What was used previously?")
	if EvidenceSufficient(historical, concepts) {
		t.Fatal("current evidence must not satisfy historical query")
	}
	concepts[0].Body = "This was used previously"
	concepts[0].Tags = []string{"historical"}
	if !EvidenceSufficient(historical, concepts) {
		t.Fatal("historical evidence not accepted")
	}
}
func TestParseAndValidateModelAnswer(t *testing.T) {
	response := ModelResponse{OutputText: `{"answer":"A","confidence":"high","unresolved":[1],"citations":[{"id":"id","path":"/a.md","revision":"rev"}],"model_chain":["one",{"model":"two","outcome":"success"}]}`}
	record, err := ParseModelAnswer(response, "deep_agent")
	if err != nil {
		t.Fatal(err)
	}
	if record.Answer != "A" || record.Unresolved[0] != "1" || len(record.ModelChain) != 2 {
		t.Fatalf("record=%#v", record)
	}
	concepts := []AnswerReadConcept{{ID: "id", Path: "/a.md"}}
	record = ValidateAnswerCitations(record, concepts, "rev", "deep_agent")
	if record.Answer != "A" || len(record.Citations) != 1 {
		t.Fatalf("validated=%#v", record)
	}
	record.Citations[0].Revision = "bad"
	record = ValidateAnswerCitations(record, concepts, "rev", "deep_agent")
	if record.Answer != UnknownAnswer || record.Unresolved[0] != "citation_validation_failed" {
		t.Fatalf("repaired=%#v", record)
	}
}
func TestModelAnswerStrictFailuresAndResponseChain(t *testing.T) {
	cases := []string{`bad`, `{} {}`, `{"citations":{}}`, `{"citations":[{"id":1,"path":"/a","revision":"r"}]}`, `{"model_chain":{}}`, `{"model_chain":[{"model":"m"}]}`}
	for _, raw := range cases {
		if _, err := ParseModelAnswer(ModelResponse{OutputText: raw}, "deep_agent"); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	record, err := ParseModelAnswer(ModelResponse{OutputText: `{"answer":"UNKNOWN","model_chain":{}}`, ModelChain: []control.ModelAttempt{{Model: "provider", Outcome: "success"}}}, "deep_agent")
	if err != nil || len(record.ModelChain) != 1 {
		t.Fatalf("record=%#v err=%v", record, err)
	}
	missing := ValidateAnswerCitations(AnswerRecord{Answer: "A", ModelChain: []control.ModelAttempt{}}, nil, "rev", "deep_agent")
	if missing.Answer != UnknownAnswer || !strings.Contains(missing.Unresolved[0], "missing") {
		t.Fatalf("missing=%#v", missing)
	}
}
func TestAnswerEvidenceBranches(t *testing.T) {
	personal := ProfileQuestion("personal note")
	if personal.NamespaceHint == nil || *personal.NamespaceHint != "personal" {
		t.Fatal(personal)
	}
	if EvidenceTextMatches(QueryProfile{}, "title", "body") {
		t.Fatal("empty terms")
	}
	if !EvidenceTextMatches(QueryProfile{Terms: []string{"prefixing"}}, "prefix", "") {
		t.Fatal("prefix match")
	}
	if EvidenceTextMatches(QueryProfile{Terms: []string{"one", "two"}}, "one", "") {
		t.Fatal("two terms require two matches")
	}
	if EvidenceSufficient(QueryProfile{NamespaceHint: personal.NamespaceHint}, []AnswerReadConcept{{Title: "personal note", Body: "personal note", Path: "/work/x", Status: "active"}}) {
		t.Fatal("wrong namespace")
	}
	if EvidenceSufficient(QueryProfile{TemporalIntent: "current", Terms: []string{"thing"}}, []AnswerReadConcept{{Title: "thing", Status: "deprecated"}}) {
		t.Fatal("deprecated current")
	}
	for _, tag := range []string{"conflicting", "historical", "obsolete", "stale"} {
		if !CurrentlyIneligible("active", []string{tag}) {
			t.Fatal(tag)
		}
	}
	if !CurrentlyIneligible("tombstone", nil) {
		t.Fatal("tombstone")
	}
	if !hasAnyTag([]string{"one", "two"}, "two") || hasAnyTag([]string{"one"}, "two") {
		t.Fatal("tag matching")
	}
}

func TestAnswerModelParsingBranches(t *testing.T) {
	record, err := ParseModelAnswer(ModelResponse{OutputText: `{"answer":1,"confidence":2,"unresolved":null,"citations":null,"model_chain":null}`}, "deep_agent")
	if err != nil || record.Answer != "1" || record.Confidence != "2" {
		t.Fatalf("record=%#v err=%v", record, err)
	}
	for _, raw := range []string{`{"unresolved":"bad"}`, `{"citations":[1]}`, `{"citations":[{"id":"i","path":"p","revision":"r","extra":"x"}]}`, `{"model_chain":[1]}`, `{"model_chain":[{"model":1,"outcome":"x"}]}`} {
		if _, err = ParseModelAnswer(ModelResponse{OutputText: raw}, "deep_agent"); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	unknown := AnswerRecord{Answer: UnknownAnswer, Citations: []AnswerCitation{{ID: "x"}}}
	trace := "trace"
	unknown.TraceID = &trace
	unknown = ValidateAnswerCitations(unknown, nil, "rev", "deep_agent")
	if unknown.TraceID != nil || len(unknown.Citations) != 0 {
		t.Fatalf("unknown=%#v", unknown)
	}
}

func TestAnswerConfigStrictDecoding(t *testing.T) {
	if _, err := DecodeDeepAnswersConfig([]byte(`{"limits":{"max_steps":13}}`)); err == nil {
		t.Fatal("accepted max_steps")
	}
	if _, err := DecodeExactAnswerCacheConfig([]byte(`{"unknown":1}`)); err == nil {
		t.Fatal("accepted unknown")
	}
	if _, err := DecodeHotWorkingMemoryConfig([]byte(`{"max_answers":11}`)); err == nil {
		t.Fatal("accepted max_answers")
	}
	if got, err := DecodeDeepAnswersConfig(nil); err != nil || got.Limits.MaxSteps != 8 {
		t.Fatalf("defaults=%#v err=%v", got, err)
	}
	for _, raw := range []string{`{} {}`, `{"limits":{"max_chars":1}}`, `{"model_policy_revision":""}`} {
		if _, err := DecodeDeepAnswersConfig([]byte(raw)); err == nil {
			t.Fatalf("accepted deep %s", raw)
		}
	}
	for _, raw := range []string{`{"ttl_seconds":0}`, `{} {}`} {
		if _, err := DecodeExactAnswerCacheConfig([]byte(raw)); err == nil {
			t.Fatalf("accepted cache %s", raw)
		}
	}
	for _, raw := range []string{`{"max_excerpt_chars":1}`, `{} {}`} {
		if _, err := DecodeHotWorkingMemoryConfig([]byte(raw)); err == nil {
			t.Fatalf("accepted hot %s", raw)
		}
	}
}
