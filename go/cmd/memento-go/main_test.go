package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/service"
	"github.com/rcarmo/memento/go/umcp"
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
	if code := runContext(context.Background(), []string{"version"}, nil, &out, &stderr); code != 0 || out.String() != "memento-go development\n" || stderr.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if code := runContext(context.Background(), []string{"healthcheck", "--address", listener.Addr().String(), "--timeout", "1s"}, nil, io.Discard, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	listener.Close()
	if code := runContext(context.Background(), []string{"healthcheck", "--address", listener.Addr().String(), "--timeout", "1ms"}, nil, io.Discard, &stderr); code != 1 {
		t.Fatal(code)
	}
	for _, args := range [][]string{{"healthcheck", "--bad", "x"}, {"healthcheck", "--timeout", "bad"}, {"healthcheck", "--timeout", "0s"}, {"healthcheck", "--address"}} {
		if code := runContext(context.Background(), args, nil, io.Discard, io.Discard); code != 2 {
			t.Fatal(args, code)
		}
	}
}
func TestHealthcheckCloseFailure(t *testing.T) {
	previous := dialHealthcheck
	t.Cleanup(func() { dialHealthcheck = previous })
	dialHealthcheck = func(string, string, time.Duration) (net.Conn, error) { return closeFailureConn{}, nil }
	if code := runHealthcheck(nil, io.Discard); code != 1 {
		t.Fatal(code)
	}
}

type closeFailureConn struct{ net.Conn }

func (closeFailureConn) Close() error { return errors.New("close") }

func TestEnvironmentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memento.env")
	if err := os.WriteFile(path, []byte("# comment\nexport MEMENTO_TEST_ONE='one two'\nMEMENTO_TEST_TWO=three\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnvironmentFile(path); err != nil || os.Getenv("MEMENTO_TEST_ONE") != "one two" || os.Getenv("MEMENTO_TEST_TWO") != "three" {
		t.Fatal(err)
	}
	for _, raw := range []string{"invalid\n", "BAD NAME=x\n"} {
		bad := filepath.Join(t.TempDir(), "bad.env")
		if err := os.WriteFile(bad, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if err := loadEnvironmentFile(bad); err == nil {
			t.Fatal(raw)
		}
	}
	if err := loadEnvironmentFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing environment file")
	}
	if err := loadEnvironment(strings.NewReader("FAIL=x\n"), func(string, string) error { return errors.New("setenv") }); err == nil {
		t.Fatal("setenv failure")
	}
	for _, args := range [][]string{{"--env-file", "", "--config", "x", "serve"}, {"--env-file", filepath.Join(t.TempDir(), "missing"), "--config", "x", "serve"}} {
		if code := runContext(context.Background(), args, nil, io.Discard, io.Discard); code == 0 {
			t.Fatal(args)
		}
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
	envFile := filepath.Join(t.TempDir(), "memento.env")
	if err := os.WriteFile(envFile, []byte("MEMENTO_TEST_NATIVE=yes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runtime, server = stubRuntime(t)
	if code := runContext(context.Background(), []string{"--env-file", envFile, "--config", "x", "serve"}, nil, io.Discard, io.Discard); code != 0 || os.Getenv("MEMENTO_TEST_NATIVE") != "yes" {
		t.Fatal(code)
	}
	runtime, server = stubRuntime(t)
	if code := runContext(context.Background(), []string{"--config", "x", "serve", "--tcp", "--port", "1", "--max-request-bytes", "9", "--allowed-origin", "https://override"}, nil, io.Discard, io.Discard); code != 0 || strings.Join(got, " ") != "--tcp --port 1 --max-request-bytes 9 --allowed-origin https://override" {
		t.Fatal(code, got)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write") }
func TestRunStatus(t *testing.T) {
	oldLoad, oldBuild, oldNow := loadConfig, buildRuntime, now
	t.Cleanup(func() { loadConfig, buildRuntime, now = oldLoad, oldBuild, oldNow })
	now = func() time.Time { return time.Unix(123, 0) }
	runtime, server := stubRuntime(t)
	loadConfig = func(string) (service.RuntimeConfig, error) {
		var c service.RuntimeConfig
		c.SchemaVersion = 2
		return c, nil
	}
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	var out, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", "x", "status"}, nil, &out, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil || payload["schema_version"] != float64(2) || payload["closed"] != false {
		t.Fatal(payload, err)
	}
	runtime, server = stubRuntime(t)
	out.Reset()
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--config", "x", "status", "--format", "prometheus"}, nil, &out, &stderr); code != 0 || stderr.Len() != 0 || !strings.HasPrefix(out.String(), "# HELP memento_service_up") || !strings.HasSuffix(out.String(), " 1\n") {
		t.Fatal(code, out.String(), stderr.String())
	}
	runtime, server = stubRuntime(t)
	out.Reset()
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--config", "x", "status", "--format", "graphite"}, nil, &out, &stderr); code != 0 || out.String() == "" || !strings.Contains(out.String(), "memento.service_up 1 123") {
		t.Fatal(code, out.String(), stderr.String())
	}
	runtime, server = stubRuntime(t)
	out.Reset()
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--config", "x", "status", "--format", "graphite", "--graphite-prefix", "services.memento"}, nil, &out, &stderr); code != 0 || !strings.Contains(out.String(), "services.memento.proposal_backlog") {
		t.Fatal(code, out.String(), stderr.String())
	}
	runtime, server = stubRuntime(t)
	out.Reset()
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--config", "x", "status", "--format", "json"}, nil, &out, &stderr); code != 0 || !json.Valid(out.Bytes()) {
		t.Fatal(code, out.String(), stderr.String())
	}
	if code := runContext(context.Background(), []string{"--config", "x", "status", "--format", "bad"}, nil, io.Discard, &stderr); code != 2 {
		t.Fatal(code)
	}
	runtime, server = stubRuntime(t)
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--config", "x", "status", "--format", "graphite"}, nil, failingWriter{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "write") {
		t.Fatal(code, stderr.String())
	}
	runtime, server = stubRuntime(t)
	stderr.Reset()
	if code := runContext(context.Background(), []string{"--config", "x", "status", "--format", "graphite", "--graphite-prefix", "bad prefix"}, nil, io.Discard, &stderr); code != 1 {
		t.Fatal(code)
	}
	runtime, server = stubRuntime(t)
	if code := runContext(context.Background(), []string{"--config", "x", "status"}, nil, failingWriter{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "write") {
		t.Fatal(code, stderr.String())
	}
	runtime, server = stubRuntime(t)
	runtime.Closers = []func() error{func() error { return errors.New("close") }}
	if code := runContext(context.Background(), []string{"--config", "x", "status"}, nil, io.Discard, &stderr); code != 1 {
		t.Fatal(code)
	}
}
func TestRunDream(t *testing.T) {
	oldLoad, oldBuild := loadConfig, buildRuntime
	t.Cleanup(func() { loadConfig, buildRuntime = oldLoad, oldBuild })
	runtime, server := stubRuntime(t)
	runtime.Dream = service.DefaultDreamConfig()
	loadConfig = func(string) (service.RuntimeConfig, error) { return service.RuntimeConfig{}, nil }
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	var out, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", "x", "dream", "--mode", "disabled"}, nil, &out, &stderr); code != 0 || !strings.Contains(out.String(), `"state": "disabled"`) {
		t.Fatal(code, out.String(), stderr.String())
	}
	runtime, server = stubRuntime(t)
	loadConfig = func(string) (service.RuntimeConfig, error) {
		return service.RuntimeConfig{IntelligentTiers: service.IntelligentTiersConfig{Dream: []byte(`{"extra":1}`)}}, nil
	}
	if code := runContext(context.Background(), []string{"--config", "x", "dream"}, nil, io.Discard, &stderr); code != 1 {
		t.Fatal(code)
	}
	stderr.Reset()
	loadConfig = func(string) (service.RuntimeConfig, error) { return service.RuntimeConfig{}, nil }
	if code := runContext(context.Background(), []string{"--config", "x", "dream", "--mode", "propose"}, nil, io.Discard, &stderr); code != 1 || !strings.Contains(stderr.String(), "configured dream model provider") {
		t.Fatal(code, stderr.String())
	}
	for _, field := range []string{"ModelProposals", "ModelProviderSlots"} {
		stderr.Reset()
		loadConfig = func(string) (service.RuntimeConfig, error) {
			tiers := service.IntelligentTiersConfig{}
			if field == "ModelProposals" {
				tiers.ModelProposals = []byte(`{"x":1}`)
			} else {
				tiers.ModelProviderSlots = []byte(`{"x":1}`)
			}
			return service.RuntimeConfig{IntelligentTiers: tiers}, nil
		}
		if code := runContext(context.Background(), []string{"--config", "x", "dream"}, nil, io.Discard, &stderr); code != 1 {
			t.Fatal(field, code)
		}
	}
	loadConfig = func(string) (service.RuntimeConfig, error) {
		return service.RuntimeConfig{IntelligentTiers: service.IntelligentTiersConfig{ModelProviderSlots: []byte(`{"dream":{"primary":{"base_url":"http://localhost","api_format":"openai","model":"m"},"allowed_data_classifications":["restricted"]}}`)}}, nil
	}
	buildRuntime = func(_ context.Context, _ service.RuntimeConfig, options service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		if options.ModelClient == nil {
			t.Fatal("client")
		}
		return nil, nil, errors.New("built")
	}
	if code := runContext(context.Background(), []string{"--config", "x", "dream", "--mode", "propose"}, nil, io.Discard, io.Discard); code != 1 {
		t.Fatal(code)
	}
	for _, args := range [][]string{{"--config", "x", "dream", "--mode", "bad"}, {"--config", "x", "dream", "extra"}} {
		if code := runContext(context.Background(), args, nil, io.Discard, io.Discard); code != 2 {
			t.Fatal(args, code)
		}
	}
}
func TestRunRotateMasterKey(t *testing.T) {
	oldLoad := loadConfig
	t.Cleanup(func() { loadConfig = oldLoad })
	ctx := context.Background()
	var config service.RuntimeConfig
	config.Repository.RootPath = t.TempDir()
	db, err := control.Connect(ctx, service.RuntimePathsFor(config).ControlDB)
	if err != nil {
		t.Fatal(err)
	}
	if err = control.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = access.OpenStore(ctx, db, "old"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loadConfig = func(string) (service.RuntimeConfig, error) { return config, nil }
	t.Setenv("MEMENTO_ADMIN_PREVIOUS_MASTER_KEY", "old")
	t.Setenv("MEMENTO_ADMIN_MASTER_KEY", "new")
	var out, stderr bytes.Buffer
	if code := runContext(ctx, []string{"--config", "x", "rotate-master-key"}, nil, &out, &stderr); code != 0 || stderr.Len() != 0 || !strings.Contains(out.String(), `"rotated": true`) {
		t.Fatal(code, out.String(), stderr.String())
	}
	t.Setenv("MEMENTO_ADMIN_PREVIOUS_MASTER_KEY", "wrong")
	if code := runContext(ctx, []string{"--config", "x", "rotate-master-key"}, nil, io.Discard, &stderr); code != 1 {
		t.Fatal(code)
	}
	t.Setenv("MEMENTO_ADMIN_PREVIOUS_MASTER_KEY", "new")
	t.Setenv("MEMENTO_ADMIN_MASTER_KEY", "newer")
	if code := runContext(ctx, []string{"--config", "x", "rotate-master-key"}, nil, failingWriter{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "write") {
		t.Fatal(code, stderr.String())
	}
}
func TestRunBackupRestore(t *testing.T) {
	oldLoad, oldBuild := loadConfig, buildRuntime
	t.Cleanup(func() { loadConfig, buildRuntime = oldLoad, oldBuild })
	runtime, server := stubRuntime(t)
	root := runtime.Paths.Root
	loadConfig = func(string) (service.RuntimeConfig, error) {
		var config service.RuntimeConfig
		config.Repository.RootPath = root
		return config, nil
	}
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	backup := filepath.Join(t.TempDir(), "backup")
	var out, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", "x", "backup", "--output", backup}, nil, &out, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
	var manifest map[string]any
	if err := json.Unmarshal(out.Bytes(), &manifest); err != nil || manifest["schema_version"] != float64(1) {
		t.Fatal(manifest, err)
	}
	out.Reset()
	if code := runContext(context.Background(), []string{"--config", "x", "restore", "--input", backup, "--no-rebuild-derived"}, nil, &out, &stderr); code != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil || payload["rebuild_derived"] != false {
		t.Fatal(payload, err)
	}
	badRoot := filepath.Join(t.TempDir(), "bad-root")
	loadConfig = func(string) (service.RuntimeConfig, error) {
		var config service.RuntimeConfig
		config.Repository.RootPath = badRoot
		return config, nil
	}
	if code := runContext(context.Background(), []string{"--config", "x", "restore", "--input", filepath.Join(t.TempDir(), "missing")}, nil, io.Discard, &stderr); code != 1 || !strings.Contains(stderr.String(), "memento-go:") {
		t.Fatal(code, stderr.String())
	}
	for _, args := range [][]string{{"--config", "x", "backup"}, {"--config", "x", "backup", "--output", ""}, {"--config", "x", "restore"}, {"--config", "x", "restore", "--input", ""}, {"--config", "x", "restore", "--input", backup, "bad"}} {
		if code := runContext(context.Background(), args, nil, io.Discard, io.Discard); code != 2 {
			t.Fatal(args, code)
		}
	}
}
func TestRunAudit(t *testing.T) {
	oldLoad, oldBuild := loadConfig, buildRuntime
	t.Cleanup(func() { loadConfig, buildRuntime = oldLoad, oldBuild })
	runtime, server := stubRuntime(t)
	loadConfig = func(string) (service.RuntimeConfig, error) { return service.RuntimeConfig{}, nil }
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	var out, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", "x", "audit", "--path", "/x.md"}, nil, &out, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil || payload["ok"] != true {
		t.Fatal(payload, err)
	}
	for _, args := range [][]string{{"--config", "x", "audit", "--bad", "x"}, {"--config", "x", "audit", "--path", ""}, {"--config", "x", "audit", "extra"}} {
		if code := runContext(context.Background(), args, nil, io.Discard, io.Discard); code != 2 {
			t.Fatal(args, code)
		}
	}
}
func TestRunRebuildIndex(t *testing.T) {
	oldLoad, oldBuild := loadConfig, buildRuntime
	t.Cleanup(func() { loadConfig, buildRuntime = oldLoad, oldBuild })
	runtime, server := stubRuntime(t)
	loadConfig = func(string) (service.RuntimeConfig, error) { return service.RuntimeConfig{}, nil }
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	var out, stderr bytes.Buffer
	if code := runContext(context.Background(), []string{"--config", "x", "rebuild-index"}, nil, &out, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil || payload["parity_matches"] != true {
		t.Fatal(payload, err)
	}
	runtime, server = stubRuntime(t)
	runtime.Paths.Repository.BareDir = filepath.Join(t.TempDir(), "missing")
	if code := runContext(context.Background(), []string{"--config", "x", "rebuild-index"}, nil, io.Discard, &stderr); code != 1 {
		t.Fatal(code, stderr.String())
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
	loadConfig = func(string) (service.RuntimeConfig, error) {
		return service.RuntimeConfig{IntelligentTiers: service.IntelligentTiersConfig{SemanticSearch: []byte(`{"extra":1}`)}}, nil
	}
	if code := runContext(context.Background(), []string{"--config", "x", "serve"}, nil, io.Discard, io.Discard); code != 1 {
		t.Fatal(code)
	}
	loadConfig = func(string) (service.RuntimeConfig, error) {
		return service.RuntimeConfig{IntelligentTiers: service.IntelligentTiersConfig{NeedleRouter: []byte(`{"extra":1}`)}}, nil
	}
	if code := runContext(context.Background(), []string{"--config", "x", "serve"}, nil, io.Discard, io.Discard); code != 1 {
		t.Fatal(code)
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
