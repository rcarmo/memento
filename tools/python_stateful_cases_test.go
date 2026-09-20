package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type statefulFamily struct {
	FamilyID        string       `json:"family_id"`
	SourceFixture   string       `json:"source_fixture"`
	CaseSelector    string       `json:"case_selector"`
	Comparator      string       `json:"comparator"`
	ValidationLevel string       `json:"validation_level"`
	CaseCount       int          `json:"case_count"`
	StateDimensions []string     `json:"state_dimensions"`
	GoAdapter       parityGoTest `json:"go_adapter"`
}

func TestPythonStatefulCaseFamilies(t *testing.T) {
	root := parityRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "testdata/parity/python-stateful-case-families.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SchemaVersion int              `json:"schema_version"`
		PythonCommit  string           `json:"python_commit"`
		TotalCases    int              `json:"total_cases"`
		Families      []statefulFamily `json:"families"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || manifest.PythonCommit != "7f29e8b003557f0105f47ed353b7f65a33619456" || len(manifest.Families) != 31 || manifest.TotalCases != 1339 {
		t.Fatal(manifest.SchemaVersion, manifest.PythonCommit, len(manifest.Families), manifest.TotalCases)
	}
	ids, total := map[string]bool{}, 0
	for _, family := range manifest.Families {
		if family.FamilyID == "" || ids[family.FamilyID] || family.SourceFixture == "" || family.CaseSelector == "" || family.Comparator == "" || family.CaseCount < 1 || len(family.StateDimensions) < 3 || (family.ValidationLevel != "executable_stateful_fixture" && family.ValidationLevel != "structured_behavior_spec") {
			t.Fatal("invalid stateful family", family.FamilyID)
		}
		ids[family.FamilyID] = true
		total += family.CaseCount
		assertMappedGoTest(t, root, family.GoAdapter)
		source := filepath.Join(root, "testdata/parity", family.SourceFixture)
		body, readErr := os.ReadFile(source)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !json.Valid(body) {
			t.Fatal("invalid source fixture", family.SourceFixture)
		}
		if got := countCapturedCases(t, body, family.CaseSelector); got != family.CaseCount {
			t.Fatal("captured case count drift", family.FamilyID, got, family.CaseCount)
		}
	}
	if total != manifest.TotalCases {
		t.Fatal(total, manifest.TotalCases)
	}
}
func countCapturedCases(t *testing.T, raw []byte, selector string) int {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(selector, "rows:node_file=") {
		file := strings.TrimPrefix(selector, "rows:node_file=")
		object := value.(map[string]any)
		count := 0
		for _, rawRow := range object["rows"].([]any) {
			if rawRow.(map[string]any)["python_file"] == file {
				count++
			}
		}
		return count
	}
	if selector == "rows:behavior=model_proposals" {
		object := value.(map[string]any)
		count := 0
		for _, rawRow := range object["rows"].([]any) {
			if strings.HasPrefix(rawRow.(map[string]any)["node_name"].(string), "test_model_proposals_") {
				count++
			}
		}
		return count
	}
	if strings.HasPrefix(selector, "cases:surface=") {
		object := value.(map[string]any)
		target := strings.TrimPrefix(selector, "cases:surface=")
		count := 0
		for _, rawCase := range object["cases"].([]any) {
			surface := rawCase.(map[string]any)["surface"].(string)
			if surface == target || (target == "model_proposals" && (surface == "memory_propose_freeform" || surface == "memory_propose_update")) {
				count++
			}
		}
		return count
	}
	if selector == "cases:adapter=cli" {
		object := value.(map[string]any)
		count := 0
		for _, rawCase := range object["cases"].([]any) {
			if strings.HasPrefix(rawCase.(map[string]any)["adapter"].(string), "cli_") {
				count++
			}
		}
		return count
	}
	if selector == "cases:adapter=admin_http" {
		object := value.(map[string]any)
		count := 0
		for _, rawCase := range object["cases"].([]any) {
			if rawCase.(map[string]any)["adapter"] == "admin_http" {
				count++
			}
		}
		return count
	}
	if selector == "rows:runtime_cli" {
		object := value.(map[string]any)
		allowed := map[any]bool{"test_package.py": true, "test_release_deploy.py": true, "test_workflows.py": true}
		count := 0
		for _, rawRow := range object["rows"].([]any) {
			if allowed[rawRow.(map[string]any)["python_file"]] {
				count++
			}
		}
		return count
	}
	if selector == "top-level array/object entries" {
		switch item := value.(type) {
		case []any:
			return len(item)
		case map[string]any:
			return len(item)
		}
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatal("selector requires object", selector)
	}
	items, ok := object[selector].([]any)
	if !ok {
		t.Fatal("selector is not an array", selector)
	}
	return len(items)
}
func TestPythonExactRPCReplayManifest(t *testing.T) {
	root := parityRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "testdata/parity/python-tool-replay-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SchemaVersion int              `json:"schema_version"`
		PythonCommit  string           `json:"python_commit"`
		Families      []string         `json:"families"`
		Definitions   []map[string]any `json:"definitions"`
		Cases         []struct {
			CaseID        string         `json:"case_id"`
			Tool          string         `json:"tool"`
			SourceFixture string         `json:"source_fixture"`
			Comparator    string         `json:"comparator"`
			Request       map[string]any `json:"request"`
			Expected      map[string]any `json:"expected"`
		} `json:"cases"`
		Counts map[string]int `json:"counts"`
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || manifest.PythonCommit != "7f29e8b003557f0105f47ed353b7f65a33619456" || len(manifest.Families) != 11 || len(manifest.Definitions) != 29 || len(manifest.Cases) != 75 || len(manifest.Counts) != 29 {
		t.Fatal(manifest.SchemaVersion, manifest.PythonCommit, len(manifest.Families), len(manifest.Definitions), len(manifest.Cases), len(manifest.Counts))
	}
	idPattern := regexp.MustCompile(`^rpc-[0-9a-f]{16}$`)
	ids, counts := map[string]bool{}, map[string]int{}
	for _, item := range manifest.Cases {
		if !idPattern.MatchString(item.CaseID) || ids[item.CaseID] || item.Tool == "" || item.SourceFixture == "" || item.Comparator != "json_value_with_text_json_normalization" || item.Request == nil || item.Expected == nil {
			t.Fatal("invalid replay case", item.CaseID)
		}
		ids[item.CaseID] = true
		counts[item.Tool]++
	}
	for tool, count := range manifest.Counts {
		if counts[tool] != count {
			t.Fatal(tool, counts[tool], count)
		}
	}
}
