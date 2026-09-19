// Command memento-go runs the verified pure-Go models-off Memento service.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/rcarmo/memento/go/service"
	"github.com/rcarmo/memento/go/umcp"
)

const version = "memento-go development (compatibility baseline: 0.5.9; models-off service)"

var exit = os.Exit
var loadConfig = service.LoadRuntimeConfig
var buildRuntime = service.BuildModelsOffRuntime
var runServer = func(ctx context.Context, server *umcp.Server, args []string, input io.Reader, output io.Writer, hooks umcp.HTTPHooks) error {
	return server.RunTransport(ctx, args, input, output, true, hooks)
}

func hasFlag(args []string, name string) bool {
	for _, arg := range args {
		if arg == name {
			return true
		}
	}
	return false
}
func usage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: memento-go --config PATH serve [uMCP transport options]")
	fmt.Fprintln(stderr, "       memento-go --config PATH status|rebuild-index")
	fmt.Fprintln(stderr, "       memento-go version")
}
func runContext(ctx context.Context, args []string, input io.Reader, out, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(out, version)
		return 0
	}
	if len(args) < 3 || args[0] != "--config" || args[1] == "" || (args[2] != "serve" && args[2] != "status" && args[2] != "rebuild-index") || (args[2] != "serve" && len(args) != 3) {
		usage(stderr)
		return 2
	}
	config, err := loadConfig(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "memento-go:", err)
		return 1
	}
	limits := config.MCP.ExecuteLimits()
	semanticConfig, configErr := service.DecodeSemanticSearchConfig(config.IntelligentTiers.SemanticSearch, os.LookupEnv)
	if configErr != nil {
		fmt.Fprintln(stderr, "memento-go:", configErr)
		return 1
	}
	needleConfig, configErr := service.DecodeNeedleRouterConfig(config.IntelligentTiers.NeedleRouter)
	if configErr != nil {
		fmt.Fprintln(stderr, "memento-go:", configErr)
		return 1
	}
	runtime, server, err := buildRuntime(ctx, config, service.ModelsOffRuntimeOptions{Surface: config.MCP.ToolSurface, Limits: limits, Graph: config.Observability.GraphExplorer.HTTPConfig(), Needle: needleConfig, Semantic: semanticConfig})
	if err != nil {
		fmt.Fprintln(stderr, "memento-go:", err)
		return 1
	}
	if args[2] == "status" || args[2] == "rebuild-index" {
		var payload map[string]any
		var statusErr error
		if args[2] == "status" {
			payload, statusErr = runtime.StatusSnapshot(ctx, config.SchemaVersion)
		} else {
			payload, statusErr = runtime.RebuildIndex(ctx)
		}
		closeErr := runtime.Close(context.Background())
		if statusErr == nil {
			encoder := json.NewEncoder(out)
			encoder.SetIndent("", "  ")
			statusErr = encoder.Encode(payload)
		}
		if statusErr != nil {
			fmt.Fprintln(stderr, "memento-go:", statusErr)
		}
		if closeErr != nil {
			fmt.Fprintln(stderr, "memento-go: shutdown:", closeErr)
		}
		if statusErr != nil || closeErr != nil {
			return 1
		}
		return 0
	}
	transportArgs := append([]string{}, args[3:]...)
	if len(transportArgs) == 0 {
		transportArgs = []string{"--http", "--host", "127.0.0.1", "--port", "8000", "--endpoint", "/mcp"}
	}
	if !hasFlag(transportArgs, "--max-request-bytes") {
		transportArgs = append(transportArgs, "--max-request-bytes", strconv.FormatInt(config.MCP.MaxRequestBytes, 10))
	}
	if len(config.MCP.AllowedOrigins) > 0 && !hasFlag(transportArgs, "--allowed-origin") {
		for _, origin := range config.MCP.NormalizedOrigins() {
			transportArgs = append(transportArgs, "--allowed-origin", origin)
		}
	}
	err = runServer(ctx, server, transportArgs, input, out, runtime.HTTPHooks)
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		err = nil
	}
	closeErr := runtime.Close(context.Background())
	if err != nil {
		fmt.Fprintln(stderr, "memento-go:", err)
	}
	if closeErr != nil {
		fmt.Fprintln(stderr, "memento-go: shutdown:", closeErr)
	}
	if err != nil || closeErr != nil {
		return 1
	}
	return 0
}
func run(args []string, out, stderr io.Writer) int {
	return runContext(context.Background(), args, os.Stdin, out, stderr)
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	exit(runContext(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
