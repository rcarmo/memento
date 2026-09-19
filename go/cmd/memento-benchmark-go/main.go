package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/derived"
	"path/filepath"
	"time"
)

type result struct {
	Runtime     string  `json:"runtime"`
	RebuildNS   []int64 `json:"rebuild_ns"`
	SearchNS    []int64 `json:"search_ns"`
	ResultCount int     `json:"result_count"`
}

type benchmarkOps struct {
	rebuild func(*derived.Index, context.Context, string) error
	search  func(*derived.Index, context.Context, access.EffectivePolicy) (derived.SearchPage, error)
}

func defaultBenchmarkOps() benchmarkOps {
	return benchmarkOps{func(index *derived.Index, ctx context.Context, root string) error {
		return index.Rebuild(ctx, root, "benchmark-revision")
	}, func(index *derived.Index, ctx context.Context, policy access.EffectivePolicy) (derived.SearchPage, error) {
		return index.SearchLexical(ctx, policy, derived.SearchOptions{Query: "shared benchmark", Syntax: "plain", Limit: 20})
	}}
}
func run(root string, rebuilds, searches int) (result, error) {
	return runWithOps(root, rebuilds, searches, defaultBenchmarkOps())
}
func runWithOps(root string, rebuilds, searches int, ops benchmarkOps) (result, error) {
	ctx := context.Background()
	if rebuilds < 1 || searches < 1 {
		return result{}, fmt.Errorf("rebuilds and searches must be positive")
	}
	out := result{Runtime: "go", RebuildNS: []int64{}, SearchNS: []int64{}}
	var index *derived.Index
	for i := 0; i < rebuilds; i++ {
		path := filepath.Join(os.TempDir(), fmt.Sprintf("memento-go-benchmark-%d-%d.sqlite", os.Getpid(), i))
		_ = os.Remove(path)
		index = &derived.Index{Path: path}
		start := time.Now()
		if err := ops.rebuild(index, ctx, root); err != nil {
			return out, err
		}
		out.RebuildNS = append(out.RebuildNS, time.Since(start).Nanoseconds())
		defer os.Remove(path)
	}
	policy := access.EffectivePolicy{ReadPrefixes: []string{"/"}}
	for i := 0; i < 10; i++ {
		_, _ = ops.search(index, ctx, policy)
	}
	for i := 0; i < searches; i++ {
		start := time.Now()
		page, err := ops.search(index, ctx, policy)
		if err != nil {
			return out, err
		}
		out.SearchNS = append(out.SearchNS, time.Since(start).Nanoseconds())
		out.ResultCount = len(page.Results)
	}
	return out, nil
}
func runCLI(args []string, out, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "usage: memento-benchmark-go ROOT REBUILDS SEARCHES")
		return 2
	}
	var rebuilds, searches int
	if _, err := fmt.Sscan(args[1], &rebuilds); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if _, err := fmt.Sscan(args[2], &searches); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	value, err := run(args[0], rebuilds, searches)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err = json.NewEncoder(out).Encode(value); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

var exit = os.Exit

func main() { exit(runCLI(os.Args[1:], os.Stdout, os.Stderr)) }
