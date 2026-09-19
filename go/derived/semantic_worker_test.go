package derived

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type semanticWorkerIndex struct {
	mu       sync.Mutex
	pending  []string
	calls    [][]string
	failures []error
}

func (s *semanticWorkerIndex) PendingEmbeddingPaths(context.Context, int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.failures) > 0 {
		err := s.failures[0]
		s.failures = s.failures[1:]
		return nil, err
	}
	return append([]string{}, s.pending...), nil
}
func (s *semanticWorkerIndex) RefreshEmbeddingPaths(_ context.Context, _ string, paths []string, _ SemanticRefreshConfig, _ SemanticClient) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, append([]string{}, paths...))
	if len(s.failures) > 0 {
		err := s.failures[0]
		s.failures = s.failures[1:]
		return err
	}
	if len(s.pending) > 0 && s.pending[0] == paths[0] {
		s.pending = s.pending[1:]
	}
	return nil
}
func TestSemanticWorkerSelectedAndFull(t *testing.T) {
	index := &semanticWorkerIndex{pending: []string{"/c"}}
	worker := NewSemanticWorker(index, &semanticClientStub{}, SemanticRefreshConfig{})
	if !worker.Enqueue("root", "r", []string{"/b", "/a", "/b"}, true) {
		t.Fatal("enqueue")
	}
	_ = worker.Enqueue("root", "r", []string{"/a"}, false)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	state := worker.State()
	if !state.Alive || state.Running || state.Pending || state.Completed != 3 {
		t.Fatal(state)
	}
	index.mu.Lock()
	calls := append([][]string{}, index.calls...)
	index.mu.Unlock()
	if !reflect.DeepEqual(calls, [][]string{{"/b"}, {"/a"}, {"/c"}}) {
		t.Fatal(calls)
	}
	worker.Close()
	worker.Close()
	if worker.Enqueue("", "", nil, false) || worker.State().Alive {
		t.Fatal("closed")
	}
}
func TestSemanticWorkerFailuresAndWait(t *testing.T) {
	busy := &UnavailableError{"busy"}
	index := &semanticWorkerIndex{failures: []error{busy}, pending: []string{"/a"}}
	worker := NewSemanticWorker(index, &semanticClientStub{}, SemanticRefreshConfig{})
	worker.Enqueue("root", "r", nil, true)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if worker.State().Completed != 1 {
		t.Fatal(worker.State())
	}
	worker.Close()
	index = &semanticWorkerIndex{failures: []error{busy}}
	worker = NewSemanticWorker(index, &semanticClientStub{}, SemanticRefreshConfig{})
	worker.Enqueue("root", "r", []string{"/retry"}, false)
	if err := worker.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if worker.State().Completed != 1 {
		t.Fatal(worker.State())
	}
	worker.Close()
	index = &semanticWorkerIndex{failures: []error{errors.New(strings.Repeat("x", 501))}}
	worker = NewSemanticWorker(index, &semanticClientStub{}, SemanticRefreshConfig{})
	worker.Enqueue("root", "r", []string{"/a"}, false)
	if err := worker.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
	state := worker.State()
	if state.LastError == nil || state.PauseReason == nil || *state.PauseReason != "error" {
		t.Fatal(state)
	}
	worker.Close()
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	worker = NewSemanticWorker(&semanticWorkerIndex{}, &semanticClientStub{}, SemanticRefreshConfig{})
	worker.Enqueue("", "", []string{"/x"}, false)
	if err := worker.WaitIdle(ctx2); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	worker.Close()
}
func TestTransientSemanticError(t *testing.T) {
	for _, err := range []error{&UnavailableError{"x"}, errors.New("database is locked"), errors.New("database is busy"), errors.New("database table is locked")} {
		if !transientSemanticError(err) {
			t.Fatal(err)
		}
	}
	if transientSemanticError(errors.New("other")) {
		t.Fatal("other")
	}
	value := "x"
	copy := copyWorkerText(&value)
	value = "y"
	if copy == nil || *copy != "x" || copyWorkerText(nil) != nil {
		t.Fatal(copy)
	}
}
