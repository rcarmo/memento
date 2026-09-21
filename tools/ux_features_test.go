package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This checks evidence bindings, not execution of Gherkin steps. Browser and Go
// assertions remain the executable evidence; partial/manual coverage is explicit.
func TestUXFeatureInventory(t *testing.T) {
	root := ".."
	var inventory struct {
		SchemaVersion int `json:"schema_version"`
		Scenarios     []struct {
			ID           string   `json:"id"`
			Feature      string   `json:"feature"`
			Title        string   `json:"title"`
			Status       string   `json:"status"`
			BrowserCases []string `json:"browser_cases"`
			GoTests      []struct {
				File string `json:"file"`
				Name string `json:"name"`
			} `json:"go_tests"`
			EvidencePaths []string `json:"evidence_paths"`
			Notes         string   `json:"notes"`
		} `json:"scenarios"`
	}
	read := func(name string) string {
		t.Helper()
		if !filepath.IsLocal(name) {
			t.Fatalf("non-local evidence path %q", name)
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if err := json.Unmarshal([]byte(read("testdata/features/ux/scenario-inventory.json")), &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.SchemaVersion != 1 || len(inventory.Scenarios) == 0 {
		t.Fatal("empty or unsupported inventory")
	}
	features, err := filepath.Glob(filepath.Join(root, "testdata/features/ux/*/*.feature"))
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]string{}
	scenarioRE := regexp.MustCompile(`(?m)^\s*@([a-z0-9-]+)\s*\n\s*Scenario: (.+)$`)
	for _, file := range features {
		relative, _ := filepath.Rel(root, file)
		text := read(relative)
		matches := scenarioRE.FindAllStringSubmatch(text, -1)
		if len(matches) != strings.Count(text, "Scenario:") {
			t.Fatalf("untagged scenario in %s", file)
		}
		for _, m := range matches {
			if _, exists := declared[m[1]]; exists {
				t.Fatalf("duplicate scenario %s", m[1])
			}
			declared[m[1]] = relative + "\n" + m[2]
		}
	}
	browser := map[string]bool{}
	for _, match := range regexp.MustCompile(`await check\('([^']+)'`).FindAllStringSubmatch(read("tools/browser/audit.mjs"), -1) {
		browser[match[1]] = false
	}
	seen := map[string]bool{}
	for _, s := range inventory.Scenarios {
		if seen[s.ID] {
			t.Fatalf("duplicate binding %s", s.ID)
		}
		seen[s.ID] = true
		if declared[s.ID] != s.Feature+"\n"+s.Title {
			t.Errorf("scenario drift %s", s.ID)
		}
		if s.Notes == "" {
			t.Errorf("missing coverage note %s", s.ID)
		}
		switch s.Status {
		case "automated":
			if len(s.BrowserCases)+len(s.GoTests) == 0 {
				t.Errorf("automated without evidence %s", s.ID)
			}
		case "partial", "documented_only":
		default:
			t.Errorf("invalid status %s", s.ID)
		}
		for _, name := range s.BrowserCases {
			if _, exists := browser[name]; !exists {
				t.Errorf("missing browser case %s: %s", s.ID, name)
			}
			browser[name] = true
		}
		for _, ref := range s.GoTests {
			if !regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(ref.Name) + `\(t \*testing.T\)`).MatchString(read(ref.File)) {
				t.Errorf("missing Go test %s: %s#%s", s.ID, ref.File, ref.Name)
			}
		}
		for _, p := range s.EvidencePaths {
			read(p)
		}
	}
	for id := range declared {
		if !seen[id] {
			t.Errorf("unmapped feature %s", id)
		}
	}
	for name, mapped := range browser {
		if !mapped {
			t.Errorf("unmapped browser case %s", name)
		}
	}
}
