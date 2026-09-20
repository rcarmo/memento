package service

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

type structuredSurfaceCase struct {
	CaseID     string         `json:"case_id"`
	Surface    string         `json:"surface"`
	Adapter    string         `json:"adapter"`
	Setup      map[string]any `json:"setup"`
	Request    map[string]any `json:"request"`
	Expected   map[string]any `json:"expected"`
	StateDelta map[string]any `json:"state_delta"`
}

func loadStructuredSurfaceCases(t *testing.T) []structuredSurfaceCase {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/parity/python-structured-surface-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SchemaVersion int                     `json:"schema_version"`
		PythonCommit  string                  `json:"python_commit"`
		Cases         []structuredSurfaceCase `json:"cases"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || fixture.PythonCommit != "7f29e8b003557f0105f47ed353b7f65a33619456" || len(fixture.Cases) != 23 {
		t.Fatal(fixture.SchemaVersion, fixture.PythonCommit, len(fixture.Cases))
	}
	return fixture.Cases
}

func configureFixtureRoles(jobs *Jobs, roles []string) {
	policy := jobs.Identity.authorization.Principals["actor"]
	policy.Roles = append([]string{}, roles...)
	jobs.Identity.authorization.Principals["actor"] = policy
	principal := jobs.Identity.names["actor"]
	principal.Roles = append([]string{}, roles...)
	jobs.Identity.names["actor"] = principal
}

func fixtureStrings(value any) []string {
	raw := value.([]any)
	out := make([]string, len(raw))
	for i, item := range raw {
		out[i] = item.(string)
	}
	return out
}

func invokeStructuredTool(t *testing.T, jobs *Jobs, handlers map[string]CatalogHandler, surface string, arguments map[string]any) string {
	t.Helper()
	jobs.Controls.Metadata, _ = NewModelsOffMetadata("standard")
	server := umcp.NewServer("python-structured-case")
	if err := jobs.RegisterConfiguredServer(server, ConfiguredServerOptions{Catalog: CatalogConfig{Surface: "standard", AnswerEnabled: true}, Limits: endpointLimits(), ModelHandlers: handlers}); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": surface, "arguments": arguments}})
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Process(context.Background(), body, umcp.RequestContext{Principal: "actor"})
	if err != nil || response == nil {
		t.Fatal(response, err)
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestPythonStructuredServiceSurfaceCases(t *testing.T) {
	seen := map[string]bool{}
	for _, item := range loadStructuredSurfaceCases(t) {
		if strings.HasPrefix(item.Adapter, "cli_") || item.Adapter == "admin_http" {
			continue
		}
		item := item
		t.Run(item.CaseID, func(t *testing.T) {
			seen[item.CaseID] = true
			switch item.Adapter {
			case "service_jsonrpc", "service_call":
				runStructuredServiceCall(t, item)
			case "model_proposal_state":
				runStructuredModelProposalState(t, item)
			case "dream_state":
				runStructuredDreamState(t, item)
			default:
				t.Fatal("unknown service adapter", item.Adapter)
			}
		})
	}
	if len(seen) != 9 {
		t.Fatal("service structured cases", len(seen))
	}
}

func runStructuredServiceCall(t *testing.T, item structuredSurfaceCase) {
	jobs, beforeRevision := jobsTest(t)
	configureFixtureRoles(jobs, fixtureStrings(item.Setup["principal_roles"]))
	handlers := modelHandlers()
	switch item.Surface {
	case "memory_answer":
		handlers[item.Surface] = (&AnswerEndpoint{Jobs: jobs}).Call
	case "memory_propose_freeform", "memory_propose_update":
		endpoint := &ModelProposalEndpoint{Jobs: jobs}
		if item.Surface == "memory_propose_freeform" {
			handlers[item.Surface] = endpoint.Freeform
		} else {
			handlers[item.Surface] = endpoint.Update
		}
	default:
		t.Fatal(item.Surface)
	}
	var proposalsBefore int
	if err := jobs.Controls.Queue.Proposals.DB.QueryRow(`SELECT COUNT(*) FROM proposals`).Scan(&proposalsBefore); err != nil {
		t.Fatal(err)
	}
	text := invokeStructuredTool(t, jobs, handlers, item.Surface, item.Request)
	for key, value := range item.Expected {
		switch key {
		case "message_not_contains":
			if strings.Contains(text, value.(string)) {
				t.Fatalf("response contains forbidden text %q: %s", value, text)
			}
		case "message", "error_class", "status", "answer", "answer_source", "confidence":
			if !strings.Contains(text, value.(string)) {
				t.Fatalf("response lacks %s=%q: %s", key, value, text)
			}
		}
	}
	if got, err := repository.GetMainRevision(jobs.Controls.Queue.Paths); err != nil || got != beforeRevision {
		t.Fatalf("repository changed: %s %s %v", beforeRevision, got, err)
	}
	if expected, ok := item.StateDelta["proposals_created"]; ok {
		var count int
		if err := jobs.Controls.Queue.Proposals.DB.QueryRow(`SELECT COUNT(*) FROM proposals`).Scan(&count); err != nil || count-proposalsBefore != int(expected.(float64)) {
			t.Fatal(count, proposalsBefore, expected, err)
		}
	}
}

func runStructuredModelProposalState(t *testing.T, item structuredSurfaceCase) {
	jobs, revision := jobsTest(t)
	configureFixtureRoles(jobs, fixtureStrings(item.Setup["principal_roles"]))
	index := &answerIndex{page: derived.SearchPage{RepoRevision: revision, Results: []derived.SearchResult{{ConceptID: "12345678", Path: "/a.md", Title: "Title", Status: "active"}}}, graph: derived.GraphNeighborhood{RepoRevision: revision}}
	jobs.Controls.Index = index
	responseJSON := `{"intent":"model intent","rationale":"because","consulted_concepts":[{"id":"12345678","path":"/a.md","revision":"` + revision + `","title":"Title"}],"contradictions":[{"path":"/a.md","summary":"stale"}],"reciprocal_links":[{"source_path":"/a.md","target_path":"/a.md","justification":"self"}],"changes":[{"kind":"patch","path":"/a.md","body":"changed"}]}`
	model := &stubModelClient{response: ModelResponse{OutputText: responseJSON}}
	config := DefaultModelProposalsConfig()
	config.Enabled = true
	endpoint := &ModelProposalEndpoint{Jobs: jobs, Client: model, Config: config, Timeout: time.Second}
	handlers := modelHandlers()
	handlers[item.Surface] = endpoint.Update
	_ = invokeStructuredTool(t, jobs, handlers, item.Surface, item.Request)
	if len(model.requests) != 1 || len(index.calls) == 0 || index.calls[0].Query != item.Expected["search_query"] || index.calls[0].Syntax != item.Expected["search_syntax"] || !index.calls[0].Strict || len(index.graphCalls) != 1 || index.graphCalls[0].Depth != int(item.Setup["graph_depth"].(float64)) {
		t.Fatalf("model=%#v search=%#v graph=%#v", model.requests, index.calls, index.graphCalls)
	}
	for _, field := range fixtureStrings(item.Expected["metadata_fields"]) {
		if _, ok := model.requests[0].Metadata[field]; !ok {
			t.Fatal("missing model metadata", field)
		}
	}
	rows := tableRows(t, jobs.Controls.Queue.Proposals.DB, `SELECT status,patch_json FROM proposals WHERE intent='model intent'`)
	if len(rows) != int(item.StateDelta["proposals_created"].(float64)) || rows[0]["status"] != item.StateDelta["proposal_status"] {
		t.Fatal(rows)
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(rows[0]["patch_json"].(string)), &patch); err != nil {
		t.Fatal(err)
	}
	for _, field := range fixtureStrings(item.Expected["stored_patch_fields"]) {
		if _, ok := patch[field]; !ok {
			t.Fatal("missing stored field", field, patch)
		}
	}
	if got, err := repository.GetMainRevision(jobs.Controls.Queue.Paths); err != nil || got != revision {
		t.Fatal(got, revision, err)
	}
}

func runStructuredDreamState(t *testing.T, item structuredSurfaceCase) {
	mode := item.Request["mode"].(string)
	if mode == "disabled" {
		runtime := &Runtime{Dream: DefaultDreamConfig()}
		payload, err := runtime.runDream(context.Background(), mode, time.Unix(21600, 0), dreamOps())
		if err != nil || !reflect.DeepEqual(payload, item.Expected) {
			t.Fatal(payload, item.Expected, err)
		}
		return
	}
	ctx, runtime, revision := configuredDreamRuntime(t)
	defer runtime.Close(ctx)
	writeDreamConcept(t, runtime.Paths.Repository.CurrentDir, "/a.md", "a", "A", "body")
	model := &stubModelClient{err: context.Canceled}
	actionableCount := int(item.Setup["actionable_signals"].(float64))
	if actionableCount > 0 {
		raw := `{"rationale":"why","consulted_concepts":[{"id":"a","path":"/a.md","revision":"` + revision + `","title":"A"}],"changes":[{"kind":"patch","path":"/a.md","body":"updated"}]}`
		model = &stubModelClient{response: ModelResponse{ModelName: "m", OutputText: raw, ModelChain: []control.ModelAttempt{{Model: "m", Outcome: "success"}}}}
	}
	runtime.ModelClient = model
	if value, ok := item.Setup["max_model_proposals_per_run"]; ok {
		runtime.Dream.Budgets.MaxModelProposalsPerRun = int(value.(float64))
	}
	ops := dreamOps()
	ops.revision = func(repository.GitRepositoryPaths) (string, error) { return revision, nil }
	ops.scan = func(string, repository.BundleFilter) (repository.RepositoryBundle, error) {
		return repository.ScanBundle(runtime.Paths.Repository.CurrentDir, repository.BundleFilter{})
	}
	ops.actionable = func(context.Context) ([]control.DreamSignal, error) {
		out := make([]control.DreamSignal, actionableCount)
		for i := range out {
			out[i] = control.DreamSignal{DedupeKey: string(rune('a' + i)), SignalType: "orphan", EntityRefs: []string{"/a.md"}, EvidenceJSON: `{}`}
		}
		return out, nil
	}
	payload, err := runtime.runDream(ctx, mode, time.Unix(21600, 0), ops)
	if err != nil {
		t.Fatal(err)
	}
	for key, expected := range item.Expected {
		if !reflect.DeepEqual(payload[key], normalizeJSONNumber(expected)) {
			t.Fatalf("%s=%#v want %#v payload=%#v", key, payload[key], expected, payload)
		}
	}
	if len(model.requests) != int(item.StateDelta["model_calls"].(float64)) {
		t.Fatal("model calls", len(model.requests), item.StateDelta)
	}
}

func normalizeJSONNumber(value any) any {
	if number, ok := value.(float64); ok && number == float64(int(number)) {
		return int(number)
	}
	return value
}
