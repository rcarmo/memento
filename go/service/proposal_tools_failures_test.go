package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/umcp"
)

func TestProposalToolTypeAndCallbackFailures(t *testing.T) {
	ctx := context.Background()
	// uMCP already rejects unknown/missing parameters; the final Go boundary
	// rejects malformed dynamic types instead of allowing unsafe assertions.
	for _, name := range []string{"memory_propose", "memory_proposal_get", "memory_proposal_list", "memory_proposal_asset_get", "memory_proposal_rebase", "memory_proposal_revise", "memory_proposal_review", "memory_proposal_apply", "memory_operation_get", "memory_read", "memory_list", "memory_search", "memory_graph", "memory_asset_prune", "memory_create", "memory_patch", "memory_rename", "memory_trash", "memory_restore", "memory_purge", "unknown"} {
		args := map[string]any{"proposal_id": 1, "idempotency_key": 1, "status": 1}
		if _, _, err := runProposalTool(ctx, nil, ProposalActor{}, name, args); err == nil {
			t.Fatal(name)
		}
	}
	args := toolArguments{values: map[string]any{"indexes": []any{json.Number("1.5")}, "bad": "nonnumeric"}}
	if indexes := args.indexes("indexes"); indexes != nil || args.err == nil {
		t.Fatal(indexes, args.err)
	}
	for _, c := range []struct {
		value any
		want  int
	}{{true, 1}, {false, 0}, {json.Number("7"), 7}} {
		got, err := toolInteger(c.value)
		if err != nil || got != c.want {
			t.Fatal(got, err)
		}
	}
	for _, value := range []any{nil, "x", json.Number("999999999999999999999999")} {
		if _, err := toolInteger(value); err == nil {
			t.Fatal(value)
		}
	}
	for _, kind := range []string{"call", "encode", "notify"} {
		t.Run(kind, func(t *testing.T) {
			server := umcp.NewServer("failure")
			server.SetNotificationOutput(nil)
			call := func(context.Context, string, map[string]any) (any, error) {
				switch kind {
				case "call":
					return nil, io.ErrClosedPipe
				case "encode":
					return make(chan int), nil
				}
				return map[string]any{"status": "success", "data": map[string]any{"changed_paths": []string{"/a.md"}}}, nil
			}
			if err := registerProposalTools(server, call, func(context.Context, string, map[string]any, []string) error { return io.ErrClosedPipe }, proposalToolDefinitions); err != nil {
				t.Fatal(err)
			}
			response, err := server.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"memory_proposal_apply","arguments":{"proposal_id":"p","expected_revision":"r","idempotency_key":"k"}}}`), umcp.RequestContext{Transport: "streamable-http"})
			if err != nil || response.Error == nil || response.Error.Message != "Tool execution failed" {
				t.Fatal(response, err)
			}
		})
	}
	server := umcp.NewServer("notifications")
	server.SetNotificationOutput(nil)
	called := 0
	notify := func(context.Context, string, map[string]any, []string) error { called++; return nil }
	for _, value := range []any{nil, umcp.OrderedObject{}, umcp.OrderedObject{{Name: "status", Value: "error"}, {Name: "data", Value: umcp.OrderedObject{}}}, umcp.OrderedObject{{Name: "status", Value: "success"}, {Name: "data", Value: umcp.OrderedObject{}}}} {
		if err := notifyAppliedEnvelope(ctx, server, notify, value); err != nil {
			t.Fatal(err)
		}
	}
	if called != 0 {
		t.Fatal(called)
	}
	// Subscription delivery failure propagates after a successful result, just as
	// source notification awaits do. No mutation is rolled back by this error.
	if _, _, err := server.Resources.Subscribe(ctx, map[string]any{"uri": "memory://status"}); err != nil {
		t.Fatal(err)
	}
	value := umcp.OrderedObject{{Name: "status", Value: "success"}, {Name: "data", Value: umcp.OrderedObject{}}}
	if err := notifyAppliedEnvelope(ctx, server, func(context.Context, string, map[string]any, []string) error { return io.ErrClosedPipe }, value); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}
func TestJobCursorFailureClosesHandle(t *testing.T) {
	j, _ := jobsTest(t)
	j.Controls.Random = bytes.NewReader(nil)
	identity, err := j.Identity.resolvePrincipal(context.Background(), "actor")
	if err != nil {
		t.Fatal(err)
	}
	var closed bool
	_, err = j.run(context.Background(), identity, nil, "memory_propose", func(context.Context, *ProposalControls, ProposalActor) (map[string]any, SuccessOptions, error) {
		t.Fatal("callback before cursor setup")
		return nil, SuccessOptions{}, nil
	}, func(ctx context.Context, path string) (*sql.DB, error) {
		db, err := control.Connect(ctx, path)
		t.Cleanup(func() {
			closed = db.Ping() != nil
			if !closed {
				t.Error("handle leak")
			}
		})
		return db, err
	})
	if err == nil {
		t.Fatal("missing cursor failure")
	}
}
