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
	"time"

	"github.com/rcarmo/memento/go/service"
	"github.com/rcarmo/memento/go/umcp"
)

var version = "memento-go development (compatibility baseline: 0.5.9; models-off service)"

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
	fmt.Fprintln(stderr, "       memento-go --config PATH status|rebuild-index|audit [--path PATH]")
	fmt.Fprintln(stderr, "       memento-go --config PATH backup --output DIR")
	fmt.Fprintln(stderr, "       memento-go --config PATH rotate-master-key")
	fmt.Fprintln(stderr, "       memento-go --config PATH dream [--mode disabled|report_only|propose]")
	fmt.Fprintln(stderr, "       memento-go --config PATH restore --input DIR [--no-rebuild-derived]")
	fmt.Fprintln(stderr, "       memento-go version")
}
func runContext(ctx context.Context, args []string, input io.Reader, out, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(out, version)
		return 0
	}
	if len(args) < 3 || args[0] != "--config" || args[1] == "" || (args[2] != "serve" && args[2] != "status" && args[2] != "rebuild-index" && args[2] != "audit" && args[2] != "backup" && args[2] != "restore" && args[2] != "rotate-master-key" && args[2] != "dream") || (args[2] != "serve" && args[2] != "audit" && args[2] != "backup" && args[2] != "restore" && args[2] != "rotate-master-key" && args[2] != "dream" && len(args) != 3) || (args[2] == "audit" && len(args) != 3 && !(len(args) == 5 && args[3] == "--path" && args[4] != "")) || (args[2] == "backup" && !(len(args) == 5 && args[3] == "--output" && args[4] != "")) || (args[2] == "dream" && len(args) != 3 && !(len(args) == 5 && args[3] == "--mode" && (args[4] == "disabled" || args[4] == "report_only" || args[4] == "propose"))) || (args[2] == "restore" && !(len(args) == 5 && args[3] == "--input" && args[4] != "" || len(args) == 6 && args[3] == "--input" && args[4] != "" && args[5] == "--no-rebuild-derived")) {
		usage(stderr)
		return 2
	}
	config, err := loadConfig(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "memento-go:", err)
		return 1
	}
	if args[2] == "rotate-master-key" {
		rotationErr := service.RotateMasterKey(ctx, config)
		if rotationErr != nil {
			fmt.Fprintln(stderr, "memento-go:", rotationErr)
			return 1
		}
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if rotationErr = encoder.Encode(map[string]any{"rotated": true}); rotationErr != nil {
			fmt.Fprintln(stderr, "memento-go:", rotationErr)
			return 1
		}
		return 0
	}
	if args[2] == "restore" {
		payload, restoreErr := service.RestoreBackup(ctx, config, args[4], !hasFlag(args, "--no-rebuild-derived"))
		if restoreErr == nil {
			encoder := json.NewEncoder(out)
			encoder.SetIndent("", "  ")
			restoreErr = encoder.Encode(payload)
		}
		if restoreErr != nil {
			fmt.Fprintln(stderr, "memento-go:", restoreErr)
			return 1
		}
		return 0
	}
	limits := config.MCP.ExecuteLimits()
	semanticConfig, configErr := service.DecodeSemanticSearchConfig(config.IntelligentTiers.SemanticSearch, os.LookupEnv)
	if configErr != nil {
		fmt.Fprintln(stderr, "memento-go:", configErr)
		return 1
	}
	dreamConfig, configErr := service.DecodeDreamConfig(config.IntelligentTiers.Dream)
	if configErr != nil {
		fmt.Fprintln(stderr, "memento-go:", configErr)
		return 1
	}
	proposalConfig, configErr := service.DecodeModelProposalsConfig(config.IntelligentTiers.ModelProposals)
	if configErr != nil {
		fmt.Fprintln(stderr, "memento-go:", configErr)
		return 1
	}
	providerConfig, configErr := service.DecodeModelProviderSlots(config.IntelligentTiers.ModelProviderSlots)
	if configErr != nil {
		fmt.Fprintln(stderr, "memento-go:", configErr)
		return 1
	}
	effectiveDreamMode := dreamConfig.Mode
	if args[2] == "dream" && len(args) == 5 {
		effectiveDreamMode = args[4]
	}
	var modelClient service.ModelClient
	if effectiveDreamMode == "propose" {
		if providerConfig.Dream.Primary == nil {
			fmt.Fprintln(stderr, "memento-go: dream propose requires a configured dream model provider")
			return 1
		}
		modelClient = &service.RoutedModelClient{Slots: providerConfig}
	}
	needleConfig, configErr := service.DecodeNeedleRouterConfig(config.IntelligentTiers.NeedleRouter)
	if configErr != nil {
		fmt.Fprintln(stderr, "memento-go:", configErr)
		return 1
	}
	runtime, server, err := buildRuntime(ctx, config, service.ModelsOffRuntimeOptions{Surface: config.MCP.ToolSurface, Limits: limits, Graph: config.Observability.GraphExplorer.HTTPConfig(), Needle: needleConfig, Semantic: semanticConfig, Dream: dreamConfig, ModelClient: modelClient, ModelProposals: proposalConfig})
	if err != nil {
		fmt.Fprintln(stderr, "memento-go:", err)
		return 1
	}
	if args[2] == "status" || args[2] == "rebuild-index" || args[2] == "audit" || args[2] == "backup" || args[2] == "dream" {
		var payload map[string]any
		var statusErr error
		switch args[2] {
		case "status":
			payload, statusErr = runtime.StatusSnapshot(ctx, config.SchemaVersion)
		case "rebuild-index":
			payload, statusErr = runtime.RebuildIndex(ctx)
		case "audit":
			var path *string
			if len(args) == 5 {
				path = &args[4]
			}
			payload, statusErr = runtime.AuditRepository(ctx, path)
		case "dream":
			mode := ""
			if len(args) == 5 {
				mode = args[4]
			}
			payload, statusErr = runtime.RunDream(ctx, mode, time.Now())
		case "backup":
			manifest, e := runtime.CreateBackup(ctx, args[4])
			statusErr = e
			payload = map[string]any{"schema_version": manifest.SchemaVersion, "repo_revision": manifest.RepoRevision, "files": manifest.Files}
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
