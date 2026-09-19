package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
	"io"
	"io/fs"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func backupRuntime(t *testing.T) *Runtime {
	t.Helper()
	var config RuntimeConfig
	config.Repository.RootPath = filepath.Join(t.TempDir(), "runtime")
	runtime, _, err := BuildModelsOffRuntime(context.Background(), config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runtime.Close(context.Background()) })
	return runtime
}
func TestCreateAndLoadBackup(t *testing.T) {
	runtime := backupRuntime(t)
	destination := filepath.Join(t.TempDir(), "backup")
	manifest, err := runtime.CreateBackup(context.Background(), destination)
	if err != nil || manifest.SchemaVersion != 1 || len(manifest.Files) != 3 {
		t.Fatal(manifest, err)
	}
	loaded, err := LoadBackupManifest(destination)
	if err != nil || loaded.RepoRevision != manifest.RepoRevision {
		t.Fatal(loaded, err)
	}
	db, err := sql.Open("sqlite", filepath.Join(destination, "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	var table string
	if err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='operations'").Scan(&table); err != nil || table != "operations" {
		t.Fatal(table, err)
	}
	db.Close()
	file, err := os.Open(filepath.Join(destination, "repo.git.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewReader(gz)
	found := false
	for {
		header, e := archive.Next()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if header.Name == "repo.git" {
			found = true
		}
	}
	gz.Close()
	file.Close()
	if !found {
		t.Fatal("repo.git")
	}
	raw, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
	if err != nil || !strings.HasSuffix(string(raw), "\n") {
		t.Fatal(err, string(raw))
	}
}
func TestRestoreBackup(t *testing.T) {
	ctx := context.Background()
	for _, rebuild := range []bool{true, false} {
		t.Run(map[bool]string{true: "rebuild", false: "copy"}[rebuild], func(t *testing.T) {
			runtime := backupRuntime(t)
			config := RuntimeConfig{}
			config.Repository.RootPath = runtime.Paths.Root
			destination := filepath.Join(t.TempDir(), "backup")
			manifest, err := runtime.CreateBackup(ctx, destination)
			if err != nil {
				t.Fatal(err)
			}
			if err = runtime.Close(ctx); err != nil {
				t.Fatal(err)
			}
			runtime.closed = false
			entries, err := os.ReadDir(runtime.Paths.Root)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Name() != "locks" {
					if err = os.RemoveAll(filepath.Join(runtime.Paths.Root, entry.Name())); err != nil {
						t.Fatal(err)
					}
				}
			}
			payload, err := RestoreBackup(ctx, config, destination, rebuild)
			if err != nil || payload["repo_revision"] != manifest.RepoRevision || payload["rebuild_derived"] != rebuild {
				t.Fatal(payload, err)
			}
			revision, err := repository.GetMainRevision(RuntimePathsFor(config).Repository)
			if err != nil || revision != manifest.RepoRevision {
				t.Fatal(revision, err)
			}
		})
	}
}
func TestRestoreBackupFailures(t *testing.T) {
	ctx := context.Background()
	runtime := backupRuntime(t)
	config := RuntimeConfig{}
	config.Repository.RootPath = runtime.Paths.Root
	destination := filepath.Join(t.TempDir(), "backup")
	if _, err := runtime.CreateBackup(ctx, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreBackup(ctx, config, destination, true); err == nil {
		t.Fatal("active writer")
	}
	if _, err := RestoreBackup(ctx, config, filepath.Join(runtime.Paths.Root, "nested"), true); err == nil {
		t.Fatal("nested")
	}
	if err := runtime.Close(ctx); err != nil {
		t.Fatal(err)
	}
	runtime.closed = false
	raw, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest BackupManifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.RepoRevision = strings.Repeat("0", 40)
	raw, _ = json.Marshal(manifest)
	os.WriteFile(filepath.Join(destination, "manifest.json"), raw, 0600)
	if _, err = RestoreBackup(ctx, config, destination, true); err == nil || !strings.Contains(err.Error(), "revision") {
		t.Fatal(err)
	}
}

type fakeDirEntry string

func (e fakeDirEntry) Name() string             { return string(e) }
func (fakeDirEntry) IsDir() bool                { return false }
func (fakeDirEntry) Type() os.FileMode          { return 0 }
func (fakeDirEntry) Info() (os.FileInfo, error) { return fakeFileInfo{}, nil }
func TestRestoreBackupInjectedFailures(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	config := RuntimeConfig{}
	config.Repository.RootPath = filepath.Join(t.TempDir(), "state")
	leasePath := filepath.Join(t.TempDir(), "lock")
	lease, err := repository.AcquireWriterLease(leasePath, "test")
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
	base := restoreBackupOps{lease: func(string, string) (*repository.WriterLease, error) {
		return repository.AcquireWriterLease(leasePath, "test")
	}, manifest: func(string) (BackupManifest, error) {
		return BackupManifest{1, "r", map[string]string{"derived.sqlite": "x"}}, nil
	}, stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }, temp: func(string, string) (string, error) { return t.TempDir(), nil }, mkdirAll: func(string, os.FileMode) error { return nil }, mkdir: func(string, os.FileMode) error { return nil }, removeAll: func(string) error { return nil }, extract: func(string, string) error { return nil }, copy: func(string, string) error { return nil }, revision: func(repository.GitRepositoryPaths) (string, error) { return "r", nil }, materialize: func(context.Context, repository.GitRepositoryPaths, string) (repository.MaterializedCheckout, error) {
		return repository.MaterializedCheckout{}, nil
	}, rebuild: func(context.Context, *derived.Index, string, string) error { return nil }, readDir: func(string) ([]os.DirEntry, error) { return nil, nil }, rename: func(string, string) error { return nil }}
	for _, stage := range []string{"lease", "manifest", "parent", "temp", "staging", "extract", "control", "derived", "revision", "mismatch", "materialize", "rebuild", "root", "previous", "read-root", "move-root", "read-staging"} {
		ops := base
		copies := 0
		mkdirs := 0
		reads := 0
		switch stage {
		case "lease":
			ops.lease = func(string, string) (*repository.WriterLease, error) { return nil, boom }
		case "manifest":
			ops.manifest = func(string) (BackupManifest, error) { return BackupManifest{}, boom }
		case "parent":
			ops.stat = func(string) (os.FileInfo, error) { return nil, boom }
			ops.temp = func(parent, prefix string) (string, error) {
				if parent != filepath.Dir(t.TempDir()) {
					return "", boom
				}
				return "", boom
			}
		case "temp":
			ops.temp = func(string, string) (string, error) { return "", boom }
		case "staging":
			ops.mkdirAll = func(string, os.FileMode) error { return boom }
		case "extract":
			ops.extract = func(string, string) error { return boom }
		case "control":
			ops.copy = func(string, string) error { return boom }
		case "derived":
			ops.copy = func(string, string) error {
				copies++
				if copies == 2 {
					return boom
				}
				return nil
			}
		case "revision":
			ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
		case "mismatch":
			ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "other", nil }
		case "materialize":
			ops.materialize = func(context.Context, repository.GitRepositoryPaths, string) (repository.MaterializedCheckout, error) {
				return repository.MaterializedCheckout{}, boom
			}
		case "rebuild":
			ops.rebuild = func(context.Context, *derived.Index, string, string) error { return boom }
		case "root":
			ops.mkdirAll = func(string, os.FileMode) error {
				mkdirs++
				if mkdirs == 2 {
					return boom
				}
				return nil
			}
		case "previous":
			ops.mkdir = func(string, os.FileMode) error { return boom }
		case "read-root":
			ops.readDir = func(string) ([]os.DirEntry, error) { return nil, boom }
		case "move-root":
			ops.readDir = func(string) ([]os.DirEntry, error) {
				return []os.DirEntry{fakeDirEntry("locks"), fakeDirEntry("old")}, nil
			}
			ops.rename = func(string, string) error { return boom }
		case "read-staging":
			ops.readDir = func(string) ([]os.DirEntry, error) {
				reads++
				if reads == 2 {
					return nil, boom
				}
				return nil, nil
			}
		}
		_, err := restoreBackup(ctx, config, t.TempDir(), stage != "derived", ops)
		if stage == "mismatch" {
			if err == nil || !strings.Contains(err.Error(), "revision") {
				t.Fatal(stage, err)
			}
		} else if !errors.Is(err, boom) {
			t.Fatal(stage, err)
		}
	}
}
func TestRestoreInstallRollback(t *testing.T) {
	ctx := context.Background()
	config := RuntimeConfig{}
	config.Repository.RootPath = filepath.Join(t.TempDir(), "state")
	leasePath := filepath.Join(t.TempDir(), "lock")
	phase := 0
	renames := 0
	ops := restoreBackupOps{lease: func(string, string) (*repository.WriterLease, error) {
		return repository.AcquireWriterLease(leasePath, "test")
	}, manifest: func(string) (BackupManifest, error) { return BackupManifest{1, "r", map[string]string{}}, nil }, stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }, temp: func(string, string) (string, error) { return t.TempDir(), nil }, mkdirAll: func(string, os.FileMode) error { return nil }, mkdir: func(string, os.FileMode) error { return nil }, removeAll: func(string) error { return nil }, extract: func(string, string) error { return nil }, copy: func(string, string) error { return nil }, revision: func(repository.GitRepositoryPaths) (string, error) { return "r", nil }, materialize: func(context.Context, repository.GitRepositoryPaths, string) (repository.MaterializedCheckout, error) {
		return repository.MaterializedCheckout{}, nil
	}, rebuild: func(context.Context, *derived.Index, string, string) error { return nil }, readDir: func(string) ([]os.DirEntry, error) {
		phase++
		if phase == 1 {
			return []os.DirEntry{fakeDirEntry("locks"), fakeDirEntry("old")}, nil
		}
		return []os.DirEntry{fakeDirEntry("locks"), fakeDirEntry("new"), fakeDirEntry("fail")}, nil
	}, rename: func(from, to string) error {
		renames++
		if strings.HasSuffix(from, "fail") {
			return errors.New("install")
		}
		return nil
	}}
	if _, err := restoreBackup(ctx, config, t.TempDir(), false, ops); err == nil {
		t.Fatal("install")
	}
	if renames < 4 {
		t.Fatal("rollback", renames)
	}
}
func TestExtractBackupArchiveSafety(t *testing.T) {
	for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink} {
		path := writeTestArchive(t, "../bad", kind)
		if err := extractBackupArchive(path, t.TempDir()); err == nil || !strings.Contains(err.Error(), "linked") {
			t.Fatal(kind, err)
		}
	}
	path := writeTestArchive(t, "../bad", tar.TypeReg)
	if err := extractBackupArchive(path, t.TempDir()); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatal(err)
	}
	path = writeTestArchive(t, "fifo", tar.TypeFifo)
	if err := extractBackupArchive(path, t.TempDir()); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal(err)
	}
}
func writeTestArchive(t *testing.T, name string, kind byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	archive := tar.NewWriter(gz)
	if err = archive.WriteHeader(&tar.Header{Name: name, Typeflag: kind, Mode: 0600}); err != nil {
		t.Fatal(err)
	}
	archive.Close()
	gz.Close()
	file.Close()
	return path
}
func TestBackupPathAndManifestFailures(t *testing.T) {
	runtime := backupRuntime(t)
	if _, err := runtime.CreateBackup(context.Background(), filepath.Join(runtime.Paths.Root, "backup")); err == nil {
		t.Fatal("nested")
	}
	outside := t.TempDir()
	for name, payload := range map[string]any{"schema": map[string]any{"schema_version": 2, "repo_revision": "r", "files": map[string]string{}}, "unknown": map[string]any{"schema_version": 1, "repo_revision": "r", "files": map[string]string{"repo.git.tar.gz": "x", "control.sqlite": "x", "extra": "x"}}, "field": map[string]any{"schema_version": 1, "repo_revision": "r", "files": map[string]string{"repo.git.tar.gz": "x", "control.sqlite": "x"}, "extra": 1}} {
		directory := filepath.Join(outside, name)
		os.MkdirAll(directory, 0700)
		raw, _ := json.Marshal(payload)
		os.WriteFile(filepath.Join(directory, "manifest.json"), raw, 0600)
		if _, err := LoadBackupManifest(directory); err == nil {
			t.Fatal(name)
		}
	}
	if _, err := LoadBackupManifest(filepath.Join(outside, "absent")); err == nil {
		t.Fatal("absent")
	}
	destination := filepath.Join(outside, "valid")
	manifest, err := runtime.CreateBackup(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(destination, "control.sqlite"))
	if _, err = LoadBackupManifest(destination); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(destination, "control.sqlite"), []byte("bad"), 0600)
	manifest.Files["control.sqlite"] = "wrong"
	raw, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(destination, "manifest.json"), raw, 0600)
	if _, err = LoadBackupManifest(destination); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal(err)
	}
}
func TestCreateBackupInjectedFailures(t *testing.T) {
	runtime := &Runtime{Paths: RuntimePaths{Root: "/state", ControlDB: "control", DerivedDB: "derived"}}
	boom := errors.New("boom")
	base := backupCreateOps{mkdir: func(string, os.FileMode) error { return nil }, archive: func(string, string, string) error { return nil }, snapshot: func(context.Context, string, string) error { return nil }, stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }, hash: func(string) (string, error) { return "hash", nil }, revision: func(repository.GitRepositoryPaths) (string, error) { return "r", nil }, write: func(string, []byte, os.FileMode) error { return nil }}
	for _, stage := range []string{"mkdir", "archive", "control", "derived-stat", "derived", "hash", "revision", "write"} {
		ops := base
		snapshots := 0
		switch stage {
		case "mkdir":
			ops.mkdir = func(string, os.FileMode) error { return boom }
		case "archive":
			ops.archive = func(string, string, string) error { return boom }
		case "control":
			ops.snapshot = func(context.Context, string, string) error { return boom }
		case "derived-stat":
			ops.stat = func(string) (os.FileInfo, error) { return nil, boom }
		case "derived":
			ops.snapshot = func(context.Context, string, string) error {
				snapshots++
				if snapshots == 2 {
					return boom
				}
				return nil
			}
		case "hash":
			ops.hash = func(string) (string, error) { return "", boom }
		case "revision":
			ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
		case "write":
			ops.write = func(string, []byte, os.FileMode) error { return boom }
		}
		if _, err := runtime.createBackup(context.Background(), t.TempDir(), ops); !errors.Is(err, boom) {
			t.Fatal(stage, err)
		}
	}
	ops := base
	ops.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	manifest, err := runtime.createBackup(context.Background(), t.TempDir(), ops)
	if err != nil || len(manifest.Files) != 2 {
		t.Fatal(manifest, err)
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string      { return "x" }
func (fakeFileInfo) Size() int64       { return 0 }
func (fakeFileInfo) Mode() os.FileMode { return 0600 }

func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }
func TestLoadBackupManifestUnknownFile(t *testing.T) {
	raw := []byte(`{"schema_version":1,"repo_revision":"r","files":{"repo.git.tar.gz":"x","control.sqlite":"x","extra":"x"}}`)
	ops := backupManifestOps{read: func(string) ([]byte, error) { return raw, nil }, stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }, hash: func(string) (string, error) { return "x", nil }}
	if _, err := loadBackupManifest("x", ops); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal(err)
	}
}
func TestLoadBackupManifestHashFailure(t *testing.T) {
	boom := errors.New("boom")
	valid := []byte(`{"schema_version":1,"repo_revision":"r","files":{"repo.git.tar.gz":"x","control.sqlite":"x"}}`)
	ops := backupManifestOps{read: func(string) ([]byte, error) { return valid, nil }, stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }, hash: func(string) (string, error) { return "", boom }}
	if _, err := loadBackupManifest("x", ops); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
func TestLoadBackupManifestInjectedFailures(t *testing.T) {
	boom := errors.New("boom")
	valid := []byte(`{"schema_version":1,"repo_revision":"r","files":{"repo.git.tar.gz":"x","control.sqlite":"x"}}`)
	base := backupManifestOps{read: func(string) ([]byte, error) { return valid, nil }, stat: func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }, hash: func(string) (string, error) { return "x", nil }}
	for _, stage := range []string{"read", "stat", "hash"} {
		ops := base
		switch stage {
		case "read":
			ops.read = func(string) ([]byte, error) { return nil, boom }
		case "stat":
			ops.stat = func(string) (os.FileInfo, error) { return nil, boom }
		case "hash":
			ops.hash = func(string) (string, error) { return "", boom }
		}
		if _, err := loadBackupManifest("x", ops); !errors.Is(err, boom) {
			t.Fatal(stage, err)
		}
	}
}

type failingDirEntry struct{ infoErr error }

func (failingDirEntry) Name() string                 { return "x" }
func (failingDirEntry) IsDir() bool                  { return false }
func (failingDirEntry) Type() os.FileMode            { return 0 }
func (e failingDirEntry) Info() (os.FileInfo, error) { return nil, e.infoErr }
func TestInjectedArchiveFailures(t *testing.T) {
	boom := errors.New("boom")
	root := t.TempDir()
	target := filepath.Join(root, "x.gz")
	if err := archiveDirectoryWith(root, target, "repo", os.Create, func(string, fs.WalkDirFunc) error { return boom }, os.Open, tar.FileInfoHeader); !errors.Is(err, boom) {
		t.Fatal("walk", err)
	}
	walkEntry := func(entry fs.DirEntry, walkErr error) func(string, fs.WalkDirFunc) error {
		return func(root string, callback fs.WalkDirFunc) error { return callback(root, entry, walkErr) }
	}
	if err := archiveDirectoryWith(root, target, "repo", os.Create, walkEntry(nil, boom), os.Open, tar.FileInfoHeader); !errors.Is(err, boom) {
		t.Fatal("callback", err)
	}
	if err := archiveDirectoryWith(root, target, "repo", os.Create, walkEntry(failingDirEntry{infoErr: boom}, nil), os.Open, tar.FileInfoHeader); !errors.Is(err, boom) {
		t.Fatal("info", err)
	}
	regular := fakeArchiveEntry{info: fakeFileInfo{}}
	if err := archiveDirectoryWith(root, target, "repo", os.Create, walkEntry(regular, nil), func(string) (*os.File, error) { return nil, boom }, tar.FileInfoHeader); !errors.Is(err, boom) {
		t.Fatal("open", err)
	}
	if err := archiveDirectoryWith(root, target, "repo", os.Create, walkEntry(fakeArchiveEntry{info: fakeFileInfo{}}, nil), os.Open, func(fs.FileInfo, string) (*tar.Header, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal("header", err)
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	if err = archiveDirectoryWith(root, target, "repo", func(string) (*os.File, error) { return closed, nil }, walkEntry(fakeArchiveEntry{info: fakeFileInfo{}}, nil), os.Open, tar.FileInfoHeader); err == nil {
		t.Fatal("header write")
	}
	directory, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	if err = archiveDirectoryWith(root, target, "repo", os.Create, walkEntry(regular, nil), func(string) (*os.File, error) { return directory, nil }, tar.FileInfoHeader); err == nil {
		t.Fatal("archive copy")
	}
}

type fakeArchiveEntry struct{ info os.FileInfo }

func (fakeArchiveEntry) Name() string                 { return "x" }
func (fakeArchiveEntry) IsDir() bool                  { return false }
func (fakeArchiveEntry) Type() os.FileMode            { return 0 }
func (e fakeArchiveEntry) Info() (os.FileInfo, error) { return e.info, nil }
func TestInjectedExtractCopyFailures(t *testing.T) {
	boom := errors.New("boom")
	directoryArchive := writeTestArchive(t, "dir", tar.TypeDir)
	if err := extractBackupArchiveWith(directoryArchive, t.TempDir(), func(string, os.FileMode) error { return boom }, os.OpenFile); !errors.Is(err, boom) {
		t.Fatal("mkdir dir", err)
	}
	fileArchive := writeTestArchive(t, "dir/file", tar.TypeReg)
	if err := extractBackupArchiveWith(fileArchive, t.TempDir(), func(string, os.FileMode) error { return boom }, os.OpenFile); !errors.Is(err, boom) {
		t.Fatal("mkdir file", err)
	}
	if err := extractBackupArchiveWith(fileArchive, t.TempDir(), os.MkdirAll, func(string, int, os.FileMode) (*os.File, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal("open file", err)
	}
	closedOutput, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	closedOutput.Close()
	if err = extractBackupArchiveWith(fileArchive, t.TempDir(), os.MkdirAll, func(string, int, os.FileMode) (*os.File, error) { return closedOutput, nil }); err == nil {
		t.Fatal("extract copy")
	}
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	source := filepath.Join(t.TempDir(), "source")
	os.WriteFile(source, []byte("x"), 0600)
	if err = copyFileWith(source, "target", func(string) (*os.File, error) { return closed, nil }, os.OpenFile); err == nil {
		t.Fatal("stat")
	}
}
func TestLowLevelBackupFailures(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "dir")
	os.Mkdir(directory, 0700)
	if _, err := sha256File(directory); err == nil {
		t.Fatal("hash copy")
	}
	if err := archiveDirectory(root, directory, "repo.git"); err == nil {
		t.Fatal("archive create")
	}
	badArchive := filepath.Join(root, "bad.gz")
	os.WriteFile(badArchive, []byte("bad"), 0600)
	if err := extractBackupArchive(badArchive, t.TempDir()); err == nil {
		t.Fatal("gzip")
	}
	truncated := filepath.Join(root, "truncated.gz")
	file, _ := os.Create(truncated)
	gz := gzip.NewWriter(file)
	gz.Write([]byte("bad tar"))
	gz.Close()
	file.Close()
	if err := extractBackupArchive(truncated, t.TempDir()); err == nil {
		t.Fatal("tar")
	}
	if err := extractBackupArchive(filepath.Join(root, "missing"), t.TempDir()); err == nil {
		t.Fatal("open archive")
	}
	if err := copyFile(filepath.Join(root, "missing"), filepath.Join(root, "target")); err == nil {
		t.Fatal("copy source")
	}
	source := filepath.Join(root, "source")
	os.WriteFile(source, []byte("x"), 0600)
	if err := copyFile(source, directory); err == nil {
		t.Fatal("copy target")
	}
	path := writeTestArchive(t, "nested/file", tar.TypeReg)
	if err := extractBackupArchive(path, directory); err != nil {
		t.Fatal(err)
	}
	path = writeTestArchive(t, "nested", tar.TypeDir)
	if err := extractBackupArchive(path, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
func TestBackupHelpers(t *testing.T) {
	root := t.TempDir()
	if err := rejectNestedStatePath(root, root, "x"); err == nil {
		t.Fatal("same")
	}
	if err := rejectNestedStatePath(filepath.Join(root, "child"), root, "x"); err == nil {
		t.Fatal("child")
	}
	if err := rejectNestedStatePath(t.TempDir(), root, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := sha256File(filepath.Join(root, "missing")); err == nil {
		t.Fatal("hash")
	}
	if err := sqliteSnapshot(context.Background(), root, filepath.Join(root, "snapshot")); err == nil {
		t.Fatal("snapshot")
	}
	if err := archiveDirectory(filepath.Join(root, "missing"), filepath.Join(root, "x.tar.gz"), "repo.git"); err == nil {
		t.Fatal("archive")
	}
}
