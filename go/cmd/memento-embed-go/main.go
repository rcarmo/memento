// Command memento-embed-go serves scalar GTE through the existing framed worker
// protocol. It is not the Memento service or an MCP endpoint.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rcarmo/memento/go/embedding"
	"github.com/rcarmo/memento/go/gte"
)

var exit = os.Exit

const maxRequestBytes = 4 << 20

func run(args []string, input io.Reader, output, stderr io.Writer, load func(string) (*gte.Model, error)) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: memento-embed-go MODEL.gtemodel")
		return 2
	}
	model, err := load(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = embedding.Serve(input, output, model, maxRequestBytes); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
func main() { exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, gte.Load)) }
