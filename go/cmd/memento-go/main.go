// Command memento-go runs the verified pure-Go models-off Memento service.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/rcarmo/memento/go/execute"
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

func usage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: memento-go --config PATH serve [uMCP transport options]")
	fmt.Fprintln(stderr, "       memento-go version")
}
func runContext(ctx context.Context, args []string, input io.Reader, out, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(out, version)
		return 0
	}
	if len(args) < 3 || args[0] != "--config" || args[1] == "" || args[2] != "serve" {
		usage(stderr)
		return 2
	}
	config, err := loadConfig(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "memento-go:", err)
		return 1
	}
	limits := execute.Limits{MaxOperations: 12, MaxIntermediates: 12, MaxRecords: 50, MaxOutputBytes: "65536", MaxTimeSeconds: 3}
	runtime, server, err := buildRuntime(ctx, config, service.ModelsOffRuntimeOptions{Surface: "standard", Limits: limits})
	if err != nil {
		fmt.Fprintln(stderr, "memento-go:", err)
		return 1
	}
	transportArgs := args[3:]
	if len(transportArgs) == 0 {
		transportArgs = []string{"--http", "--host", "127.0.0.1", "--port", "8000", "--endpoint", "/mcp"}
	}
	err = runServer(ctx, server, transportArgs, input, out, runtime.HTTPHooks)
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
