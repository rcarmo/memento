package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/derived"
)

func TestReadServiceReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/service-reads.json")
	if err != nil {
		t.Fatal(err)
	}
	type scenario struct {
		Method    string
		Arguments map[string]any
		Policy    access.EffectivePolicy
		Expected  map[string]any
	}
	var fixture struct {
		Files     map[string]string
		Cases     []scenario
		Malformed []struct {
			Path, Method string
			Arguments    map[string]any
			Expected     map[string]any
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	root := t.TempDir()
	installMutationFiles(t, root, fixture.Files)
	index := &derived.Index{Path: filepath.Join(t.TempDir(), "index.sqlite")}
	if err = index.Rebuild(ctx, root, "indexed"); err != nil {
		t.Fatal(err)
	}
	if err = index.SetRepoRevision(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	controls, _ := rebaseTest(t)
	controls.Queue.Paths.CurrentDir = root
	controls.Index = index
	run := func(c scenario) {
		t.Helper()
		args := map[string]any{}
		for key, value := range c.Arguments {
			args[key] = value
		}
		options := SuccessOptions{}
		var data map[string]any
		var err error
		str := func(key string) *string {
			if value, ok := args[key].(string); ok {
				return &value
			}
			return nil
		}
		actor := ProposalActor{Policy: c.Policy}
		switch c.Method {
		case "read":
			data, err = controls.Read(ctx, actor, args["id_or_path"].(string))
		case "list":
			prefix := "/"
			if str("path_prefix") != nil {
				prefix = *str("path_prefix")
			}
			data, err = controls.List(ctx, actor, prefix)
		case "search":
			limit := 20
			if value, ok := args["limit"].(float64); ok {
				limit = int(value)
			}
			syntax := "plain"
			if str("query_syntax") != nil {
				syntax = *str("query_syntax")
			}
			data, options, err = controls.Search(ctx, actor, args["query"].(string), nil, limit, str("cursor"), str("search_mode"), syntax)
		case "graph":
			depth := 1
			if value, ok := args["depth"].(float64); ok {
				depth = int(value)
			}
			data, options, err = controls.Graph(ctx, actor, args["id_or_path"].(string), depth)
		}
		var envelope any
		if err != nil {
			failure, mapping := FailureEnvelope(err)
			if mapping != nil {
				t.Fatal(c.Method, args, err)
			}
			envelope = failure
		} else {
			envelope, err = controls.Queue.successEnvelope(data, options, fakeRepo(nil))
			if err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(jsonNormal(envelope), c.Expected) {
			t.Fatal(c.Method, args, envelope, c.Expected)
		}
	}
	for _, c := range fixture.Cases {
		run(c)
	}
	for _, c := range fixture.Malformed {
		path := filepath.Join(root, c.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("not a concept"), 0600); err != nil {
			t.Fatal(err)
		}
		run(scenario{Method: c.Method, Arguments: c.Arguments, Policy: fixture.Cases[0].Policy, Expected: c.Expected})
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}
func TestReadNoIndexAndInvalidContent(t *testing.T) {
	ctx := context.Background()
	c, actor, _ := realApplyTest(t)
	if _, _, err := c.Search(ctx, actor, "query", nil, 20, nil, nil, "plain"); err == nil {
		t.Fatal("missing index")
	}
	if _, _, err := c.Graph(ctx, actor, "id", 1); err == nil {
		t.Fatal("missing index")
	}
	if err := os.WriteFile(filepath.Join(c.Queue.Paths.CurrentDir, "bad.md"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Read(ctx, actor, "/bad.md"); err == nil {
		t.Fatal("invalid direct read")
	}
	if got := modelTimestamp(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); got != "2026-01-01T00:00:00Z" {
		t.Fatal(got)
	}
}
