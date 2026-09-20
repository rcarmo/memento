package assets

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func validatedImportPack(t *testing.T) ValidatedPack {
	t.Helper()
	raw := importPack(t, "# Demo\n")
	pack, err := ValidateSkillPack("demo", "1.0.0", "# Demo\n", raw)
	if err != nil {
		t.Fatal(err)
	}
	return pack
}
func TestImportSkillPackInjectedSetupFailures(t *testing.T) {
	if closer := skillBzipReader(bytes.NewReader(nil)); closer == nil {
		t.Fatal("reader")
	} else {
		_ = closer.Close()
	}
	boom := errors.New("boom")
	pack := validatedImportPack(t)
	for i, mutate := range []func(*skillImportIO){func(o *skillImportIO) { o.evalSymlinks = func(string) (string, error) { return "", boom } }, func(o *skillImportIO) { o.abs = func(string) (string, error) { return "", boom } }, func(o *skillImportIO) { o.lstat = func(string) (os.FileInfo, error) { return nil, boom } }, func(o *skillImportIO) {
		calls := 0
		o.lstat = func(string) (os.FileInfo, error) {
			calls++
			if calls == 3 {
				return nil, boom
			}
			return nil, os.ErrNotExist
		}
	}, func(o *skillImportIO) { o.mkdirAll = func(string, os.FileMode) error { return boom } }, func(o *skillImportIO) { o.mkdirTemp = func(string, string) (string, error) { return "", boom } }} {
		ops := defaultSkillImportIO()
		mutate(&ops)
		if _, err := importValidatedSkillPack(t.TempDir(), pack, ops); !errors.Is(err, boom) {
			t.Fatal(i, err)
		}
	}
}
func TestImportSkillPackInjectedPublishFailures(t *testing.T) {
	boom := errors.New("boom")
	pack := validatedImportPack(t)
	ops := defaultSkillImportIO()
	if _, err := ops.newArchive([]byte("bad")); err == nil {
		t.Fatal("archive")
	}
	for i, mutate := range []func(*skillImportIO){func(o *skillImportIO) { o.newArchive = func([]byte) (*zip.Reader, error) { return nil, boom } }, func(o *skillImportIO) { o.readArchive = func(*zip.File) ([]byte, error) { return nil, boom } }, func(o *skillImportIO) { o.writeFile = func(string, []byte, os.FileMode) error { return boom } }, func(o *skillImportIO) { o.chmod = func(string, os.FileMode) error { return boom } }, func(o *skillImportIO) { o.rename = func(string, string) error { return boom } }, func(o *skillImportIO) {
		o.rename = func(_, destination string) error {
			if err := os.Mkdir(destination, 0755); err != nil {
				return err
			}
			return boom
		}
	}} {
		workspace := t.TempDir()
		ops := defaultSkillImportIO()
		mutate(&ops)
		_, err := importValidatedSkillPack(workspace, pack, ops)
		if i == 5 {
			var conflict *SkillImportConflictError
			if !errors.As(err, &conflict) {
				t.Fatal(i, err)
			}
			continue
		}
		if !errors.Is(err, boom) {
			t.Fatal(i, err)
		}
		entries, readErr := os.ReadDir(filepath.Join(workspace, ".pi", "skills"))
		if readErr != nil || len(entries) != 0 {
			t.Fatal(i, entries, readErr)
		}
	}
}
func TestExtractValidatedSkillPackChangedArchive(t *testing.T) {
	boom := errors.New("boom")
	pack := validatedImportPack(t)
	ops := defaultSkillImportIO()
	archive, _ := zip.NewReader(bytes.NewReader(pack.ZIPBytes), int64(len(pack.ZIPBytes)))
	archive.File[0].Name = "extra"
	ops.newArchive = func([]byte) (*zip.Reader, error) { return archive, nil }
	if err := extractValidatedSkillPack(pack, t.TempDir(), ops); err == nil {
		t.Fatal("manifest")
	}
	pack = validatedImportPack(t)
	pack.Manifest.Entries[0].Size++
	if err := extractValidatedSkillPack(pack, t.TempDir(), defaultSkillImportIO()); err == nil {
		t.Fatal("size")
	}
	ops = defaultSkillImportIO()
	ops.mkdirAll = func(string, os.FileMode) error { return boom }
	if err := extractValidatedSkillPack(validatedImportPack(t), t.TempDir(), ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	ops = defaultSkillImportIO()
	ops.chmod = func(path string, mode os.FileMode) error {
		if mode == 0755 {
			return boom
		}
		return nil
	}
	if err := extractValidatedSkillPack(validatedImportPack(t), t.TempDir(), ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	pack = validatedImportPack(t)
	archive, _ = zip.NewReader(bytes.NewReader(pack.ZIPBytes), int64(len(pack.ZIPBytes)))
	archive.File[0].Name = "../bad"
	ops = defaultSkillImportIO()
	ops.newArchive = func([]byte) (*zip.Reader, error) { return archive, nil }
	if err := extractValidatedSkillPack(pack, t.TempDir(), ops); err == nil {
		t.Fatal("path")
	}
	pack = validatedImportPack(t)
	archive, _ = zip.NewReader(bytes.NewReader(pack.ZIPBytes), int64(len(pack.ZIPBytes)))
	archive.File = append([]*zip.File{{FileHeader: zip.FileHeader{Name: "directory/"}}}, archive.File...)
	ops = defaultSkillImportIO()
	ops.newArchive = func([]byte) (*zip.Reader, error) { return archive, nil }
	if err := extractValidatedSkillPack(pack, t.TempDir(), ops); err != nil {
		t.Fatal(err)
	}
}
