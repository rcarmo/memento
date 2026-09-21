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
type scenarioBindingManifest struct {
	SchemaVersion int    `json:"schema_version"`
	PythonCommit  string `json:"python_commit"`
	Counts        struct {
		Application int `json:"application"`
		UMCP        int `json:"umcp"`
		Surface     int `json:"surface"`
		Total       int `json:"total"`
	} `json:"counts"`
	Bindings []struct {
		ScenarioID         string         `json:"scenario_id"`
		ScenarioKind       string         `json:"scenario_kind"`
		BehaviorRowID      string         `json:"behavior_row_id"`
		EvidenceLevel      string         `json:"evidence_level"`
		PythonTest         string         `json:"python_test"`
		PythonSourceSHA256 string         `json:"python_source_sha256"`
		ValidationRefs     []string       `json:"validation_refs"`
		GoTests            []parityGoTest `json:"go_tests"`
		GoSources          []string       `json:"go_sources"`
		Expected           map[string]any `json:"expected"`
	} `json:"bindings"`
}
type surfaceManifest struct {
	SchemaVersion int            `json:"schema_version"`
	PythonCommit  string         `json:"python_commit"`
	Counts        map[string]int `json:"counts"`
	Rows          []struct {
		RowID              string            `json:"row_id"`
		Category           string            `json:"category"`
		Name               string            `json:"name"`
		Status             string            `json:"status"`
		Roles              []string          `json:"roles"`
		GoTests            []parityGoTest    `json:"go_tests"`
		GoSources          []string          `json:"go_sources"`
		RequiredArguments  []string          `json:"required_arguments"`
		OptionalArguments  []string          `json:"optional_arguments"`
		Defaults           map[string]any    `json:"defaults"`
		SuccessEnvelope    []string          `json:"success_envelope"`
		SuccessFields      []string          `json:"success_fields"`
		ErrorContract      []string          `json:"error_contract"`
		SideEffects        string            `json:"side_effects"`
		Idempotency        string            `json:"idempotency"`
		Pagination         string            `json:"pagination"`
		PolicyScope        string            `json:"policy_scope"`
		BehaviorProvenance []string          `json:"behavior_provenance"`
		ProfileOutcomes    map[string]string `json:"profile_outcomes"`
		ValidationLevel    string            `json:"validation_level"`
		ValidationCases    []string          `json:"validation_cases"`
	} `json:"rows"`
}

var goTestDeclaration = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)

func TestFinalPythonParityTargetIsNativeGo(t *testing.T) {
	root := parityRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^python-parity:\n((?:\t[^\n]*\n)+)`).FindSubmatch(raw)
	if len(match) != 2 {
		t.Fatal("python-parity target is absent")
	}
	commands := string(match[1])
	for _, forbidden := range []string{"python", "pytest", "node", "bun", "npm", "uv ", "npx"} {
		if strings.Contains(strings.ToLower(commands), forbidden) {
			t.Fatal("final parity target invokes non-Go runtime", forbidden, commands)
		}
	}
	if !strings.Contains(commands, "$(GO) test ./...") || !strings.Contains(commands, "$(MAKE) -C umcp test") {
		t.Fatal("final parity target does not run both Go modules", commands)
	}
}

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

func TestGherkinScenariosBindDirectlyToGoTestsAndProduction(t *testing.T) {
	root := parityRoot(t)
	var manifest scenarioBindingManifest
	readParityJSON(t, root, "gherkin-go-bindings.json", &manifest)
	var application, upstream functionalManifest
	var surfaces surfaceManifest
	readParityJSON(t, root, "python-functional-manifest.json", &application)
	raw, err := os.ReadFile(filepath.Join(root, "umcp", "testdata", "python-functional-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &upstream); err != nil {
		t.Fatal(err)
	}
	readParityJSON(t, root, "python-surface-manifest.json", &surfaces)
	type functionalTrace struct {
		status, test, hash string
		tests              []parityGoTest
	}
	appRows, upstreamRows := map[string]functionalTrace{}, map[string]functionalTrace{}
	for _, row := range application.Rows {
		appRows[row.RowID] = functionalTrace{row.Status, row.PythonTest, row.PythonSourceSHA256, row.GoTests}
	}
	for _, row := range upstream.Rows {
		upstreamRows[row.RowID] = functionalTrace{row.Status, row.PythonTest, row.PythonSourceSHA256, row.GoTests}
	}
	type surfaceTrace struct {
		level         string
		refs, sources []string
		tests         []parityGoTest
		profiles      map[string]string
	}
	surfaceRows := map[string]surfaceTrace{}
	for _, row := range surfaces.Rows {
		surfaceRows[row.RowID] = surfaceTrace{row.ValidationLevel, row.ValidationCases, row.GoSources, row.GoTests, row.ProfileOutcomes}
	}
	if manifest.SchemaVersion != 1 || manifest.PythonCommit != "7f29e8b003557f0105f47ed353b7f65a33619456" || manifest.Counts.Application != 396 || manifest.Counts.UMCP != 254 || manifest.Counts.Surface != 612 || manifest.Counts.Total != 1262 || len(manifest.Bindings) != 1262 {
		t.Fatal(manifest.SchemaVersion, manifest.PythonCommit, manifest.Counts, len(manifest.Bindings))
	}
	tags := readFeatureTags(t, filepath.Join(root, "testdata", "parity", "features"))
	for id, count := range scanFeatureTags(t, filepath.Join(root, "umcp", "testdata", "features")) {
		tags[id] += count
	}
	bindings := map[string]bool{}
	for _, binding := range manifest.Bindings {
		if binding.ScenarioID == "" || binding.BehaviorRowID == "" || bindings[binding.ScenarioID] || tags[binding.ScenarioID] != 1 || len(binding.ValidationRefs) == 0 || len(binding.GoTests) == 0 || len(binding.GoSources) == 0 || len(binding.Expected) == 0 {
			t.Fatal("invalid direct Gherkin binding", binding.ScenarioID, tags[binding.ScenarioID])
		}
		bindings[binding.ScenarioID] = true
		switch binding.ScenarioKind {
		case "application_behavior":
			row, ok := appRows[binding.BehaviorRowID]
			if !ok || binding.EvidenceLevel != row.status || binding.PythonTest != row.test || binding.PythonSourceSHA256 != row.hash || fmt.Sprint(binding.GoTests) != fmt.Sprint(row.tests) {
				t.Fatal("application binding drift", binding.ScenarioID)
			}
		case "umcp_behavior":
			row, ok := upstreamRows[binding.BehaviorRowID]
			if !ok || binding.EvidenceLevel != row.status || binding.PythonTest != row.test || binding.PythonSourceSHA256 != row.hash || fmt.Sprint(binding.GoTests) != fmt.Sprint(row.tests) {
				t.Fatal("uMCP binding drift", binding.ScenarioID)
			}
		case "success", "failure", "role":
			row, ok := surfaceRows[binding.BehaviorRowID]
			if !ok || binding.EvidenceLevel != row.level || fmt.Sprint(binding.ValidationRefs) != fmt.Sprint(row.refs) || fmt.Sprint(binding.GoTests) != fmt.Sprint(row.tests) || fmt.Sprint(binding.GoSources) != fmt.Sprint(row.sources) {
				t.Fatal("surface binding drift", binding.ScenarioID)
			}
			if binding.ScenarioKind == "role" {
				profile, _ := binding.Expected["profile"].(string)
				outcome, _ := binding.Expected["outcome"].(string)
				if row.profiles[profile] != outcome {
					t.Fatal("surface role binding drift", binding.ScenarioID, profile, outcome, row.profiles[profile])
				}
			}
		default:
			t.Fatal("unknown scenario binding kind", binding.ScenarioID, binding.ScenarioKind)
		}
		for _, item := range binding.GoTests {
			assertMappedGoTest(t, root, item)
		}
		for _, source := range binding.GoSources {
			if strings.HasSuffix(source, "_test.go") {
				t.Fatal("Gherkin binding points to test as production", binding.ScenarioID, source)
			}
			info, err := os.Stat(filepath.Join(root, source))
			if err != nil || info.IsDir() {
				t.Fatal("Gherkin production source is absent", binding.ScenarioID, source, err)
			}
		}
	}
	if len(tags) != 1262 || len(bindings) != len(tags) {
		t.Fatal("Gherkin/binding cardinality", len(tags), len(bindings))
	}
	for id := range tags {
		if !bindings[id] {
			t.Fatal("Gherkin scenario lacks direct Go binding", id)
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
		if row.RowID == "" || ids[row.RowID] || names[key] || len(row.Roles) == 0 || len(row.GoTests) == 0 || len(row.GoSources) == 0 || len(row.SuccessFields) == 0 || len(row.ErrorContract) == 0 || len(row.BehaviorProvenance) < 3 || row.SideEffects == "" || row.Idempotency == "" || row.Pagination == "" || row.PolicyScope == "" || len(row.ProfileOutcomes) != 4 || len(row.ValidationCases) == 0 || (row.ValidationLevel != "exact_jsonrpc_replay" && row.ValidationLevel != "exact_protocol_fixture" && row.ValidationLevel != "stateful_fixture" && row.ValidationLevel != "structured_behavior_spec" && row.ValidationLevel != "go_extension_test") || (row.Status != "mapped" && row.Status != "go_extension") {
			t.Fatal("invalid detailed surface row", row.Category, row.Name)
		}
		for _, value := range append(append(append([]string{}, row.SuccessFields...), row.ErrorContract...), row.BehaviorProvenance...) {
			lower := strings.ToLower(strings.TrimSpace(value))
			if lower == "" || lower == "none" || strings.Contains(lower, "route-specific") || lower == "unknown" || strings.Contains(lower, "unknown behavior") || strings.Contains(lower, "todo") {
				t.Fatal("generic placeholder in surface row", row.Category, row.Name, value)
			}
		}
		for _, profile := range []string{"reader", "proposer", "curator", "admin"} {
			if strings.TrimSpace(row.ProfileOutcomes[profile]) == "" || strings.Contains(strings.ToLower(row.ProfileOutcomes[profile]), "route-specific") {
				t.Fatal("missing detailed profile outcome", row.Category, row.Name, profile)
			}
		}
		ids[row.RowID], names[key] = true, true
		for _, item := range row.GoTests {
			assertMappedGoTest(t, root, item)
		}
		for _, source := range row.GoSources {
			if strings.HasSuffix(source, "_test.go") {
				t.Fatal("surface production mapping points to test", row.Name, source)
			}
			if info, err := os.Stat(filepath.Join(root, source)); err != nil || info.IsDir() {
				t.Fatal("surface production mapping is absent", row.Name, source, err)
			}
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
	assertValidationCaseReferences(t, root, manifest)
	assertExactSurfaceSets(t, manifest)
	assertCatalogSurface(t, root, names)
	assertSurfaceSchemas(t, root, manifest)
	assertAccessToolSurface(t, root, names)
}

func scanFeatureTags(t *testing.T, directory string) map[string]int {
	t.Helper()
	tags := map[string]int{}
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".feature") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pending := []string{}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "@") {
				pending = strings.Fields(line)
				continue
			}
			if strings.HasPrefix(line, "Scenario:") {
				for _, tag := range pending {
					tags[strings.TrimPrefix(tag, "@")]++
				}
				pending = nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tags
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
		for _, forbidden := range []string{"@go_", "@python_", "test_", ".py", "Behavior captured from", "pinned Python", "Go uMCP"} {
			if strings.Contains(text, forbidden) {
				t.Fatal("implementation reference in behavior feature", file, forbidden)
			}
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
func assertValidationCaseReferences(t *testing.T, root string, manifest surfaceManifest) {
	t.Helper()
	var replay struct {
		Cases []struct {
			CaseID string `json:"case_id"`
		} `json:"cases"`
	}
	readParityJSON(t, root, "python-tool-replay-cases.json", &replay)
	rpc := map[string]bool{}
	for _, item := range replay.Cases {
		rpc[item.CaseID] = true
	}
	var stateful struct {
		Families []struct {
			FamilyID        string `json:"family_id"`
			ValidationLevel string `json:"validation_level"`
		} `json:"families"`
	}
	readParityJSON(t, root, "python-stateful-case-families.json", &stateful)
	families := map[string]string{}
	for _, item := range stateful.Families {
		families[item.FamilyID] = item.ValidationLevel
	}
	for _, row := range manifest.Rows {
		for _, ref := range row.ValidationCases {
			switch {
			case strings.HasPrefix(ref, "rpc-"):
				if !rpc[ref] {
					t.Fatal("unknown RPC validation case", row.Name, ref)
				}
			case strings.HasPrefix(ref, "stateful:"):
				id := strings.TrimPrefix(ref, "stateful:")
				if families[id] != "executable_stateful_fixture" {
					t.Fatal("non-executable stateful reference", row.Name, ref, families[id])
				}
			case strings.HasPrefix(ref, "behavior:"):
				id := strings.TrimPrefix(ref, "behavior:")
				if families[id] != "structured_behavior_spec" {
					t.Fatal("invalid behavior specification reference", row.Name, ref, families[id])
				}
			case strings.HasPrefix(ref, "umcp:"):
			case strings.HasPrefix(ref, "go:"):
				if row.ValidationLevel != "go_extension_test" {
					t.Fatal("Go-only validation reference on Python surface", row.Name, ref)
				}
			default:
				t.Fatal("unknown validation reference", row.Name, ref)
			}
		}
	}
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
func assertSurfaceSchemas(t *testing.T, root string, manifest surfaceManifest) {
	t.Helper()
	byName := map[string]surfaceManifestRow{}
	for _, row := range manifest.Rows {
		byName[row.Name] = surfaceManifestRow{Required: row.RequiredArguments, Optional: row.OptionalArguments, Defaults: row.Defaults}
	}
	var catalog struct {
		Operations []struct {
			ToolName string   `json:"tool_name"`
			Roles    []string `json:"roles"`
		} `json:"operations"`
		Contracts map[string]struct {
			ToolName    string `json:"tool_name"`
			InputSchema struct {
				Required   []string                   `json:"required"`
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"input_schema"`
		} `json:"contracts"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "internal", "service", "catalog_data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	roles := map[string][]string{}
	for _, op := range catalog.Operations {
		roles[op.ToolName] = op.Roles
	}
	for _, contract := range catalog.Contracts {
		row, ok := byName[contract.ToolName]
		if !ok {
			continue
		}
		required := append([]string{}, contract.InputSchema.Required...)
		sort.Strings(required)
		actual := append([]string{}, row.Required...)
		sort.Strings(actual)
		if fmt.Sprint(required) != fmt.Sprint(actual) {
			t.Fatal("required argument drift", contract.ToolName, actual, required)
		}
		optional := []string{}
		for name := range contract.InputSchema.Properties {
			if !containsString(required, name) {
				optional = append(optional, name)
			}
		}
		sort.Strings(optional)
		actual = append([]string{}, row.Optional...)
		sort.Strings(actual)
		if fmt.Sprint(optional) != fmt.Sprint(actual) {
			t.Fatal("optional argument drift", contract.ToolName, actual, optional)
		}
	}
	for name, wantRoles := range roles {
		rowFound := false
		for _, row := range manifest.Rows {
			if row.Name == name {
				rowFound = true
				actual := append([]string{}, row.Roles...)
				sort.Strings(actual)
				want := append([]string{}, wantRoles...)
				sort.Strings(want)
				if fmt.Sprint(actual) != fmt.Sprint(want) {
					t.Fatal("role drift", name, actual, want)
				}
			}
		}
		if !rowFound {
			t.Fatal("tool role row absent", name)
		}
	}
}

type surfaceManifestRow struct {
	Required, Optional []string
	Defaults           map[string]any
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
