package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/graphdebug"
	"github.com/rcarmo/memento/umcp"
)

type graphRefreshWorker struct{ accepted bool }

func (w graphRefreshWorker) Enqueue(string, string, []string, bool) bool { return w.accepted }
func (graphRefreshWorker) State() graphdebug.RefreshWorkerState {
	return graphdebug.RefreshWorkerState{Alive: true}
}

type graphSnapshotsStub struct {
	err    error
	policy *access.EffectivePolicy
	id     string
}

func (s *graphSnapshotsStub) ExportSelection(context.Context, []string, int, int, *access.EffectivePolicy) ([]graphdebug.Node, []graphdebug.Edge, graphdebug.Revisions, error) {
	return []graphdebug.Node{}, []graphdebug.Edge{}, graphdebug.Revisions{}, s.err
}
func (s *graphSnapshotsStub) Overview(_ context.Context, p *access.EffectivePolicy, _ graphdebug.OverviewOptions) (graphdebug.Overview, error) {
	s.policy = p
	return graphdebug.Overview{SchemaVersion: 1}, s.err
}
func (s *graphSnapshotsStub) Search(_ context.Context, _ string, p *access.EffectivePolicy) (graphdebug.SearchResults, error) {
	s.policy = p
	return graphdebug.SearchResults{SchemaVersion: 1}, s.err
}
func (s *graphSnapshotsStub) ExpandCluster(_ context.Context, id string, _ *access.EffectivePolicy, _ graphdebug.ClusterOptions) (graphdebug.ClusterExpansion, error) {
	s.id = id
	return graphdebug.ClusterExpansion{SchemaVersion: 1, ClusterID: id}, s.err
}
func (s *graphSnapshotsStub) Detail(_ context.Context, id string, _ *access.EffectivePolicy, _, _, _ int) (graphdebug.Detail, error) {
	s.id = id
	return graphdebug.Detail{SchemaVersion: 1, Node: graphdebug.Node{ID: id}}, s.err
}
func (s *graphSnapshotsStub) Neighbourhood(_ context.Context, id string, _ *access.EffectivePolicy, _ graphdebug.NeighbourhoodOptions) (graphdebug.Neighbourhood, error) {
	s.id = id
	return graphdebug.Neighbourhood{SchemaVersion: 1, CenterID: id}, s.err
}
func graphHandler(snapshot GraphSnapshots) GraphHTTP {
	return GraphHTTP{Config: GraphHTTPConfig{Enabled: true, ExportNodeLimit: 10, RoutePrefix: "/graph", Overview: graphdebug.OverviewOptions{DirectNodeLimit: 10, EdgeLimit: 10}, Neighbourhood: graphdebug.NeighbourhoodOptions{Depth: 1}, Cluster: graphdebug.ClusterOptions{RefreshMaxPaths: 10, EdgeLimit: 10, ExpansionNodeLimit: 10, ClusterLimit: 10}, PreviewChars: 10, SummaryLimit: 10}, Snapshots: snapshot}
}
func TestGraphHTTPThroughUMCP(t *testing.T) {
	server := umcp.NewServer("graph")
	options := umcp.DefaultHTTPOptions()
	options.AllowedOrigins = []string{"http://localhost"}
	authenticated := false
	graph := graphHandler(&graphSnapshotsStub{})
	transport, err := umcp.NewStreamableHTTP(server, options, umcp.HTTPHooks{Authenticate: func(context.Context, string, string, map[string]string, string) (*umcp.Principal, error) {
		authenticated = true
		return nil, nil
	}, Route: graph.Handle})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "http://localhost/graph/api/v1/status", nil)
	request.Header.Set("Origin", "http://localhost")
	response := httptest.NewRecorder()
	transport.ServeHTTP(response, request)
	if response.Code != 200 || authenticated || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Access-Control-Allow-Origin") != "http://localhost" {
		t.Fatal(response.Code, authenticated, response.Header(), response.Body.String())
	}
}

func TestGraphHTTPBoundary(t *testing.T) {
	disabled := GraphHTTP{Config: GraphHTTPConfig{RoutePrefix: "/graph"}}
	if response, err := disabled.Handle(context.Background(), "GET", "/other", nil, nil, ""); err != nil || response != nil {
		t.Fatal(response, err)
	}
	response, _ := disabled.Handle(context.Background(), "GET", "/graph", nil, nil, "")
	if response.Status != 404 {
		t.Fatal(response)
	}
	h := graphHandler(&graphSnapshotsStub{})
	for _, tc := range []struct {
		method, path string
		body         []byte
		status       int
	}{{"GET", "/graph", nil, 200}, {"GET", "/graph/api/v1/status", nil, 200}, {"GET", "/graph/api/v1/embeddings/status", nil, 200}, {"POST", "/graph/api/v1/status", nil, 405}, {"GET", "/graph/api/v1/status", []byte("x"), 400}, {"GET", "/graph/missing", nil, 404}} {
		response, err := h.Handle(context.Background(), tc.method, tc.path, nil, tc.body, "")
		if err != nil || response.Status != tc.status || response.Headers[0][1] != "no-store" {
			t.Fatal(tc, response, err)
		}
	}
}
func TestGraphHTTPSnapshots(t *testing.T) {
	stub := &graphSnapshotsStub{}
	h := graphHandler(stub)
	for _, tc := range []struct{ path, want string }{{"/graph/api/v1/clusters/cluster%3Aone", "cluster:one"}, {"/graph/api/v1/memories/node%3A1", "node:1"}, {"/graph/api/v1/neighbourhood/node%3A1", "node:1"}} {
		response, err := h.Handle(context.Background(), "GET", tc.path, nil, nil, "")
		if err != nil || response.Status != 200 || stub.id != tc.want {
			t.Fatal(tc, response, err, stub.id)
		}
	}
	response, err := h.Handle(context.Background(), "GET", "/graph/api/v1/overview", map[string]string{"x-memento-include-trash": "true"}, nil, "")
	if err != nil || response.Status != 200 {
		t.Fatal(response, err)
	}
	response, err = h.Handle(context.Background(), "POST", "/graph/api/v1/search", nil, []byte(`{"query":"test"}`), "")
	if err != nil || response.Status != 200 {
		t.Fatal(response, err)
	}
	h.Refresh = &graphdebug.RefreshCoordinator{}
	response, err = h.Handle(context.Background(), "GET", "/graph/api/v1/embeddings/status", nil, nil, "")
	if err != nil || response.Status != 200 {
		t.Fatal(response, err)
	}
}
func TestGraphHTTPExport(t *testing.T) {
	h := graphHandler(&graphSnapshotsStub{})
	for _, tc := range []struct{ path, mime string }{{"/graph/api/v1/export/json", "application/json; charset=utf-8"}, {"/graph/api/v1/export/svg", "image/svg+xml"}} {
		response, err := h.Handle(context.Background(), "POST", tc.path, nil, []byte(`{"concept_ids":["a"],"settings":{"theme":"dark"}}`), "")
		if err != nil || response.Status != 200 || response.ContentType == nil || *response.ContentType != tc.mime {
			t.Fatal(tc, response, err)
		}
	}
	for _, body := range [][]byte{[]byte("bad"), []byte(`[]`), []byte(`{}`), []byte(`{"concept_ids":1}`), []byte(`{"concept_ids":[1]}`)} {
		response, _ := h.Handle(context.Background(), "POST", "/graph/api/v1/export/json", nil, body, "")
		if response.Status != 400 {
			t.Fatal(body, response)
		}
	}
	var response *umcp.HTTPResponse
	deep := any(nil)
	for range 102 {
		deep = []any{deep}
	}
	encoded, _ := json.Marshal(map[string]any{"concept_ids": []string{}, "settings": map[string]any{"deep": deep}})
	response, _ = h.Handle(context.Background(), "POST", "/graph/api/v1/export/json", nil, encoded, "")
	if response.Status != 400 {
		t.Fatal(response)
	}
	h.Snapshots = &graphSnapshotsStub{err: errors.New("export")}
	response, _ = h.Handle(context.Background(), "POST", "/graph/api/v1/export/json", nil, []byte(`{"concept_ids":[]}`), "")
	if response.Status != 400 {
		t.Fatal(response)
	}
	h.Snapshots = nil
	response, _ = h.Handle(context.Background(), "POST", "/graph/api/v1/export/json", nil, []byte(`{}`), "")
	if response.Status != 503 {
		t.Fatal(response)
	}
}

func TestGraphHTTPRefresh(t *testing.T) {
	root, path := nodeDBAdapter(t)
	coordinator := &graphdebug.RefreshCoordinator{Service: graphdebug.NewSnapshotService(root, path, controlDBAdapter(t)), Worker: graphRefreshWorker{accepted: true}, RepositoryRoot: root, RefreshMaxPaths: 10, DirectNodeLimit: 10, EdgeLimit: 10}
	h := graphHandler(&graphSnapshotsStub{})
	h.Refresh = coordinator
	response, err := h.Handle(context.Background(), "POST", "/graph/api/v1/embeddings/refresh", nil, []byte(`{"scope":"full","confirm_full":true}`), "")
	if err != nil || response.Status != 202 {
		t.Fatal(response, err)
	}
	for _, body := range [][]byte{[]byte("bad"), []byte(`{"scope":"selected","concept_ids":1}`), []byte(`{"scope":"selected","concept_ids":[1]}`), []byte(`{"scope":"selected","concept_ids":["missing"]}`)} {
		response, _ = h.Handle(context.Background(), "POST", "/graph/api/v1/embeddings/refresh", nil, body, "")
		if response.Status != 400 {
			t.Fatal(response)
		}
	}
	response, _ = h.Handle(context.Background(), "POST", "/graph/api/v1/embeddings/refresh", nil, []byte(`{"scope":"bad"}`), "")
	if response.Status != 400 {
		t.Fatal(response)
	}
	h.Refresh = nil
	response, _ = h.Handle(context.Background(), "POST", "/graph/api/v1/embeddings/refresh", nil, []byte(`{}`), "")
	if response.Status != 503 {
		t.Fatal(response)
	}
	h.Refresh = coordinator
	h.Policy = func(context.Context, map[string]string) (*access.EffectivePolicy, error) {
		return &access.EffectivePolicy{}, nil
	}
	response, _ = h.Handle(context.Background(), "POST", "/graph/api/v1/embeddings/refresh", nil, []byte(`{}`), "")
	if response.Status != 400 {
		t.Fatal(response)
	}
}

func TestGraphHTTPErrors(t *testing.T) {
	stub := &graphSnapshotsStub{err: errors.New("missing")}
	h := graphHandler(stub)
	for _, path := range []string{"/graph/api/v1/overview", "/graph/api/v1/clusters/x", "/graph/api/v1/memories/x", "/graph/api/v1/neighbourhood/x"} {
		response, err := h.Handle(context.Background(), "GET", path, nil, nil, "")
		if err != nil || response.Status != 404 || !strings.Contains(string(response.Body), "missing") {
			t.Fatal(path, response, err)
		}
	}
	response, err := h.Handle(context.Background(), "POST", "/graph/api/v1/search", nil, []byte(`{"query":"x"}`), "")
	if err != nil || response.Status != 400 || !strings.Contains(string(response.Body), "missing") {
		t.Fatal(response, err)
	}
	for _, body := range [][]byte{[]byte("bad"), []byte(`{"query":1}`)} {
		response, err := h.Handle(context.Background(), "POST", "/graph/api/v1/search", nil, body, "")
		if err != nil || response.Status != 400 {
			t.Fatal(response, err)
		}
	}
	response, _ = h.Handle(context.Background(), "GET", "/graph/api/v1/memories/%zz", nil, nil, "")
	if response.Status != 404 {
		t.Fatal(response)
	}
	h.Snapshots = nil
	for _, tc := range []struct{ method, path string }{{"GET", "/graph/api/v1/overview"}, {"POST", "/graph/api/v1/search"}, {"GET", "/graph/api/v1/memories/x"}} {
		var body []byte
		if tc.method == "POST" {
			body = []byte(`{"query":"x"}`)
		}
		response, _ := h.Handle(context.Background(), tc.method, tc.path, nil, body, "")
		if response.Status != 503 {
			t.Fatal(tc, response)
		}
	}
	h = graphHandler(&graphSnapshotsStub{})
	h.Policy = func(context.Context, map[string]string) (*access.EffectivePolicy, error) {
		return nil, errors.New("unknown simulated principal")
	}
	response, _ = h.Handle(context.Background(), "GET", "/graph/api/v1/status", nil, nil, "")
	if response.Status != 400 {
		t.Fatal(response)
	}
	if _, err := graphJSON(map[string]any{"bad": func() {}}, 200); err == nil {
		t.Fatal("json")
	}
	var payload map[string]any
	_ = json.Unmarshal(response.Body, &payload)
	h = graphHandler(&graphSnapshotsStub{})
	h.Policies = &GraphPolicyDirectory{Managed: graphManagedFailure{}}
	if _, err = h.Handle(context.Background(), "GET", "/graph/api/v1/principals", nil, nil, ""); err == nil {
		t.Fatal("principal list")
	}
	h = graphHandler(&graphSnapshotsStub{})
	h.Policies = &GraphPolicyDirectory{Static: access.AuthorizationConfig{ProtectedReadPrefixes: []string{"/private/"}, Principals: map[string]access.NamespacePolicy{"reader": {Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}}}}
	response, err = h.Handle(context.Background(), "GET", "/graph/api/v1/principals", nil, nil, "")
	if err != nil || response.Status != 200 || !strings.Contains(string(response.Body), "reader") {
		t.Fatal(response, err)
	}
}
