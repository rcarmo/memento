package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func insertOperationRows(t *testing.T, db *sql.DB, rows []map[string]any) {
	t.Helper()
	for _, row := range rows {
		keys := []string{}
		for k := range row {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		args := []any{}
		marks := []string{}
		for _, key := range keys {
			args = append(args, row[key])
			marks = append(marks, "?")
		}
		if _, err := db.Exec("INSERT INTO operations("+strings.Join(keys, ",")+") VALUES("+strings.Join(marks, ",")+")", args...); err != nil {
			t.Fatal(err)
		}
	}
}
func TestOperationGetReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/operation-get.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Revision, Exception string
		Policy                        access.EffectivePolicy
		Key, ID                       *string
		Before, After, Proposals      []map[string]any
		Expected                      map[string]any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			restoreSubmitRows(t, q.Proposals.DB, map[string][]map[string]any{"proposals": c.Proposals})
			insertOperationRows(t, q.Proposals.DB, c.Before)
			controls := ProposalControls{Queue: q}
			repo := fakeRepo(nil)
			repo.main = func(repository.GitRepositoryPaths) (string, error) { return c.Revision, nil }
			probe := func(_ context.Context, _ repository.GitRepositoryPaths, fn func() error) (bool, error) {
				if c.Scenario == "writer" {
					return false, nil
				}
				if c.Scenario == "late-insert" {
					insertOperationRows(t, q.Proposals.DB, c.After)
				}
				return true, fn()
			}
			outcome, err := controls.operationGet(ctx, ProposalActor{Policy: c.Policy}, c.Key, c.ID, repo, probe)
			var envelope any
			if err != nil {
				failure, mapping := FailureEnvelope(err)
				if mapping != nil {
					if c.Exception == "" {
						t.Fatal(err)
					}
					return
				}
				envelope = failure
			} else {
				success, err := q.successEnvelope(outcome.Data, SuccessOptions{RepoRevision: outcome.Revision, OperationID: outcome.OperationID}, repo)
				if err != nil {
					t.Fatal(err)
				}
				envelope = success
			}
			if c.Scenario == "bad-replay" {
				actual := jsonNormal(envelope).(map[string]any)
				if actual["error_class"] != "validation_error" {
					t.Fatal(actual)
				}
				actual["message"] = c.Expected["message"]
				envelope = actual
			}
			if !reflect.DeepEqual(jsonNormal(envelope), c.Expected) {
				t.Fatal(envelope, c.Expected)
			}
		})
	}
}
func TestOperationGetFailuresAndWriterProbe(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	actor.Policy.Roles = []string{"curator"}
	key := "missing"
	locked := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- repository.WithTransactionLock(ctx, c.Queue.Paths, func() error { close(locked); <-release; return nil })
	}()
	<-locked
	result, err := c.OperationGet(ctx, actor, &key, nil)
	if err != nil || result.Data["final_state"] != "in_progress" {
		t.Fatal(result, err)
	}
	close(release)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	result, err = c.OperationGet(ctx, actor, &key, nil)
	if err != nil || result.Data["safe_to_retry"] != true {
		t.Fatal(result, err)
	}
	if _, err = c.operationGet(ctx, actor, &key, nil, fakeRepo(nil), func(context.Context, repository.GitRepositoryPaths, func() error) (bool, error) {
		return false, io.ErrClosedPipe
	}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if _, err = c.operationGet(ctx, actor, &key, nil, fakeRepo(nil), func(_ context.Context, _ repository.GitRepositoryPaths, fn func() error) (bool, error) {
		c.Queue.Proposals.DB.Close()
		return true, fn()
	}); err == nil {
		t.Fatal("recheck DB")
	}
	if _, err = c.OperationGet(ctx, actor, &key, nil); err == nil {
		t.Fatal("lookup DB")
	}
	c, actor, _ = realApplyTest(t)
	actor.Policy.Roles = []string{"curator"}
	bad := "{"
	if _, err = c.operationOutcome(ctx, actor.Policy, control.OperationRecord{ResultJSON: &bad}, fakeRepo(nil)); err == nil {
		t.Fatal("bad replay")
	}
	bad = `{"proposal_id":1}`
	if _, err = c.operationOutcome(ctx, actor.Policy, control.OperationRecord{ToolName: "memory_proposal_review", ResultJSON: &bad}, fakeRepo(nil)); err == nil {
		t.Fatal("nonstring replay proposal")
	}
	// Explicit empty ID exercises Python's empty lookup; no coercion to missing.
	if _, err = c.operationOutcome(ctx, actor.Policy, control.OperationRecord{ToolName: "memory_proposal_review"}, fakeRepo(nil)); err == nil {
		t.Fatal("missing control proposal")
	}
	broken := fakeRepo(nil)
	broken.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, err = c.operationOutcome(ctx, actor.Policy, control.OperationRecord{State: control.Failed, BaseRevision: &base}, broken); err == nil {
		t.Fatal("repository failure")
	}
	if _, err = c.Queue.Proposals.DB.Exec("UPDATE proposals SET proposal_id=''"); err != nil {
		t.Fatal(err)
	}
	got, err := c.operationOutcome(ctx, actor.Policy, control.OperationRecord{ToolName: "memory_proposal_review"}, fakeRepo(nil))
	if err != nil || got.Data["result"] == nil {
		t.Fatal(got, err)
	}
}

func TestTimedOutApplyReconcilesOriginalKey(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	actor.Policy.Roles = []string{"curator"}
	key := "original-key"
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	c.DerivedUpdate = func(context.Context, string, string, []string) error { close(started); <-release; return nil }
	workers := Workers{timeout: time.Millisecond}
	call := make(chan workerResult, 1)
	go func() {
		value, err := workers.Call(ctx, "memory_proposal_apply", func(job context.Context) (any, error) {
			defer close(finished)
			return c.Apply(job, actor, "proposal", base, key)
		})
		call <- workerResult{value, err}
	}()
	<-started
	timed := <-call
	if timed.err != nil || timed.value.(map[string]any)["error_class"] != "indeterminate" {
		t.Fatal(timed)
	}
	outcome, err := workers.Call(ctx, "memory_operation_get", func(job context.Context) (any, error) { return c.OperationGet(job, actor, &key, nil) })
	if err != nil || outcome.(OperationOutcome).Data["final_state"] != "in_progress" {
		t.Fatal(outcome, err)
	}
	close(release)
	<-finished
	if err = workers.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	reconciled, err := c.OperationGet(ctx, actor, &key, nil)
	if err != nil || reconciled.Data["final_state"] != "committed" || reconciled.Data["safe_to_retry"] != false {
		t.Fatal(reconciled, err)
	}
	if paths := reconciled.Data["changed_paths"].([]string); !reflect.DeepEqual(paths, []string{"/a.md"}) {
		t.Fatal(paths)
	}
	replay, err := c.Apply(ctx, actor, "proposal", base, key)
	if err != nil || replay.OperationID != *reconciled.OperationID || replay.Data["replayed"] != true {
		t.Fatal(replay, err)
	}
}
