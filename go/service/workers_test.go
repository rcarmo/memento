package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/umcp"
)

func workerCount(w *Workers) int { w.mu.Lock(); defer w.mu.Unlock(); return len(w.active) }
func TestWorkersReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/service-workers.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Kind, Method, Error string
		Closing, Busy       bool
		Active              int
		Expected            map[string]any
		Before              int `json:"active_before_release"`
		After               int `json:"active_after_drain"`
		Finished            bool
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if c.Kind == "admission" {
			w := Workers{closing: c.Closing}
			w.SetExecuteBusy(c.Busy)
			if c.Active > 0 {
				w.active = map[chan struct{}]struct{}{}
				for range c.Active {
					w.active[make(chan struct{})] = struct{}{}
				}
			}
			got, err := w.Call(context.Background(), c.Method, func(context.Context) (any, error) { return map[string]any{"worked": true}, nil })
			if err != nil || !reflect.DeepEqual(got, c.Expected) {
				t.Fatal(c, got, err)
			}
			continue
		}
		w := Workers{timeout: time.Millisecond}
		if c.Kind == "error" {
			w.timeout = time.Hour
		}
		ctx, cancel := context.WithCancel(context.Background())
		start, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
		result := make(chan workerResult, 1)
		go func() {
			value, err := w.Call(ctx, "memory_propose", func(ctx context.Context) (any, error) {
				close(start)
				<-release
				if ctx.Err() != nil {
					t.Error("shielded context cancelled")
				}
				close(finished)
				if c.Kind == "error" {
					return nil, errors.New("synthetic")
				}
				return map[string]any{"worked": true}, nil
			})
			result <- workerResult{value, err}
		}()
		<-start
		if c.Kind == "cancel" {
			cancel()
		}
		if c.Kind == "error" {
			close(release)
		}
		got := <-result
		if c.Kind == "error" {
			if got.err == nil || got.err.Error() != c.Error {
				t.Fatal(got)
			}
		} else if c.Kind == "cancel" {
			if !errors.Is(got.err, context.Canceled) {
				t.Fatal(got)
			}
		} else if got.err != nil || !reflect.DeepEqual(got.value, c.Expected) {
			t.Fatal(got, c.Expected)
		}
		if count := workerCount(&w); count != c.Before {
			t.Fatal(c.Kind, count, c.Before)
		}
		if c.Kind != "error" {
			close(release)
		}
		if err := w.Drain(context.Background()); err != nil {
			t.Fatal(err)
		}
		cancel()
		select {
		case <-finished:
		default:
			t.Fatal("job did not finish")
		}
		if workerCount(&w) != c.After || !w.closing {
			t.Fatal("drain state")
		}
	}
}
func TestWorkersAdmissionDrainAndPanics(t *testing.T) {
	w := Workers{timeout: time.Hour}
	start := make(chan struct{}, 2)
	release := make(chan struct{})
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.Call(context.Background(), "memory_operation_get", func(context.Context) (any, error) { start <- struct{}{}; <-release; return nil, nil })
			if err != nil {
				t.Error(err)
			}
		}()
	}
	<-start
	<-start
	got, err := w.Call(context.Background(), "memory_operation_get", func(context.Context) (any, error) { t.Error("admitted third job"); return nil, nil })
	if err != nil || got.(map[string]any)["error_class"] != "busy" {
		t.Fatal(got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = w.Drain(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	wg.Wait()
	if err = w.Drain(context.Background()); err != nil {
		t.Fatal(err)
	}
	fresh := Workers{}
	if _, err = fresh.Call(context.Background(), "panic", func(context.Context) (any, error) { panic("synthetic") }); err == nil {
		t.Fatal("panic escaped")
	} else {
		var execution *umcp.ExecutionError
		if !errors.As(err, &execution) || execution.Type != "RuntimeError" {
			t.Fatal(err)
		}
	}
	if workerCount(&fresh) != 0 {
		t.Fatal("panic leaked slot")
	}
}
