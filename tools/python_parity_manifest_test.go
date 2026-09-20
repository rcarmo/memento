package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type parityGoTest struct {
	Package string `json:"package"`
	Name    string `json:"name"`
	File    string `json:"file"`
}
type functionalManifest struct {
	SchemaVersion int    `json:"schema_version"`
	PythonCommit  string `json:"python_commit"`
	Reference     struct {
		TestFiles             int `json:"test_files"`
		TestNodes             int `json:"test_nodes"`
		ExpandedCasesEstimate int `json:"expanded_cases_estimate"`
	} `json:"reference"`
	Rows []struct {
		RowID              string         `json:"row_id"`
		PythonTest         string         `json:"python_test"`
		NodeFile           string         `json:"node_file"`
		NodeName           string         `json:"node_name"`
		BehaviorSummary    string         `json:"behavior_summary"`
		Status             string         `json:"status"`
		PythonSourceSHA256 string         `json:"python_source_sha256"`
		GoTests            []parityGoTest `json:"go_tests"`
	} `json:"rows"`
}
type surfaceManifest struct {
	SchemaVersion int            `json:"schema_version"`
	PythonCommit  string         `json:"python_commit"`
	Counts        map[string]int `json:"counts"`
	Rows          []struct {
		RowID    string         `json:"row_id"`
		Category string         `json:"category"`
		Name     string         `json:"name"`
		Status   string         `json:"status"`
		Roles    []string       `json:"roles"`
		GoTests  []parityGoTest `json:"go_tests"`
	} `json:"rows"`
}

var goTestDeclaration = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)

func parityRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func readParityJSON(t *testing.T, root, name string, target any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "testdata", "parity", name))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}
func assertMappedGoTest(t *testing.T, root string, item parityGoTest) {
	t.Helper()
	if item.Package == "" || item.File == "" || item.Name == "" {
		t.Fatal("incomplete Go test mapping", item)
	}
	raw, err := os.ReadFile(filepath.Join(root, item.File))
	if err != nil {
		t.Fatal(item, err)
	}
	found := false
	for _, match := range goTestDeclaration.FindAllSubmatch(raw, -1) {
		if string(match[1]) == item.Name {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mapped Go test %s is absent from %s", item.Name, item.File)
	}
}

func TestPythonFunctionalParityManifest(t *testing.T) {
	root := parityRoot(t)
	var manifest functionalManifest
	readParityJSON(t, root, "python-functional-manifest.json", &manifest)
	if manifest.SchemaVersion != 1 || manifest.PythonCommit != "7f29e8b003557f0105f47ed353b7f65a33619456" {
		t.Fatal(manifest.SchemaVersion, manifest.PythonCommit)
	}
	if manifest.Reference.TestFiles != 40 || manifest.Reference.TestNodes != 396 || manifest.Reference.ExpandedCasesEstimate != 490 || len(manifest.Rows) != 396 {
		t.Fatal(manifest.Reference, len(manifest.Rows))
	}
	ids, tests, files, hashes := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, row := range manifest.Rows {
		if row.RowID == "" || ids[row.RowID] {
			t.Fatal("duplicate/empty row id", row.RowID)
		}
		ids[row.RowID], tests[row.PythonTest], files[row.NodeFile] = true, true, true
		if previous, ok := hashes[row.NodeFile]; ok && previous != row.PythonSourceSHA256 {
			t.Fatal("inconsistent Python source hash", row.NodeFile)
		}
		hashes[row.NodeFile] = row.PythonSourceSHA256
		if row.PythonTest == "" || row.NodeName == "" || row.BehaviorSummary == "" || len(row.GoTests) == 0 {
			t.Fatal("incomplete Python mapping", row.PythonTest)
		}
		if len(row.PythonSourceSHA256) != 64 {
			t.Fatal("missing pinned source hash", row.PythonTest)
		}
		if row.Status != "mapped_domain_suite" && row.Status != "intentional_replacement" {
			t.Fatal("unreviewed status", row.PythonTest, row.Status)
		}
		for _, item := range row.GoTests {
			assertMappedGoTest(t, root, item)
		}
	}
	if len(tests) != 396 || len(files) != 40 || len(hashes) != 40 {
		t.Fatal(len(tests), len(files), len(hashes))
	}
	featureTags := readFeatureTags(t, filepath.Join(root, "testdata", "parity", "features"))
	for id := range ids {
		if featureTags[id] != 1 {
			t.Fatalf("functional row %s has %d Gherkin scenarios", id, featureTags[id])
		}
	}
}

func TestPythonPublicSurfaceParityManifest(t *testing.T) {
	root := parityRoot(t)
	var manifest surfaceManifest
	readParityJSON(t, root, "python-surface-manifest.json", &manifest)
	want := map[string]int{"mcp_tool": 44, "mcp_resource": 3, "mcp_resource_template": 2, "mcp_prompt": 1, "mcp_protocol": 12, "http_endpoint": 31, "cli": 9}
	if manifest.SchemaVersion != 1 || manifest.PythonCommit != "7f29e8b003557f0105f47ed353b7f65a33619456" || len(manifest.Rows) != 102 {
		t.Fatal(manifest.SchemaVersion, manifest.PythonCommit, len(manifest.Rows))
	}
	if fmt.Sprint(manifest.Counts) != fmt.Sprint(want) {
		t.Fatal(manifest.Counts, want)
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for _, row := range manifest.Rows {
		key := row.Category + "\x00" + row.Name
		if row.RowID == "" || ids[row.RowID] || names[key] || len(row.Roles) == 0 || len(row.GoTests) == 0 || (row.Status != "mapped" && row.Status != "go_extension") {
			t.Fatal("invalid surface row", row.Category, row.Name)
		}
		ids[row.RowID], names[key] = true, true
		for _, item := range row.GoTests {
			assertMappedGoTest(t, root, item)
		}
	}
	featureTags := readFeatureTags(t, filepath.Join(root, "testdata", "parity", "features"))
	for _, row := range manifest.Rows {
		id := row.RowID
		if featureTags[id] != 1 || featureTags[id+"_failure"] != 1 {
			t.Fatalf("surface row %s lacks exact success/failure Gherkin scenarios", id)
		}
		for _, profile := range []string{"reader", "proposer", "curator", "admin"} {
			if featureTags[id+"_role_"+profile] != 1 {
				t.Fatalf("surface row %s lacks %s role Gherkin scenario", id, profile)
			}
		}
	}
	assertExactSurfaceSets(t, manifest)
	assertCatalogSurface(t, root, names)
	assertAccessToolSurface(t, root, names)
}

func readFeatureTags(t *testing.T, directory string) map[string]int {
	t.Helper()
	entries := []string{}
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".feature") {
			entries = append(entries, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 23 {
		t.Fatalf("got %d logically grouped Gherkin feature files", len(entries))
	}
	for _, required := range []string{"application/access/", "application/repository/", "application/proposals/", "application/retrieval/", "application/models/", "application/graph/", "application/runtime/", "application/mcp/", "surfaces/mcp/", "surfaces/http/", "surfaces/cli/"} {
		found := false
		for _, file := range entries {
			if strings.Contains(filepath.ToSlash(file), required) {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("missing logical Gherkin domain", required)
		}
	}
	tags := map[string]int{}
	for _, file := range entries {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.Contains(text, "Feature:") || !strings.Contains(text, "Scenario:") || !strings.Contains(text, "Given ") || !strings.Contains(text, "When ") || !strings.Contains(text, "Then ") {
			t.Fatal("invalid feature", file)
		}
		lines := strings.Split(text, "\n")
		pending := []string{}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "@") {
				pending = strings.Fields(line)
				continue
			}
			if strings.HasPrefix(line, "Scenario:") {
				if len(pending) == 0 {
					t.Fatal("untagged scenario", file, line)
				}
				for _, tag := range pending {
					value := strings.TrimPrefix(tag, "@")
					tags[value]++
				}
				pending = nil
			}
		}
	}
	return tags
}
func assertExactSurfaceSets(t *testing.T, manifest surfaceManifest) {
	t.Helper()
	want := map[string][]string{
		"mcp_protocol": {"initialize", "tools/list", "tools/call", "prompts/list", "prompts/get", "resources/list", "resources/templates/list", "resources/read", "resources/subscribe", "resources/unsubscribe", "completion/complete", "logging/setLevel"},
		"cli":          {"serve", "audit", "rebuild-index", "backup", "restore", "status", "dream", "rotate-master-key", "healthcheck"},
	}
	for category, expected := range want {
		actual := []string{}
		for _, row := range manifest.Rows {
			if row.Category == category {
				actual = append(actual, row.Name)
			}
		}
		sort.Strings(actual)
		sort.Strings(expected)
		if fmt.Sprint(actual) != fmt.Sprint(expected) {
			t.Fatal(category, actual, expected)
		}
	}
}
func assertCatalogSurface(t *testing.T, root string, names map[string]bool) {
	t.Helper()
	var catalog struct {
		Operations []struct {
			ToolName string `json:"tool_name"`
		} `json:"operations"`
		Resources []struct {
			URI string `json:"uri"`
		} `json:"resources"`
		Templates []struct {
			URI string `json:"uriTemplate"`
		} `json:"templates"`
		Prompts []struct {
			Name string `json:"name"`
		} `json:"prompts"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "internal", "service", "catalog_data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	actual := []string{}
	for _, x := range catalog.Operations {
		actual = append(actual, "mcp_tool\x00"+x.ToolName)
	}
	for _, x := range catalog.Resources {
		actual = append(actual, "mcp_resource\x00"+x.URI)
	}
	for _, x := range catalog.Templates {
		actual = append(actual, "mcp_resource_template\x00"+x.URI)
	}
	for _, x := range catalog.Prompts {
		actual = append(actual, "mcp_prompt\x00"+x.Name)
	}
	sort.Strings(actual)
	for _, key := range actual {
		if !names[key] {
			t.Fatal("catalog surface missing from Python matrix", key)
		}
	}
}
func assertAccessToolSurface(t *testing.T, root string, names map[string]bool) {
	t.Helper()
	var definitions []struct {
		Name string `json:"name"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "internal", "service", "access_tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &definitions); err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 10 {
		t.Fatal("access tool count", len(definitions))
	}
	for _, item := range definitions {
		if !names["mcp_tool\x00"+item.Name] {
			t.Fatal("access tool missing from Python matrix", item.Name)
		}
	}
}
