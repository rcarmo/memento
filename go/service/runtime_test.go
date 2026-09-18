package service

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func TestRuntimeCloseOrderAndIdempotence(t *testing.T) {
	ctx := context.Background()
	db, err := control.Connect(ctx, filepath.Join(t.TempDir(), "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	lease, err := repository.AcquireWriterLease(filepath.Join(t.TempDir(), "writer.lock"), "test")
	if err != nil {
		t.Fatal(err)
	}
	order := []string{}
	boom := errors.New("boom")
	runtime := &Runtime{DB: db, Lease: lease, Closers: []func() error{func() error { order = append(order, "first"); return nil }, func() error { order = append(order, "second"); return boom }}}
	if err = runtime.Close(ctx); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"second", "first"}) {
		t.Fatal(order)
	}
	if err = runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if other, err := repository.AcquireWriterLease(lease.Path, "other"); err != nil {
		t.Fatal(err)
	} else {
		_ = other.Release()
	}
	if err = db.Ping(); err == nil {
		t.Fatal("database open")
	}
}
func TestRuntimeDrainsBeforeClosers(t *testing.T) {
	jobs := &Jobs{}
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = jobs.Workers.Call(context.Background(), "memory_read", func(context.Context) (any, error) { close(started); <-release; return nil, nil })
	}()
	<-started
	order := []string{}
	runtime := &Runtime{Jobs: jobs, Closers: []func() error{func() error { order = append(order, "closed"); return nil }}}
	closed := make(chan error, 1)
	go func() { closed <- runtime.Close(context.Background()) }()
	select {
	case <-closed:
		t.Fatal("did not drain")
	default:
	}
	close(release)
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	<-done
	if !reflect.DeepEqual(order, []string{"closed"}) {
		t.Fatal(order)
	}
}
func TestRuntimeDrainCancellationStillCleansUp(t *testing.T) {
	jobs := &Jobs{}
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = jobs.Workers.Call(context.Background(), "memory_read", func(context.Context) (any, error) { close(started); <-release; return nil, nil })
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	runtime := &Runtime{Jobs: jobs, Closers: []func() error{func() error { called = true; return nil }}}
	if err := runtime.Close(ctx); !errors.Is(err, context.Canceled) || !called {
		t.Fatal(err, called)
	}
	close(release)
}
