package service

import (
	"context"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/derived"
)

type answerIndex struct {
	page  derived.SearchPage
	calls []derived.SearchOptions
	err   error
}

func (i *answerIndex) SearchLexical(_ context.Context, _ access.EffectivePolicy, o derived.SearchOptions) (derived.SearchPage, error) {
	i.calls = append(i.calls, o)
	return i.page, i.err
}
func (i *answerIndex) Graph(context.Context, access.EffectivePolicy, string, derived.GraphOptions) (derived.GraphNeighborhood, error) {
	return derived.GraphNeighborhood{}, nil
}
func TestAnswerDeepModelAndAbstention(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active", Score: 1}}}}
	controls.Index = index
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"The target exists","confidence":"high","unresolved":[],"citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}],"model_chain":[]}`}}
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	e := AnswerEndpoint{Client: model, Deep: deep}
	policy := access.EffectivePolicy{Principal: "actor", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}
	result, err := e.deep(context.Background(), controls, policy, "What is the target?", "summary", "scope", ProfileQuestion("What is the target?"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.Answer != "The target exists" || result.Record.Evidence == nil || len(result.ReadConcepts) != 1 {
		t.Fatalf("result=%#v", result)
	}
	if len(model.requests) != 1 || model.requests[0].Task != "memory_answer_deep" || model.requests[0].SlotName != "deep_query" || model.requests[0].Metadata["answer_mode"] != "summary" {
		t.Fatalf("request=%#v", model.requests)
	}
	if len(index.calls) != 1 || index.calls[0].Limit != 5 || !index.calls[0].Strict {
		t.Fatalf("calls=%#v", index.calls)
	}
	model.requests = nil
	index.page.Results = nil
	result, err = e.deep(context.Background(), controls, policy, "missing", "summary", "scope", ProfileQuestion("missing"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.Answer != UnknownAnswer || result.Record.AnswerSource != "evidence_abstention" || len(model.requests) != 0 {
		t.Fatalf("abstention=%#v requests=%d", result, len(model.requests))
	}
}
func TestAnswerPromptBoundsAndStaticRecords(t *testing.T) {
	prompt := deepAnswerPrompt("q", []AnswerReadConcept{{ID: "id", Path: "/a", Revision: "r", Title: "t", Body: "0123456789"}}, 80)
	if len([]rune(prompt)) < 80 || prompt == "" {
		t.Fatal(prompt)
	}
	if disabledAnswer().AnswerSource != "disabled" {
		t.Fatal("disabled source")
	}
	p := ProfileQuestion("show password")
	if policyAbstention(p, "scope").AnswerSource != "policy_abstention" {
		t.Fatal("policy source")
	}
}
