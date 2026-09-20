package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/umcp"
)

func proposalTestLimits() ModelProposalLimitsConfig {
	return ModelProposalLimitsConfig{MaxSearchResults: 5, MaxConsultedConcepts: 6, MaxContextChars: 64, MaxOutputChars: 8000, MaxDiffChars: 2000000, MaxChanges: 20, MaxBodyChars: 32000, MaxRationaleChars: 4000, MaxSecretEntropyChars: 32}
}

func proposalTestDraft(revision string) DreamProposalDraft {
	return DreamProposalDraft{
		Intent: "improve", Rationale: "because",
		Consulted:      []map[string]any{{"id": "12345678", "path": "/a.md", "revision": revision, "title": "Title"}},
		Contradictions: []map[string]any{}, ReciprocalLinks: []map[string]any{},
		Changes: []map[string]any{{"kind": "patch", "path": "/a.md", "body": "changed"}},
	}
}

func TestModelProposalAuthorizationPrecedesDisabledState(t *testing.T) {
	jobs, _ := jobsTest(t)
	policy := jobs.Identity.authorization.Principals["actor"]
	policy.Roles = []string{"reader"}
	jobs.Identity.authorization.Principals["actor"] = policy
	principal := jobs.Identity.names["actor"]
	principal.Roles = []string{"reader"}
	jobs.Identity.names["actor"] = principal
	endpoint := &ModelProposalEndpoint{Jobs: jobs}
	jobs.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	handlers := modelHandlers()
	handlers["memory_propose_freeform"] = endpoint.Freeform
	server := umcp.NewServer("model-proposal-policy-order")
	if err := jobs.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard"}, Limits: endpointLimits(), ModelHandlers: handlers}); err != nil {
		t.Fatal(err)
	}
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_propose_freeform","arguments":{"content":"x"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
	raw, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	text := string(raw)
	if !strings.Contains(text, "forbidden") || strings.Contains(text, "model-assisted proposals are disabled") {
		t.Fatalf("response=%s", text)
	}
}

func TestModelProposalArgumentsAndDisabled(t *testing.T) {
	e := &ModelProposalEndpoint{}
	if _, err := e.Freeform(context.Background(), map[string]any{}); err == nil || err.Error() != "content must be a string" {
		t.Fatalf("freeform error = %v", err)
	}
	if _, err := e.Update(context.Background(), map[string]any{}); err == nil || err.Error() != "instruction must be a string" {
		t.Fatalf("instruction error = %v", err)
	}
	if _, err := e.Freeform(context.Background(), map[string]any{"content": "x", "intent": 1}); err == nil || err.Error() != "intent must be a string or null" {
		t.Fatalf("intent error = %v", err)
	}
	if _, err := e.Freeform(context.Background(), map[string]any{"content": "x", "suggested_path": 1}); err == nil || err.Error() != "suggested_path must be a string or null" {
		t.Fatalf("suggested error = %v", err)
	}
	if _, err := e.Update(context.Background(), map[string]any{"instruction": "x", "target_hint": 1}); err == nil || err.Error() != "target_hint must be a string or null" {
		t.Fatalf("update error = %v", err)
	}
	if _, err := e.Freeform(context.Background(), map[string]any{"content": "x"}); err == nil || err.Error() != "model-assisted proposals are disabled" {
		t.Fatalf("disabled error = %v", err)
	}
}

func TestModelProposalPromptBoundsUntrustedContent(t *testing.T) {
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/public/"}, WritePrefixes: []string{"/public/"}}
	prompt := modelProposalPrompt("TASK", []consultedConcept{{"id", "/public/a.md", "rev", "Title", "0123456789"}}, policy, 96)
	for _, want := range []string{"AUTHORIZED_WRITE_PREFIXES: /public/", "UNTRUSTED_CONCEPT_BEGIN", "01234", "create(path, concept_type, title, body, description?, tags?, aliases?)", "file a GitHub issue instead of inventing a workaround"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
	if strings.Contains(prompt, "0123456789\nUNTRUSTED_CONCEPT_END") {
		t.Fatalf("context was not bounded: %s", prompt)
	}
}

func TestValidateModelProposalDraft(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	policy := access.EffectivePolicy{Principal: "actor", Roles: []string{"proposer"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}
	consulted := []consultedConcept{{"12345678", "/a.md", revision, "Title", "body"}}
	limits := proposalTestLimits()
	if err := validateModelProposalDraft(controls.Queue.Paths.CurrentDir, policy, consulted, proposalTestDraft(revision), limits); err != nil {
		t.Fatal(err)
	}
	if err := validateModelProposalDraft(filepath.Join(t.TempDir(), "missing"), policy, consulted, proposalTestDraft(revision), limits); err == nil {
		t.Fatal("repository root")
	}
	cases := []struct {
		name     string
		mutate   func(*DreamProposalDraft)
		contains string
	}{
		{"empty rationale", func(d *DreamProposalDraft) { d.Rationale = "" }, "rationale"},
		{"empty changes", func(d *DreamProposalDraft) { d.Changes = nil }, "at least one"},
		{"too many changes", func(d *DreamProposalDraft) { limits.MaxChanges = 0 }, "change limits"},
		{"missing citation", func(d *DreamProposalDraft) { d.Consulted = nil }, "cite every"},
		{"unknown citation", func(d *DreamProposalDraft) { d.Consulted[0]["id"] = "other" }, "unconsulted"},
		{"wrong citation path", func(d *DreamProposalDraft) { d.Consulted[0]["path"] = "/other.md" }, "citations must match"},
		{"archive", func(d *DreamProposalDraft) { d.Changes[0]["kind"] = "trash" }, "may not rename or archive"},
		{"write acl", func(d *DreamProposalDraft) { policy.WritePrefixes = []string{"/allowed/"} }, "cannot write"},
		{"invalid path", func(d *DreamProposalDraft) { d.Changes[0]["path"] = "relative.md" }, "absolute"},
		{"repository path", func(d *DreamProposalDraft) {
			d.Changes = []map[string]any{{"kind": "create", "path": "/x\x00.md", "concept_type": "concept", "title": "X", "body": "body", "description": nil, "tags": []string{}, "aliases": []string{}}}
		}, "control characters"},
		{"body limit", func(d *DreamProposalDraft) { limits.MaxBodyChars = 1 }, "body exceeds"},
		{"diff limit", func(d *DreamProposalDraft) { limits.MaxDiffChars = 1 }, "diff exceeds"},
		{"preview failure", func(d *DreamProposalDraft) { d.Changes[0]["path"] = "/missing.md"; d.Changes[0]["kind"] = "patch" }, "does not exist"},
		{"secret", func(d *DreamProposalDraft) { d.Changes[0]["body"] = "ghp_12345678901234567890" }, "secret"},
		{"reciprocal acl", func(d *DreamProposalDraft) {
			d.ReciprocalLinks = []map[string]any{{"source_path": "/denied/a.md", "target_path": "/a.md", "justification": "x"}}
		}, "cannot write"},
		{"reciprocal read acl", func(d *DreamProposalDraft) {
			policy.WritePrefixes = []string{"/"}
			policy.ReadPrefixes = []string{"/a.md"}
			d.ReciprocalLinks = []map[string]any{{"source_path": "/a.md", "target_path": "/denied.md", "justification": "x"}}
		}, "cannot read"},
	}
	accepted := proposalTestDraft(revision)
	accepted.Consulted[0]["title"] = "Model-supplied title is not authoritative"
	if err := validateModelProposalDraft(controls.Queue.Paths.CurrentDir, policy, consulted, accepted, limits); err != nil {
		t.Fatalf("Python-compatible citation title must be accepted: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy = access.EffectivePolicy{Principal: "actor", Roles: []string{"proposer"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}
			consulted = []consultedConcept{{"12345678", "/a.md", revision, "Title", "body"}}
			limits = proposalTestLimits()
			d := proposalTestDraft(revision)
			if tc.name == "reciprocal acl" {
				policy.WritePrefixes = []string{"/a.md"}
			}
			tc.mutate(&d)
			err := validateModelProposalDraft(controls.Queue.Paths.CurrentDir, policy, consulted, d, limits)
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestModelProposalFreeformIntentOverride(t *testing.T) {
	jobs, revision := jobsTest(t)
	model := &stubModelClient{response: ModelResponse{OutputText: fmt.Sprintf(`{"intent":"model","rationale":"because","consulted_concepts":[{"id":"12345678","path":"/a.md","revision":%q,"title":"Title"}],"changes":[{"kind":"patch","path":"/a.md","body":"changed"}]}`, revision)}}
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	e := ModelProposalEndpoint{Jobs: jobs, Client: model, Config: config}
	server := umcp.NewServer("x")
	_ = server.Tools.Register(umcp.Tool{Name: "p", Parameters: []umcp.Parameter{{Name: "content", Types: []umcp.ParamType{umcp.StringParam}}, {Name: "suggested_path", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: nil}, {Name: "intent", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: nil}}, Call: e.Freeform})
	_, _ = server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"p","arguments":{"content":"x","suggested_path":"/a.md","intent":"override"}}}`), umcp.RequestContext{Principal: "actor"})
	rows := tableRows(t, jobs.Controls.Queue.Proposals.DB, `SELECT intent FROM proposals WHERE intent='override'`)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
}

func TestModelProposalEndpointStoresAuthenticatedProposal(t *testing.T) {
	jobs, revision := jobsTest(t)
	model := &stubModelClient{response: ModelResponse{OutputText: fmt.Sprintf(`{"intent":"model intent","rationale":"because","consulted_concepts":[{"id":"12345678","path":"/a.md","revision":%q,"title":"Title"}],"contradictions":[{"path":"/a.md","summary":"stale"}],"reciprocal_links":[{"source_path":"/a.md","target_path":"/a.md","justification":"self reference"}],"changes":[{"kind":"patch","path":"/a.md","body":"model changed"}]}`, revision)}}
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	endpoint := &ModelProposalEndpoint{Jobs: jobs, Client: model, Config: config}
	server := umcp.NewServer("test")
	if err := server.Tools.Register(umcp.Tool{Name: "model", Parameters: []umcp.Parameter{{Name: "instruction", Types: []umcp.ParamType{umcp.StringParam}}, {Name: "target_hint", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: nil}}, Call: endpoint.Update}); err != nil {
		t.Fatal(err)
	}
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"model","arguments":{"instruction":"improve it","target_hint":"/a.md"}}}`
	response, err := server.Process(context.Background(), []byte(request), umcp.RequestContext{Principal: "actor", SessionID: "session"})
	if err != nil || response == nil {
		t.Fatalf("response=%#v err=%v", response, err)
	}
	// The bare test tool has no generated output schema/envelope adapter; its
	// transport-level output validation may fail after the authenticated handler
	// has completed. The durable record below is the endpoint assertion.
	if len(model.requests) != 1 {
		t.Fatalf("model requests = %d", len(model.requests))
	}
	got := model.requests[0]
	if got.Task != "memory_proposal_draft" || got.SlotName != "proposal" || got.DataClassification != "restricted" || got.Metadata["tool_version"] != config.ToolVersion || got.Metadata["prompt_version"] != config.PromptVersion || got.Metadata["model_policy_revision"] != config.ModelPolicyRevision || !strings.Contains(got.Prompt, "UNTRUSTED_INPUT_BEGIN\nimprove it") {
		t.Fatalf("request = %#v", got)
	}
	rows := tableRows(t, jobs.Controls.Queue.Proposals.DB, `SELECT author_principal,intent,base_revision,status,patch_json FROM proposals WHERE intent='model intent'`)
	if len(rows) != 1 || rows[0]["author_principal"] != "actor" || rows[0]["base_revision"] != revision || rows[0]["status"] != "submitted" {
		t.Fatalf("stored = %#v", rows)
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(rows[0]["patch_json"].(string)), &patch); err != nil {
		t.Fatal(err)
	}
	if patch["target_hint"] != "/a.md" || len(patch["consulted_concepts"].([]any)) != 1 || len(patch["contradictions"].([]any)) != 1 || len(patch["reciprocal_links"].([]any)) != 1 {
		t.Fatalf("stored model evidence = %#v", patch)
	}
}

func TestModelProposalCallContextError(t *testing.T) {
	jobs, _ := jobsTest(t)
	jobs.Controls.Index = &answerIndex{err: errors.New("search")}
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	e := ModelProposalEndpoint{Jobs: jobs, Client: &stubModelClient{}, Config: config}
	server := umcp.NewServer("x")
	_ = server.Tools.Register(umcp.Tool{Name: "p", Parameters: []umcp.Parameter{{Name: "instruction", Types: []umcp.ParamType{umcp.StringParam}}}, Call: e.Update})
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"p","arguments":{"instruction":"x"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
}

func TestModelProposalRoleAndEmptyContext(t *testing.T) {
	jobs, _ := jobsTest(t)
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	e := ModelProposalEndpoint{Jobs: jobs, Client: &stubModelClient{}, Config: config}
	server := umcp.NewServer("x")
	_ = server.Tools.Register(umcp.Tool{Name: "p", Parameters: []umcp.Parameter{{Name: "instruction", Types: []umcp.ParamType{umcp.StringParam}}}, Call: e.Update})
	policy := jobs.Identity.authorization.Principals["actor"]
	policy.Roles = []string{"reader"}
	jobs.Identity.authorization.Principals["actor"] = policy
	principal := jobs.Identity.names["actor"]
	principal.Roles = []string{"reader"}
	jobs.Identity.names["actor"] = principal
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"p","arguments":{"instruction":"x"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
	policy.Roles = []string{"proposer"}
	jobs.Identity.authorization.Principals["actor"] = policy
	principal.Roles = []string{"proposer"}
	jobs.Identity.names["actor"] = principal
	if err = os.Remove(filepath.Join(jobs.Controls.Queue.Paths.CurrentDir, "a.md")); err != nil {
		t.Fatal(err)
	}
	response, err = server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"p","arguments":{"instruction":"x"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
}

func TestModelProposalCallContextAndPersistenceFailures(t *testing.T) {
	jobs, revision := jobsTest(t)
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	valid := fmt.Sprintf(`{"rationale":"because","consulted_concepts":[{"id":"12345678","path":"/a.md","revision":%q,"title":"Title"}],"changes":[{"kind":"patch","path":"/a.md","body":"changed"}]}`, revision)
	e := ModelProposalEndpoint{Jobs: jobs, Client: &stubModelClient{response: ModelResponse{OutputText: valid}}, Config: config}
	server := umcp.NewServer("x")
	_ = server.Tools.Register(umcp.Tool{Name: "p", Parameters: []umcp.Parameter{{Name: "instruction", Types: []umcp.ParamType{umcp.StringParam}}, {Name: "target_hint", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: nil}}, Call: e.Update})
	for id, target := range map[int]string{1: "/missing.md", 2: "/a.md"} {
		if id == 2 {
			jobs.Controls.Random = errorReader{}
		}
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"p","arguments":{"instruction":"x","target_hint":%q}}}`, id, target)
		response, err := server.Process(context.Background(), []byte(body), umcp.RequestContext{Principal: "actor"})
		if err != nil || response == nil {
			t.Fatal(response, err)
		}
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("random") }

func TestModelProposalCallFailures(t *testing.T) {
	jobs, revision := jobsTest(t)
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	valid := fmt.Sprintf(`{"rationale":"because","consulted_concepts":[{"id":"12345678","path":"/a.md","revision":%q,"title":"Title"}],"changes":[{"kind":"patch","path":"/a.md","body":"changed"}]}`, revision)
	for name, model := range map[string]*stubModelClient{"provider": {err: errors.New("provider")}, "parse": {response: ModelResponse{OutputText: "bad"}}, "validation": {response: ModelResponse{OutputText: `{"rationale":"","changes":[]}`}}} {
		t.Run(name, func(t *testing.T) {
			e := ModelProposalEndpoint{Jobs: jobs, Client: model, Config: config}
			server := umcp.NewServer("x")
			_ = server.Tools.Register(umcp.Tool{Name: "p", Parameters: []umcp.Parameter{{Name: "instruction", Types: []umcp.ParamType{umcp.StringParam}}, {Name: "target_hint", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: nil}}, Call: e.Update})
			response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"p","arguments":{"instruction":"x","target_hint":"/a.md"}}}`), umcp.RequestContext{Principal: "actor"})
			if err != nil || response == nil {
				t.Fatal(response, err)
			}
		})
	}
	config.Limits.MaxRationaleChars = 1
	model := &stubModelClient{response: ModelResponse{OutputText: valid}}
	e := ModelProposalEndpoint{Jobs: jobs, Client: model, Config: config}
	server := umcp.NewServer("x")
	_ = server.Tools.Register(umcp.Tool{Name: "p", Parameters: []umcp.Parameter{{Name: "instruction", Types: []umcp.ParamType{umcp.StringParam}}, {Name: "target_hint", Types: []umcp.ParamType{umcp.StringParam}, HasDefault: true, Default: nil}}, Call: e.Update})
	_, _ = server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"p","arguments":{"instruction":"x","target_hint":"/a.md"}}}`), umcp.RequestContext{Principal: "actor"})
}

func TestConsultModelProposalRepositoryFailuresAndLimit(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	path := "/missing.md"
	if _, _, err := consultModelProposalContext(context.Background(), controls, policy, &path, proposalTestLimits(), 0); err == nil {
		t.Fatal("missing target")
	}
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{Path: "/a.md"}, {Path: "/missing.md"}}}}
	controls.Index = index
	limits := proposalTestLimits()
	limits.MaxConsultedConcepts = 1
	got, _, err := consultModelProposalContext(context.Background(), controls, policy, nil, limits, 0)
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	bad := controls.Queue.Paths
	bad.BareDir = filepath.Join(t.TempDir(), "missing.git")
	controls.Queue.Paths = bad
	if _, _, err = consultModelProposalContext(context.Background(), controls, policy, nil, limits, 0); err == nil {
		t.Fatal("revision")
	}
}

func TestConsultModelProposalSearchBranches(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{Path: "/a.md"}, {Path: "/a.md"}}}}
	controls.Index = index
	got, gotRevision, err := consultModelProposalContext(context.Background(), controls, policy, nil, proposalTestLimits(), 0)
	if err != nil || gotRevision != revision || len(got) != 1 {
		t.Fatalf("got=%#v rev=%s err=%v", got, gotRevision, err)
	}
	index.page.Results = []derived.SearchResult{{Path: "/missing.md"}}
	if _, _, err = consultModelProposalContext(context.Background(), controls, policy, nil, proposalTestLimits(), 0); err == nil {
		t.Fatal("search result read")
	}
	index.err = errors.New("search")
	if _, _, err = consultModelProposalContext(context.Background(), controls, policy, nil, proposalTestLimits(), 0); err == nil {
		t.Fatal("search error")
	}
}

func TestConsultModelProposalGraphExpansionReference(t *testing.T) {
	controls, _, baseRevision := realApplyTest(t)
	neighbor := strings.ReplaceAll(strings.ReplaceAll(mutationConcept, "12345678", "87654321"), "title: Title", "title: Neighbor")
	installMutationFiles(t, controls.Queue.Paths.CurrentDir, map[string]string{"/b.md": neighbor})
	index := &answerIndex{
		page:  derived.SearchPage{RepoRevision: "search-revision", Results: []derived.SearchResult{{Path: "/a.md"}}},
		graph: derived.GraphNeighborhood{RepoRevision: "graph-revision", Outbound: []derived.GraphEdge{{ConceptID: "87654321", Path: "/b.md"}}, Inbound: []derived.GraphEdge{{ConceptID: "12345678", Path: "/a.md"}}},
	}
	controls.Index = index
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}
	got, proposalRevision, err := consultModelProposalContext(context.Background(), controls, policy, nil, proposalTestLimits(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if proposalRevision != baseRevision || len(got) != 2 || got[0].Path != "/a.md" || got[0].Revision != "search-revision" || got[1].Path != "/b.md" || got[1].Revision != "graph-revision" {
		t.Fatalf("consulted=%#v proposalRevision=%s", got, proposalRevision)
	}
	if len(index.calls) != 1 || index.calls[0].Query != `"project" OR "instance" OR "service" OR "system" OR "concept"` || index.calls[0].Syntax != "fts5" || !index.calls[0].Strict || index.calls[0].Timeout != time.Second || len(index.graphCalls) != 1 || index.graphCalls[0].Depth != 1 || index.graphCalls[0].Strict {
		t.Fatalf("search=%#v graph=%#v", index.calls, index.graphCalls)
	}
}

func TestModelProposalSearchQueryReference(t *testing.T) {
	got := modelProposalSearchQuery("  What\nchanged? ")
	if got != `"what" OR "changed"` {
		t.Fatal(got)
	}
	if got = modelProposalSearchQuery(" ? "); got != "" {
		t.Fatal(got)
	}
}

func TestModelProposalSecretFailureEnvelope(t *testing.T) {
	jobs, revision := jobsTest(t)
	jobs.Controls.Index = &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{Path: "/a.md"}}}}
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	payload := fmt.Sprintf(`{"intent":"model","rationale":"because","consulted_concepts":[{"id":"12345678","path":"/a.md","revision":%q,"title":"Title"}],"changes":[{"kind":"patch","path":"/a.md","body":"ghp_12345678901234567890"}]}`, revision)
	endpoint := &ModelProposalEndpoint{Jobs: jobs, Client: &stubModelClient{response: ModelResponse{OutputText: payload}}, Config: config}
	jobs.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	handlers := modelHandlers()
	handlers["memory_propose_update"] = endpoint.Update
	server := umcp.NewServer("secret-envelope")
	if err := jobs.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard"}, Limits: endpointLimits(), ModelHandlers: handlers}); err != nil {
		t.Fatal(err)
	}
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_propose_update","arguments":{"instruction":"x"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
	raw, _ := json.Marshal(response)
	if !strings.Contains(string(raw), `"error_class":"validation_error"`) || !strings.Contains(string(raw), "secret scanner") {
		t.Fatal(string(raw))
	}
}

func TestModelProposalBaseRevisionRefreshFailure(t *testing.T) {
	jobs, revision := jobsTest(t)
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	valid := fmt.Sprintf(`{"intent":"model","rationale":"because","consulted_concepts":[{"id":"12345678","path":"/a.md","revision":%q,"title":"Title"}],"changes":[{"kind":"patch","path":"/a.md","body":"changed"}]}`, revision)
	model := &stubModelClient{response: ModelResponse{OutputText: valid}, hook: func() {
		if err := os.Rename(jobs.Controls.Queue.Paths.BareDir, jobs.Controls.Queue.Paths.BareDir+".gone"); err != nil {
			t.Fatal(err)
		}
	}}
	endpoint := &ModelProposalEndpoint{Jobs: jobs, Client: model, Config: config}
	jobs.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	handlers := modelHandlers()
	handlers["memory_propose_update"] = endpoint.Update
	server := umcp.NewServer("revision-refresh")
	if err := jobs.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard"}, Limits: endpointLimits(), ModelHandlers: handlers}); err != nil {
		t.Fatal(err)
	}
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_propose_update","arguments":{"instruction":"x","target_hint":"/a.md"}}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil || response.Error == nil {
		t.Fatal("expected refreshed base revision failure", response, err)
	}
}

func TestConsultModelProposalAdditionalBranches(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	path := "/a.md"
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/a.md"}, WritePrefixes: []string{"/"}}
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{Path: "/denied.md"}}}}
	controls.Index = index
	if _, _, err := consultModelProposalContext(context.Background(), controls, policy, nil, proposalTestLimits(), 0); err == nil {
		t.Fatal("search result authorization")
	}
	limits := proposalTestLimits()
	limits.MaxConsultedConcepts = 0
	index.page.Results = []derived.SearchResult{{Path: path}}
	if got, _, err := consultModelProposalContext(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, nil, limits, 0); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	limits.MaxConsultedConcepts = 2
	index.page.Results = []derived.SearchResult{{Path: path}}
	index.graph = derived.GraphNeighborhood{RepoRevision: revision, Outbound: []derived.GraphEdge{{Path: "/missing.md"}}}
	if _, _, err := consultModelProposalContext(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, nil, limits, 0); err == nil {
		t.Fatal("graph edge read")
	}
	controls.Index = &graphErrorIndex{answerIndex: *index}
	if _, _, err := consultModelProposalContext(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, nil, limits, 0); err == nil {
		t.Fatal("graph lookup")
	}
	controls.Index = index
	neighbor := strings.ReplaceAll(strings.ReplaceAll(mutationConcept, "12345678", "87654321"), "title: Title", "title: Neighbor")
	installMutationFiles(t, controls.Queue.Paths.CurrentDir, map[string]string{"/b.md": neighbor})
	index.graph.Outbound = []derived.GraphEdge{{Path: "/b.md"}, {Path: "/missing.md"}}
	got, _, err := consultModelProposalContext(context.Background(), controls, access.EffectivePolicy{ReadPrefixes: []string{"/"}}, nil, limits, 0)
	if err != nil || len(got) != 2 {
		t.Fatal(got, err)
	}
}

func TestConsultModelProposalExactTarget(t *testing.T) {
	controls, _, revision := realApplyTest(t)
	path := "/a.md"
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}
	got, gotRevision, err := consultModelProposalContext(context.Background(), controls, policy, &path, proposalTestLimits(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if gotRevision != revision || len(got) != 1 || got[0].ID != "12345678" || got[0].Path != path {
		t.Fatalf("context = %#v revision=%s", got, gotRevision)
	}
	denied := access.EffectivePolicy{ReadPrefixes: []string{"/public/"}}
	deniedResult, deniedRevision, err := consultModelProposalContext(context.Background(), controls, denied, &path, proposalTestLimits(), 0)
	if err != nil || deniedRevision != revision || len(deniedResult) != 0 {
		t.Fatalf("unauthorized target hint should be ignored as a direct read: %#v %s %v", deniedResult, deniedRevision, err)
	}
}
