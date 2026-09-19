package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
	"io"
	"io/fs"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const backupManifestName = "manifest.json"

type BackupManifest struct {
	SchemaVersion int               `json:"schema_version"`
	RepoRevision  string            `json:"repo_revision"`
	Files         map[string]string `json:"files"`
}

func rejectNestedStatePath(path, root, kind string) error {
	candidate, _ := filepath.Abs(path)
	state, _ := filepath.Abs(root)
	relative, _ := filepath.Rel(state, candidate)
	if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s must be outside repository root_path", kind)
	}
	return nil
}
func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err = io.CopyBuffer(digest, file, make([]byte, 65536)); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
func sqliteSnapshot(ctx context.Context, source, target string) error {
	db, _ := sql.Open("sqlite", source)
	defer db.Close()
	escaped := strings.ReplaceAll(target, "'", "''")
	_, err := db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'")
	return err
}
func archiveDirectory(source, target, name string) error {
	return archiveDirectoryWith(source, target, name, os.Create, filepath.WalkDir, os.Open, tar.FileInfoHeader)
}
func archiveDirectoryWith(source, target, name string, create func(string) (*os.File, error), walk func(string, fs.WalkDirFunc) error, open func(string) (*os.File, error), headerFor func(fs.FileInfo, string) (*tar.Header, error)) error {
	file, err := create(target)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(file)
	gz.Header.ModTime = time.Unix(0, 0)
	archive := tar.NewWriter(gz)
	walkErr := walk(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, _ := filepath.Rel(source, path)
		archivePath := name
		if relative != "." {
			archivePath = filepath.ToSlash(filepath.Join(name, relative))
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := headerFor(info, "")
		if err != nil {
			return err
		}
		header.Name = archivePath
		header.ModTime = time.Unix(0, 0)
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if err = archive.WriteHeader(header); err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			input, err := open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(archive, input)
			closeErr := input.Close()
			return errors.Join(copyErr, closeErr)
		}
		return nil
	})
	return errors.Join(walkErr, archive.Close(), gz.Close(), file.Close())
}

type backupCreateOps struct {
	mkdir    func(string, os.FileMode) error
	archive  func(string, string, string) error
	snapshot func(context.Context, string, string) error
	stat     func(string) (os.FileInfo, error)
	hash     func(string) (string, error)
	revision func(repository.GitRepositoryPaths) (string, error)
	write    func(string, []byte, os.FileMode) error
}

func defaultBackupCreateOps() backupCreateOps {
	return backupCreateOps{os.MkdirAll, archiveDirectory, sqliteSnapshot, os.Stat, sha256File, repository.GetMainRevision, os.WriteFile}
}
func (r *Runtime) CreateBackup(ctx context.Context, destination string) (BackupManifest, error) {
	return r.createBackup(ctx, destination, defaultBackupCreateOps())
}
func (r *Runtime) createBackup(ctx context.Context, destination string, ops backupCreateOps) (BackupManifest, error) {
	if err := rejectNestedStatePath(destination, r.Paths.Root, "backup destination"); err != nil {
		return BackupManifest{}, err
	}
	if err := ops.mkdir(destination, 0777); err != nil {
		return BackupManifest{}, err
	}
	files := map[string]string{}
	repoArchive := filepath.Join(destination, "repo.git.tar.gz")
	if err := ops.archive(r.Paths.Repository.BareDir, repoArchive, "repo.git"); err != nil {
		return BackupManifest{}, err
	}
	controlCopy := filepath.Join(destination, "control.sqlite")
	if err := ops.snapshot(ctx, r.Paths.ControlDB, controlCopy); err != nil {
		return BackupManifest{}, err
	}
	names := []string{"repo.git.tar.gz", "control.sqlite"}
	if _, err := ops.stat(r.Paths.DerivedDB); err == nil {
		if err = ops.snapshot(ctx, r.Paths.DerivedDB, filepath.Join(destination, "derived.sqlite")); err != nil {
			return BackupManifest{}, err
		}
		names = append(names, "derived.sqlite")
	} else if !errors.Is(err, os.ErrNotExist) {
		return BackupManifest{}, err
	}
	sort.Strings(names)
	for _, name := range names {
		digest, err := ops.hash(filepath.Join(destination, name))
		if err != nil {
			return BackupManifest{}, err
		}
		files[name] = digest
	}
	revision, err := ops.revision(r.Paths.Repository)
	if err != nil {
		return BackupManifest{}, err
	}
	manifest := BackupManifest{1, revision, files}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	raw = append(raw, '\n')
	if err = ops.write(filepath.Join(destination, backupManifestName), raw, 0666); err != nil {
		return BackupManifest{}, err
	}
	return manifest, nil
}
func extractBackupArchive(source, destination string) error {
	return extractBackupArchiveWith(source, destination, os.MkdirAll, os.OpenFile)
}
func extractBackupArchiveWith(source, destination string, mkdir func(string, os.FileMode) error, openFile func(string, int, os.FileMode) (*os.File, error)) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	archive := tar.NewReader(gz)
	for {
		header, e := archive.Next()
		if errors.Is(e, io.EOF) {
			return nil
		}
		if e != nil {
			return e
		}
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			return fmt.Errorf("refusing to restore linked archive member: %s", header.Name)
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("refusing to restore archive member outside destination: %s", header.Name)
		}
		target := filepath.Join(destination, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if e = mkdir(target, fs.FileMode(header.Mode)&0777); e != nil {
				return e
			}
		case tar.TypeReg:
			if e = mkdir(filepath.Dir(target), 0777); e != nil {
				return e
			}
			output, e := openFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fs.FileMode(header.Mode)&0777)
			if e != nil {
				return e
			}
			_, copyErr := io.Copy(output, archive)
			closeErr := output.Close()
			if e = errors.Join(copyErr, closeErr); e != nil {
				return e
			}
		default:
			return fmt.Errorf("unsupported backup archive member: %s", header.Name)
		}
	}
}
func copyFile(source, target string) error { return copyFileWith(source, target, os.Open, os.OpenFile) }
func copyFileWith(source, target string, open func(string) (*os.File, error), openFile func(string, int, os.FileMode) (*os.File, error)) error {
	input, err := open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	return errors.Join(copyErr, output.Close())
}

type restoreBackupOps struct {
	lease       func(string, string) (*repository.WriterLease, error)
	manifest    func(string) (BackupManifest, error)
	stat        func(string) (os.FileInfo, error)
	temp        func(string, string) (string, error)
	mkdirAll    func(string, os.FileMode) error
	mkdir       func(string, os.FileMode) error
	removeAll   func(string) error
	extract     func(string, string) error
	copy        func(string, string) error
	revision    func(repository.GitRepositoryPaths) (string, error)
	materialize func(context.Context, repository.GitRepositoryPaths, string) (repository.MaterializedCheckout, error)
	rebuild     func(context.Context, *derived.Index, string, string) error
	readDir     func(string) ([]os.DirEntry, error)
	rename      func(string, string) error
}

func defaultRestoreBackupOps() restoreBackupOps {
	return restoreBackupOps{repository.AcquireWriterLease, LoadBackupManifest, os.Stat, os.MkdirTemp, os.MkdirAll, os.Mkdir, os.RemoveAll, extractBackupArchive, copyFile, repository.GetMainRevision, repository.MaterializeCurrentCheckout, func(ctx context.Context, index *derived.Index, root, revision string) error {
		return index.Rebuild(ctx, root, revision)
	}, os.ReadDir, os.Rename}
}
func RestoreBackup(ctx context.Context, config RuntimeConfig, directory string, rebuildDerived bool) (map[string]any, error) {
	return restoreBackup(ctx, config, directory, rebuildDerived, defaultRestoreBackupOps())
}
func restoreBackup(ctx context.Context, config RuntimeConfig, directory string, rebuildDerived bool, ops restoreBackupOps) (map[string]any, error) {
	paths := RuntimePathsFor(config)
	if err := rejectNestedStatePath(directory, paths.Root, "backup source"); err != nil {
		return nil, err
	}
	lease, err := ops.lease(paths.WriterLock, "memento-restore")
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	manifest, err := ops.manifest(directory)
	if err != nil {
		return nil, err
	}
	parent := filepath.Dir(paths.Root)
	if _, e := ops.stat(parent); e != nil {
		parent = filepath.Dir(directory)
	}
	temp, err := ops.temp(parent, "memento-restore-")
	if err != nil {
		return nil, err
	}
	defer ops.removeAll(temp)
	staging := filepath.Join(temp, "state")
	if err = ops.mkdirAll(staging, 0777); err != nil {
		return nil, err
	}
	if err = ops.extract(filepath.Join(directory, "repo.git.tar.gz"), staging); err != nil {
		return nil, err
	}
	if err = ops.copy(filepath.Join(directory, "control.sqlite"), filepath.Join(staging, "control.sqlite")); err != nil {
		return nil, err
	}
	if !rebuildDerived && manifest.Files["derived.sqlite"] != "" {
		if err = ops.copy(filepath.Join(directory, "derived.sqlite"), filepath.Join(staging, "derived.sqlite")); err != nil {
			return nil, err
		}
	}
	staged := config
	staged.Repository.RootPath = staging
	stagedPaths := RuntimePathsFor(staged)
	revision, err := ops.revision(stagedPaths.Repository)
	if err != nil {
		return nil, err
	}
	if revision != manifest.RepoRevision {
		return nil, errors.New("backup manifest revision does not match archived main")
	}
	if _, err = ops.materialize(ctx, stagedPaths.Repository, revision); err != nil {
		return nil, err
	}
	if rebuildDerived {
		index := &derived.Index{Path: stagedPaths.DerivedDB}
		if err = ops.rebuild(ctx, index, stagedPaths.Repository.CurrentDir, revision); err != nil {
			return nil, err
		}
	}
	if err = ops.mkdirAll(paths.Root, 0777); err != nil {
		return nil, err
	}
	previous := filepath.Join(temp, "previous")
	if err = ops.mkdir(previous, 0777); err != nil {
		return nil, err
	}
	moved := []string{}
	installed := []string{}
	entries, err := ops.readDir(paths.Root)
	if err != nil {
		return nil, err
	}
	rollback := func() {
		for i := len(installed) - 1; i >= 0; i-- {
			name := installed[i]
			_ = ops.rename(filepath.Join(paths.Root, name), filepath.Join(staging, name))
		}
		for i := len(moved) - 1; i >= 0; i-- {
			name := moved[i]
			_ = ops.rename(filepath.Join(previous, name), filepath.Join(paths.Root, name))
		}
	}
	for _, entry := range entries {
		if entry.Name() == "locks" {
			continue
		}
		if err = ops.rename(filepath.Join(paths.Root, entry.Name()), filepath.Join(previous, entry.Name())); err != nil {
			rollback()
			return nil, err
		}
		moved = append(moved, entry.Name())
	}
	stagedEntries, err := ops.readDir(staging)
	if err != nil {
		rollback()
		return nil, err
	}
	for _, entry := range stagedEntries {
		if entry.Name() == "locks" {
			continue
		}
		if err = ops.rename(filepath.Join(staging, entry.Name()), filepath.Join(paths.Root, entry.Name())); err != nil {
			rollback()
			return nil, err
		}
		installed = append(installed, entry.Name())
	}
	return map[string]any{"repo_revision": manifest.RepoRevision, "restored_root": paths.Root, "rebuild_derived": rebuildDerived}, nil
}

type backupManifestOps struct {
	read func(string) ([]byte, error)
	stat func(string) (os.FileInfo, error)
	hash func(string) (string, error)
}

func LoadBackupManifest(directory string) (BackupManifest, error) {
	return loadBackupManifest(directory, backupManifestOps{os.ReadFile, os.Stat, sha256File})
}
func loadBackupManifest(directory string, ops backupManifestOps) (BackupManifest, error) {
	raw, err := ops.read(filepath.Join(directory, backupManifestName))
	if err != nil {
		return BackupManifest{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var manifest BackupManifest
	if err = decoder.Decode(&manifest); err != nil {
		return BackupManifest{}, err
	}
	if manifest.SchemaVersion != 1 || manifest.Files["repo.git.tar.gz"] == "" || manifest.Files["control.sqlite"] == "" {
		return BackupManifest{}, errors.New("backup manifest must include supported schema and required checksums")
	}
	for name, digest := range manifest.Files {
		if name != "repo.git.tar.gz" && name != "control.sqlite" && name != "derived.sqlite" {
			return BackupManifest{}, errors.New("unsupported backup manifest file")
		}
		path := filepath.Join(directory, name)
		if _, err = ops.stat(path); errors.Is(err, os.ErrNotExist) {
			return BackupManifest{}, fmt.Errorf("backup file is missing: %s", name)
		} else if err != nil {
			return BackupManifest{}, err
		}
		actual, e := ops.hash(path)
		if e != nil {
			return BackupManifest{}, e
		}
		if actual != digest {
			return BackupManifest{}, fmt.Errorf("checksum mismatch for %s", name)
		}
	}
	return manifest, nil
}
