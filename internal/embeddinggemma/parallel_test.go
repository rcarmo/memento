package embeddinggemma

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestParallelReference(t *testing.T) {
	m := realModel(t, "auto")
	raw, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture goldenFixture
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	w := &Workspace{Workers: 4}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			want := append([]float32(nil), c.Embedding[:c.Dimensions]...)
			var norm float64
			for _, x := range want {
				norm += float64(x) * float64(x)
			}
			for i := range want {
				want[i] /= float32(math.Sqrt(norm))
			}
			v, err := m.Embed(context.Background(), c.Text, Options{Role: Role(c.Role), Title: c.Title, Dimensions: c.Dimensions}, w)
			if err != nil {
				t.Fatal(err)
			}
			serial, err := m.Embed(context.Background(), c.Text, Options{Role: Role(c.Role), Title: c.Title, Dimensions: c.Dimensions}, &Workspace{})
			if err != nil {
				t.Fatal(err)
			}
			compareEmbedding(t, v, serial)
			compareEmbedding(t, v, want)
		})
	}
}
func TestParallelCancelClose(t *testing.T) {
	m := realModel(t, "auto")
	ids, _ := m.Tokenize(context.Background(), "task: search result | query: hello world", Options{})
	before := runtime.NumGoroutine()
	w := &Workspace{Workers: 4}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := m.EmbedTokens(ctx, ids, 768, w); done <- err }()
	deadline := time.Now().Add(time.Second)
	for {
		if !w.mu.TryLock() {
			break
		}
		w.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("not started")
		}
		time.Sleep(time.Millisecond)
	}
	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
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
	time.Sleep(10 * time.Millisecond)
	if runtime.NumGoroutine() > before+2 {
		t.Fatal("workers leaked")
	}
	if w.requestContext != nil {
		t.Fatal("retained context")
	}
}
func TestParallelConcurrent(t *testing.T) {
	m := realModel(t, "auto")
	want, err := m.Embed(context.Background(), "hello world", Options{}, &Workspace{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := m.Embed(context.Background(), "hello world", Options{}, &Workspace{Workers: 3})
			if err != nil {
				t.Error(err)
				return
			}
			compareEmbedding(t, v, want)
		}()
	}
	wg.Wait()
	for _, n := range []int{-1, 9} {
		if _, err := m.EmbedTokens(context.Background(), []int{2, 1}, 768, &Workspace{Workers: n}); err == nil {
			t.Fatal(n)
		}
	}
}
