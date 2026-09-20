package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/envelope"
	"github.com/rcarmo/memento/internal/repository"
	"github.com/rcarmo/memento/umcp"
)

func jobsTest(t *testing.T) (*Jobs, string) {
	t.Helper()
	c, _, base := realApplyTest(t)
	config := access.AuthorizationConfig{Principals: map[string]access.NamespacePolicy{"actor": {Roles: []string{"proposer", "curator"}, ReadPrefixes: []string{"/"}, WritePrefixes: []string{"/"}}}}
	identity, err := NewIdentity([]BearerPrincipal{{"token", access.Principal{Name: "actor", Roles: []string{"proposer", "curator"}}}}, config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := ""
	for _, r := range tableRows(t, c.Queue.Proposals.DB, "PRAGMA database_list") {
		if r["name"] == "main" {
			path = r["file"].(string)
		}
	}
	return &Jobs{Controls: c, Identity: identity, DBPath: path}, base
}
func TestPerJobHandlesAndCursorSharing(t *testing.T) {
	j, base := jobsTest(t)
	ctx := context.Background()
	principal := access.Principal{Name: "actor", Roles: []string{"proposer", "curator"}}
	var handle *sql.DB
	var cursor string
	call := func(ctx context.Context, c *ProposalControls, actor ProposalActor) (map[string]any, SuccessOptions, error) {
		handle = c.Queue.Proposals.DB
		if handle == j.Controls.Queue.Proposals.DB {
			t.Fatal("shared DB handle")
		}
		var err error
		cursor, err = c.encodeProposalCursor(proposalListScope(actor.Policy, nil, base), "proposal")
		return map[string]any{"ok": true}, SuccessOptions{}, err
	}
	if _, err := j.run(ctx, principal, nil, "memory_propose", call, control.Connect); err != nil {
		t.Fatal(err)
	}
	if err := handle.Ping(); err == nil {
		t.Fatal("worker handle not closed")
	}
	_, err := j.run(ctx, principal, nil, "memory_propose", func(_ context.Context, c *ProposalControls, actor ProposalActor) (map[string]any, SuccessOptions, error) {
		after, err := c.decodeProposalCursor(cursor, proposalListScope(actor.Policy, nil, base))
		if err != nil || after != "proposal" {
			t.Fatal(after, err)
		}
		return nil, SuccessOptions{}, nil
	}, control.Connect)
	if err != nil {
		t.Fatal(err)
	}
	// SQL failures and policy errors must still close handles; unexpected errors
	// propagate, while known policy failures become complete service envelopes.
	for _, cause := range []error{io.ErrClosedPipe, &Error{"validation_error", "bad"}} {
		got, err := j.run(ctx, principal, nil, "memory_propose", func(_ context.Context, c *ProposalControls, _ ProposalActor) (map[string]any, SuccessOptions, error) {
			handle = c.Queue.Proposals.DB
			return nil, SuccessOptions{}, cause
		}, control.Connect)
		if cause == io.ErrClosedPipe {
			if !errors.Is(err, cause) {
				t.Fatal(err)
			}
		} else if err != nil || got.(envelope.Failure).ErrorClass != "validation_error" {
			t.Fatal(got, err)
		}
		if err := handle.Ping(); err == nil {
			t.Fatal("leaked handle")
		}
	}
	if _, err := j.run(ctx, principal, nil, "memory_propose", call, func(context.Context, string) (*sql.DB, error) { return nil, io.ErrClosedPipe }); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if _, err := j.Call(ctx, "memory_propose", call); err == nil {
		t.Fatal("missing identity")
	}
	j.Identity.managed = identityStore{}
	if _, err := j.run(ctx, principal, nil, "memory_propose", call, control.Connect); err == nil {
		t.Fatal("unsupported custom managed worker")
	}
	j.Identity.managed = nil
	principal.Roles = nil
	if result, err := j.run(ctx, principal, nil, "memory_propose", call, control.Connect); err != nil || result.(envelope.Failure).ErrorClass != "forbidden" {
		t.Fatal(result, err)
	}
}
func TestManagedJobUsesLivePolicy(t *testing.T) {
	j, _ := jobsTest(t)
	ctx := context.Background()
	store, err := access.OpenStore(ctx, j.Controls.Queue.Proposals.DB, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Create(ctx, "bootstrap", "actor", []string{"proposer"}, []string{"/managed/"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	j.Identity.managed = store
	if _, err = j.run(ctx, access.Principal{Name: "actor", Roles: []string{"curator"}}, nil, "memory_propose", func(_ context.Context, c *ProposalControls, actor ProposalActor) (map[string]any, SuccessOptions, error) {
		if actor.Policy.ReadPrefixes[0] != "/managed/" || actor.Policy.Roles[0] != "proposer" {
			t.Fatal(actor)
		}
		return nil, SuccessOptions{}, nil
	}, control.Connect); err != nil {
		t.Fatal(err)
	}
	// Capture principal before admission, then re-read policy on the worker.
	dispatcher := umcp.Dispatcher{Handlers: map[string]umcp.Handler{"job": func(ctx context.Context, _ map[string]any) (any, *umcp.RPCError, error) {
		value, err := j.Call(ctx, "memory_propose", func(_ context.Context, _ *ProposalControls, actor ProposalActor) (map[string]any, SuccessOptions, error) {
			return map[string]any{"principal": actor.Policy.Principal}, SuccessOptions{}, nil
		})
		return value, nil, err
	}}}
	if response, err := dispatcher.Process(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"job"}`), umcp.RequestContext{Principal: "actor", SessionID: "session"}); err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
}

func TestModelBackedReadsDoNotHoldRepositoryTransactionLock(t *testing.T) {
	for _, method := range []string{"memory_answer", "memory_propose_freeform", "memory_propose_update"} {
		t.Run(method, func(t *testing.T) {
			j, _ := jobsTest(t)
			ctx := context.Background()
			locked, release := make(chan struct{}), make(chan struct{})
			lockDone := make(chan error, 1)
			go func() {
				lockDone <- repository.WithTransactionLock(ctx, j.Controls.Queue.Paths, func() error { close(locked); <-release; return nil })
			}()
			<-locked
			finished := make(chan error, 1)
			go func() {
				_, err := j.run(ctx, access.Principal{Name: "actor", Roles: []string{"proposer", "curator"}}, nil, method, func(context.Context, *ProposalControls, ProposalActor) (map[string]any, SuccessOptions, error) {
					return map[string]any{"ok": true}, SuccessOptions{}, nil
				}, control.Connect)
				finished <- err
			}()
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("model-backed read waited for repository transaction lock")
			}
			close(release)
			if err := <-lockDone; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestJobPolicyRecheckedAfterWriterWait(t *testing.T) {
	j, _ := jobsTest(t)
	ctx := context.Background()
	store, err := access.OpenStore(ctx, j.Controls.Queue.Proposals.DB, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Create(ctx, "bootstrap", "actor", []string{"proposer"}, []string{"/allowed/"}, []string{"/allowed/"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	j.Identity.managed = store
	locked, release := make(chan struct{}), make(chan struct{})
	lockDone := make(chan error, 1)
	go func() {
		lockDone <- repository.WithTransactionLock(ctx, j.Controls.Queue.Paths, func() error { close(locked); <-release; return nil })
	}()
	<-locked
	opened := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := j.run(ctx, access.Principal{Name: "actor", Roles: []string{"proposer"}}, nil, "memory_propose", func(_ context.Context, _ *ProposalControls, actor ProposalActor) (map[string]any, SuccessOptions, error) {
			if len(actor.Policy.WritePrefixes) != 0 {
				t.Error("stale grant used", actor.Policy)
			}
			return nil, SuccessOptions{}, nil
		}, func(ctx context.Context, path string) (*sql.DB, error) {
			db, err := control.Connect(ctx, path)
			close(opened)
			return db, err
		})
		finished <- err
	}()
	<-opened
	if _, err = store.Update(ctx, "bootstrap", "actor", []string{"proposer"}, []string{"/allowed/"}, nil); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err = <-lockDone; err != nil {
		t.Fatal(err)
	}
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
}
