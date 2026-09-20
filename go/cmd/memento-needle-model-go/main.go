// Command memento-needle-model-go prepares the mmapable FP32 Needle sidecar.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rcarmo/memento/go/needle"
)

var exit = os.Exit

func run(args []string, stdout, stderr io.Writer) int {
	return runWith(args, stdout, stderr, needle.PrepareFP32, needle.ReadFP32Info, needle.VerifyFP32)
}
func runWith(args []string, stdout, stderr io.Writer, prepare func(string, string) error, info func(string) (needle.FP32Info, error), verify func(string) error) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: memento-needle-model-go SOURCE.ndl DESTINATION.nfp32")
		return 2
	}
	if err := prepare(args[0], args[1]); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	model, err := info(args[1])
	if err == nil {
		err = verify(args[1])
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "prepared %s: %d tensors, %d bytes\n", args[1], model.TensorCount, model.FileSize)
	return 0
}
func main() { exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
