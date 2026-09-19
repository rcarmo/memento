package service

import (
	"context"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/umcp"
)

type answerIndex struct {
	page       derived.SearchPage
	graph      derived.GraphNeighborhood
	calls      []derived.SearchOptions
	graphCalls []derived.GraphOptions
	err        error
}

func (i *answerIndex) SearchLexical(_ context.Context, _ access.EffectivePolicy, o derived.SearchOptions) (derived.SearchPage, error) {
	i.calls = append(i.calls, o)
	return i.page, i.err
}
func (i *answerIndex) Graph(_ context.Context, _ access.EffectivePolicy, _ string, options derived.GraphOptions) (derived.GraphNeighborhood, error) {
	i.graphCalls = append(i.graphCalls, options)
	return i.graph, i.err
}
func TestAnswerCallArgumentAndRoleGuards(t *testing.T) {
	e := &AnswerEndpoint{}
	if _, err := e.Call(context.Background(), map[string]any{}); err == nil || err.Error() != "question must be a string" {
		t.Fatalf("question=%v", err)
	}
	if _, err := e.Call(context.Background(), map[string]any{"question": "q", "answer_mode": 1}); err == nil || err.Error() != "answer_mode must be a string" {
		t.Fatalf("mode=%v", err)
	}
	if _, err := e.Call(context.Background(), map[string]any{"question": " \n "}); err == nil || err.Error() != "question must not be empty" {
		t.Fatalf("empty=%v", err)
	}
	jobs, _ := jobsTest(t)
	e.Jobs = jobs
	server := umcp.NewServer("answer-guard")
	if err := server.Tools.Register(umcp.Tool{Name: "answer", Parameters: []umcp.Parameter{{Name: "question", Types: []umcp.ParamType{umcp.StringParam}}}, Call: e.Call}); err != nil {
		t.Fatal(err)
	}
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"answer","arguments":{"question":"q"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatalf("response=%#v err=%v", response, err)
	}
	// jobsTest actor has proposer/curator but no reader; the service returns a
	// forbidden failure envelope before touching repository/model dependencies.
	if response.Error == nil {
		t.Fatal("expected output validation failure after forbidden envelope")
	}
}

func TestAnswerHelperBranches(t *testing.T) {
	p := QueryProfile{TemporalIntent: "historical"}
	concepts := []AnswerReadConcept{{ID: "old", Supersedes: []string{"prior"}}, {ID: "prior"}, {ID: "old"}}
	if got := filterSuperseded(p, concepts); len(got) != 2 {
		t.Fatalf("historical=%#v", got)
	}
	p.TemporalIntent = "neutral"
	if got := filterSuperseded(p, concepts); len(got) != 1 || got[0].ID != "old" {
		t.Fatalf("current=%#v", got)
	}
	if hasTerm([]string{"one"}, "two") || !hasTerm([]string{"one"}, "two", "one") {
		t.Fatal("hasTerm")
	}
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
func TestAnswerDeepRelationalGraphClosure(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/b.md", "neighbor", "Neighbour", "The neighbour is linked to the target")
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}, graph: derived.GraphNeighborhood{Outbound: []derived.GraphEdge{{ConceptID: "neighbor", Path: "/b.md", Depth: 1, Direction: "outbound"}}}}
	controls.Index = index
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"linked","confidence":"high","citations":[{"id":"neighbor","path":"/b.md","revision":"` + revision + `"}]}`}}
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	e := AnswerEndpoint{Client: model, Deep: deep}
	policy := access.EffectivePolicy{Principal: "actor", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}
	result, err := e.deep(context.Background(), controls, policy, "Which service is linked to the target?", "summary", "scope", ProfileQuestion("Which service is linked to the target?"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ReadConcepts) != 2 || len(index.graphCalls) != 1 || index.graphCalls[0].Depth != 1 {
		t.Fatalf("result=%#v graph=%#v", result, index.graphCalls)
	}
	found := false
	for _, step := range result.Steps {
		if step.Action == "graph_neighbors" {
			found = true
		}
	}
	if !found {
		t.Fatalf("steps=%#v", result.Steps)
	}
	evidence := result.Record.Evidence.(map[string]any)
	if evidence["retrieval_strategy"] != "hybrid_top_5_to_10_relational_depth_1" {
		t.Fatalf("evidence=%#v", evidence)
	}
}

func TestAnswerHotReuseAndSynthesis(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	store, _ := answerStoreTest(t)
	hot := DefaultHotWorkingMemoryConfig()
	hot.Enabled = true
	deep := DefaultDeepAnswersConfig()
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"hot answer","confidence":"high","unresolved":[],"citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	e := AnswerEndpoint{Client: model, Store: store, Deep: deep, Hot: hot}
	policy := access.EffectivePolicy{Principal: "actor", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}
	profile := ProfileQuestion("What target changed?")
	if err := store.PutHotChanged(context.Background(), "scope", []string{"12345678"}, 10); err != nil {
		t.Fatal(err)
	}
	record, err := e.hot(context.Background(), controls, policy, "What target changed?", NormalizeQuestion("What target changed?"), "summary", "scope", profile, revision)
	if err != nil || record == nil || record.Answer != "hot answer" {
		t.Fatalf("record=%#v err=%v", record, err)
	}
	if len(model.requests) != 1 || model.requests[0].Task != "memory_answer_hot" || model.requests[0].SlotName != "hot_query" {
		t.Fatalf("requests=%#v", model.requests)
	}
	if err = store.PutHot(context.Background(), "scope", "cached", "summary", revision, *record, []string{"12345678"}, 60, 10); err != nil {
		t.Fatal(err)
	}
	model.requests = nil
	record, err = e.hot(context.Background(), controls, policy, "cached", "cached", "summary", "scope", profile, revision)
	if err != nil || record == nil || record.AnswerSource != "hot_memory" || len(model.requests) != 0 {
		t.Fatalf("cached=%#v err=%v requests=%d", record, err, len(model.requests))
	}
	model.response.OutputText = `{"answer":"UNKNOWN","confidence":"low","citations":[]}`
	if err = store.PutHotChanged(context.Background(), "other", []string{"12345678"}, 10); err != nil {
		t.Fatal(err)
	}
	record, err = e.hot(context.Background(), controls, policy, "other", "other", "summary", "other", profile, revision)
	if err != nil || record != nil {
		t.Fatalf("unknown=%#v err=%v", record, err)
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
