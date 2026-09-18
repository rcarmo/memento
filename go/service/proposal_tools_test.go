package service

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/umcp"
)

func TestProposalToolReference(t *testing.T) {
	testToolReference(t, "proposal-tools.json", proposalToolDefinitions)
}
func TestReadToolReference(t *testing.T) {
	testToolReference(t, "read-tools.json", readToolDefinitions)
}
func testToolReference(t *testing.T, file string, definitions []byte) {
	raw, err := os.ReadFile("../testdata/parity/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Definitions []map[string]any
		Calls       []struct {
			Request  string
			Expected map[string]any
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("proposal-tools-test")
	server.SetNotificationOutput(nil)
	if err := registerProposalTools(server, func(_ context.Context, method string, args map[string]any) (any, error) {
		return map[string]any{"method": method, "arguments": args, "context": "trusted"}, nil
	}, nil, definitions); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixture.Calls {
		response, err := server.Process(context.Background(), []byte(c.Request), umcp.RequestContext{Transport: "streamable-http", Principal: "actor"})
		if err != nil {
			t.Fatal(err)
		}
		got := jsonNormal(response).(map[string]any)
		// Source dict insertion order affects text, while Go map order is lexical.
		// Compare the JSON values inside MCP text, retaining exact other fields.
		normalizeText := func(response map[string]any) {
			if result, ok := response["result"].(map[string]any); ok {
				for _, value := range result["content"].([]any) {
					item := value.(map[string]any)
					if item["type"] == "text" {
						var parsed any
						if err := json.Unmarshal([]byte(item["text"].(string)), &parsed); err != nil {
							t.Fatal(err)
						}
						item["text"] = parsed
					}
				}
			}
		}
		normalizeText(got)
		normalizeText(c.Expected)
		if !reflect.DeepEqual(got, c.Expected) {
			t.Fatal(c.Request, got, c.Expected)
		}
	}
	expected := []any{}
	for _, d := range fixture.Definitions {
		expected = append(expected, map[string]any{"name": d["name"], "description": d["description"], "inputSchema": d["inputSchema"], "annotations": d["annotations"]})
	}
	response, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), umcp.RequestContext{Principal: "actor"})
	if err != nil {
		t.Fatal(err)
	}
	actual := jsonNormal(response.Result).(map[string]any)["tools"]
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal(actual, expected)
	}
	// Discovery uses principal/list-scoped generic uMCP pagination.
	response, err = server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"pageSize":2}}`), umcp.RequestContext{Principal: "actor"})
	if err != nil || response.Error != nil {
		t.Fatal(response, err)
	}
	if _, err := server.Process(context.Background(), []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{"pageSize":0}}`), umcp.RequestContext{}); err != nil {
		t.Fatal(err)
	}
	if err := registerProposalTools(server, nil, nil, []byte("{")); err == nil {
		t.Fatal("bad metadata")
	}
}
