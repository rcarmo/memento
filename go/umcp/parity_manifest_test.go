package umcp

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestDifferentialFixtureManifest(t *testing.T) {
	expected := []string{"umcp-cli.json", "umcp-completion.json", "umcp-dispatch.json", "umcp-file.json", "umcp-http-modes.json", "umcp-http-rules.json", "umcp-http.json", "umcp-notifications.json", "umcp-pagination.json", "umcp-progress.json", "umcp-prompts.json", "umcp-raw-http.json", "umcp-resources.json", "umcp-shared.json", "umcp-sse.json", "umcp-stdio.json", "umcp-sync-http.json", "umcp-sync-wire.json", "umcp-tcp.json", "umcp-tool-results.json", "umcp-tools.json", "umcp-urls.json"}
	entries, err := os.ReadDir("testdata/parity")
	if err != nil {
		t.Fatal(err)
	}
	actual := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "umcp-") && strings.HasSuffix(entry.Name(), ".json") {
			actual = append(actual, entry.Name())
		}
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("fixtures differ\n%v\n%v", actual, expected)
	}
	sources, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	combined := ""
	for _, path := range sources {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		combined += string(raw)
	}
	for _, name := range expected {
		if !strings.Contains(combined, "testdata/parity/"+name) {
			t.Errorf("fixture is not referenced: %s", name)
		}
	}
}
