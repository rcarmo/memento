package embeddinggemma

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func TestInspect(t *testing.T) {
	p := fixture(t, nil)
	var b bytes.Buffer
	if err := Inspect(&b, p); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(b.Bytes(), &result); err != nil || result["status"] == nil {
		t.Fatal(err)
	}
	if err := Inspect(failedWriter{}, p); err == nil {
		t.Fatal("write error swallowed")
	}
	if err := Inspect(nil, p); err == nil {
		t.Fatal("nil writer")
	}
	if err := Inspect(&b, "missing"); err == nil {
		t.Fatal("missing file")
	}
}
