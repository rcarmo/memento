package service

import (
	"context"
	"github.com/rcarmo/memento/internal/access"
	"testing"
)

func TestRawExecuteDispatcherGuards(t *testing.T) {
	principal := access.Principal{Name: "actor"}
	j, _ := jobsTest(t)
	for _, tc := range []struct {
		jobs       *Jobs
		principal  access.Principal
		operations []string
	}{{nil, principal, []string{"read"}}, {&Jobs{}, principal, []string{"read"}}, {j, access.Principal{}, []string{"read"}}, {j, principal, nil}, {j, principal, []string{""}}} {
		if _, err := NewRawExecuteDispatcher(tc.jobs, tc.principal, nil, tc.operations); err == nil {
			t.Fatal(tc)
		}
	}
	template, err := NewRawExecuteTemplate(j, []string{"read", "read", "purge"})
	if err != nil || len(template.allowed) != 2 {
		t.Fatal(template, err)
	}
	if _, err = template.Bind(access.Principal{}, nil); err == nil {
		t.Fatal("principal")
	}
	for range 16 {
		bound, err := template.Bind(principal, nil)
		if err != nil || bound.allowed["read"] != true {
			t.Fatal(bound, err)
		}
	}
	dispatcher, err := template.Bind(principal, nil)
	if err != nil || len(dispatcher.allowed) != 2 {
		t.Fatal(dispatcher, err)
	}
	if _, err = dispatcher.Call(context.Background(), "search", nil); err == nil {
		t.Fatal("unavailable")
	}
	if _, err = dispatcher.Dispatch(context.Background(), "search", nil); err == nil {
		t.Fatal("dispatch unavailable")
	}
}
func TestRawExecuteDispatcherCalls(t *testing.T) {
	j, _ := jobsTest(t)
	principal := access.Principal{Name: "actor", Roles: []string{"reader"}}
	dispatcher, _ := NewRawExecuteDispatcher(j, principal, nil, []string{"read", "purge"})
	value, err := dispatcher.Call(context.Background(), "read", map[string]any{"id_or_path": "/missing"})
	if err != nil || value == nil {
		t.Fatal(value, err)
	}
	result, err := dispatcher.Dispatch(context.Background(), "read", map[string]any{"id_or_path": "/missing"})
	if err != nil || result.Status != "error" {
		t.Fatal(result, err)
	}
	for _, confirm := range []bool{false, true} {
		value, err = dispatcher.Call(context.Background(), "purge", map[string]any{"path": "/x", "expected_revision": "r", "idempotency_key": "k", "confirm": confirm})
		if err != nil || value == nil {
			t.Fatal(value, err)
		}
	}
}
