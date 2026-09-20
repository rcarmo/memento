package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/service"
	"github.com/rcarmo/memento/umcp"
)

type cliSurfaceCase struct {
	CaseID     string         `json:"case_id"`
	Surface    string         `json:"surface"`
	Adapter    string         `json:"adapter"`
	Setup      map[string]any `json:"setup"`
	Request    map[string]any `json:"request"`
	Expected   map[string]any `json:"expected"`
	StateDelta map[string]any `json:"state_delta"`
}

func loadCLISurfaceCases(t *testing.T) []cliSurfaceCase {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/parity/python-structured-surface-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SchemaVersion int              `json:"schema_version"`
		PythonCommit  string           `json:"python_commit"`
		Cases         []cliSurfaceCase `json:"cases"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || fixture.PythonCommit != "7f29e8b003557f0105f47ed353b7f65a33619456" || len(fixture.Cases) != 23 {
		t.Fatal(fixture.SchemaVersion, fixture.PythonCommit, len(fixture.Cases))
	}
	out := []cliSurfaceCase{}
	for _, item := range fixture.Cases {
		if item.Adapter == "cli_serve" || item.Adapter == "cli_rotate_master_key" {
			out = append(out, item)
		}
	}
	return out
}
func cliStrings(value any) []string {
	raw := value.([]any)
	out := make([]string, len(raw))
	for i, item := range raw {
		out[i] = item.(string)
	}
	return out
}

func TestPythonStructuredCLISurfaceCases(t *testing.T) {
	cases := loadCLISurfaceCases(t)
	if len(cases) != 2 {
		t.Fatal(len(cases))
	}
	for _, item := range cases {
		item := item
		t.Run(item.CaseID, func(t *testing.T) {
			switch item.Adapter {
			case "cli_serve":
				runStructuredServe(t, item)
			case "cli_rotate_master_key":
				runStructuredRotateMasterKey(t, item)
			default:
				t.Fatal(item.Adapter)
			}
		})
	}
}
func runStructuredServe(t *testing.T, item cliSurfaceCase) {
	oldLoad, oldBuild, oldRun := loadConfig, buildRuntime, runServer
	t.Cleanup(func() { loadConfig, buildRuntime, runServer = oldLoad, oldBuild, oldRun })
	loadConfig = func(string) (service.RuntimeConfig, error) {
		var config service.RuntimeConfig
		config.MCP = service.MCPConfig{ToolSurface: "standard", MaxRequestBytes: int64(item.Setup["max_request_bytes"].(float64))}
		return config, nil
	}
	runtime, server := stubRuntime(t)
	closed := false
	runtime.Closers = append(runtime.Closers, func() error { closed = true; return nil })
	buildRuntime = func(context.Context, service.RuntimeConfig, service.ModelsOffRuntimeOptions) (*service.Runtime, *umcp.Server, error) {
		return runtime, server, nil
	}
	var transport []string
	runServer = func(_ context.Context, _ *umcp.Server, args []string, _ io.Reader, _ io.Writer, _ umcp.HTTPHooks) error {
		transport = append([]string{}, args...)
		return nil
	}
	code := runContext(context.Background(), cliStrings(item.Request["arguments"]), nil, io.Discard, io.Discard)
	if code != int(item.Expected["exit_code"].(float64)) || !closed || !item.StateDelta["runtime_closed"].(bool) || !equalStrings(transport, cliStrings(item.Expected["transport_arguments"])) {
		t.Fatal(code, closed, transport, item.Expected)
	}
}
func runStructuredRotateMasterKey(t *testing.T, item cliSurfaceCase) {
	oldLoad := loadConfig
	t.Cleanup(func() { loadConfig = oldLoad })
	var config service.RuntimeConfig
	config.Repository.RootPath = t.TempDir()
	ctx := context.Background()
	db, err := control.Connect(ctx, service.RuntimePathsFor(config).ControlDB)
	if err != nil {
		t.Fatal(err)
	}
	if err = control.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	oldKey, newKey := item.Setup["previous_key"].(string), item.Setup["new_key"].(string)
	if _, err = access.OpenStore(ctx, db, oldKey); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MEMENTO_ADMIN_PREVIOUS_MASTER_KEY", oldKey)
	t.Setenv("MEMENTO_ADMIN_MASTER_KEY", newKey)
	loadConfig = func(string) (service.RuntimeConfig, error) { return config, nil }
	var out, stderr bytes.Buffer
	code := runContext(ctx, cliStrings(item.Request["arguments"]), nil, &out, &stderr)
	if code != int(item.Expected["exit_code"].(float64)) || stderr.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
	var payload map[string]any
	if err = json.Unmarshal(out.Bytes(), &payload); err != nil || payload["rotated"] != item.Expected["json"].(map[string]any)["rotated"] {
		t.Fatal(payload, err)
	}
	db, err = control.Connect(ctx, service.RuntimePathsFor(config).ControlDB)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = access.OpenStore(ctx, db, newKey); err != nil {
		t.Fatal(err)
	}
	if _, err = access.OpenStore(ctx, db, oldKey); err == nil {
		t.Fatal("old key remained valid")
	}
}
func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
