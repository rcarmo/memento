package tools

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGemmaDefaultOffBuildBoundaries(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	read := func(path string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if !bytes.Contains(read("Makefile"), []byte(".DEFAULT_GOAL := help")) {
		t.Fatal("plain make must not select an optional Gemma target")
	}
	for _, path := range []string{"Dockerfile", "tools/release.sh", "models/runtime-models.json", "tools/prepare_runtime_models.py"} {
		if bytes.Contains(bytes.ToLower(read(path)), []byte("gemma")) {
			t.Fatalf("default production artifact path includes Gemma: %s", path)
		}
	}
	for _, target := range []string{"build", "install", "release"} {
		cmd := exec.Command("make", "--no-print-directory", "-n", target)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", target, err, out)
		}
		if bytes.Contains(bytes.ToLower(out), []byte("gemma")) {
			t.Fatalf("default %s builds/installs optional Gemma: %s", target, out)
		}
	}
	cmd := exec.Command("make", "--no-print-directory", "-n")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plain make: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("printf")) || bytes.Contains(out, []byte("go test")) || bytes.Contains(out, []byte("go build")) {
		t.Fatalf("plain make must print help, not run Gemma: %s", out)
	}
}

func TestProductionCommandsDoNotLinkGemma(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	commands := []string{"memento-go", "memento-embed-go", "memento-needle-go", "memento-needle-model-go", "memento-skill-import-go"}
	args := []string{"list", "-deps"}
	for _, name := range commands {
		args = append(args, "./cmd/"+name)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dependency isolation: %v\n%s", err, out)
	}
	for _, path := range strings.Fields(string(out)) {
		if strings.Contains(path, "/embeddinggemma") {
			t.Fatalf("production dependency includes optional model: %s", path)
		}
	}
}
