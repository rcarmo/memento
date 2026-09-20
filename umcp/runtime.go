package umcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type cancellation struct{ cancelled atomic.Bool }
type requestRuntime struct {
	progressToken json.RawMessage
	cancellation  *cancellation
	notify        func(string, map[string]any) error
}
type runtimeKey struct{}

// Registry preserves the source's server-scoped request/progress cancellation
// mapping. Transports must use the appropriate server/session security boundary.
// The zero value is usable; do not copy a Registry after use.
type Registry struct {
	mu         sync.Mutex
	byID       map[string]*cancellation
	byProgress map[string]map[string]struct{}
}

func rpcKey(raw json.RawMessage) string {
	value, ok := decode(raw)
	if !ok || !validID(value) || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return "s:" + text
	}
	// JSON integer -0 and 0 are the same Python integer dictionary key.
	n, _ := new(big.Int).SetString(value.(json.Number).String(), 10)
	return "i:" + n.String()
}

func (r *Registry) register(id, token json.RawMessage) *cancellation {
	state := &cancellation{}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := rpcKey(id)
	if key != "" {
		if r.byID == nil {
			r.byID = make(map[string]*cancellation)
			r.byProgress = make(map[string]map[string]struct{})
		}
		r.byID[key] = state
		if progress := rpcKey(token); progress != "" {
			if r.byProgress[progress] == nil {
				r.byProgress[progress] = make(map[string]struct{})
			}
			r.byProgress[progress][key] = struct{}{}
		}
	}
	return state
}

func (r *Registry) cleanup(id, token json.RawMessage, state *cancellation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := rpcKey(id)
	if r.byID[key] == state {
		delete(r.byID, key)
	}
	if progress := rpcKey(token); progress != "" && key != "" {
		delete(r.byProgress[progress], key)
		if len(r.byProgress[progress]) == 0 {
			delete(r.byProgress, progress)
		}
	}
}

// Cancel marks both direct request IDs and requests grouped under a progress token.
// Null and malformed IDs are ignored. It never forcibly interrupts committed work.
func (r *Registry) Cancel(raw json.RawMessage) {
	key := rpcKey(raw)
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if state := r.byID[key]; state != nil {
		state.cancelled.Store(true)
	}
	for id := range r.byProgress[key] {
		if state := r.byID[id]; state != nil {
			state.cancelled.Store(true)
		}
	}
}

// ProgressToken returns an isolated token from the active runtime or request context.
func ProgressToken(ctx context.Context) json.RawMessage {
	if runtime, ok := ctx.Value(runtimeKey{}).(requestRuntime); ok {
		return append(json.RawMessage(nil), runtime.progressToken...)
	}
	return Context(ctx).ProgressToken
}

// IsRequestCancelled reports cooperative uMCP cancellation, not unrelated deadlines.
func IsRequestCancelled(ctx context.Context) bool {
	runtime, _ := ctx.Value(runtimeKey{}).(requestRuntime)
	return runtime.cancellation != nil && runtime.cancellation.cancelled.Load()
}

// CheckCancelled is the handler checkpoint equivalent of raise_if_cancelled.
func CheckCancelled(ctx context.Context) error {
	if IsRequestCancelled(ctx) {
		return ErrRequestCancelled
	}
	return nil
}

func progressValue(name string, value any) (float64, error) {
	var number float64
	switch v := value.(type) {
	case int:
		number = float64(v)
	case int64:
		number = float64(v)
	case float64:
		number = v
	case float32:
		number = float64(v)
	case json.Number:
		var err error
		number, err = strconv.ParseFloat(v.String(), 64)
		if err != nil {
			return 0, fmt.Errorf("Invalid progress notification: '%s' must be a finite non-negative number", name)
		}
	default:
		return 0, fmt.Errorf("Invalid progress notification: '%s' must be a finite non-negative number", name)
	}
	if math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
		return 0, fmt.Errorf("Invalid progress notification: '%s' must be a finite non-negative number", name)
	}
	return number, nil
}

// Preserve Python integer comparisons above float64's exact-integer range;
// mixed float/int comparisons use the float's exact binary rational value.
func exactProgress(value any, approximate float64) *big.Rat {
	switch v := value.(type) {
	case int:
		return new(big.Rat).SetInt64(int64(v))
	case int64:
		return new(big.Rat).SetInt64(v)
	case json.Number:
		if integer.MatchString(v.String()) {
			n, _ := new(big.Int).SetString(v.String(), 10)
			return new(big.Rat).SetInt(n)
		}
	}
	return new(big.Rat).SetFloat64(approximate)
}

// NotifyProgress mirrors notify_progress for typed Go callers. The optional
// message is a string; a nil total omits it. Transport notification errors propagate.
func NotifyProgress(ctx context.Context, progress, total any, message *string) error {
	token := ProgressToken(ctx)
	if len(token) == 0 || string(token) == "null" {
		return nil
	}
	p, err := progressValue("progress", progress)
	if err != nil {
		return err
	}
	params := map[string]any{"progressToken": token, "progress": progress}
	if total != nil {
		t, err := progressValue("total", total)
		if err != nil {
			return err
		}
		if exactProgress(total, t).Cmp(exactProgress(progress, p)) < 0 {
			return fmt.Errorf("Invalid progress notification: 'total' must be greater than or equal to 'progress'")
		}
		params["total"] = total
	}
	if message != nil {
		text := []rune(strings.ReplaceAll(*message, "\x00", ""))
		if len(text) > 4096 {
			text = text[:4096]
		}
		if len(text) > 0 {
			params["message"] = string(text)
		}
	}
	runtime, _ := ctx.Value(runtimeKey{}).(requestRuntime)
	if runtime.notify != nil {
		return runtime.notify("notifications/progress", params)
	}
	return nil
}
