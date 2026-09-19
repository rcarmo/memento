package service

import (
	"errors"
	"github.com/rcarmo/memento/go/internal/envelope"
	"testing"
)

func TestProjectRoutedEnvelope(t *testing.T) {
	repo, index, op := "r", "i", "o"
	value := envelope.Success[map[string]any]{Status: "success", Data: map[string]any{"results": []any{map[string]any{"path": "/a", "title": "A"}}}, RepoRevision: repo, IndexRevision: index, IndexStale: true, OperationID: &op, Warnings: []string{"w"}, NextTools: []string{}}
	limit := 1
	result, options, err := projectRoutedEnvelope(value, &RouteProjection{Ref: "results", Fields: []string{"path"}, Limit: &limit})
	if err != nil || result["data"].(map[string]any)["value"] == nil || options.RepoRevision == nil || *options.RepoRevision != "r" || options.IndexRevision == nil || *options.IndexRevision != "i" || !options.IndexStale || options.OperationID == nil || len(options.Warnings) != 1 {
		t.Fatal(result, options, err)
	}
	failure := envelope.Failure{Status: "error", ErrorClass: "x", Message: "m"}
	result, options, err = projectRoutedEnvelope(failure, &RouteProjection{Ref: "missing"})
	if err != nil || result["status"] != "error" || options.RepoRevision != nil {
		t.Fatal(result, options, err)
	}
}
func TestProjectRoutedEnvelopeErrors(t *testing.T) {
	for _, value := range []any{func() {}, map[string]any{"status": "success", "data": 1}} {
		if _, _, err := projectRoutedEnvelope(value, &RouteProjection{Ref: "x"}); err == nil {
			t.Fatal(value)
		}
	}
	if _, _, err := projectRoutedEnvelope(map[string]any{"status": "success", "data": map[string]any{}}, &RouteProjection{Ref: "x"}); err == nil {
		t.Fatal("projection")
	}
	if _, err := routeResultMap(errors.New("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := routeResultMap([]any{}); err == nil {
		t.Fatal("object")
	}
}
