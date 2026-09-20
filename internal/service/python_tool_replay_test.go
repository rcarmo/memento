package service

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/umcp"
)

type pythonReplayCase struct {
	CaseID        string         `json:"case_id"`
	Tool          string         `json:"tool"`
	SourceFixture string         `json:"source_fixture"`
	Comparator    string         `json:"comparator"`
	SourceIndex   int            `json:"source_index"`
	Request       map[string]any `json:"request"`
	Expected      map[string]any `json:"expected"`
}

func TestPythonCapturedToolReplayCases(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/python-tool-replay-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SchemaVersion int                `json:"schema_version"`
		PythonCommit  string             `json:"python_commit"`
		Definitions   []map[string]any   `json:"definitions"`
		Cases         []pythonReplayCase `json:"cases"`
		Counts        map[string]int     `json:"counts"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || fixture.PythonCommit != "7f29e8b003557f0105f47ed353b7f65a33619456" || len(fixture.Definitions) != 29 || len(fixture.Cases) != 75 || len(fixture.Counts) != 29 {
		t.Fatal(fixture.SchemaVersion, fixture.PythonCommit, len(fixture.Definitions), len(fixture.Cases), len(fixture.Counts))
	}
	definitionBytes, err := json.Marshal(fixture.Definitions)
	if err != nil {
		t.Fatal(err)
	}
	server := umcp.NewServer("python-captured-tool-replay")
	server.SetNotificationOutput(nil)
	if err = registerProposalTools(server, func(_ context.Context, method string, args map[string]any) (any, error) {
		return map[string]any{"method": method, "arguments": args, "context": "trusted"}, nil
	}, nil, definitionBytes); err != nil {
		t.Fatal(err)
	}
	seenCases, seenTools := map[string]bool{}, map[string]int{}
	for _, item := range fixture.Cases {
		item := item
		t.Run(item.Tool+"/"+item.CaseID, func(t *testing.T) {
			if item.CaseID == "" || seenCases[item.CaseID] || item.Comparator != "json_value_with_text_json_normalization" {
				t.Fatal(item.CaseID, item.Comparator)
			}
			seenCases[item.CaseID], seenTools[item.Tool] = true, seenTools[item.Tool]+1
			request, marshalErr := json.Marshal(item.Request)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			response, processErr := server.Process(context.Background(), request, umcp.RequestContext{Transport: "streamable-http", Principal: "actor"})
			if processErr != nil {
				t.Fatal(processErr)
			}
			got, want := jsonNormal(response).(map[string]any), jsonNormal(item.Expected).(map[string]any)
			normalizeCapturedMCPText(t, got)
			normalizeCapturedMCPText(t, want)
			if !reflect.DeepEqual(got, want) {
				actual, _ := json.Marshal(got)
				expected, _ := json.Marshal(want)
				t.Fatalf("source=%s[%d]\nactual=%s\nexpected=%s", item.SourceFixture, item.SourceIndex, actual, expected)
			}
		})
	}
	if len(seenCases) != 75 || len(seenTools) != 29 {
		t.Fatal(len(seenCases), len(seenTools))
	}
	for tool, count := range fixture.Counts {
		if seenTools[tool] != count {
			t.Fatal(tool, seenTools[tool], count)
		}
	}
}
func normalizeCapturedMCPText(t *testing.T, response map[string]any) {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		return
	}
	content, ok := result["content"].([]any)
	if !ok {
		return
	}
	for _, value := range content {
		item, ok := value.(map[string]any)
		if !ok || item["type"] != "text" {
			continue
		}
		text, ok := item["text"].(string)
		if !ok {
			continue
		}
		var parsed any
		if json.Unmarshal([]byte(text), &parsed) == nil {
			item["text"] = parsed
		}
	}
}
