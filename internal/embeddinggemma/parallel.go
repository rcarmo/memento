package embeddinggemma

import (
	"context"
	"fmt"
	"github.com/rcarmo/memento/internal/simd"
	"sync"
)

// A team belongs to one locked request. Channel publication orders job writes
// before worker reads, and completion receives publish errors and output writes.
// Workers park between projections instead of consuming CPU during serial work.
// All jobs drain before reuse; workers join before releasing the model read lock.
type projectionJob struct {
	matrix            *Matrix
	dst, packed       []float32
	n, nt, start, end int
}
type projectionWorker struct {
	decoded, sums []float32
	job           projectionJob
	err           error
	wake          chan struct{}
}
type projectionTeam struct {
	workers []projectionWorker
	ctx     context.Context
	engine  simd.Engine
	done    chan struct{}
	joined  sync.WaitGroup
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
	t := &projectionTeam{workers: make([]projectionWorker, w.Workers), ctx: ctx, engine: engine, done: make(chan struct{}, w.Workers-1)}
	for i := range t.workers {
		worker := &t.workers[i]
		worker.decoded = w.parallelBuffers[i].decoded
		worker.sums = w.parallelBuffers[i].sums
		if i == len(t.workers)-1 {
			continue
		}
		worker.wake = make(chan struct{}, 1)
		t.joined.Add(1)
		go func() {
			defer t.joined.Done()
			for range worker.wake {
				j := worker.job
				if j.start < j.end {
					worker.err = j.matrix.projectPanels(ctx, j.dst, j.n, j.nt, worker.decoded, j.packed, worker.sums, engine, j.start, j.end)
				} else {
					worker.err = nil
				}
				t.done <- struct{}{}
			}
		}()
	}
	return t
}
func (t *projectionTeam) close() {
	if t == nil {
		return
	}
	for i := 0; i < len(t.workers)-1; i++ {
		close(t.workers[i].wake)
	}
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
	nt := (n + 15) &^ 15
	packed := w.tilePacked[:nt*m.columns]
	clear(packed)
	packWide(x, packed, m.columns, n, nt)
	panels := (m.rows + 7) / 8
	step := (panels + len(t.workers) - 1) / len(t.workers)
	for i := range t.workers {
		start := min(m.rows, i*step*8)
		end := min(m.rows, start+step*8)
		t.workers[i].job = projectionJob{m, dst, packed, n, nt, start, end}
	}
	for i := 0; i < len(t.workers)-1; i++ {
		t.workers[i].wake <- struct{}{}
	}
	last := &t.workers[len(t.workers)-1]
	j := last.job
	if j.start < j.end {
		last.err = m.projectPanels(t.ctx, dst, n, nt, last.decoded, packed, last.sums, t.engine, j.start, j.end)
	} else {
		last.err = nil
	}
	for i := 0; i < len(t.workers)-1; i++ {
		<-t.done
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
