package service

import (
	"context"
	"errors"
	"testing"
	"time"
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
func TestExecuteAdmissionCancellationAndTimeout(t *testing.T) {
	workers := Workers{timeout: 5 * time.Millisecond}
	release := make(chan struct{})
	value, err := workers.Execute(context.Background(), func(context.Context) (any, error) { <-release; return "late", nil })
	if err != nil || value.(map[string]any)["error_class"] != "indeterminate" {
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
