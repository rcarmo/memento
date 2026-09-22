package service

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/embedding"
	"github.com/rcarmo/memento/internal/processnice"
)

const (
	subprocessSemanticHelperEnv      = "MEMENTO_TEST_SUBPROCESS_SEMANTIC_HELPER"
	subprocessSemanticHelperNiceFile = "MEMENTO_TEST_SUBPROCESS_SEMANTIC_NICE_FILE"
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

func TestSubprocessSemanticNiceHelper(t *testing.T) {
	if os.Getenv(subprocessSemanticHelperEnv) != "1" {
		return
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		_, _ = io.WriteString(os.Stderr, err.Error())
		os.Exit(11)
	}
	priority, err := processnice.Resolve(os.LookupEnv)
	if err != nil {
		_, _ = io.WriteString(os.Stderr, err.Error())
		os.Exit(12)
	}
	if err = processnice.Apply(priority); err != nil {
		_, _ = io.WriteString(os.Stderr, err.Error())
		os.Exit(13)
	}
	kernelPriority, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		_, _ = io.WriteString(os.Stderr, err.Error())
		os.Exit(14)
	}
	nice := 20 - kernelPriority
	if path := os.Getenv(subprocessSemanticHelperNiceFile); path != "" {
		if err = os.WriteFile(path, []byte(strconv.Itoa(nice)), 0600); err != nil {
			_, _ = io.WriteString(os.Stderr, err.Error())
			os.Exit(15)
		}
	}
	if _, err = os.Stdout.Write(subprocessFrame(t, 1, 1, []float32{1})); err != nil {
		_, _ = io.WriteString(os.Stderr, err.Error())
		os.Exit(16)
	}
	os.Exit(0)
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
	if err != nil || client.Info.Dimensions != 384 || client.Info.Revision == "" || client.Nice != config.ProgressiveNice {
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
	config.ModelPath = &model
	config.ProgressiveNice = 20
	if _, err = LoadSubprocessSemanticClient(config); err == nil {
		t.Fatal("invalid nice")
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
	client := subprocessSemanticClient{WorkerPath: worker, ModelPath: "model", Info: derived.SemanticModelInfo{Dimensions: 1}, MaxBatch: 1, MaxInputChars: 10, Timeout: time.Second, Nice: 15, command: exec.CommandContext}
	if _, err := client.Embed("x"); err == nil {
		t.Fatal("worker failure")
	}
}

func TestSetCommandEnv(t *testing.T) {
	key := processnice.EnvVar
	values := setCommandEnv([]string{"A=1", key + "=4"}, key, "9")
	if got := commandEnvValue(values, key); got != "9" {
		t.Fatal(got)
	}
	values = setCommandEnv([]string{"A=1"}, key, "7")
	if got := commandEnvValue(values, key); got != "7" {
		t.Fatal(got)
	}
	values = setCommandEnv(nil, key, "5")
	if got := commandEnvValue(values, key); got != "5" {
		t.Fatal(got)
	}
}

func TestSubprocessSemanticChildEffectiveNice(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux nice semantics")
	}
	niceFile := filepath.Join(t.TempDir(), "nice")
	client := subprocessSemanticClient{WorkerPath: os.Args[0], ModelPath: "model", Info: derived.SemanticModelInfo{Dimensions: 1}, MaxBatch: 1, MaxInputChars: 10, Timeout: 5 * time.Second, Nice: 19,
		command: func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestSubprocessSemanticNiceHelper")
			cmd.Env = setCommandEnv(os.Environ(), subprocessSemanticHelperEnv, "1")
			cmd.Env = setCommandEnv(cmd.Env, subprocessSemanticHelperNiceFile, niceFile)
			cmd.Env = setCommandEnv(cmd.Env, processnice.EnvVar, "1")
			return cmd
		},
	}
	values, err := client.Embed("x")
	if err != nil && strings.Contains(err.Error(), "operation not supported") {
		t.Skip("test binary uses cgo; release workers are CGO_ENABLED=0")
	}
	if err != nil || values[0] != 1 {
		t.Fatal(values, err)
	}
	raw, err := os.ReadFile(niceFile)
	if err != nil {
		t.Fatal(err)
	}
	nice, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || nice != 19 {
		t.Fatal(nice, err)
	}
}

func commandEnvValue(values []string, key string) string {
	prefix := key + "="
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return ""
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
