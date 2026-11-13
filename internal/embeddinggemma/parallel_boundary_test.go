package embeddinggemma

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/simd"
)

type parallelGateContext struct {
	context.Context
	calls            atomic.Int64
	once             sync.Once
	entered, release chan struct{}
}

func (c *parallelGateContext) Err() error {
	if c.calls.Add(1) >= 40 {
		c.once.Do(func() { close(c.entered) })
		<-c.release
		return context.Canceled
	}
	return nil
}
func TestParallelInFlightCancelClose(t *testing.T) {
	m := realModel(t, "auto")
	ctx := &parallelGateContext{Context: context.Background(), entered: make(chan struct{}), release: make(chan struct{})}
	w := &Workspace{Workers: 4}
	done := make(chan error, 1)
	go func() { _, err := m.Embed(ctx, "task: search result | query: hello world", Options{}, w); done <- err }()
	select {
	case <-ctx.entered:
	case <-time.After(5 * time.Second):
		close(ctx.release)
		t.Fatal("worker checkpoint not reached")
	}
	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	select {
	case <-closed:
		close(ctx.release)
		t.Fatal("Close did not wait for in-flight projections")
	case <-time.After(10 * time.Millisecond):
	}
	close(ctx.release)
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel stalled")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close stalled")
	}
}

type parallelErrorContext struct {
	context.Context
	calls atomic.Int64
	fail  atomic.Bool
}

func (c *parallelErrorContext) Err() error {
	if c.fail.Load() && c.calls.Add(1) >= 8 {
		return context.Canceled
	}
	return nil
}
func TestParallelOversubscribedPanels(t *testing.T) {
	e, err := simd.New("avx2")
	if err != nil {
		t.Skip(err)
	}
	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)
	const rows = 137
	const cols = 512
	const n = 15
	raw := encodedRows(Q4K, cols, rows)
	m := &Matrix{owner: &File{data: raw}, tensor: Tensor{Type: Q4K, Bytes: uint64(len(raw))}, rows: rows, columns: cols}
	w := &Workspace{Workers: 8}
	w.prepare(n)
	ctx := &parallelErrorContext{Context: context.Background()}
	w.requestContext, w.requestEngine = ctx, e
	team := startProjectionTeam(ctx, w, e)
	defer team.close()
	x := make([]float32, n*cols)
	for i := range x {
		x[i] = float32(i%23-11) / 23
	}
	dst, want := make([]float32, n*rows), make([]float32, n*rows)
	ctx.fail.Store(true)
	if err := team.project(m, dst, x, n, w); err != context.Canceled {
		t.Fatal(err)
	}
	ctx.fail.Store(false)
	if err := team.project(m, dst, x, n, w); err != nil {
		t.Fatal(err)
	}
	if err := m.projectPackedLocked(context.Background(), want, x, n, w.scratch, w.tilePacked, w.tileSums, e); err != nil {
		t.Fatal(err)
	}
	for i := range dst {
		if dst[i] != want[i] {
			t.Fatal(i, dst[i], want[i])
		}
	}
}
