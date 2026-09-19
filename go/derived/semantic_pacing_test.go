package derived

import (
	"context"
	"testing"
	"time"
)

func TestSemanticWorkerPacingReasons(t *testing.T) {
	now := time.Unix(100, 0)
	idle := time.Duration(0)
	var cpu *float64
	worker := &SemanticWorker{Policy: SemanticWorkerPolicy{Enabled: true, StartupDelay: 10 * time.Second, InteractiveIdle: 5 * time.Second, Delay: 3 * time.Second, CPUBusyLimit: 75}, IdleSeconds: func() time.Duration { return idle }, CPUUsage: func() *float64 { return cpu }, Now: func() time.Time { return now }, started: now}
	if reason, _ := worker.pauseForWork(); reason != "startup" {
		t.Fatal(reason)
	}
	now = now.Add(11 * time.Second)
	if reason, _ := worker.pauseForWork(); reason != "interactive" {
		t.Fatal(reason)
	}
	idle = 6 * time.Second
	if reason, _ := worker.pauseForWork(); reason != "cpu-sampling" {
		t.Fatal(reason)
	}
	busy := 80.0
	cpu = &busy
	if reason, _ := worker.pauseForWork(); reason != "cpu" {
		t.Fatal(reason)
	}
	free := 10.0
	cpu = &free
	worker.lastCompleted = now
	if reason, _ := worker.pauseForWork(); reason != "pacing" {
		t.Fatal(reason)
	}
	now = now.Add(4 * time.Second)
	if reason, _ := worker.pauseForWork(); reason != "" {
		t.Fatal(reason)
	}
	worker.Policy.Enabled = false
	if reason, _ := worker.pauseForWork(); reason != "" {
		t.Fatal(reason)
	}
}
func TestSemanticWorkerProgressiveExecution(t *testing.T) {
	now := time.Unix(100, 0)
	free := 0.0
	index := &semanticWorkerIndex{}
	worker := NewProgressiveSemanticWorker(index, &semanticClientStub{}, SemanticRefreshConfig{}, SemanticWorkerPolicy{Enabled: true, StartupDelay: time.Hour, CPUBusyLimit: 75}, func() time.Duration { return time.Hour }, func() *float64 { return &free }, func() time.Time { return now })
	worker.Enqueue("root", "r", []string{"/a"}, false)
	time.Sleep(5 * time.Millisecond)
	if state := worker.State(); state.PauseReason == nil || *state.PauseReason != "startup" {
		t.Fatal(state)
	}
	now = now.Add(2 * time.Hour)
	worker.Enqueue("root", "r", nil, false)
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.WaitIdle(waitCtx); err != nil {
		t.Fatal(err)
	}
	if worker.State().Completed != 1 {
		t.Fatal(worker.State())
	}
	worker.Close()
	nilClock := NewProgressiveSemanticWorker(&semanticWorkerIndex{}, &semanticClientStub{}, SemanticRefreshConfig{}, SemanticWorkerPolicy{}, nil, nil, nil)
	nilClock.Close()
	w := &SemanticWorker{wake: make(chan struct{}, 1)}
	w.wake <- struct{}{}
	w.waitWake(time.Hour)
	w.waitWake(0)
}
