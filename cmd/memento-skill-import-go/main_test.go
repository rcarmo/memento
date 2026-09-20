package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commandPack(t *testing.T, text string) []byte {
	t.Helper()
	var out bytes.Buffer
	archive := zip.NewWriter(&out)
	file, _ := archive.Create("SKILL.md")
	_, _ = file.Write([]byte(text))
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func TestRunSkillImport(t *testing.T) {
	root := t.TempDir()
	skill, archive := filepath.Join(root, "SKILL.md"), filepath.Join(root, "demo.zip")
	if err := os.WriteFile(skill, []byte("# Demo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, commandPack(t, "# Demo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--workspace", workspace, "--name", "demo", "--version", "1.0.0", "--skill-md", skill, "--zip", archive}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || strings.TrimSpace(stdout.String()) != filepath.Join(workspace, ".pi", "skills", "demo") {
		t.Fatal(code, stdout.String(), stderr.String())
	}
}
func TestRunSkillImportFailures(t *testing.T) {
	cases := [][]string{{}, {"--bad"}, {"--name", "x", "extra"}, {"--name", "x", "--version", "1.0.0", "--skill-md", "missing", "--zip", "missing"}}
	for _, args := range cases {
		var stderr bytes.Buffer
		if code := run(args, &bytes.Buffer{}, &stderr); code == 0 || stderr.Len() == 0 {
			t.Fatal(args, code)
		}
	}
	root := t.TempDir()
	skill := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(skill, []byte("# Demo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{"--name", "demo", "--version", "1.0.0", "--skill-md", skill, "--zip", "missing"}, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatal(code)
	}
	archive := filepath.Join(root, "bad.zip")
	if err := os.WriteFile(archive, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"--name", "demo", "--version", "1.0.0", "--skill-md", skill, "--zip", archive}, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatal(code)
	}
}
func TestMainEntryPointCoverage(t *testing.T) {
	oldArgs, oldExit := os.Args, exit
	defer func() { os.Args = oldArgs; exit = oldExit }()
	os.Args = []string{"memento-skill-import-go"}
	called := false
	exit = func(code int) {
		called = true
		if code != 2 {
			t.Fatal(code)
		}
	}
	main()
	if !called {
		t.Fatal("exit")
	}
}
