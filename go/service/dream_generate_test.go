package service

import (
	"context"
	"errors"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stubModelClient struct {
	response ModelResponse
	err      error
	requests []ModelRequest
}

func (s *stubModelClient) Complete(_ context.Context, r ModelRequest) (ModelResponse, error) {
	s.requests = append(s.requests, r)
	return s.response, s.err
}
func writeDreamConcept(t *testing.T, root, path, id, title, body string) {
	t.Helper()
	stamp := time.Unix(1, 0).UTC()
	doc := repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{SchemaVersion: 1, ID: id, Type: "concept", Title: title, Status: "active", Aliases: []string{}, Tags: []string{}, SourceRefs: []string{}, Supersedes: []string{}, CreatedAt: stamp, UpdatedAt: stamp, UpdatedBy: "test"}, Body: body}
	raw, err := repository.SerializeConcept(doc)
	if err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, strings.TrimPrefix(path, "/"))
	if err = os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(full, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
}
func configuredDreamRuntime(t *testing.T) (context.Context, *Runtime, string) {
	t.Helper()
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = t.TempDir()
	dream := DefaultDreamConfig()
	dream.Mode = "propose"
	dream.QuietPeriodSeconds = 0
	proposals := DefaultModelProposalsConfig()
	runtime, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}, Dream: dream, ModelProposals: proposals})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, runtime, revision
}
func TestModelProposalConfig(t *testing.T) {
	defaults, err := DecodeModelProposalsConfig(nil)
	if err != nil || defaults.Limits.MaxOutputChars != 8000 {
		t.Fatal(defaults, err)
	}
	good, err := DecodeModelProposalsConfig([]byte(`{"enabled":true,"limits":{"max_output_chars":300}}`))
	if err != nil || !good.Enabled || good.Limits.MaxOutputChars != 300 {
		t.Fatal(good, err)
	}
	bad := []string{`{} {}`, `{"x":1}`, `{"prompt_version":""}`, `{"limits":{"max_search_results":0}}`, `{"limits":{"max_consulted_concepts":11}}`, `{"limits":{"max_context_chars":1}}`, `{"limits":{"max_output_chars":1}}`, `{"limits":{"max_diff_chars":0}}`, `{"limits":{"max_changes":101}}`, `{"limits":{"max_body_chars":0}}`, `{"limits":{"max_rationale_chars":0}}`, `{"limits":{"max_secret_entropy_chars":7}}`}
	for _, raw := range bad {
		if _, err := DecodeModelProposalsConfig([]byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
}
func TestDreamPromptBoundedDeterministic(t *testing.T) {
	root := t.TempDir()
	writeDreamConcept(t, root, "/b.md", "b", "B", strings.Repeat("b", 30))
	writeDreamConcept(t, root, "/a.md", "a", "A", "alpha")
	signals := []control.DreamSignal{{SignalType: "orphan", DedupeKey: "k", EntityRefs: []string{"/b.md", "/a.md", "/none.md"}, EvidenceJSON: `{"x":1}`}}
	prompt, citations, err := DreamPrompt(root, signals, "rev", 1, 4)
	if err != nil || len(citations) != 1 || citations[0].Path != "/a.md" || !strings.Contains(prompt, "UNTRUSTED_SIGNAL_BEGIN") || !strings.Contains(prompt, "BODY:\nalph") || strings.Contains(prompt, "/b.md\nREVISION") {
		t.Fatal(prompt, citations, err)
	}
}
func TestDreamGenerateProposalAndBudgets(t *testing.T) {
	ctx, runtime, revision := configuredDreamRuntime(t)
	defer runtime.Close(ctx)
	writeDreamConcept(t, runtime.Paths.Repository.CurrentDir, "/a.md", "a", "A", "body")
	signal := control.DreamSignal{DedupeKey: "key", SignalType: "orphan", EntityRefs: []string{"a", "/a.md"}, EvidenceJSON: `{}`}
	raw := `{"intent":"maintain","rationale":"why","consulted_concepts":[{"id":"a","path":"/a.md","revision":"` + revision + `","title":"A"}],"contradictions":[],"reciprocal_links":[],"changes":[{"kind":"patch","path":"/a.md","body":"updated"}]}`
	client := &stubModelClient{response: ModelResponse{ModelName: "m", OutputText: raw}}
	runtime.ModelClient = client
	count, chain, err := runtime.generateDreamProposal(ctx, []control.DreamSignal{signal}, revision, time.Second, time.Unix(100, 0))
	if err != nil || count != 1 || len(chain) != 1 || chain[0].Model != "m" || len(client.requests) != 1 || client.requests[0].Metadata["prompt_version"] != "v1" {
		t.Fatal(count, chain, client.requests, err)
	}
	records, err := (control.Proposals{DB: runtime.DB}).List(ctx, control.ProposalQuery{})
	if err != nil || len(records) != 1 {
		t.Fatal(records, err)
	}
	runtime.Dream.Budgets.MaxModelProposalsPerRun = 0
	if count, _, err = runtime.generateDreamProposal(ctx, nil, revision, time.Second, time.Unix(100, 0)); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	runtime.Dream.Budgets.MaxModelProposalsPerRun = 5
	runtime.Dream.Budgets.DailyProposalLimit = 0
	if count, _, err = runtime.generateDreamProposal(ctx, nil, revision, time.Second, time.Unix(100, 0)); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
func TestDreamGenerateFailures(t *testing.T) {
	ctx, runtime, revision := configuredDreamRuntime(t)
	defer runtime.Close(ctx)
	runtime.ModelClient = &stubModelClient{err: errors.New("provider")}
	if _, _, err := runtime.generateDreamProposal(ctx, nil, revision, time.Second, time.Now()); err == nil {
		t.Fatal("provider")
	}
	runtime.ModelClient = &stubModelClient{response: ModelResponse{OutputText: "{"}}
	if _, _, err := runtime.generateDreamProposal(ctx, nil, revision, time.Second, time.Now()); err == nil {
		t.Fatal("parse")
	}
	tooLong := strings.Repeat("x", runtime.ModelProposals.Limits.MaxRationaleChars+1)
	runtime.ModelClient = &stubModelClient{response: ModelResponse{OutputText: `{"rationale":"` + tooLong + `","changes":[{"kind":"create","path":"/a.md","concept_type":"concept","title":"A","body":"b"}]}`}}
	if _, _, err := runtime.generateDreamProposal(ctx, nil, revision, time.Second, time.Now()); err == nil {
		t.Fatal("rationale")
	}
	if err := validateDreamCitations([]map[string]any{{"id": "x", "path": "/x", "revision": "r", "title": "X"}}, nil); err == nil {
		t.Fatal("uncited")
	}
	expected := []DreamCitation{{"x", "/x", "r", "X"}}
	if err := validateDreamCitations([]map[string]any{{"id": "y", "path": "/x", "revision": "r", "title": "X"}}, expected); err == nil {
		t.Fatal("unknown")
	}
	if err := validateDreamCitations(nil, expected); err == nil {
		t.Fatal("missing")
	}
	if err := validateDreamCitations([]map[string]any{{"id": "x", "path": "/bad", "revision": "r", "title": "X"}}, expected); err == nil {
		t.Fatal("mismatch")
	}
	writeDreamConcept(t, runtime.Paths.Repository.CurrentDir, "/a.md", "a", "A", "body")
	citation := `[{"id":"a","path":"/a.md","revision":"` + revision + `","title":"A"}]`
	runtime.ModelClient = &stubModelClient{response: ModelResponse{OutputText: `{"rationale":"x","consulted_concepts":[{"id":"a","path":"/wrong","revision":"` + revision + `","title":"A"}],"changes":[{"kind":"patch","path":"/a.md","body":"x"}]}`}}
	if _, _, err := runtime.generateDreamProposal(ctx, []control.DreamSignal{{EntityRefs: []string{"/a.md"}}}, revision, time.Second, time.Now()); err == nil {
		t.Fatal("citation integration")
	}
	runtime.ModelClient = &stubModelClient{response: ModelResponse{OutputText: `{"rationale":"x","consulted_concepts":` + citation + `,"changes":[{"kind":"patch","path":"relative","body":"x"}]}`}}
	if _, _, err := runtime.generateDreamProposal(ctx, []control.DreamSignal{{EntityRefs: []string{"/a.md"}}}, revision, time.Second, time.Now()); err == nil {
		t.Fatal("validation")
	}
	runtime.ModelClient = &stubModelClient{response: ModelResponse{OutputText: `{"rationale":"x","consulted_concepts":` + citation + `,"changes":[{"kind":"patch","path":"/a.md","body":"ghp_12345678901234567890"}]}`}}
	if _, _, err := runtime.generateDreamProposal(ctx, []control.DreamSignal{{EntityRefs: []string{"/a.md"}}}, revision, time.Second, time.Now()); err == nil {
		t.Fatal("secret")
	}
	if _, err := runtime.DB.Exec(`CREATE TRIGGER fail_proposal BEFORE INSERT ON proposals BEGIN SELECT RAISE(FAIL,'proposal'); END`); err != nil {
		t.Fatal(err)
	}
	runtime.ModelClient = &stubModelClient{response: ModelResponse{OutputText: `{"rationale":"x","consulted_concepts":` + citation + `,"changes":[{"kind":"patch","path":"/a.md","body":"safe"}]}`}}
	if _, _, err := runtime.generateDreamProposal(ctx, []control.DreamSignal{{EntityRefs: []string{"/a.md"}}}, revision, time.Second, time.Now()); err == nil {
		t.Fatal("storage")
	}
}
func TestDreamRunProposeOrchestration(t *testing.T) {
	ctx, runtime, revision := configuredDreamRuntime(t)
	defer runtime.Close(ctx)
	writeDreamConcept(t, runtime.Paths.Repository.CurrentDir, "/a.md", "a", "A", "body")
	raw := `{"rationale":"why","consulted_concepts":[{"id":"a","path":"/a.md","revision":"` + revision + `","title":"A"}],"changes":[{"kind":"patch","path":"/a.md","body":"updated"}]}`
	runtime.ModelClient = &stubModelClient{response: ModelResponse{ModelName: "m", OutputText: raw, ModelChain: []control.ModelAttempt{{Model: "m", Outcome: "success"}}}}
	ops := dreamOps()
	ops.revision = func(repository.GitRepositoryPaths) (string, error) { return revision, nil }
	ops.scan = func(string, repository.BundleFilter) (repository.RepositoryBundle, error) {
		return repository.ScanBundle(runtime.Paths.Repository.CurrentDir, repository.BundleFilter{})
	}
	ops.actionable = func(context.Context) ([]control.DreamSignal, error) {
		return []control.DreamSignal{{DedupeKey: "key", SignalType: "orphan", EntityRefs: []string{"/a.md"}, EvidenceJSON: `{}`}}, nil
	}
	var proposals int
	var chain []control.ModelAttempt
	ops.finish = func(_ context.Context, _ string, state string, _ *string, _ int, p int, c []control.ModelAttempt, _ *string) (control.SchedulerRunRecord, error) {
		if state == "succeeded" {
			proposals = p
			chain = c
		}
		return control.SchedulerRunRecord{}, nil
	}
	payload, err := runtime.runDream(ctx, "propose", time.Unix(21600, 0), ops)
	if err != nil || payload["proposal_count"] != 1 || proposals != 1 || len(chain) != 1 {
		t.Fatal(payload, proposals, chain, err)
	}
}

func TestDreamGenerateInfrastructureFailures(t *testing.T) {
	ctx, runtime, revision := configuredDreamRuntime(t)
	rootFile := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootFile, "bad.md"), []byte("not frontmatter"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := DreamPrompt(rootFile, nil, "r", 1, 10); err == nil {
		t.Fatal("prompt")
	}
	if err := os.WriteFile(filepath.Join(runtime.Paths.Repository.CurrentDir, "bad.md"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	runtime.ModelClient = &stubModelClient{}
	if _, _, err := runtime.generateDreamProposal(ctx, nil, revision, time.Second, time.Now()); err == nil {
		t.Fatal("generation prompt")
	}
	if err := os.Remove(filepath.Join(runtime.Paths.Repository.CurrentDir, "bad.md")); err != nil {
		t.Fatal(err)
	}
	runtime.ModelClient = &stubModelClient{}
	if err := runtime.DB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runtime.generateDreamProposal(ctx, nil, revision, time.Second, time.Now()); err == nil {
		t.Fatal("daily")
	}
	runtime.DB = nil
	_ = runtime.Close(ctx)
}

func TestDreamDailyProposalCount(t *testing.T) {
	ctx, runtime, _ := configuredDreamRuntime(t)
	defer runtime.Close(ctx)
	now := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	_, err := runtime.DB.ExecContext(ctx, "INSERT INTO scheduler_runs(run_id,job_name,window_key,state,proposal_count,started_at) VALUES('a','dream','a','succeeded',2,?),('b','dream','b','failed',9,?),('c','other','c','succeeded',9,?)", now.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	count, err := DreamDailyProposalCount(ctx, runtime.DB, now)
	if err != nil || count != 2 {
		t.Fatal(count, err)
	}
}
