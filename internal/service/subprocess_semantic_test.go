package service

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/embedding"
)

func subprocessFrame(t *testing.T, count, dimensions int, values []float32) []byte {
	t.Helper()
	payload := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(payload[index*4:], math.Float32bits(value))
	}
	headerRaw, err := json.Marshal(embedding.Header{OK: true, Method: "embed_batch", Dimensions: &dimensions, Count: &count, PayloadLength: len(payload)})
	if err != nil {
		t.Fatal(err)
	}
	wire := make([]byte, 8+len(headerRaw)+len(payload))
	binary.LittleEndian.PutUint32(wire, uint32(4+len(headerRaw)+len(payload)))
	binary.LittleEndian.PutUint32(wire[4:], uint32(len(headerRaw)))
	copy(wire[8:], headerRaw)
	copy(wire[8+len(headerRaw):], payload)
	return wire
}

func TestDecodeEmbeddingResponse(t *testing.T) {
	wire := subprocessFrame(t, 2, 2, []float32{1, 2, 3, 4})
	values, err := decodeEmbeddingResponse(wire, 2, 2)
	if err != nil || values[1][1] != 4 {
		t.Fatal(values, err)
	}
	for _, bad := range [][]byte{nil, wire[:7], append([]byte{}, wire...)} {
		if len(bad) == len(wire) {
			binary.LittleEndian.PutUint32(bad, 1)
		}
		if _, err = decodeEmbeddingResponse(bad, 2, 2); err == nil {
			t.Fatal("invalid frame")
		}
	}
	nonfinite := subprocessFrame(t, 1, 1, []float32{float32(math.NaN())})
	if _, err = decodeEmbeddingResponse(nonfinite, 1, 1); err == nil {
		t.Fatal("nonfinite")
	}
}

type hashFailingReader struct{}

func (hashFailingReader) Read([]byte) (int, error) { return 0, errors.New("read") }

func TestLoadSubprocessSemanticClient(t *testing.T) {
	if _, err := readerSHA256(hashFailingReader{}); err == nil {
		t.Fatal("hash read")
	}
	model := filepath.Join(t.TempDir(), "model")
	if err := os.WriteFile(model, []byte("model"), 0600); err != nil {
		t.Fatal(err)
	}
	config := DefaultSemanticSearchConfig()
	config.Enabled = true
	config.ModelPath = &model
	client, err := LoadSubprocessSemanticClient(config)
	if err != nil || client.Info.Dimensions != 384 || client.Info.Revision == "" {
		t.Fatal(client, err)
	}
	config.ModelPath = nil
	if _, err = LoadSubprocessSemanticClient(config); err == nil {
		t.Fatal("model path")
	}
	missing := filepath.Join(t.TempDir(), "missing")
	config.ModelPath = &missing
	if _, err = LoadSubprocessSemanticClient(config); err == nil {
		t.Fatal("missing model")
	}
}

func TestSubprocessSemanticLimits(t *testing.T) {
	client := subprocessSemanticClient{Info: derived.SemanticModelInfo{ModelID: "m", Dimensions: 2}, MaxBatch: 1, MaxInputChars: 2, Timeout: time.Second}
	if values, err := client.EmbedBatch(nil); err != nil || len(values) != 0 {
		t.Fatal(values, err)
	}
	if _, err := client.EmbedBatch([]string{"a", "b"}); err == nil {
		t.Fatal("batch")
	}
	if _, err := client.EmbedBatch([]string{"abc"}); err == nil {
		t.Fatal("input")
	}
	client.MaxInputChars = 10
	client.run = func(context.Context, []byte) ([]byte, []byte, error) {
		return subprocessFrame(t, 1, 2, []float32{1, 2}), nil, nil
	}
	if client.ModelInfo().ModelID != "m" {
		t.Fatal("model info")
	}
	value, err := client.Embed("ok")
	if err != nil || value[1] != 2 {
		t.Fatal(value, err)
	}
	boom := errors.New("boom")
	client.run = func(context.Context, []byte) ([]byte, []byte, error) { return nil, []byte("stderr"), boom }
	if _, err = client.Embed("ok"); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	client.Timeout = time.Nanosecond
	client.run = func(ctx context.Context, _ []byte) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	if _, err = client.EmbedBatch([]string{"ok"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestSubprocessSemanticExecPath(t *testing.T) {
	dir := t.TempDir()
	worker := filepath.Join(dir, "worker")
	if err := os.WriteFile(worker, []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	client := subprocessSemanticClient{WorkerPath: worker, ModelPath: "model", Info: derived.SemanticModelInfo{Dimensions: 1}, MaxBatch: 1, MaxInputChars: 10, Timeout: time.Second, command: exec.CommandContext}
	if _, err := client.Embed("x"); err == nil {
		t.Fatal("worker failure")
	}
}

func TestDecodeEmbeddingResponseFailures(t *testing.T) {
	if _, err := decodeEmbeddingPayload(make([]byte, 4), 4, 1, 2); err == nil {
		t.Fatal("payload shape")
	}
	if _, err := decodeEmbeddingPayload(make([]byte, 4), 3, 1, 1); err == nil {
		t.Fatal("payload header")
	}
	if validatePayloadLength(4, 4, 1, 1) != nil {
		t.Fatal("payload length")
	}
	count, dimensions := 1, 1
	for _, mutate := range []func(*embedding.Header, *[]byte){
		func(h *embedding.Header, _ *[]byte) { h.OK = false; message := "worker"; h.Error = &message },
		func(h *embedding.Header, _ *[]byte) { h.OK = false },
		func(h *embedding.Header, _ *[]byte) { h.Dimensions = nil },
		func(h *embedding.Header, _ *[]byte) { h.Count = nil },
		func(h *embedding.Header, _ *[]byte) { *h.Count = 2 },
		func(h *embedding.Header, _ *[]byte) { h.PayloadLength = 0 },
		func(h *embedding.Header, payload *[]byte) {
			*payload = (*payload)[:len(*payload)-1]
			h.PayloadLength = len(*payload)
		},
	} {
		payload := []byte{0, 0, 128, 63}
		header := embedding.Header{OK: true, Method: "embed_batch", Count: &count, Dimensions: &dimensions, PayloadLength: len(payload)}
		mutate(&header, &payload)
		raw, _ := json.Marshal(header)
		wire := make([]byte, 8+len(raw)+len(payload))
		binary.LittleEndian.PutUint32(wire, uint32(4+len(raw)+len(payload)))
		binary.LittleEndian.PutUint32(wire[4:], uint32(len(raw)))
		copy(wire[8:], raw)
		copy(wire[8+len(raw):], payload)
		if _, err := decodeEmbeddingResponse(wire, 1, 1); err == nil {
			t.Fatal("expected failure")
		}
	}
	wire := subprocessFrame(t, 1, 1, []float32{1})
	headerLength := int(binary.LittleEndian.Uint32(wire[4:]))
	wire[8] = '{'
	for i := 9; i < 8+headerLength; i++ {
		wire[i] = 0xff
	}
	if _, err := decodeEmbeddingResponse(wire, 1, 1); err == nil {
		t.Fatal("invalid json")
	}
}

func TestSubprocessSemanticCharacterLimit(t *testing.T) {
	raw := subprocessFrame(t, 1, 2, []float32{1, 0})
	output := filepath.Join(t.TempDir(), "frame")
	if err := os.WriteFile(output, raw, 0600); err != nil {
		t.Fatal(err)
	}
	client := subprocessSemanticClient{Info: derived.SemanticModelInfo{Dimensions: 2}, MaxBatch: 1, MaxInputChars: 2, Timeout: time.Second,
		command: func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "cat", output)
		},
	}
	if _, err := client.Embed("界😀"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Embed("界😀x"); err == nil {
		t.Fatal("over character limit")
	}
}
