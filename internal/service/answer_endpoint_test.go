package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

type failingAnswers struct{ stage string }

func (f failingAnswers) GetExact(context.Context, string) (*AnswerRecord, error) {
	if f.stage == "get_exact" {
		return nil, errors.New("get exact")
	}
	return nil, nil
}
func (f failingAnswers) PutExact(context.Context, string, string, string, string, string, AnswerRecord, []string, []string, int, int) error {
	if f.stage == "put_exact" {
		return errors.New("put exact")
	}
	return nil
}
func (f failingAnswers) GetHotContext(context.Context, string, string, string, string) ([]string, *AnswerRecord, error) {
	if f.stage == "get_hot" {
		return nil, nil, errors.New("get hot")
	}
	return nil, nil, nil
}
func (f failingAnswers) PutHot(context.Context, string, string, string, string, AnswerRecord, []string, int, int) error {
	if f.stage == "put_hot" {
		return errors.New("put hot")
	}
	return nil
}
func (f failingAnswers) InsertTrace(context.Context, string, string, string, string, DeepAnswerResult, int, int) (string, error) {
	if f.stage == "trace" {
		return "", errors.New("trace")
	}
	return "trace", nil
}

type semanticAnswerIndex struct {
	answerIndex
	semantic    derived.SearchPage
	semanticErr error
}

func (i *semanticAnswerIndex) SearchSemantic(_ context.Context, _ access.EffectivePolicy, _ derived.SemanticSearchOptions, _ derived.SemanticClient) (derived.SearchPage, error) {
	return i.semantic, i.semanticErr
}

type noopSemanticClient struct{}

func (noopSemanticClient) Embed(string) ([]float32, error)      { return nil, nil }
func (noopSemanticClient) ModelInfo() derived.SemanticModelInfo { return derived.SemanticModelInfo{} }

type sequenceAnswerIndex struct {
	pages []derived.SearchPage
	errs  []error
	calls int
	graph derived.GraphNeighborhood
}

func (i *sequenceAnswerIndex) SearchLexical(context.Context, access.EffectivePolicy, derived.SearchOptions) (derived.SearchPage, error) {
	n := i.calls
	i.calls++
	if n < len(i.errs) && i.errs[n] != nil {
		return derived.SearchPage{}, i.errs[n]
	}
	return i.pages[min(n, len(i.pages)-1)], nil
}
func (i *sequenceAnswerIndex) Graph(context.Context, access.EffectivePolicy, string, derived.GraphOptions) (derived.GraphNeighborhood, error) {
	return i.graph, nil
}

type graphErrorIndex struct{ answerIndex }

func (g *graphErrorIndex) Graph(context.Context, access.EffectivePolicy, string, derived.GraphOptions) (derived.GraphNeighborhood, error) {
	return derived.GraphNeighborhood{}, errors.New("graph")
}

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
func TestAnswerAllowsProposerOnlyPrincipal(t *testing.T) {
	jobs, _ := jobsTest(t)
	jobs.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	handlers := modelHandlers()
	handlers["memory_answer"] = (&AnswerEndpoint{Jobs: jobs}).Call
	server := umcp.NewServer("answer-policy-order")
	if err := jobs.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard", AnswerEnabled: true}, Limits: endpointLimits(), ModelHandlers: handlers}); err != nil {
		t.Fatal(err)
	}
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_answer","arguments":{"question":"What is the target?"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
	raw, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	text := string(raw)
	if !strings.Contains(text, `"status":"success"`) || !strings.Contains(text, `"answer_source":"disabled"`) || strings.Contains(text, "forbidden") {
		t.Fatalf("response=%s", text)
	}
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
	// The bare test tool has no generated output schema. The proposer-only actor
	// reaches the endpoint, and the bare registration then rejects its envelope.
	if response.Error == nil {
		t.Fatal("expected output validation failure after endpoint completion")
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

func TestAnswerCallDeepError(t *testing.T) {
	jobs, _ := jobsTest(t)
	policy := jobs.Identity.authorization.Principals["actor"]
	policy.Roles = append(policy.Roles, "reader")
	jobs.Identity.authorization.Principals["actor"] = policy
	principal := jobs.Identity.names["actor"]
	principal.Roles = append(principal.Roles, "reader")
	jobs.Identity.names["actor"] = principal
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	e := AnswerEndpoint{Jobs: jobs, Client: &stubModelClient{}, Store: failingAnswers{}, Deep: deep}
	server := umcp.NewServer("x")
	_ = server.Tools.Register(umcp.Tool{Name: "a", Parameters: []umcp.Parameter{{Name: "question", Types: []umcp.ParamType{umcp.StringParam}}}, Call: e.Call})
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"a","arguments":{"question":"q"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
}

func TestAnswerCallPersistenceFailures(t *testing.T) {
	for _, stage := range []string{"get_exact", "trace", "put_exact", "get_hot", "put_hot"} {
		t.Run(stage, func(t *testing.T) {
			jobs, revision := jobsTest(t)
			policy := jobs.Identity.authorization.Principals["actor"]
			policy.Roles = append(policy.Roles, "reader")
			jobs.Identity.authorization.Principals["actor"] = policy
			principal := jobs.Identity.names["actor"]
			principal.Roles = append(principal.Roles, "reader")
			jobs.Identity.names["actor"] = principal
			jobs.Controls.Index = &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}}
			model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"answer","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
			deep := DefaultDeepAnswersConfig()
			deep.Enabled = true
			cache := DefaultExactAnswerCacheConfig()
			cache.Enabled = stage == "get_exact" || stage == "put_exact"
			hot := DefaultHotWorkingMemoryConfig()
			hot.Enabled = stage == "get_hot" || stage == "put_hot"
			e := AnswerEndpoint{Jobs: jobs, Client: model, Store: failingAnswers{stage}, Deep: deep, Cache: cache, Hot: hot}
			server := umcp.NewServer("x")
			_ = server.Tools.Register(umcp.Tool{Name: "a", Parameters: []umcp.Parameter{{Name: "question", Types: []umcp.ParamType{umcp.StringParam}}}, Call: e.Call})
			response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"a","arguments":{"question":"What target?"}}}`), umcp.RequestContext{Principal: "actor"})
			if err != nil || response == nil {
				t.Fatal(response, err)
			}
		})
	}
}

func TestAnswerCallDeepCacheEndToEnd(t *testing.T) {
	jobs, revision := jobsTest(t)
	policy := jobs.Identity.authorization.Principals["actor"]
	policy.Roles = append(policy.Roles, "reader")
	jobs.Identity.authorization.Principals["actor"] = policy
	principal := jobs.Identity.names["actor"]
	principal.Roles = append(principal.Roles, "reader")
	jobs.Identity.names["actor"] = principal
	jobs.Controls.Index = &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}}
	store := AnswerStore{DB: jobs.Controls.Queue.Proposals.DB}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"answer","confidence":"high","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	cache := DefaultExactAnswerCacheConfig()
	cache.Enabled = true
	e := AnswerEndpoint{Jobs: jobs, Client: model, Store: store, Deep: deep, Cache: cache}
	server := umcp.NewServer("answer")
	if err := server.Tools.Register(umcp.Tool{Name: "answer", Parameters: []umcp.Parameter{{Name: "question", Types: []umcp.ParamType{umcp.StringParam}}, {Name: "answer_mode", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: "summary"}}, Call: e.Call}); err != nil {
		t.Fatal(err)
	}
	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"answer","arguments":{"question":"What is the target?"}}}`)
	for i := 0; i < 2; i++ {
		response, err := server.Process(context.Background(), request, umcp.RequestContext{Principal: "actor"})
		if err != nil || response == nil {
			t.Fatal(response, err)
		}
	}
	if len(model.requests) != 1 {
		t.Fatalf("requests=%d", len(model.requests))
	}
	var caches, traces int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM answer_cache`).Scan(&caches); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM answer_traces`).Scan(&traces); err != nil {
		t.Fatal(err)
	}
	if caches != 1 || traces != 1 {
		t.Fatalf("cache=%d traces=%d", caches, traces)
	}
}

func TestAnswerCallHotUnknownFallsBackDeep(t *testing.T) {
	jobs, revision := jobsTest(t)
	policy := jobs.Identity.authorization.Principals["actor"]
	policy.Roles = append(policy.Roles, "reader")
	jobs.Identity.authorization.Principals["actor"] = policy
	principal := jobs.Identity.names["actor"]
	principal.Roles = append(principal.Roles, "reader")
	jobs.Identity.names["actor"] = principal
	jobs.Controls.Index = &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}}
	store := AnswerStore{DB: jobs.Controls.Queue.Proposals.DB}
	_ = store.Migrate(context.Background())
	scope := ScopeFingerprint("actor", policy.Roles, policy.ReadPrefixes, nil)
	_ = store.PutHotChanged(context.Background(), scope, []string{"12345678"}, 10)
	model := &sequenceModelClient{responses: []ModelResponse{{OutputText: `{"answer":"UNKNOWN","citations":[]}`}, {OutputText: `{"answer":"deep","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}}
	hot := DefaultHotWorkingMemoryConfig()
	hot.Enabled = true
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	e := AnswerEndpoint{Jobs: jobs, Client: model, Store: store, Hot: hot, Deep: deep}
	server := umcp.NewServer("x")
	_ = server.Tools.Register(umcp.Tool{Name: "a", Parameters: []umcp.Parameter{{Name: "question", Types: []umcp.ParamType{umcp.StringParam}}}, Call: e.Call})
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"a","arguments":{"question":"What target?"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil || model.calls != 2 {
		t.Fatalf("response=%#v err=%v calls=%d", response, err, model.calls)
	}
}

type sequenceModelClient struct {
	responses []ModelResponse
	calls     int
}

func (s *sequenceModelClient) Complete(context.Context, ModelRequest) (ModelResponse, error) {
	r := s.responses[s.calls]
	s.calls++
	return r, nil
}

func TestAnswerCallHotEndToEnd(t *testing.T) {
	jobs, revision := jobsTest(t)
	policy := jobs.Identity.authorization.Principals["actor"]
	policy.Roles = append(policy.Roles, "reader")
	jobs.Identity.authorization.Principals["actor"] = policy
	principal := jobs.Identity.names["actor"]
	principal.Roles = append(principal.Roles, "reader")
	jobs.Identity.names["actor"] = principal
	store := AnswerStore{DB: jobs.Controls.Queue.Proposals.DB}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.PutHotChanged(context.Background(), ScopeFingerprint("actor", policy.Roles, policy.ReadPrefixes, nil), []string{"12345678"}, 10); err != nil {
		t.Fatal(err)
	}
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"hot","confidence":"high","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	hot := DefaultHotWorkingMemoryConfig()
	hot.Enabled = true
	e := AnswerEndpoint{Jobs: jobs, Client: model, Store: store, Deep: DefaultDeepAnswersConfig(), Hot: hot}
	server := umcp.NewServer("answer")
	_ = server.Tools.Register(umcp.Tool{Name: "answer", Parameters: []umcp.Parameter{{Name: "question", Types: []umcp.ParamType{umcp.StringParam}}}, Call: e.Call})
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"answer","arguments":{"question":"What target changed?"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil || len(model.requests) != 1 {
		t.Fatalf("response=%#v err=%v requests=%d", response, err, len(model.requests))
	}
}

func TestAnswerCallRepositoryAndDeepErrors(t *testing.T) {
	jobs, _ := jobsTest(t)
	policy := jobs.Identity.authorization.Principals["actor"]
	policy.Roles = append(policy.Roles, "reader")
	jobs.Identity.authorization.Principals["actor"] = policy
	principal := jobs.Identity.names["actor"]
	principal.Roles = append(principal.Roles, "reader")
	jobs.Identity.names["actor"] = principal
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	e := AnswerEndpoint{Jobs: jobs, Client: &stubModelClient{}, Store: failingAnswers{}, Deep: deep}
	server := umcp.NewServer("x")
	_ = server.Tools.Register(umcp.Tool{Name: "a", Parameters: []umcp.Parameter{{Name: "question", Types: []umcp.ParamType{umcp.StringParam}}}, Call: e.Call})
	bad := jobs.Controls.Queue.Paths
	bad.BareDir = filepath.Join(t.TempDir(), "missing")
	jobs.Controls.Queue.Paths = bad
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"a","arguments":{"question":"q"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
}

func TestAnswerHotFailureBranches(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	hot := DefaultHotWorkingMemoryConfig()
	deep := DefaultDeepAnswersConfig()
	closed, _ := answerStoreTest(t)
	_ = closed.DB.Close()
	e := AnswerEndpoint{Store: closed, Hot: hot, Deep: deep, Client: &stubModelClient{}}
	if _, err := e.hot(context.Background(), controls, policy, "q", "q", "summary", "scope", ProfileQuestion("q"), revision); err == nil {
		t.Fatal("store")
	}
	store, _ := answerStoreTest(t)
	e.Store = store
	if record, err := e.hot(context.Background(), controls, policy, "q", "q", "summary", "scope", ProfileQuestion("q"), revision); err != nil || record != nil {
		t.Fatal(record, err)
	}
	if err := os.WriteFile(filepath.Join(controls.Queue.Paths.CurrentDir, "broken.md"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.PutHotChanged(context.Background(), "broken", []string{"12345678"}, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := e.hot(context.Background(), controls, policy, "q", "q", "summary", "broken", ProfileQuestion("q"), revision); err == nil {
		t.Fatal("bundle")
	}
	_ = os.Remove(filepath.Join(controls.Queue.Paths.CurrentDir, "broken.md"))
	if err := store.PutHotChanged(context.Background(), "scope", []string{"missing"}, 10); err != nil {
		t.Fatal(err)
	}
	if record, err := e.hot(context.Background(), controls, policy, "q", "q", "summary", "scope", ProfileQuestion("q"), revision); err != nil || record != nil {
		t.Fatal(record, err)
	}
	if err := store.PutHotChanged(context.Background(), "model", []string{"12345678"}, 10); err != nil {
		t.Fatal(err)
	}
	for name, model := range map[string]*stubModelClient{"provider": {err: errors.New("provider")}, "parse": {response: ModelResponse{OutputText: "bad"}}} {
		t.Run(name, func(t *testing.T) {
			e.Client = model
			if _, err := e.hot(context.Background(), controls, policy, "q", "q", "summary", "model", ProfileQuestion("q"), revision); err == nil {
				t.Fatal(name)
			}
		})
	}
}

func TestAnswerDeepEscalationAndGraphLimits(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	e := AnswerEndpoint{Client: &stubModelClient{}, Deep: deep}
	index := &sequenceAnswerIndex{pages: []derived.SearchPage{{RepoRevision: revision}}, errs: []error{nil, errors.New("second")}}
	controls.Index = index
	if _, err := e.deep(context.Background(), controls, policy, "q", "summary", "scope", ProfileQuestion("q")); err == nil {
		t.Fatal("second search")
	}
	index = &sequenceAnswerIndex{pages: []derived.SearchPage{{RepoRevision: revision}, {RepoRevision: revision, Results: []derived.SearchResult{{Path: "/missing.md"}}}}}
	controls.Index = index
	if _, err := e.deep(context.Background(), controls, policy, "q", "summary", "scope", ProfileQuestion("q")); err == nil {
		t.Fatal("second read")
	}
	deep.Limits.MaxConcepts = 1
	deep.Limits.MaxSteps = 1
	e.Deep = deep
	index = &sequenceAnswerIndex{pages: []derived.SearchPage{{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}}, graph: derived.GraphNeighborhood{Outbound: []derived.GraphEdge{{ConceptID: "x", Path: "/missing.md"}}}}
	controls.Index = index
	e.Client = &stubModelClient{response: ModelResponse{OutputText: `{"answer":"a","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	result, err := e.deep(context.Background(), controls, policy, "Which service is linked to target?", "summary", "scope", ProfileQuestion("Which service is linked to target?"))
	if err != nil || len(result.ReadConcepts) != 1 || len(result.Steps) != 1 {
		t.Fatal(result, err)
	}
}

func TestAnswerDeepFailureBranches(t *testing.T) {
	controls, _, _ := realApplyTest(t)
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	e := AnswerEndpoint{Client: &stubModelClient{}, Deep: deep}
	if _, err := e.deep(context.Background(), controls, policy, "q", "summary", "scope", ProfileQuestion("q")); err == nil {
		t.Fatal("missing index")
	}
	index := &answerIndex{err: errors.New("search")}
	controls.Index = index
	if _, err := e.deep(context.Background(), controls, policy, "q", "summary", "scope", ProfileQuestion("q")); err == nil {
		t.Fatal("search")
	}
	index.err = nil
	index.page = derived.SearchPage{RepoRevision: "rev", Results: []derived.SearchResult{{Path: "/missing.md", Status: "active"}}}
	if _, err := e.deep(context.Background(), controls, policy, "q", "summary", "scope", ProfileQuestion("q")); err == nil {
		t.Fatal("read")
	}
	index.page = derived.SearchPage{RepoRevision: "rev", Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}
	e.Client = &stubModelClient{err: errors.New("model")}
	if _, err := e.deep(context.Background(), controls, policy, "What target?", "summary", "scope", ProfileQuestion("What target?")); err == nil {
		t.Fatal("model")
	}
	e.Client = &stubModelClient{response: ModelResponse{OutputText: "bad"}}
	if _, err := e.deep(context.Background(), controls, policy, "What target?", "summary", "scope", ProfileQuestion("What target?")); err == nil {
		t.Fatal("parse")
	}
	index.err = errors.New("graph")
	if _, err := e.deep(context.Background(), controls, policy, "Which service is linked?", "summary", "scope", ProfileQuestion("Which service is linked?")); err == nil {
		t.Fatal("graph")
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
func TestAnswerDeepFilteringLimitsAndEscalation(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/b.md", "b", "B", "beta target")
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/c.md", "c", "C", "gamma target")
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "skip-secret", Path: "/a.md", Tags: []string{"secret"}, Status: "active"}, {ConceptID: "skip-namespace", Path: "/a.md", Status: "active"}, {ConceptID: "skip-current", Path: "/a.md", Status: "deprecated"}, {ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}, {ConceptID: "b", Path: "/b.md", Title: "B", Status: "active"}, {ConceptID: "c", Path: "/c.md", Title: "C", Status: "active"}}}}
	controls.Index = index
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	deep.Limits.MaxConcepts = 2
	deep.Limits.MaxSteps = 1
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"a","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	e := AnswerEndpoint{Client: model, Deep: deep}
	personal := "personal"
	profile := QueryProfile{TemporalIntent: "current", NamespaceHint: &personal, Terms: []string{"target"}}
	result, err := e.deep(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "recent target", "summary", "scope", profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ReadConcepts) != 0 || len(result.Steps) != 1 {
		t.Fatalf("result=%#v", result)
	}
	profile.NamespaceHint = nil
	result, err = e.deep(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "changed target", "summary", "scope", profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ReadConcepts) != 2 || !strings.Contains(model.requests[len(model.requests)-1].Prompt, "QUESTION: changed target") {
		t.Fatalf("result=%#v", result)
	}
}

func TestAnswerDeepSemanticAndGraphFiltering(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/b.md", "neighbor", "Neighbour", "linked target")
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/c.md", "secret", "Secret", "linked target")
	index := &semanticAnswerIndex{semantic: derived.SearchPage{RepoRevision: revision, Warnings: []string{"fallback"}, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}, answerIndex: answerIndex{graph: derived.GraphNeighborhood{Outbound: []derived.GraphEdge{{ConceptID: "12345678", Path: "/a.md"}, {ConceptID: "missing", Path: "/missing.md"}}}}}
	controls.Index = index
	controls.SemanticClient = noopSemanticClient{}
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"a","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	e := AnswerEndpoint{Client: model, Deep: deep}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	if _, err := e.deep(context.Background(), controls, policy, "Which service is linked?", "summary", "scope", ProfileQuestion("Which service is linked?")); err == nil {
		t.Fatal("missing graph neighbor")
	}
	index.graph = derived.GraphNeighborhood{Outbound: []derived.GraphEdge{{ConceptID: "12345678", Path: "/a.md"}, {ConceptID: "neighbor", Path: "/b.md"}, {ConceptID: "secret", Path: "/c.md"}}}
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/c.md", "secret", "Secret", "linked target")
	entry, _ := repository.ReadBundleEntry(controls.Queue.Paths.CurrentDir, "/c.md")
	entry.Document.Frontmatter.Tags = []string{"secret"}
	raw, _ := repository.SerializeConcept(entry.Document)
	_ = os.WriteFile(filepath.Join(controls.Queue.Paths.CurrentDir, "c.md"), []byte(raw), 0600)
	result, err := e.deep(context.Background(), controls, policy, "Which service is linked?", "summary", "scope", ProfileQuestion("Which service is linked?"))
	if err != nil {
		t.Fatal(err)
	}
	strategy := result.Record.Evidence.(map[string]any)["retrieval_strategy"].(string)
	if !strings.Contains(strategy, "lexical_fallback") || len(result.ReadConcepts) != 2 {
		t.Fatalf("strategy=%s concepts=%#v", strategy, result.ReadConcepts)
	}
	index.semanticErr = errors.New("semantic")
	if _, err = e.deep(context.Background(), controls, policy, "q", "summary", "scope", ProfileQuestion("q")); err == nil {
		t.Fatal("semantic")
	}
}

func TestAnswerDeepRemainingGraphBranches(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/b.md", "b", "B", "linked target")
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/c.md", "c", "C", "linked target")
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}, {ConceptID: "b", Path: "/b.md", Title: "B", Status: "active"}, {ConceptID: "c", Path: "/c.md", Title: "C", Status: "active"}}}, graph: derived.GraphNeighborhood{Outbound: []derived.GraphEdge{{ConceptID: "12345678", Path: "/a.md"}, {ConceptID: "b", Path: "/b.md"}, {ConceptID: "c", Path: "/c.md"}}}}
	controls.Index = index
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	deep.Limits.MaxConcepts = 2
	deep.Limits.MaxSteps = 2
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"a","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	e := AnswerEndpoint{Client: model, Deep: deep}
	profile := ProfileQuestion("Which service is linked to changed target?")
	result, err := e.deep(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "Which service is linked to changed target?", "summary", "scope", profile)
	if err != nil || len(result.ReadConcepts) != 2 || len(result.Steps) != 2 {
		t.Fatal(result, err)
	}
	// Make b superseded by the anchor and verify non-historical graph filtering.
	entry, _ := repository.ReadBundleEntry(controls.Queue.Paths.CurrentDir, "/a.md")
	entry.Document.Frontmatter.Supersedes = []string{"b"}
	raw, _ := repository.SerializeConcept(entry.Document)
	_ = os.WriteFile(filepath.Join(controls.Queue.Paths.CurrentDir, "a.md"), []byte(raw), 0600)
	deep.Limits.MaxConcepts = 3
	e.Deep = deep
	result, err = e.deep(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "Which service is linked to target?", "summary", "scope", ProfileQuestion("Which service is linked to target?"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range result.ReadConcepts {
		if c.ID == "b" {
			t.Fatal("superseded b retained")
		}
	}
}

func TestAnswerDeepThreeAnchorsAndGraphError(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/b.md", "b", "B", "linked target")
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/c.md", "c", "C", "linked target")
	page := derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}, {ConceptID: "b", Path: "/b.md", Title: "B", Status: "active"}, {ConceptID: "c", Path: "/c.md", Title: "C", Status: "active"}}}
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	deep.Limits.MaxConcepts = 5
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"a","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	e := AnswerEndpoint{Client: model, Deep: deep}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	controls.Index = &answerIndex{page: page}
	if _, err := e.deep(context.Background(), controls, policy, "Which service is linked to target?", "summary", "scope", ProfileQuestion("Which service is linked to target?")); err != nil {
		t.Fatal(err)
	}
	controls.Index = &graphErrorIndex{answerIndex: answerIndex{page: page}}
	if _, err := e.deep(context.Background(), controls, policy, "Which service is linked to target?", "summary", "scope", ProfileQuestion("Which service is linked to target?")); err == nil {
		t.Fatal("graph")
	}
}

func TestAnswerDeepGraphSeenAndCapacity(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/b.md", "b", "B", "linked target")
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/c.md", "c", "C", "linked target")
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}, graph: derived.GraphNeighborhood{Outbound: []derived.GraphEdge{{ConceptID: "12345678", Path: "/a.md"}, {ConceptID: "b", Path: "/b.md"}, {ConceptID: "c", Path: "/c.md"}}}}
	controls.Index = index
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	deep.Limits.MaxConcepts = 2
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"b","citations":[{"id":"b","path":"/b.md","revision":"` + revision + `"}]}`}}
	e := AnswerEndpoint{Client: model, Deep: deep}
	result, err := e.deep(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "Which service is linked to target?", "summary", "scope", ProfileQuestion("Which service is linked to target?"))
	if err != nil || len(result.ReadConcepts) != 2 {
		t.Fatal(result, err)
	}
}

func TestAnswerDeepGraphLoopBranches(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/b.md", "b", "B", "linked target")
	writeDreamConcept(t, controls.Queue.Paths.CurrentDir, "/c.md", "c", "C", "linked target")
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}, graph: derived.GraphNeighborhood{Outbound: []derived.GraphEdge{{ConceptID: "12345678", Path: "/a.md"}, {ConceptID: "b", Path: "/b.md"}, {ConceptID: "c", Path: "/c.md"}}}}
	controls.Index = index
	deep := DefaultDeepAnswersConfig()
	deep.Enabled = true
	deep.Limits.MaxConcepts = 2
	deep.Limits.MaxSteps = 3
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"b","citations":[{"id":"b","path":"/b.md","revision":"` + revision + `"}]}`}}
	e := AnswerEndpoint{Client: model, Deep: deep}
	result, err := e.deep(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "Which service is linked to target?", "summary", "scope", ProfileQuestion("Which service is linked to target?"))
	if err != nil || len(result.ReadConcepts) != 2 || len(result.Steps) != 3 {
		t.Fatal(result, err)
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

func TestAnswerHotIneligibleConcept(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	store, _ := answerStoreTest(t)
	_ = store.PutHotChanged(context.Background(), "scope", []string{"12345678"}, 10)
	hot := DefaultHotWorkingMemoryConfig()
	e := AnswerEndpoint{Store: store, Hot: hot, Deep: DefaultDeepAnswersConfig(), Client: &stubModelClient{}}
	profile := QueryProfile{TemporalIntent: "current"}
	entry, _ := repository.ReadBundleEntry(controls.Queue.Paths.CurrentDir, "/a.md")
	entry.Document.Frontmatter.Status = "deprecated"
	raw, _ := repository.SerializeConcept(entry.Document)
	_ = os.WriteFile(filepath.Join(controls.Queue.Paths.CurrentDir, "a.md"), []byte(raw), 0600)
	record, err := e.hot(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, "q", "q", "summary", "scope", profile, revision)
	if err != nil || record != nil {
		t.Fatal(record, err)
	}
}

func TestAnswerHotDuplicateCapAndPromptBound(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	store, _ := answerStoreTest(t)
	scope := "scope"
	_ = store.PutHotChanged(context.Background(), scope, []string{"12345678", "12345678"}, 10)
	hot := DefaultHotWorkingMemoryConfig()
	hot.MaxChangedConcepts = 1
	hot.MaxExcerptChars = 128
	deep := DefaultDeepAnswersConfig()
	model := &stubModelClient{response: ModelResponse{OutputText: `{"answer":"a","citations":[{"id":"12345678","path":"/a.md","revision":"` + revision + `"}]}`}}
	e := AnswerEndpoint{Client: model, Store: store, Hot: hot, Deep: deep}
	record, err := e.hot(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, strings.Repeat("long ", 100), "q", "summary", scope, ProfileQuestion("q"), revision)
	if err != nil || record == nil || len([]rune(model.requests[0].Prompt)) != 128 {
		t.Fatal(record, err, len([]rune(model.requests[0].Prompt)))
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
