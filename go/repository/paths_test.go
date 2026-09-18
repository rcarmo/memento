package repository

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func pathTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "regular"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"linked": "directory", "link-target": "regular", "dangling": "absent"} {
		if err := os.Symlink(filepath.Join(root, target), filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func TestRepositoryPathReference(t *testing.T) {
	root := pathTree(t)
	raw, err := os.ReadFile("../testdata/parity/repository-paths.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Path           string
		Reserved       bool
		CanonicalError string `json:"canonical_error"`
		ReadError      string `json:"read_error"`
		WriteError     string `json:"write_error"`
		ClassifyOnly   bool   `json:"classification_only"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Path, func(t *testing.T) {
			if got := IsReservedBundlePath(c.Path); got != c.Reserved {
				t.Fatal(got, c.Reserved)
			}
			if c.ClassifyOnly {
				return
			}
			if got := errorText(ValidateBundlePath(c.Path)); got != c.CanonicalError {
				t.Fatal(got, c.CanonicalError)
			}
			for _, mode := range []string{"read", "write"} {
				check := ValidateRepositoryWritePath
				want := c.WriteError
				if mode == "read" {
					check = ValidateRepositoryReadPath
					want = c.ReadError
				}
				safe, err := check(root, c.Path)
				if errorText(err) != want {
					t.Fatal(mode, err, want)
				}
				if err == nil {
					if safe.BundlePath != c.Path || safe.AbsolutePath != filepath.Join(root, c.Path[1:]) {
						t.Fatal(safe)
					}
				}
			}
		})
	}
}
func TestRepositoryRootAndIOFailures(t *testing.T) {
	root := pathTree(t)
	link := filepath.Join(filepath.Dir(root), "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{root + "-missing", link, link + "/", filepath.Join(root, "regular")} {
		if _, err := ValidateRepositoryWritePath(bad, "/new"); errorText(err) != "repository root must be a real directory" {
			t.Fatal(bad, err)
		}
	}
	for _, failAt := range []int{1, 2, 3} {
		calls := 0
		lstat := func(path string) (fs.FileInfo, error) {
			calls++
			if calls == failAt {
				return nil, fs.ErrPermission
			}
			return os.Lstat(path)
		}
		if _, err := validateWritePath(root, "/directory/new", lstat); !errors.Is(err, fs.ErrPermission) {
			t.Fatal(failAt, err)
		}
	}
	calls := 0
	lstat := func(path string) (fs.FileInfo, error) {
		calls++
		if calls == 3 {
			return nil, fs.ErrPermission
		}
		return os.Lstat(path)
	}
	if _, err := validateReadPath(root, "/regular", lstat); !errors.Is(err, fs.ErrPermission) {
		t.Fatal(err)
	}
	var safety *PathSafetyError
	if err := ValidateBundlePath("relative"); !errors.As(err, &safety) {
		t.Fatal(err)
	}
}
func FuzzBundlePath(f *testing.F) {
	for _, path := range []string{"/", "/one/two.md", "/../x", "/.assets", "/log.md"} {
		f.Add(path)
	}
	f.Fuzz(func(t *testing.T, path string) { _ = IsReservedBundlePath(path); _ = ValidateBundlePath(path) })
}

func TestPathlibRootSemantics(t *testing.T) {
	for _, c := range []struct{ In, Want string }{{"", "."}, {"/", "/"}, {"//host/./x", "//host/x"}, {"///host/x", "/host/x"}, {"one/../two/", "one/../two"}} {
		if got := pythonPath(c.In); got != c.Want {
			t.Fatal(c, got)
		}
	}
	if joinPath(".", "file") != "file" {
		t.Fatal("relative root")
	}
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "outside", "child"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "outside", "child"), filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	root := base + "/link/.."
	safe, err := ValidateRepositoryWritePath(root, "/file")
	if err != nil || safe.AbsolutePath != root+"/file" {
		t.Fatal(safe, err)
	}
}
