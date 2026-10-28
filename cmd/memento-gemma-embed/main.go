// An offline, CPU-only embedding harness. Not linked into production services.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/rcarmo/memento/internal/embeddinggemma"
)

func run(ctx context.Context, args []string, out, stderr io.Writer) int {
	flags := flag.NewFlagSet("memento-gemma-embed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	model := flags.String("model", "", "GGUF checkpoint")
	text := flags.String("text", "", "input text")
	role := flags.String("role", "raw", "raw, query or document")
	title := flags.String("title", "", "document title")
	backend := flags.String("backend", "auto", "SIMD backend")
	dims := flags.Int("dimensions", 768, "768, 512, 256 or 128")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *model == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "-model is required; positional arguments are not accepted")
		return 2
	}
	if *dims != 768 && *dims != 512 && *dims != 256 && *dims != 128 {
		fmt.Fprintln(stderr, "invalid dimensions")
		return 2
	}
	if *role != "raw" && *role != "query" && *role != "document" {
		fmt.Fprintln(stderr, "invalid role")
		return 2
	}
	m, err := embeddinggemma.Load(*model, *backend)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer m.Close()
	opts := embeddinggemma.Options{Role: embeddinggemma.Role(*role), Title: *title, Dimensions: *dims}
	ids, err := m.Tokenize(ctx, *text, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	v, err := m.EmbedTokens(ctx, ids, *dims, &embeddinggemma.Workspace{})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = json.NewEncoder(out).Encode(struct {
		Tokens    []int     `json:"tokens"`
		Embedding []float32 `json:"embedding"`
	}{ids, v}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
