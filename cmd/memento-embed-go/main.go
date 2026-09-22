// Command memento-embed-go serves scalar GTE through the existing framed worker
// protocol. It is not the Memento service or an MCP endpoint.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rcarmo/memento/internal/embedding"
	"github.com/rcarmo/memento/internal/gte"
	"github.com/rcarmo/memento/internal/processnice"
)

var exit = os.Exit

const (
	maxRequestBytes = 4 << 20
	usage           = "usage: memento-embed-go [--nice N] MODEL.gtemodel"
)

type runConfig struct {
	ModelPath string
	Nice      int
}

type usageError struct{ message string }

func (e usageError) Error() string { return e.message }

func parseRunConfig(args []string, lookup func(string) (string, bool)) (runConfig, error) {
	config := runConfig{}
	remaining := make([]string, 0, len(args))
	flagSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--nice":
			if index+1 >= len(args) {
				return runConfig{}, usageError{message: "missing value for --nice"}
			}
			index++
			priority, err := processnice.Parse(args[index])
			if err != nil {
				return runConfig{}, usageError{message: err.Error()}
			}
			config.Nice = priority
			flagSet = true
		case strings.HasPrefix(arg, "--nice="):
			priority, err := processnice.Parse(strings.TrimPrefix(arg, "--nice="))
			if err != nil {
				return runConfig{}, usageError{message: err.Error()}
			}
			config.Nice = priority
			flagSet = true
		default:
			remaining = append(remaining, arg)
		}
	}
	if !flagSet {
		priority, err := processnice.Resolve(lookup)
		if err != nil {
			return runConfig{}, usageError{message: err.Error()}
		}
		config.Nice = priority
	}
	if len(remaining) != 1 {
		return runConfig{}, usageError{}
	}
	config.ModelPath = remaining[0]
	return config, nil
}

func runWith(args []string, input io.Reader, output, stderr io.Writer, lookup func(string) (string, bool), apply func(int) error, load func(string) (*gte.Model, error)) int {
	config, err := parseRunConfig(args, lookup)
	if err != nil {
		if message := strings.TrimSpace(err.Error()); message != "" {
			fmt.Fprintln(stderr, message)
		}
		fmt.Fprintln(stderr, usage)
		return 2
	}
	if err := apply(config.Nice); err != nil {
		fmt.Fprintf(stderr, "failed to set embedding process nice to %d: %v\n", config.Nice, err)
		return 1
	}
	model, err := load(config.ModelPath)
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

func run(args []string, input io.Reader, output, stderr io.Writer, load func(string) (*gte.Model, error)) int {
	return runWith(args, input, output, stderr, os.LookupEnv, processnice.Apply, load)
}

func main() { exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, gte.Load)) }
