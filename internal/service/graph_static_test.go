package service

import (
	"strings"
	"testing"
)

func TestGraphStaticAssets(t *testing.T) {
	for _, name := range []string{"api.js", "app.css", "app.js", "graph-scene.js", "layout-worker.js", "vendor/LICENSES.md", "vendor/manifest.json", "vendor/preact-hooks.module.js", "vendor/preact.module.js", "vendor/three.core.min.js", "vendor/three.module.min.js"} {
		response := graphStaticResponse(name, "/graph")
		if response.Status != 200 || response.ContentType == nil {
			t.Fatal(name, response)
		}
		if len(response.Body) == 0 {
			t.Fatal(name, "empty embedded asset")
		}
	}
	index := graphStaticResponse("index.html", "/custom")
	if index.Status != 200 || !strings.Contains(string(index.Body), `data-graph-prefix="/custom"`) || strings.Contains(string(index.Body), "__GRAPH_PREFIX__") {
		t.Fatal(string(index.Body))
	}
	for _, name := range []string{"", "/x", ".", "..", "a/../b", "a//b", "missing"} {
		if response := graphStaticResponse(name, "/graph"); response.Status != 404 {
			t.Fatal(name, response)
		}
	}
	app := string(graphStaticResponse("app.js", "/graph").Body)
	for _, required := range []string{
		"setDetail({ node, loading: true })",
		"selectionAbort.current?.abort()",
		"request !== selectionRequest.current",
		`data-testid": "inspector-loading"`,
	} {
		if !strings.Contains(app, required) {
			t.Fatal("missing optimistic inspector behavior", required)
		}
	}
}
func TestGraphHTTPStatic(t *testing.T) {
	h := graphHandler(&graphSnapshotsStub{})
	for _, tc := range []struct{ path, mime string }{{"/graph", "text/html; charset=utf-8"}, {"/graph/", "text/html; charset=utf-8"}, {"/graph/assets/app.css", "text/css; charset=utf-8"}, {"/graph/assets/vendor/LICENSES.md", "application/octet-stream"}, {"/graph/assets/missing", ""}, {"/graph/assets/../app.css", ""}} {
		response, err := h.Handle(t.Context(), "GET", tc.path, nil, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if tc.mime == "" {
			if response.Status != 404 {
				t.Fatal(tc, response)
			}
			continue
		}
		if response.Status != 200 || response.ContentType == nil || *response.ContentType != tc.mime {
			t.Fatal(tc, response)
		}
	}
}
