package assets

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func importPack(t *testing.T, skill string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	root, _ := archive.Create("SKILL.md")
	_, _ = root.Write([]byte(skill))
	script, _ := archive.CreateHeader(&zip.FileHeader{Name: "scripts/run.sh", Method: zip.Store, ExternalAttrs: uint32(0100755) << 16})
	_, _ = script.Write([]byte("#!/bin/sh\necho ok\n"))
	asset, _ := archive.Create("assets/icon.png")
	_, _ = asset.Write([]byte("\x89PNG\r\n\x1a\nimage"))
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
func TestImportSkillPack(t *testing.T) {
	workspace := t.TempDir()
	raw := importPack(t, "# Demo\n")
	destination, err := ImportSkillPack(workspace, "demo", "1.0.0", "# Demo\n", raw)
	if err != nil || destination != filepath.Join(workspace, ".pi", "skills", "demo") {
		t.Fatal(destination, err)
	}
	for path, want := range map[string]string{"SKILL.md": "# Demo\n", "scripts/run.sh": "#!/bin/sh\necho ok\n"} {
		data, readErr := os.ReadFile(filepath.Join(destination, path))
		if readErr != nil || string(data) != want {
			t.Fatal(path, string(data), readErr)
		}
		info, statErr := os.Stat(filepath.Join(destination, path))
		if statErr != nil || info.Mode().Perm() != 0644 {
			t.Fatal(path, info, statErr)
		}
	}
	for _, path := range []string{destination, filepath.Join(destination, "scripts"), filepath.Join(destination, "assets")} {
		info, statErr := os.Stat(path)
		if statErr != nil || info.Mode().Perm() != 0755 {
			t.Fatal(path, info, statErr)
		}
	}
	if _, err = ImportSkillPack(workspace, "demo", "1.0.0", "# Demo\n", raw); err == nil {
		t.Fatal("conflict")
	} else {
		var conflict *SkillImportConflictError
		if !errors.As(err, &conflict) || !strings.Contains(conflict.Error(), destination) {
			t.Fatal(err)
		}
	}
}
func TestImportSkillPackRejectsParentsAndValidation(t *testing.T) {
	raw := importPack(t, "# Demo\n")
	for _, part := range []string{".pi", filepath.Join(".pi", "skills")} {
		workspace, outside := t.TempDir(), t.TempDir()
		target := filepath.Join(workspace, part)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, target); err != nil {
			t.Fatal(err)
		}
		if _, err := ImportSkillPack(workspace, "demo", "1.0.0", "# Demo\n", raw); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatal(part, err)
		}
		if _, err := os.Stat(filepath.Join(outside, "demo")); !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	workspace := t.TempDir()
	if _, err := ImportSkillPack(workspace, "demo", "1.0.0", "# Different\n", raw); err == nil {
		t.Fatal("validation")
	}
	entries, err := os.ReadDir(workspace)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
	if err = os.WriteFile(filepath.Join(workspace, ".pi"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ImportSkillPack(workspace, "demo", "1.0.0", "# Demo\n", raw); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatal(err)
	}
}
