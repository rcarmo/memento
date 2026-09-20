package tools

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestReleaseScripts(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	first, second := filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")
	run := func(script, out string) {
		command := exec.Command("sh", filepath.Join(root, "tools", script))
		command.Dir = root
		command.Env = append(os.Environ(), "RELEASE_DIR="+out, "VERSION=test", "SOURCE_DATE_EPOCH=0")
		raw, runErr := command.CombinedOutput()
		if runErr != nil {
			t.Fatal(script, runErr, string(raw))
		}
	}
	run("release.sh", first)
	run("release-check.sh", first)
	run("release.sh", second)
	for _, arch := range []string{"amd64", "arm64"} {
		name := "memento-go-test-linux-" + arch + ".tar.gz"
		a, err := os.ReadFile(filepath.Join(first, name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(second, name))
		if err != nil || !bytes.Equal(a, b) {
			t.Fatal(arch, err)
		}
	}
}
