package service

import (
	"strings"
	"testing"
	"time"
)

func TestRenderPrometheusStatus(t *testing.T) {
	status := map[string]any{"repo_revision": "repo", "index_revision": "index", "index_stale": true, "visible_concepts": 2, "proposal_backlog": 3, "closed": false}
	text, err := RenderPrometheusStatus(status)
	if err != nil {
		t.Fatal(err)
	}
	expected := "# HELP memento_service_up Memento process health.\n# TYPE memento_service_up gauge\nmemento_service_up 1\n# HELP memento_control_db_open Whether the control database connection is open.\n# TYPE memento_control_db_open gauge\nmemento_control_db_open 1\n# HELP memento_index_stale Whether the derived index is stale relative to repo head.\n# TYPE memento_index_stale gauge\nmemento_index_stale 1\n# HELP memento_visible_concepts Authorized concept count for the local operator view.\n# TYPE memento_visible_concepts gauge\nmemento_visible_concepts 2\n# HELP memento_proposal_backlog Submitted or approved proposals awaiting action.\n# TYPE memento_proposal_backlog gauge\nmemento_proposal_backlog 3\n# HELP memento_repo_revision_info Repo and index revision labels.\n# TYPE memento_repo_revision_info gauge\nmemento_repo_revision_info{repo_revision=\"repo\",index_revision=\"index\"} 1\n"
	if text != expected {
		t.Fatal(text)
	}
	status["closed"] = true
	status["index_stale"] = false
	text, err = RenderPrometheusStatus(status)
	if err != nil || !strings.Contains(text, "memento_control_db_open 0") || !strings.Contains(text, "memento_index_stale 0") {
		t.Fatal(text, err)
	}
}
func TestRenderGraphiteStatus(t *testing.T) {
	status := map[string]any{"repo_revision": "r", "index_revision": "i", "index_stale": true, "visible_concepts": 2, "proposal_backlog": 3, "closed": false}
	text, err := RenderGraphiteStatus(status, "services.memento", time.Unix(123, 999))
	expected := "services.memento.service_up 1 123\nservices.memento.control_db_open 1 123\nservices.memento.index_stale 1 123\nservices.memento.visible_concepts 2 123\nservices.memento.proposal_backlog 3 123\n"
	if err != nil || text != expected {
		t.Fatal(text, err)
	}
	status["closed"] = true
	status["index_stale"] = false
	text, err = RenderGraphiteStatus(status, "memento", time.Unix(1, 0))
	if err != nil || !strings.Contains(text, "control_db_open 0 1") || !strings.Contains(text, "index_stale 0 1") {
		t.Fatal(text, err)
	}
	for _, prefix := range []string{"", "bad prefix", ".bad", "bad.", "bad..path"} {
		if _, err := RenderGraphiteStatus(status, prefix, time.Time{}); err == nil {
			t.Fatal(prefix)
		}
	}
}
func TestRenderPrometheusStatusFailures(t *testing.T) {
	base := map[string]any{"repo_revision": "r", "index_revision": "i", "index_stale": false, "visible_concepts": 1, "proposal_backlog": 2, "closed": false}
	for _, key := range []string{"repo_revision", "index_revision", "index_stale", "visible_concepts", "proposal_backlog", "closed"} {
		copy := map[string]any{}
		for k, v := range base {
			copy[k] = v
		}
		delete(copy, key)
		if _, err := RenderPrometheusStatus(copy); err == nil {
			t.Fatal(key)
		}
		if _, err := RenderGraphiteStatus(copy, "memento", time.Unix(1, 0)); err == nil {
			t.Fatal("graphite", key)
		}
	}
}
