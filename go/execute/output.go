package execute

import (
	"bytes"
	"encoding/json"

	"github.com/rcarmo/memento/go/internal/pyjson"
)

// PayloadSize uses Python's sorted, ASCII-escaped compact JSON byte count. The
// limit excludes the outer success envelope, exactly as in MemoryExecutor.
func PayloadSize(payload map[string]any) (int, error) {
	raw, err := pyjson.Dumps(payload)
	if err != nil {
		return 0, err
	}
	var compact bytes.Buffer
	_ = json.Compact(&compact, []byte(raw))
	return compact.Len(), nil
}
func EnsureOutputSize(payload map[string]any, maxBytes int, committed bool) error {
	size, err := PayloadSize(payload)
	if err != nil {
		return err
	}
	if size <= maxBytes || committed {
		return nil
	}
	return valueError("plan exceeded configured max_output_bytes")
}

// FitOutputPayload never mutates the input. A committed plan progressively loses
// result detail while preserving reconciliation revisions as long as possible.
func FitOutputPayload(payload map[string]any, maxBytes int, committed bool) (map[string]any, error) {
	size, err := PayloadSize(payload)
	if err != nil {
		return nil, err
	}
	if size <= maxBytes {
		out := map[string]any{}
		for key, v := range payload {
			out[key] = v
		}
		return out, nil
	}
	if !committed {
		return nil, valueError("plan exceeded configured max_output_bytes")
	}
	raw, _ := pyjson.Dumps(payload)
	value, _ := pyjson.Parse(raw) // already successfully encoded/validated above
	fitted := value.(map[string]any)
	fitted["truncated"] = true
	// These keys are runner-owned lists. Synthetic malformed values cannot come
	// from run(); skip them rather than fabricate arbitrary Python iterables.
	trace, _ := fitted["trace"].([]any)
	for _, entry := range trace {
		if row, ok := entry.(map[string]any); ok {
			if _, exists := row["data"]; exists {
				row["data"] = map[string]any{"truncated": true}
			}
		}
	}
	fits := func() bool { size, _ := PayloadSize(fitted); return size <= maxBytes } // JSON values remain valid
	if fits() {
		return fitted, nil
	}
	fitted["returns"] = map[string]any{"truncated": true}
	if fits() {
		return fitted, nil
	}
	if len(trace) > 4 {
		fitted["trace"] = []any{trace[0], map[string]any{"truncated": true}, trace[len(trace)-2], trace[len(trace)-1]}
	}
	if fits() {
		return fitted, nil
	}
	revisions, _ := fitted["revisions"].([]any)
	if len(revisions) > 2 {
		fitted["revisions"] = []any{revisions[len(revisions)-1]}
	}
	if fits() {
		return fitted, nil
	}
	fitted["trace"] = []any{map[string]any{"truncated": true}}
	fitted["returns"] = map[string]any{"truncated": true}
	if len(revisions) > 0 {
		fitted["revisions"] = []any{revisions[len(revisions)-1]}
	} else {
		fitted["revisions"] = []any{}
	}
	if fits() {
		return fitted, nil
	}
	return nil, valueError("plan exceeded configured max_output_bytes")
}
