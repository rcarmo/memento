// memento-gemma-inspect validates and describes an EmbeddingGemma2 GGUF checkpoint.
package main

import (
	"fmt"
	"github.com/rcarmo/memento/internal/embeddinggemma"
	"io"
	"os"
)

func run(args []string, out, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: memento-gemma-inspect <embeddinggemma-2.gguf>")
		return 2
	}
	if err := embeddinggemma.Inspect(out, args[0]); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
