package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/rcarmo/memento/go/service"
	"github.com/rcarmo/memento/go/umcp"
	"io"
	"os"
	"strings"
	"testing"
)

func stubRuntime(t *testing.T) (*service.Runtime, *umcp.Server) {
	t.Helper()
	config := service.RuntimeConfig{}
	config.Repository.RootPath = t.TempDir()
	runtime, server, err := service.BuildModelsOffRuntime(context.Background(), config, service.ModelsOffRuntimeOptions{Surface: "standard", Tokens: []service.BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	return runtime, server
}
func TestRunSyntax(t *testing.T) {
	for _, args := range [][]string{nil, {"serve"}, {"version", "extra"}, {"--config", "", "serve"}} {
		var out, stderr bytes.Buffer
		if code := runContext(context.Background(), args, nil, &out, &stderr); code != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
			t.Fatal(args, code, out.String(), stderr.String())
		}
	}
	var out, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"version"}, nil, &out, &stderr); code != 0 || !strings.Contains(out.String(), "models-off service") || stderr.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
}
func TestRunServe(t *testing.T) {
	oldLoad, oldBuild, oldRun := loadConfig, buildRuntime, runServer
	t.Cleanup(func() { loadConfig, buildRuntime, runServer = oldLoad, oldBuild, oldRun })
	loadConfig = func(string) (service.RuntimeConfig, error) {
		var c service.RuntimeConfig
		c.MCP = service.MCPConfig{ToolSurface: "standard", MaxRequestBytes: 4194304, AllowedOrigins: []string{"https://b.example", " http://a.example "}, Execute: service.MCPExecuteConfig{MaxOperations: 1, MaxIntermediates: 1, MaxRecords: 1, MaxOutputBytes: 512, MaxTimeSeconds: 1}}
		return c, nil
	}
	runtime, server := stubRuntime(t)
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	var got []string
	runServer = func(_ context.Context, _ *umcp.Server, args []string, _ io.Reader, _ io.Writer, _ umcp.HTTPHooks) error {
		got = append([]string{}, args...)
		return nil
	}
	if code := runContext(context.Background(), []string{"--config", "x", "serve"}, nil, io.Discard, io.Discard); code != 0 || strings.Join(got, " ") != "--http --host 127.0.0.1 --port 8000 --endpoint /mcp --max-request-bytes 4194304 --allowed-origin http://a.example --allowed-origin https://b.example" {
		t.Fatal(code, got)
	}
	runtime, server = stubRuntime(t)
	if code := runContext(context.Background(), []string{"--config", "x", "serve", "--tcp", "--port", "1", "--max-request-bytes", "9", "--allowed-origin", "https://override"}, nil, io.Discard, io.Discard); code != 0 || strings.Join(got, " ") != "--tcp --port 1 --max-request-bytes 9 --allowed-origin https://override" {
		t.Fatal(code, got)
	}
}
func TestDefaultRunServer(t *testing.T) {
	runtime, server := stubRuntime(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runServer(ctx, server, []string{"--tcp", "--port", "0"}, nil, io.Discard, umcp.HTTPHooks{}); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	_ = runtime.Close(context.Background())
}

func TestRunCancellation(t *testing.T) {
	oldLoad, oldBuild, oldRun := loadConfig, buildRuntime, runServer
	t.Cleanup(func() { loadConfig, buildRuntime, runServer = oldLoad, oldBuild, oldRun })
	loadConfig = func(string) (service.RuntimeConfig, error) { return service.RuntimeConfig{}, nil }
	runtime, server := stubRuntime(t)
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runServer = func(context.Context, *umcp.Server, []string, io.Reader, io.Writer, umcp.HTTPHooks) error {
		return context.Canceled
	}
	if code := runContext(ctx, []string{"--config", "x", "serve"}, nil, io.Discard, io.Discard); code != 0 {
		t.Fatal(code)
	}
}

func TestRunFailures(t *testing.T) {
	oldLoad, oldBuild, oldRun := loadConfig, buildRuntime, runServer
	t.Cleanup(func() { loadConfig, buildRuntime, runServer = oldLoad, oldBuild, oldRun })
	boom := errors.New("boom")
	loadConfig = func(string) (service.RuntimeConfig, error) { return service.RuntimeConfig{}, boom }
	var stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", "x", "serve"}, nil, io.Discard, &stderr); code != 1 || !strings.Contains(stderr.String(), "boom") {
		t.Fatal(code, stderr.String())
	}
	loadConfig = func(string) (service.RuntimeConfig, error) { return service.RuntimeConfig{}, nil }
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return nil, nil, boom
	}
	if code := runContext(context.Background(), []string{"--config", "x", "serve"}, nil, io.Discard, io.Discard); code != 1 {
		t.Fatal(code)
	}
	runtime, server := stubRuntime(t)
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	runServer = func(context.Context, *umcp.Server, []string, io.Reader, io.Writer, umcp.HTTPHooks) error { return boom }
	if code := runContext(context.Background(), []string{"--config", "x", "serve"}, nil, io.Discard, io.Discard); code != 1 {
		t.Fatal(code)
	}
	runtime, server = stubRuntime(t)
	runtime.Closers = []func() error{func() error { return boom }}
	runServer = func(context.Context, *umcp.Server, []string, io.Reader, io.Writer, umcp.HTTPHooks) error { return nil }
	var shutdown bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", "x", "serve"}, nil, io.Discard, &shutdown); code != 1 || !strings.Contains(shutdown.String(), "shutdown: boom") {
		t.Fatal(code, shutdown.String())
	}
}
func TestRunWrapperAndMain(t *testing.T) {
	oldArgs, oldExit, oldLoad := os.Args, exit, loadConfig
	t.Cleanup(func() { os.Args, exit, loadConfig = oldArgs, oldExit, oldLoad })
	var out, stderr bytes.Buffer
	if code := run([]string{"version"}, &out, &stderr); code != 0 {
		t.Fatal(code)
	}
	os.Args = []string{"memento-go", "version"}
	called := false
	exit = func(code int) {
		called = true
		if code != 0 {
			t.Fatal(code)
		}
	}
	main()
	if !called {
		t.Fatal("exit")
	}
}
