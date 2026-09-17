// Command memento-go reports the status of the in-progress pure Go port.
// It deliberately does not expose an incomplete MCP server.
package main

import (
	"fmt"
	"io"
	"os"
)

var exit = os.Exit

func run(args []string, out, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(out, "memento-go development (compatibility baseline: 0.5.9; server not implemented)")
		return 0
	}
	fmt.Fprintln(stderr, "memento-go: end-to-end port in progress; server and inference are not implemented; see docs/go-port/README.md")
	return 2
}

func main() {
	exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
