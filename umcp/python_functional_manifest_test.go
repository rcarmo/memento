package umcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type upstreamGoTest struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Package string `json:"package"`
}
type upstreamManifest struct {
	SchemaVersion int    `json:"schema_version"`
	PythonCommit  string `json:"python_commit"`
	Reference     struct {
		TestFiles int    `json:"test_files"`
		TestNodes int    `json:"test_nodes"`
		Executed  string `json:"executed_result"`
	} `json:"reference"`
	Rows []struct {
		RowID              string           `json:"row_id"`
		PythonTest         string           `json:"python_test"`
		NodeFile           string           `json:"node_file"`
		NodeName           string           `json:"node_name"`
		BehaviorSummary    string           `json:"behavior_summary"`
		PythonSourceSHA256 string           `json:"python_source_sha256"`
		Status             string           `json:"status"`
		GoTests            []upstreamGoTest `json:"go_tests"`
	} `json:"rows"`
}

var upstreamTestDeclaration = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)

func TestPythonFunctionalManifest(t *testing.T) {
	raw, err := os.ReadFile("testdata/python-functional-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest upstreamManifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || manifest.PythonCommit != "30cce7dfe08c6ee63de235f7d81754ba286dafbb" || manifest.Reference.TestFiles != 24 || manifest.Reference.TestNodes != 254 || manifest.Reference.Executed != "254 passed" || len(manifest.Rows) != 254 {
		t.Fatal(manifest.SchemaVersion, manifest.PythonCommit, manifest.Reference, len(manifest.Rows))
	}
	ids, tests, files, hashes := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, row := range manifest.Rows {
		if row.RowID == "" || ids[row.RowID] || row.PythonTest == "" || row.NodeName == "" || row.BehaviorSummary == "" || len(row.GoTests) == 0 || len(row.PythonSourceSHA256) != 64 || row.Status != "behavioral_go_test" {
			t.Fatal("invalid uMCP Python row", row.PythonTest)
		}
		ids[row.RowID], tests[row.PythonTest], files[row.NodeFile] = true, true, true
		if old, ok := hashes[row.NodeFile]; ok && old != row.PythonSourceSHA256 {
			t.Fatal("source hash drift", row.NodeFile)
		}
		hashes[row.NodeFile] = row.PythonSourceSHA256
		for _, item := range row.GoTests {
			if item.Name == "" || item.File == "" || item.Package != "./umcp" {
				t.Fatal(item)
			}
			body, readErr := os.ReadFile(strings.TrimPrefix(item.File, "umcp/"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			found := false
			for _, m := range upstreamTestDeclaration.FindAllSubmatch(body, -1) {
				if string(m[1]) == item.Name {
					found = true
				}
			}
			if !found {
				t.Fatal("missing Go test", item)
			}
		}
	}
	if len(ids) != 254 || len(tests) != 254 || len(files) != 24 || len(hashes) != 24 {
		t.Fatal(len(ids), len(tests), len(files), len(hashes))
	}
	tags := map[string]int{}
	features := []string{}
	err = filepath.WalkDir("testdata/features", func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".feature") {
			features = append(features, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 6 {
		t.Fatal(len(features))
	}
	for _, required := range []string{"schema-tools", "resources-prompts", "discovery-notifications", "protocol-errors", "streamable-http", "transports"} {
		found := false
		for _, file := range features {
			if strings.Contains(file, required) {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("missing logical uMCP Gherkin domain", required)
		}
	}
	for _, file := range features {
		body, readErr := os.ReadFile(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(body)
		if !strings.Contains(text, "Feature:") || !strings.Contains(text, "Given ") || !strings.Contains(text, "When ") || !strings.Contains(text, "Then ") {
			t.Fatal(file)
		}
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "@umcp-") {
				tags[strings.Fields(line)[0][1:]]++
			}
		}
	}
	for id := range ids {
		if tags[id] != 1 {
			t.Fatal("Gherkin mapping", id, tags[id])
		}
	}
}
