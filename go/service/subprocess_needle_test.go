package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/needleworker"
)

func preparedNeedleSidecar(t *testing.T) (string, []byte, []byte) {
	t.Helper()
	config := []byte(`{}`)
	raw := make([]byte, 4100)
	copy(raw, "NFP32LE\x00")
	binary.LittleEndian.PutUint32(raw[8:12], 1)
	headerLength := 96 + len(config) + 24 + 1 + 4
	binary.LittleEndian.PutUint32(raw[12:16], uint32(headerLength))
	binary.LittleEndian.PutUint64(raw[16:24], 4096)
	binary.LittleEndian.PutUint32(raw[24:28], 1)
	binary.LittleEndian.PutUint32(raw[28:32], uint32(len(config)))
	digest := sha256.Sum256(raw[4096:])
	copy(raw[64:96], digest[:])
	copy(raw[96:], config)
	cursor := 96 + len(config)
	binary.LittleEndian.PutUint16(raw[cursor:cursor+2], 1)
	binary.LittleEndian.PutUint16(raw[cursor+2:cursor+4], 1)
	binary.LittleEndian.PutUint64(raw[cursor+8:cursor+16], 4096)
	binary.LittleEndian.PutUint64(raw[cursor+16:cursor+24], 1)
	raw[cursor+24] = 'x'
	binary.LittleEndian.PutUint32(raw[cursor+25:cursor+29], 1)
	path := filepath.Join(t.TempDir(), "model.nfp32")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, config, raw
}

func needleResponse(t *testing.T, response needleworker.Response) []byte {
	t.Helper()
	var raw bytes.Buffer
	if err := needleworker.WriteFrame(&raw, response); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}
func TestSubprocessNeedleClient(t *testing.T) {
	inProcess := inProcessNeedleClient{}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := inProcess.Generate(cancelled, "q", "[]", needle.DefaultGenerationOptions()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := inProcess.Generate(context.Background(), "q", "[]", needle.GenerationOptions{MaxEncoded: -1}); err == nil {
		t.Fatal("in-process")
	}
	config := DefaultNeedleRouterConfig()
	config.FP32ModelPath = filepath.Join(t.TempDir(), "missing")
	if _, err := LoadSubprocessNeedleClient(config); err == nil {
		t.Fatal("missing model")
	}
	config.WorkerMode = "in_process"
	if _, err := LoadSubprocessNeedleClient(config); err == nil {
		t.Fatal("mode")
	}
	config = DefaultNeedleRouterConfig()
	boom := errors.New("boom")
	if _, err := loadSubprocessNeedleClient(config, func(string) (needle.FP32Info, error) { return needle.FP32Info{}, nil }, func(string) error { return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	clientLoaded, err := loadSubprocessNeedleClient(config, func(string) (needle.FP32Info, error) { return needle.FP32Info{}, nil }, func(string) error { return nil })
	if err != nil || clientLoaded.command == nil {
		t.Fatal(clientLoaded, err)
	}
	_, _, prepared := preparedNeedleSidecar(t)
	config = DefaultNeedleRouterConfig()
	config.FP32ModelPath = filepath.Join(t.TempDir(), "corrupt")
	prepared[len(prepared)-1] ^= 1
	if err := os.WriteFile(config.FP32ModelPath, prepared, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSubprocessNeedleClient(config); err == nil {
		t.Fatal("checksum")
	}
	client := &subprocessNeedleClient{Timeout: time.Second}
	client.run = func(context.Context, []byte) ([]byte, []byte, error) {
		return needleResponse(t, needleworker.Response{OK: true, Output: "result"}), nil, nil
	}
	got, err := client.Generate(context.Background(), "query", "[]", needle.DefaultGenerationOptions())
	if err != nil || got != "result" {
		t.Fatal(got, err)
	}
	client.run = func(context.Context, []byte) ([]byte, []byte, error) {
		return needleResponse(t, needleworker.Response{Error: "worker error"}), nil, nil
	}
	if _, err = client.Generate(context.Background(), "query", "[]", needle.DefaultGenerationOptions()); err == nil || err.Error() != "worker error" {
		t.Fatal(err)
	}
	client.run = func(context.Context, []byte) ([]byte, []byte, error) { return []byte("bad"), nil, nil }
	if _, err = client.Generate(context.Background(), "query", "[]", needle.DefaultGenerationOptions()); err == nil {
		t.Fatal("bad frame")
	}
	client.run = func(context.Context, []byte) ([]byte, []byte, error) {
		return append(needleResponse(t, needleworker.Response{OK: true}), 1), nil, nil
	}
	if _, err = client.Generate(context.Background(), "query", "[]", needle.DefaultGenerationOptions()); err == nil {
		t.Fatal("trailing")
	}
	client.run = func(context.Context, []byte) ([]byte, []byte, error) {
		return needleResponse(t, needleworker.Response{}), nil, nil
	}
	if _, err = client.Generate(context.Background(), "query", "[]", needle.DefaultGenerationOptions()); err == nil || err.Error() != "Needle worker failed" {
		t.Fatal(err)
	}
	client.run = func(context.Context, []byte) ([]byte, []byte, error) { return nil, []byte("stderr"), boom }
	if _, err = client.Generate(context.Background(), "query", "[]", needle.DefaultGenerationOptions()); !errors.Is(err, boom) || !strings.Contains(err.Error(), "stderr") {
		t.Fatal(err)
	}
	client.Timeout = time.Nanosecond
	client.run = func(ctx context.Context, _ []byte) ([]byte, []byte, error) { <-ctx.Done(); return nil, nil, ctx.Err() }
	if _, err = client.Generate(context.Background(), "query", "[]", needle.DefaultGenerationOptions()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if _, err = client.Generate(context.Background(), "", "[]", needle.DefaultGenerationOptions()); err == nil {
		t.Fatal("invalid request")
	}
}
func TestSubprocessNeedleExecFailure(t *testing.T) {
	worker := filepath.Join(t.TempDir(), "worker")
	if err := os.WriteFile(worker, []byte("#!/bin/sh\necho failure >&2\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	client := subprocessNeedleClient{WorkerPath: worker, ModelPath: "model", TokenizerPath: "tokenizer", Timeout: time.Second, command: execCommandContext}
	if _, err := client.Generate(context.Background(), "query", "[]", needle.DefaultGenerationOptions()); err == nil {
		t.Fatal("worker failure")
	}
}

var execCommandContext = exec.CommandContext
