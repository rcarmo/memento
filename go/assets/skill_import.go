package assets

import (
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SkillImportConflictError struct{ Destination string }

func (e *SkillImportConflictError) Error() string {
	return "skill destination already exists: " + e.Destination
}

type skillImportIO struct {
	evalSymlinks, abs func(string) (string, error)
	lstat             func(string) (os.FileInfo, error)
	mkdirAll          func(string, os.FileMode) error
	mkdirTemp         func(string, string) (string, error)
	removeAll         func(string) error
	rename            func(string, string) error
	writeFile         func(string, []byte, os.FileMode) error
	chmod             func(string, os.FileMode) error
	newArchive        func([]byte) (*zip.Reader, error)
	readArchive       func(*zip.File) ([]byte, error)
}

func defaultSkillImportIO() skillImportIO {
	return skillImportIO{filepath.EvalSymlinks, filepath.Abs, os.Lstat, os.MkdirAll, os.MkdirTemp, os.RemoveAll, os.Rename, os.WriteFile, os.Chmod, func(raw []byte) (*zip.Reader, error) {
		archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err == nil {
			archive.RegisterDecompressor(12, skillBzipReader)
		}
		return archive, err
	}, readArchiveFile}
}
func skillBzipReader(r io.Reader) io.ReadCloser { return io.NopCloser(bzip2.NewReader(r)) }
func ImportSkillPack(workspace, skillName, version, skillMD string, zipBytes []byte) (string, error) {
	pack, err := ValidateSkillPack(skillName, version, skillMD, zipBytes)
	if err != nil {
		return "", err
	}
	return importValidatedSkillPack(workspace, pack, defaultSkillImportIO())
}
func importValidatedSkillPack(workspace string, pack ValidatedPack, ops skillImportIO) (string, error) {
	root, err := ops.evalSymlinks(workspace)
	if err != nil {
		return "", err
	}
	root, err = ops.abs(root)
	if err != nil {
		return "", err
	}
	parent := root
	for _, part := range []string{".pi", "skills"} {
		parent = filepath.Join(parent, part)
		info, lstatErr := ops.lstat(parent)
		if lstatErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("skill import parent must not be a symlink: %s", parent)
			}
			if !info.IsDir() {
				return "", fmt.Errorf("skill import parent must be a directory: %s", parent)
			}
		} else if !errors.Is(lstatErr, os.ErrNotExist) {
			return "", lstatErr
		}
	}
	destination := filepath.Join(parent, pack.SkillName)
	if _, err = ops.lstat(destination); err == nil {
		return "", &SkillImportConflictError{destination}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err = ops.mkdirAll(parent, 0755); err != nil {
		return "", err
	}
	temp, err := ops.mkdirTemp(parent, "."+pack.SkillName+"-")
	if err != nil {
		return "", err
	}
	keep := false
	defer func() {
		if !keep {
			_ = ops.removeAll(temp)
		}
	}()
	if err = extractValidatedSkillPack(pack, temp, ops); err != nil {
		return "", err
	}
	if err = ops.rename(temp, destination); err != nil {
		if _, statErr := ops.lstat(destination); statErr == nil {
			return "", &SkillImportConflictError{destination}
		}
		return "", err
	}
	keep = true
	return destination, nil
}
func extractValidatedSkillPack(pack ValidatedPack, destination string, ops skillImportIO) error {
	allowed := map[string]ManifestEntry{}
	for _, entry := range pack.Manifest.Entries {
		allowed[entry.Path] = entry
	}
	archive, err := ops.newArchive(pack.ZIPBytes)
	if err != nil {
		return err
	}
	directories := map[string]bool{}
	for _, file := range archive.File {
		raw := file.Name
		raw, _, _ = strings.Cut(raw, "\x00")
		member, err := validateMemberPath(raw)
		if err != nil {
			return err
		}
		if strings.HasSuffix(raw, "/") {
			continue
		}
		entry, ok := allowed[member]
		if !ok {
			return fmt.Errorf("validated manifest is missing ZIP entry: %s", member)
		}
		data, err := ops.readArchive(file)
		if err != nil {
			return err
		}
		if uint64(len(data)) != entry.Size {
			return fmt.Errorf("ZIP entry size changed during import: %s", member)
		}
		target := filepath.Join(destination, filepath.FromSlash(member))
		if err = ops.mkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		directories[filepath.Dir(target)] = true
		if err = ops.writeFile(target, data, 0644); err != nil {
			return err
		}
		if err = ops.chmod(target, 0644); err != nil {
			return err
		}
	}
	paths := make([]string, 0, len(directories))
	for path := range directories {
		paths = append(paths, path)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	for _, path := range paths {
		if err = ops.chmod(path, 0755); err != nil {
			return err
		}
	}
	return ops.chmod(destination, 0755)
}
