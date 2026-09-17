package umcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRegistryGroupingAndCleanup(t *testing.T) {
	var registry Registry
	ids := []json.RawMessage{json.RawMessage(`1`), json.RawMessage(`"1"`), json.RawMessage(`2`)}
	token := json.RawMessage(`"group"`)
	a := registry.register(ids[0], token)
	b := registry.register(ids[1], token)
	c := registry.register(ids[2], nil)
	registry.Cancel(token)
	if !a.cancelled.Load() || !b.cancelled.Load() || c.cancelled.Load() {
		t.Fatal("progress group mismatch")
	}
	registry.Cancel(ids[2])
	if !c.cancelled.Load() {
		t.Fatal("direct cancellation missing")
	}
	registry.cleanup(ids[0], token, a)
	registry.cleanup(ids[1], token, b)
	registry.cleanup(ids[2], nil, c)
	if len(registry.byID) != 0 || len(registry.byProgress) != 0 {
		t.Fatal("registry leaked")
	}
	for _, id := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`false`), json.RawMessage(`[]`), json.RawMessage(`{`)} {
		registry.Cancel(id)
		s := registry.register(id, nil)
		registry.cleanup(id, nil, s)
	}
	zero := registry.register(json.RawMessage(`-0`), nil)
	registry.Cancel(json.RawMessage(`0`))
	if !zero.cancelled.Load() {
		t.Fatal("integer -0 mismatch")
	}
	// An old completion must not remove the cancellation state for a newer duplicate ID.
	first := registry.register(ids[0], token)
	second := registry.register(ids[0], token)
	registry.cleanup(ids[0], token, first)
	registry.Cancel(ids[0])
	if !second.cancelled.Load() {
		t.Fatal("old cleanup removed current request")
	}
	registry.cleanup(ids[0], token, second)
}

func TestCancellationDuringHandlerAndContextCleanup(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	d := Dispatcher{Handlers: map[string]Handler{"wait": func(ctx context.Context, p map[string]any) (any, *RPCError, error) {
		if string(ProgressToken(ctx)) != `"p"` {
			t.Error("missing token")
		}
		if err := CheckCancelled(ctx); err != nil {
			t.Error(err)
		}
		close(entered)
		<-release
		return nil, nil, CheckCancelled(ctx)
	}}}
	done := make(chan *Response, 1)
	go func() {
		r, err := d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":123,"method":"wait","params":{"_meta":{"progressToken":"p"}}}`), RequestContext{})
		if err != nil {
			t.Error(err)
		}
		done <- r
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}
	_, err := d.Process(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"p"}}`), RequestContext{})
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case r := <-done:
		if r == nil || r.Error == nil || r.Error.Code != -32800 {
			t.Fatal(r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not stop")
	}
	if len(d.registry.byID) != 0 || len(d.registry.byProgress) != 0 {
		t.Fatal("context registry leaked")
	}
	if IsRequestCancelled(context.Background()) || CheckCancelled(context.Background()) != nil {
		t.Fatal("cancellation leaked")
	}
}

func TestProgressValidationAndOutput(t *testing.T) {
	base := context.WithValue(context.Background(), requestContextKey{}, RequestContext{ProgressToken: json.RawMessage(`"context-token"`)})
	if string(ProgressToken(base)) != `"context-token"` {
		t.Fatal("context fallback")
	}
	if err := NotifyProgress(base, 1, nil, nil); err != nil {
		t.Fatal(err)
	}
	token := json.RawMessage(`"runtime-token"`)
	notifications := []map[string]any{}
	ctx := context.WithValue(base, runtimeKey{}, requestRuntime{progressToken: token, notify: func(method string, p map[string]any) error {
		if method != "notifications/progress" {
			t.Error(method)
		}
		notifications = append(notifications, p)
		return nil
	}})
	cloned := ProgressToken(ctx)
	cloned[0] = 'x'
	if string(ProgressToken(ctx)) != string(token) {
		t.Fatal("mutable token")
	}
	message := "zero\x00 removed"
	if err := NotifyProgress(ctx, 1, 2, &message); err != nil {
		t.Fatal(err)
	}
	if notifications[0]["message"] != "zero removed" || notifications[0]["progress"] != 1 || notifications[0]["total"] != 2 {
		t.Fatal(notifications)
	}
	long := strings.Repeat("界", 4098)
	if err := NotifyProgress(ctx, int64(0), nil, &long); err != nil {
		t.Fatal(err)
	}
	if len([]rune(notifications[1]["message"].(string))) != 4096 {
		t.Fatal("message not rune bounded")
	}
	empty := "\x00"
	if err := NotifyProgress(ctx, float32(0), float64(1), &empty); err != nil {
		t.Fatal(err)
	}
	if _, ok := notifications[2]["message"]; ok {
		t.Fatal("empty message emitted")
	}
	for _, valid := range []any{json.Number("1"), json.Number("1.5"), float32(1), float64(1), int64(1), 1} {
		if err := NotifyProgress(ctx, valid, 2, nil); err != nil {
			t.Error(err)
		}
	}
	for _, bad := range []any{true, "1", nil, -1, math.NaN(), math.Inf(1), json.Number("bad"), json.Number("1e999")} {
		if err := NotifyProgress(ctx, bad, nil, nil); err == nil {
			t.Errorf("accepted bad progress %v", bad)
		}
		if err := NotifyProgress(ctx, 0, bad, nil); bad != nil && err == nil {
			t.Errorf("accepted bad total %v", bad)
		}
	}
	if err := NotifyProgress(ctx, 2, 1, nil); err == nil || err.Error() != "Invalid progress notification: 'total' must be greater than or equal to 'progress'" {
		t.Fatal(err)
	}
	if err := NotifyProgress(context.Background(), true, nil, nil); err != nil {
		t.Fatal("no-token call validates unexpectedly")
	}
	nullCtx := context.WithValue(base, runtimeKey{}, requestRuntime{progressToken: json.RawMessage("null")})
	if err := NotifyProgress(nullCtx, true, nil, nil); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("transport failed")
	failCtx := context.WithValue(base, runtimeKey{}, requestRuntime{progressToken: token, notify: func(string, map[string]any) error { return boom }})
	if err := NotifyProgress(failCtx, 1, nil, nil); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}

func TestConcurrentRegistry(t *testing.T) {
	var registry Registry
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := json.RawMessage(fmt.Sprint(i))
			token := json.RawMessage(`"common"`)
			s := registry.register(id, token)
			registry.Cancel(token)
			registry.cleanup(id, token, s)
		}(i)
	}
	wg.Wait()
	if len(registry.byID) != 0 || len(registry.byProgress) != 0 {
		t.Fatal("leak")
	}
}
