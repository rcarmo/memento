package repository

import (
	"bytes"
	"crypto/sha256"
	encodinghex "encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var legacyBlobPointer = regexp.MustCompile(`\Aversion https://git-lfs\.github\.com/spec/v1\noid sha256:([0-9a-f]{64})\nsize ([0-9]+)\n?\z`)

const legacyLFSFilter = "filter=lfs"

type LegacyBlobMigrationError struct{ Message string }

func (e *LegacyBlobMigrationError) Error() string { return e.Message }

type legacyBlobIO struct {
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	remove    func(string) error
	walkDir   func(string, fs.WalkDirFunc) error
	rel       func(string, string) (string, error)
	stat      func(string) (os.FileInfo, error)
}

func defaultLegacyBlobIO() legacyBlobIO {
	return legacyBlobIO{os.ReadFile, os.WriteFile, os.Remove, filepath.WalkDir, filepath.Rel, os.Stat}
}
func RepositoryNeedsLegacyBlobMigration(root string) (bool, error) {
	return repositoryNeedsLegacyBlobMigration(root, defaultLegacyBlobIO())
}
func repositoryNeedsLegacyBlobMigration(root string, ops legacyBlobIO) (bool, error) {
	attributes := filepath.Join(root, ".gitattributes")
	if raw, err := ops.readFile(attributes); err == nil {
		if strings.Contains(string(raw), legacyLFSFilter) {
			return true, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	needed := false
	err := ops.walkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if needed || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 256 {
			return nil
		}
		raw, err := ops.readFile(path)
		if err != nil {
			return err
		}
		needed = legacyBlobPointer.Match(raw)
		return nil
	})
	return needed, err
}

func MigrateLegacyBlobsToGit(worktree, objectRoot string) ([]string, error) {
	return migrateLegacyBlobsToGit(worktree, objectRoot, defaultLegacyBlobIO())
}
func migrateLegacyBlobsToGit(worktree, objectRoot string, ops legacyBlobIO) ([]string, error) {
	changed := map[string]bool{}
	attributes := filepath.Join(worktree, ".gitattributes")
	if raw, err := ops.readFile(attributes); err == nil {
		lines := strings.Split(string(raw), "\n")
		retained := make([]string, 0, len(lines))
		for _, line := range lines {
			if line != "" && !strings.Contains(line, legacyLFSFilter) {
				retained = append(retained, line)
			}
		}
		var replacement string
		if len(retained) > 0 {
			replacement = strings.Join(retained, "\n") + "\n"
		}
		if replacement != string(raw) {
			if replacement == "" {
				if err = ops.remove(attributes); err != nil {
					return nil, err
				}
			} else if err = ops.writeFile(attributes, []byte(replacement), infoMode(attributes, ops)); err != nil {
				return nil, err
			}
			changed["/.gitattributes"] = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	files := []string{}
	if err := ops.walkDir(worktree, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, target := range files {
		raw, err := ops.readFile(target)
		if err != nil {
			return nil, err
		}
		match := legacyBlobPointer.FindSubmatch(raw)
		if match == nil {
			continue
		}
		relative, err := ops.rel(worktree, target)
		if err != nil {
			return nil, err
		}
		bundlePath := "/" + filepath.ToSlash(relative)
		expectedDigest := string(match[1])
		expectedSize, err := strconv.ParseInt(string(match[2]), 10, 64)
		if err != nil {
			return nil, err
		}
		source := filepath.Join(objectRoot, expectedDigest[:2], expectedDigest[2:4], expectedDigest)
		blob, err := ops.readFile(source)
		if errors.Is(err, os.ErrNotExist) {
			return nil, &LegacyBlobMigrationError{fmt.Sprintf("legacy blob is unavailable: %s", bundlePath)}
		}
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(blob)
		if int64(len(blob)) != expectedSize || !bytes.Equal([]byte(encodinghex.EncodeToString(digest[:])), []byte(expectedDigest)) {
			return nil, &LegacyBlobMigrationError{fmt.Sprintf("legacy blob failed verification: %s", bundlePath)}
		}
		if err = ops.writeFile(target, blob, infoMode(target, ops)); err != nil {
			return nil, err
		}
		changed[bundlePath] = true
	}
	out := make([]string, 0, len(changed))
	for path := range changed {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}
func infoMode(path string, ops legacyBlobIO) os.FileMode {
	if info, err := ops.stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}
