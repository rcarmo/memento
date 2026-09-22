package derived

import (
	"context"
	"sync"
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
func TestSemanticWorkerBatchAdmissionDoesNotPaceChunks(t *testing.T) {
	now := time.Unix(100, 0)
	idle := 10 * time.Second
	free := 10.0
	worker := &SemanticWorker{
		Policy:        SemanticWorkerPolicy{Enabled: true, InteractiveIdle: 5 * time.Second, Delay: 30 * time.Second, CPUBusyLimit: 75},
		IdleSeconds:   func() time.Duration { return idle },
		CPUUsage:      func() *float64 { return &free },
		Now:           func() time.Time { return now },
		wake:          make(chan struct{}, 1),
		lastCompleted: now,
	}
	if reason, _ := worker.pauseForWork(); reason != "pacing" {
		t.Fatal(reason)
	}
	if reason, _ := worker.pauseForBatch(); reason != "" {
		t.Fatal(reason)
	}
	if err := worker.admitChunkBatch(t.Context()); err != nil || worker.pause != nil {
		t.Fatal(err, worker.pause)
	}
	if !worker.lastCompleted.Equal(now) {
		t.Fatal(worker.lastCompleted)
	}

	worker.Policy.Enabled = false
	if reason, _ := worker.pauseForBatch(); reason != "" {
		t.Fatal(reason)
	}
	worker.Policy.Enabled = true
	idle = 0
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { result <- worker.admitChunkBatch(ctx) }()
	deadline := time.Now().Add(time.Second)
	for {
		worker.mu.Lock()
		paused := worker.pause != nil && *worker.pause == "interactive"
		worker.mu.Unlock()
		if paused {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("batch admission did not pause")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	worker.wake <- struct{}{}
	if err := <-result; err != context.Canceled {
		t.Fatal(err)
	}
}

func TestSemanticWorkerProgressiveExecution(t *testing.T) {
	now := time.Unix(100, 0)
	var clockMu sync.RWMutex
	clock := func() time.Time {
		clockMu.RLock()
		defer clockMu.RUnlock()
		return now
	}
	advance := func(duration time.Duration) {
		clockMu.Lock()
		now = now.Add(duration)
		clockMu.Unlock()
	}
	free := 0.0
	index := &semanticWorkerIndex{}
	worker := NewProgressiveSemanticWorker(index, &semanticClientStub{}, SemanticRefreshConfig{}, SemanticWorkerPolicy{Enabled: true, StartupDelay: time.Hour, CPUBusyLimit: 75}, func() time.Duration { return time.Hour }, func() *float64 { return &free }, clock)
	worker.Enqueue("root", "r", []string{"/a"}, false)
	time.Sleep(5 * time.Millisecond)
	if state := worker.State(); state.PauseReason == nil || *state.PauseReason != "startup" {
		t.Fatal(state)
	}
	advance(2 * time.Hour)
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
