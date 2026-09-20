package graphdebug

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/access"
)

type refreshSnapshots struct {
	paths                 map[string]string
	overview              Overview
	overviewErr, pathsErr error
}

func (s refreshSnapshots) Overview(context.Context, *access.EffectivePolicy, OverviewOptions) (Overview, error) {
	return s.overview, s.overviewErr
}
func (s refreshSnapshots) PathsForIDs(_ context.Context, ids []string) ([]string, error) {
	if s.pathsErr != nil {
		return nil, s.pathsErr
	}
	out := []string{}
	for _, id := range ids {
		if path, ok := s.paths[id]; ok {
			out = append(out, path)
		}
	}
	return out, nil
}
func TestRefreshFixture(t *testing.T) {
	var fixture struct {
		Selected, Full, None RefreshState
		Calls                [][]any
		Errors               map[string]string
	}
	raw, err := os.ReadFile("../../testdata/parity/graph-refresh.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	message := "boom"
	pause := "pause"
	current := "/a.md"
	worker := &refreshWorker{accepted: true, state: RefreshWorkerState{Alive: true, Running: true, LastError: &message, PauseReason: &pause, CurrentPath: &current, Completed: 2}}
	c := RefreshCoordinator{Service: refreshSnapshots{paths: map[string]string{"a": "/a.md", "b": "/b.md"}, overview: Overview{Metrics: Metrics{MemoryCount: 3}, Revisions: Revisions{Repository: "main"}}}, Worker: worker, RepositoryRoot: "/repo", RefreshMaxPaths: 2}
	selected, err := c.Enqueue(context.Background(), "selected", []string{"b", "a", "b"}, false)
	if err != nil || !reflect.DeepEqual(selected, fixture.Selected) {
		t.Fatal(selected, fixture.Selected, err)
	}
	full, err := c.Enqueue(context.Background(), "full", nil, true)
	if err != nil || !reflect.DeepEqual(full, fixture.Full) {
		t.Fatal(full, fixture.Full, err)
	}
	if got := new(RefreshCoordinator).State(); !reflect.DeepEqual(got, fixture.None) {
		t.Fatal(got, fixture.None)
	}
	worker.state = RefreshWorkerState{}
	if got := c.State(); got.Available || got.Alive || got.Running {
		t.Fatal(got)
	}
}
func TestRefreshErrorFixture(t *testing.T) {
	var fixture struct{ Errors map[string]string }
	raw, _ := os.ReadFile("../../testdata/parity/graph-refresh.json")
	_ = json.Unmarshal(raw, &fixture)
	snapshot := refreshSnapshots{paths: map[string]string{"a": "/a", "b": "/b"}, overview: Overview{Metrics: Metrics{MemoryCount: 3}, Revisions: Revisions{Repository: "main"}}}
	for name, call := range map[string]func() error{"full_confirmation": func() error {
		c := RefreshCoordinator{Service: snapshot, Worker: &refreshWorker{accepted: true}, RefreshMaxPaths: 2}
		_, e := c.Enqueue(context.Background(), "full", nil, false)
		return e
	}, "selected_ids": func() error {
		c := RefreshCoordinator{Service: snapshot, Worker: &refreshWorker{accepted: true}, RefreshMaxPaths: 2}
		_, e := c.Enqueue(context.Background(), "selected", nil, false)
		return e
	}, "limit": func() error {
		c := RefreshCoordinator{Service: snapshot, Worker: &refreshWorker{accepted: true}, RefreshMaxPaths: 2}
		_, e := c.Enqueue(context.Background(), "visible", []string{"a", "b", "c"}, false)
		return e
	}, "unknown": func() error {
		c := RefreshCoordinator{Service: snapshot, Worker: &refreshWorker{accepted: true}, RefreshMaxPaths: 2}
		_, e := c.Enqueue(context.Background(), "selected", []string{"missing"}, false)
		return e
	}, "scope": func() error {
		c := RefreshCoordinator{Service: snapshot, Worker: &refreshWorker{accepted: true}, RefreshMaxPaths: 2}
		_, e := c.Enqueue(context.Background(), "bad", nil, false)
		return e
	}} {
		err := call()
		if err == nil || err.Error() != fixture.Errors[name] {
			t.Fatal(name, err, fixture.Errors[name])
		}
	}
}
func TestRefreshSnapshotFailures(t *testing.T) {
	boom := errors.New("boom")
	worker := &refreshWorker{accepted: true}
	c := RefreshCoordinator{Service: refreshSnapshots{overviewErr: boom}, Worker: worker, RefreshMaxPaths: 2}
	if _, err := c.Enqueue(context.Background(), "full", nil, true); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	c.Service = refreshSnapshots{pathsErr: boom, overview: Overview{Revisions: Revisions{Repository: "main"}}}
	if _, err := c.Enqueue(context.Background(), "selected", []string{"a"}, false); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	c.Service = refreshSnapshots{paths: map[string]string{}, overview: Overview{Revisions: Revisions{Repository: "main"}}}
	if _, err := c.Enqueue(context.Background(), "selected", []string{"a"}, false); err == nil {
		t.Fatal("unknown")
	}
}
