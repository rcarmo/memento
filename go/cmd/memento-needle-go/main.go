// Command memento-needle-go serves routed Needle generation over a bounded
// framed stdin/stdout protocol using read-only mapped FP32 weights.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rcarmo/memento/go/needle"
	"github.com/rcarmo/memento/go/needleworker"
)

var exit = os.Exit

type mappedLoader func(string) (*needle.MappedRouter, error)
type tokenizerLoader func(string) (*needle.Tokenizer, error)
type workerServe func(io.Reader, io.Writer, needleworker.GenerateFunc, uint32) error

func mappedGenerate(router *needle.MappedRouter, tokenizer *needle.Tokenizer) needleworker.GenerateFunc {
	return func(query, tools string, options needle.GenerationOptions) (string, error) {
		return router.Generate(tokenizer, query, tools, options, nil)
	}
}

func run(args []string, input io.Reader, output, stderr io.Writer) int {
	return runWith(args, input, output, stderr, needle.LoadMappedRouter, needle.LoadTokenizer, needleworker.Serve)
}
func runWith(args []string, input io.Reader, output, stderr io.Writer, loadRouter mappedLoader, loadTokenizer tokenizerLoader, serve workerServe) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: memento-needle-go MODEL.nfp32 TOKENIZER.model")
		return 2
	}
	router, err := loadRouter(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer router.Close()
	tokenizer, err := loadTokenizer(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = serve(input, output, mappedGenerate(router, tokenizer), needleworker.DefaultMaxFrameBytes); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
func main() { exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
