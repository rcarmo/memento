package service

import (
	"reflect"
	"testing"
)

func TestRouteProjection(t *testing.T) {
	payload := map[string]any{"results": []any{map[string]any{"path": "/a", "title": "A"}, "skip", map[string]any{"path": "/b"}}, "nested": map[string]any{"value": 1}}
	limit := 2
	for _, tc := range []struct {
		projection RouteProjection
		want       map[string]any
	}{{RouteProjection{Ref: "results", Fields: []string{"path"}, Limit: &limit}, map[string]any{"value": []any{map[string]any{"path": "/a"}}}}, {RouteProjection{Ref: "results", Limit: &limit}, map[string]any{"value": []any{map[string]any{"path": "/a", "title": "A"}, "skip"}}}, {RouteProjection{Ref: "nested.value"}, map[string]any{"value": 1}}} {
		got, err := ProjectRouteResult(payload, tc.projection)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatal(got, tc.want, err)
		}
	}
	if got, err := ResolveRouteProjection(payload, "results.0.path"); err != nil || got != "/a" {
		t.Fatal(got, err)
	}
	for _, ref := range []string{"missing", "results.x", "results.9", "nested.value.x"} {
		if _, err := ResolveRouteProjection(payload, ref); err == nil {
			t.Fatal(ref)
		}
	}
	if _, err := ProjectRouteResult(payload, RouteProjection{Ref: "missing"}); err == nil {
		t.Fatal("project error")
	}
}
func TestRouteProjectionDecode(t *testing.T) {
	if routeProjection(map[string]any{}) != nil {
		t.Fatal("nil")
	}
	got := routeProjection(map[string]any{"projection": map[string]any{"ref": "results", "fields": []any{"path", 1}, "limit": 2}})
	if got == nil || got.Ref != "results" || len(got.Fields) != 1 || got.Limit == nil || *got.Limit != 2 {
		t.Fatal(got)
	}
	got = routeProjection(map[string]any{"projection": map[string]any{"ref": "x", "limit": nil}})
	if got == nil || got.Limit != nil {
		t.Fatal(got)
	}
}
