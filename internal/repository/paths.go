// Package repository ports Memento's repository invariants. Path validation
// alone does not authorise access or protect subsequent filesystem operations
// against concurrent replacement; transaction/open code must enforce that too.
package repository

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// PathSafetyError is the source's explicit unsafe-path failure. OS failures
// are returned separately rather than turned into successful validation.
type PathSafetyError struct{ Message string }

func (e *PathSafetyError) Error() string { return e.Message }
func unsafe(message string) error        { return &PathSafetyError{Message: message} }

type SafeRepositoryPath struct{ BundlePath, AbsolutePath string }

// ValidateBundlePath rejects aliases before authorisation or filesystem use.
// Slash is the namespace root, but not a writable repository file.
func ValidateBundlePath(bundlePath string) error {
	if !strings.HasPrefix(bundlePath, "/") || strings.Contains(bundlePath, `\`) {
		return unsafe("bundle paths must be canonical absolute paths")
	}
	if bundlePath != "/" {
		for _, part := range strings.Split(bundlePath[1:], "/") {
			if part == "" || part == "." || part == ".." {
				return unsafe("bundle paths must not contain empty or dot components")
			}
		}
	}
	for _, r := range bundlePath {
		if r < 32 || r == 127 {
			return unsafe("bundle paths must not contain control characters")
		}
	}
	return nil
}
func reservedFilename(name string) bool { return name == "index.md" || name == ".memory-schema.json" }

// IsReservedBundlePath is a classification helper, not path validation. Match
// pathlib's component folding for this helper only; never use it to sanitise
// an untrusted path before calling ValidateBundlePath.
func IsReservedBundlePath(bundlePath string) bool {
	if bundlePath == "/.assets" || strings.HasPrefix(bundlePath, "/.assets/") {
		return true
	}
	relative := strings.TrimPrefix(bundlePath, "/")
	parts := []string{}
	if strings.HasPrefix(relative, "/") {
		parts = append(parts, "/")
	}
	for _, part := range strings.Split(relative, "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return false
	}
	name := parts[len(parts)-1]
	return reservedFilename(name) || (len(parts) == 1 && name == "log.md")
}

// pathlib folds empty/dot components but deliberately preserves '..'. Cleaning
// those with filepath.Clean would resolve a symlink/.. root differently.
func pythonPath(path string) string {
	prefix := ""
	if strings.HasPrefix(path, "/") {
		prefix = "/"
	}
	if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		prefix = "//"
	}
	parts := []string{}
	for _, part := range strings.Split(path, "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	value := prefix + strings.Join(parts, "/")
	if value == "" {
		return "."
	}
	return value
}
func joinPath(root, relative string) string {
	if relative == "" {
		return root
	}
	if root == "." {
		return relative
	}
	return strings.TrimRight(root, "/") + "/" + relative
}

func ValidateRepositoryWritePath(root, bundlePath string) (SafeRepositoryPath, error) {
	return validateWritePath(root, bundlePath, os.Lstat)
}
func validateWritePath(root, bundlePath string, lstat func(string) (fs.FileInfo, error)) (SafeRepositoryPath, error) {
	fail := func(err error) (SafeRepositoryPath, error) { return SafeRepositoryPath{}, err }
	if err := ValidateBundlePath(bundlePath); err != nil {
		return fail(err)
	}
	// pathlib strips a trailing slash before is_symlink; retain that safeguard
	// so callers cannot turn a symlink root into a directory by adding '/'.
	root = pythonPath(root)
	info, err := lstat(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fail(unsafe("repository root must be a real directory"))
		}
		return fail(err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return fail(unsafe("repository root must be a real directory"))
	}
	relative := strings.TrimPrefix(bundlePath, "/")
	parts := []string{}
	if relative != "" {
		parts = strings.Split(relative, "/")
	}
	filename := ""
	if len(parts) > 0 {
		filename = parts[len(parts)-1]
	}
	if reservedFilename(filename) {
		return fail(unsafe("reserved file cannot be written: " + filename))
	}
	if len(parts) == 1 && filename == "log.md" {
		return fail(unsafe("reserved root file cannot be written: " + filename))
	}
	current := root
	for _, part := range parts[:max(0, len(parts)-1)] {
		current = joinPath(current, part)
		info, err = lstat(current)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fail(err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fail(unsafe("symlink path components are not allowed"))
		}
		if !info.IsDir() {
			return fail(unsafe("non-directory parent path component"))
		}
	}
	absolute := joinPath(root, relative)
	info, err = lstat(absolute)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fail(err)
		}
	} else {
		if info.Mode()&fs.ModeSymlink != 0 {
			return fail(unsafe("symlink target is not allowed"))
		}
		if !info.Mode().IsRegular() {
			return fail(unsafe("special file target is not allowed"))
		}
	}
	return SafeRepositoryPath{BundlePath: bundlePath, AbsolutePath: absolute}, nil
}
func ValidateRepositoryReadPath(root, bundlePath string) (SafeRepositoryPath, error) {
	return validateReadPath(root, bundlePath, os.Lstat)
}
func validateReadPath(root, bundlePath string, lstat func(string) (fs.FileInfo, error)) (SafeRepositoryPath, error) {
	safe, err := validateWritePath(root, bundlePath, lstat)
	if err != nil {
		return SafeRepositoryPath{}, err
	}
	if _, err = lstat(safe.AbsolutePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return SafeRepositoryPath{}, unsafe("path does not exist: " + bundlePath)
		}
		return SafeRepositoryPath{}, err
	}
	return safe, nil
}
