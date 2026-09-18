package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/rcarmo/memento/go/internal/envelope"
	"github.com/rcarmo/memento/go/umcp"
)

func TestExecuteAdapter(t *testing.T) {
	r, i, op := "r", "i", "op"
	handlers := map[string]CatalogHandler{"memory_read": func(context.Context, map[string]any) (any, error) {
		return envelope.Success[map[string]any]{Status: "success", Data: map[string]any{"path": "/a"}, Warnings: []string{}, NextTools: []string{}, RepoRevision: r, IndexRevision: i, OperationID: &op}, nil
	}, "memory_search": func(context.Context, map[string]any) (any, error) {
		return envelope.Failure{Status: "error", ErrorClass: "bad", Message: "failed", Warnings: []string{}, RepoRevision: &r}, nil
	}}
	adapter, err := NewExecuteAdapter(handlers)
	if err != nil {
		t.Fatal(err)
	}
	handlers["memory_read"] = nil
	got, err := adapter.Dispatch(context.Background(), "read", map[string]any{})
	if err != nil || got.Status != "success" || got.Data["path"] != "/a" || *got.OperationID != "op" || *got.RepoRevision != "r" || *got.IndexRevision != "i" {
		t.Fatal(got, err)
	}
	got, err = adapter.Dispatch(context.Background(), "search", nil)
	if err != nil || got.Status != "error" || got.ErrorClass != "bad" {
		t.Fatal(got, err)
	}
}
func TestExecuteAdapterFailures(t *testing.T) {
	if _, err := NewExecuteAdapter(nil); err == nil {
		t.Fatal("empty")
	}
	boom := errors.New("boom")
	cases := []CatalogHandler{func(context.Context, map[string]any) (any, error) { return nil, boom }, func(context.Context, map[string]any) (any, error) { return 1, nil }, func(context.Context, map[string]any) (any, error) { return map[string]any{"status": "bad"}, nil }, func(context.Context, map[string]any) (any, error) {
		return map[string]any{"status": "success", "data": 1}, nil
	}, func(context.Context, map[string]any) (any, error) { return map[string]any{"status": "error"}, nil }, func(context.Context, map[string]any) (any, error) {
		return map[string]any{"status": "success", "data": map[string]any{"x": math.Inf(1)}}, nil
	}}
	for _, handler := range cases {
		adapter, _ := NewExecuteAdapter(map[string]CatalogHandler{"memory_read": handler})
		if _, err := adapter.Dispatch(context.Background(), "read", nil); err == nil {
			t.Fatal("bad envelope")
		}
	}
	adapter, _ := NewExecuteAdapter(map[string]CatalogHandler{"nil": nil})
	if _, err := adapter.Dispatch(context.Background(), "read", nil); err == nil {
		t.Fatal("missing")
	}
	if got := executeValue([]any{umcp.OrderedObject{{Name: "x", Value: json.Number("1")}}}); got.([]any)[0].(map[string]any)["x"] != json.Number("1") {
		t.Fatal(got)
	}
	if optionalEnvelopeString(nil) != nil || *optionalEnvelopeString("x") != "x" {
		t.Fatal("optional")
	}
}
