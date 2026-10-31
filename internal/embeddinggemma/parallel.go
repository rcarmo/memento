package embeddinggemma

import (
	"context"
	"fmt"
	"github.com/rcarmo/memento/internal/simd"
	"runtime"
	"sync"
	"sync/atomic"
)

// A team belongs to one locked request. Atomic generation publication orders
// job writes before worker reads; done's atomic RMW chain publishes every error
// and output write before the caller reads them. All jobs drain before reuse.
// Idle polling trades CPU for latency; yields also allow oversubscribed teams
// to finish. Workers are joined before releasing the model read lock.
type projectionJob struct {
	matrix            *Matrix
	dst, packed       []float32
	n, nt, start, end int
}
type projectionWorker struct {
	decoded, sums []float32
	job           projectionJob
	err           error
}
type projectionTeam struct {
	workers          []projectionWorker
	ctx              context.Context
	engine           simd.Engine
	generation, done atomic.Uint64
	stop             atomic.Bool
	joined           sync.WaitGroup
}

func startProjectionTeam(ctx context.Context, w *Workspace, engine simd.Engine) *projectionTeam {
	if w.Workers < 2 || !packedProjectionSupported(engine.Backend()) {
		return nil
	}
	if len(w.parallelBuffers) != w.Workers {
		w.parallelBuffers = make([]projectionWorker, w.Workers)
		for i := range w.parallelBuffers {
			w.parallelBuffers[i].decoded = make([]float32, 8*2048)
			w.parallelBuffers[i].sums = make([]float32, 8*projectionTile)
		}
	}
	t := &projectionTeam{workers: make([]projectionWorker, w.Workers), ctx: ctx, engine: engine}
	for i := range t.workers {
		worker := &t.workers[i]
		worker.decoded = w.parallelBuffers[i].decoded
		worker.sums = w.parallelBuffers[i].sums
		if i == len(t.workers)-1 {
			continue
		}
		t.joined.Add(1)
		go func() {
			defer t.joined.Done()
			var seen uint64
			idle := 0
			for !t.stop.Load() {
				g := t.generation.Load()
				if g == seen {
					idle++
					if idle%128 == 0 {
						runtime.Gosched()
					}
					continue
				}
				seen = g
				idle = 0
				j := worker.job
				if j.start < j.end {
					worker.err = j.matrix.projectPanels(ctx, j.dst, j.n, j.nt, worker.decoded, j.packed, worker.sums, engine, j.start, j.end)
				} else {
					worker.err = nil
				}
				t.done.Add(1)
			}
		}()
	}
	return t
}
func (t *projectionTeam) close() {
	if t == nil {
		return
	}
	t.stop.Store(true)
	t.joined.Wait()
}
func (t *projectionTeam) project(m *Matrix, dst, x []float32, n int, w *Workspace) error {
	if t == nil || n < 8 || m.rows < 64 {
		return m.projectPackedLocked(tContext(t, w), dst, x, n, w.scratch, w.tilePacked, w.tileSums, tEngine(t, w))
	}
	if n < 1 || n > projectionTile || len(dst) != n*m.rows || len(x) != n*m.columns {
		return fmt.Errorf("parallel projection bounds")
	}
	if err := t.ctx.Err(); err != nil {
		return err
	}
	nt := (n + 7) &^ 7
	packed := w.tilePacked[:nt*m.columns]
	clear(packed)
	packBlocked(x, packed, m.columns, n, nt)
	panels := (m.rows + 7) / 8
	step := (panels + len(t.workers) - 1) / len(t.workers)
	for i := range t.workers {
		start := min(m.rows, i*step*8)
		end := min(m.rows, start+step*8)
		t.workers[i].job = projectionJob{m, dst, packed, n, nt, start, end}
	}
	t.done.Store(0)
	t.generation.Add(1)
	last := &t.workers[len(t.workers)-1]
	j := last.job
	if j.start < j.end {
		last.err = m.projectPanels(t.ctx, dst, n, nt, last.decoded, packed, last.sums, t.engine, j.start, j.end)
	} else {
		last.err = nil
	}
	idle := 0
	for t.done.Load() != uint64(len(t.workers)-1) {
		idle++
		if idle%128 == 0 {
			runtime.Gosched()
		}
	}
	for i := range t.workers {
		if err := t.workers[i].err; err != nil {
			return err
		}
	}
	return t.ctx.Err()
}
func tContext(t *projectionTeam, w *Workspace) context.Context {
	if t != nil {
		return t.ctx
	}
	return w.requestContext
}
func tEngine(t *projectionTeam, w *Workspace) simd.Engine {
	if t != nil {
		return t.engine
	}
	return w.requestEngine
}
