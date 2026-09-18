package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/execute"
)

func TestExecuteAdmission(t *testing.T) {
	workers := Workers{}
	started, release := make(chan struct{}), make(chan struct{})
	done := make(chan any, 1)
	go func() {
		value, err := workers.Execute(context.Background(), func(ctx context.Context) (any, error) {
			if ctx.Err() != nil {
				t.Error(ctx.Err())
			}
			close(started)
			<-release
			return "done", nil
		})
		if err != nil {
			t.Error(err)
		}
		done <- value
	}()
	<-started
	if value, err := workers.Call(context.Background(), "memory_read", func(context.Context) (any, error) { return nil, nil }); err != nil || value.(map[string]any)["error_class"] != "busy" {
		t.Fatal(value, err)
	}
	if value, err := workers.Call(context.Background(), "memory_operation_get", func(context.Context) (any, error) { return "reconcile", nil }); err != nil || value != "reconcile" {
		t.Fatal(value, err)
	}
	close(release)
	if value := <-done; value != "done" {
		t.Fatal(value)
	}
}
func TestExecuteReconciliationAdmission(t *testing.T) {
	if IsReconciliationPlan(execute.Plan{}) || IsReconciliationPlan(execute.Plan{Operations: []execute.PlannedOperation{{Op: "read"}}}) || !IsReconciliationPlan(execute.Plan{Operations: []execute.PlannedOperation{{Op: "operation_get"}, {Op: "operation_get"}}}) {
		t.Fatal("classification")
	}
	workers := Workers{}
	workers.SetExecuteBusy(true)
	value, err := workers.ExecutePlan(context.Background(), true, func(context.Context) (any, error) { return "ok", nil })
	if err != nil || value != "ok" {
		t.Fatal(value, err)
	}
	value, err = workers.ExecutePlan(context.Background(), false, func(context.Context) (any, error) { return nil, nil })
	if err != nil || value.(map[string]any)["message"] != "An execute request is still running; reconcile before retrying." {
		t.Fatal(value, err)
	}
	if !workers.ExecuteBusy() {
		t.Fatal("busy state")
	}
	workers.SetExecuteBusy(false)
}
func TestExecuteAdmissionCancellationAndTimeout(t *testing.T) {
	workers := Workers{timeout: 5 * time.Millisecond}
	release := make(chan struct{})
	value, err := workers.ExecutePlan(context.Background(), false, func(context.Context) (any, error) { <-release; return "late", nil })
	if err != nil || value.(map[string]any)["error_class"] != "indeterminate" || value.(map[string]any)["message"] != "Execution continues; use operation_get with the original idempotency key before retrying." {
		t.Fatal(value, err)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for {
		workers.mu.Lock()
		active := len(workers.active)
		workers.mu.Unlock()
		if active == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker leaked")
		}
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = workers.Execute(ctx, func(context.Context) (any, error) { return nil, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = workers.Execute(context.Background(), nil); err == nil {
		t.Fatal("nil work")
	}
}
