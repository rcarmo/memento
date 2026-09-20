package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestMaterializedCheckoutReference(t *testing.T) {
	paths, f := fixtureGit(t)
	raw, err := os.ReadFile("../../testdata/parity/repository-git.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Checkout map[string]struct {
			Content    []byte
			Link       string
			Executable bool
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(paths.CurrentDir, 0700)
	_ = os.WriteFile(filepath.Join(paths.CurrentDir, "obsolete"), nil, 0600)
	result, err := MaterializeCurrentCheckout(context.Background(), paths, f.Next)
	if err != nil || result.Revision != f.Next || result.Path != paths.CurrentDir {
		t.Fatal(result, err)
	}
	for name, item := range fixture.Checkout {
		path := filepath.Join(paths.CurrentDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if item.Link != "" {
			got, err := os.Readlink(path)
			if err != nil || got != item.Link {
				t.Fatal(got, err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != string(item.Content) || (info.Mode()&0111 != 0) != item.Executable {
			t.Fatal(name, string(got), info.Mode(), err)
		}
	}
	if _, err = os.Stat(filepath.Join(paths.CurrentDir, "obsolete")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if result, err = MaterializeCurrentCheckout(context.Background(), paths, ""); err != nil || result.Revision != f.Base {
		t.Fatal(result, err)
	}
	if _, err = os.Stat(filepath.Join(paths.CurrentDir, "link")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale checkout link", err)
	}
}
func TestCheckoutValidationAndHostErrors(t *testing.T) {
	paths, f := fixtureGit(t)
	for _, entry := range []GitTreeEntry{{Path: "../escape", Mode: filemode.Regular}, {Path: "", Mode: filemode.Regular}, {Path: ".git/config", Mode: filemode.Regular}, {Path: "x", Mode: filemode.Dir}} {
		if err := validateCheckoutEntries([]GitTreeEntry{entry}); err == nil {
			t.Fatal(entry)
		}
	}
	for _, entries := range [][]GitTreeEntry{{{Path: "a", Mode: filemode.Regular}, {Path: "a", Mode: filemode.Regular}}, {{Path: "a", Mode: filemode.Symlink}, {Path: "a/b", Mode: filemode.Regular}}} {
		if err := validateCheckoutEntries(entries); err == nil {
			t.Fatal(entries)
		}
	}
	for _, target := range []string{"", "/", paths.BareDir, filepath.Dir(paths.BareDir), paths.BareDir + "/inside"} {
		copy := paths
		copy.CurrentDir = target
		if _, err := MaterializeCurrentCheckout(context.Background(), copy, f.Next); err == nil {
			t.Fatal(target)
		}
	}
	_ = os.WriteFile(paths.CurrentDir, nil, 0600)
	if _, err := MaterializeCurrentCheckout(context.Background(), paths, f.Next); err == nil {
		t.Fatal("file destination")
	}
	_ = os.Remove(paths.CurrentDir)
	_ = os.Symlink(paths.BareDir, paths.CurrentDir)
	if _, err := MaterializeCurrentCheckout(context.Background(), paths, f.Next); err == nil {
		t.Fatal("link destination")
	}
	_ = os.Remove(paths.CurrentDir)
	link := filepath.Join(filepath.Dir(paths.BareDir), "alias")
	_ = os.Symlink(filepath.Dir(paths.BareDir), link)
	copy := paths
	copy.CurrentDir = link + "/sub/current"
	if _, err := MaterializeCurrentCheckout(context.Background(), copy, f.Next); err == nil {
		t.Fatal("ancestor alias")
	}
	if _, err := MaterializeCurrentCheckout(context.Background(), GitRepositoryPaths{}, f.Next); err == nil {
		t.Fatal("missing repo")
	}
	if _, err := MaterializeCurrentCheckout(context.Background(), paths, "bad"); err == nil {
		t.Fatal("bad revision")
	}
	for _, kind := range []string{"abs-current", "abs-bare", "lstat", "bare-ancestor", "remove", "mkdir", "open"} {
		ops := defaultCheckoutIO()
		switch kind {
		case "abs-current", "abs-bare":
			n := 0
			abs := ops.abs
			ops.abs = func(p string) (string, error) {
				n++
				if n == 1 && kind == "abs-current" || n == 2 && kind == "abs-bare" {
					return "", os.ErrPermission
				}
				return abs(p)
			}
		case "lstat":
			ops.lstat = func(p string) (fs.FileInfo, error) {
				if p == paths.CurrentDir {
					return nil, os.ErrPermission
				}
				return os.Lstat(p)
			}
		case "bare-ancestor":
			n := 0
			ops.lstat = func(p string) (fs.FileInfo, error) {
				if p == filepath.Dir(paths.BareDir) {
					n++
					if n == 2 {
						return nil, os.ErrPermission
					}
				}
				return os.Lstat(p)
			}
		case "remove":
			ops.removeAll = func(string) error { return os.ErrPermission }
		case "mkdir":
			ops.mkdirAll = func(string, fs.FileMode) error { return os.ErrPermission }
		case "open":
			ops.openRoot = func(string) (*os.Root, error) { return nil, os.ErrPermission }
		}
		if _, err := materializeCheckout(context.Background(), paths, f.Next, ops); err == nil {
			t.Fatal(kind)
		}
	}
	_ = os.Remove(filepath.Join(paths.BareDir, "packed-refs"))
	if _, err := MaterializeCurrentCheckout(context.Background(), paths, ""); err == nil {
		t.Fatal("missing main")
	}
}

type fakeCheckout struct {
	mkdirErr, symlinkErr, createErr error
	file                            checkoutFile
}

func (f fakeCheckout) MkdirAll(string, fs.FileMode) error               { return f.mkdirErr }
func (f fakeCheckout) Symlink(string, string) error                     { return f.symlinkErr }
func (f fakeCheckout) create(string, fs.FileMode) (checkoutFile, error) { return f.file, f.createErr }

type checkoutBuffer struct{ writeErr, closeErr error }

func (f checkoutBuffer) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}
func (f checkoutBuffer) Close() error { return f.closeErr }

type badCheckoutReader struct {
	readErr, closeErr error
	io.Reader
}

func (r badCheckoutReader) Read(p []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	return r.Reader.Read(p)
}
func (r badCheckoutReader) Close() error { return r.closeErr }
func TestMaterializeEntryFailures(t *testing.T) {
	for _, kind := range []string{"cancel", "submodule-mkdir", "mkdir", "blob", "link-read", "link-close", "symlink", "create", "copy", "read-close", "write-close", "submodule"} {
		mode := filemode.Regular
		dest := fakeCheckout{file: checkoutBuffer{}}
		ctx := context.Background()
		reader := badCheckoutReader{Reader: strings.NewReader("text")}
		var blobErr error
		switch kind {
		case "cancel":
			c, cancel := context.WithCancel(ctx)
			cancel()
			ctx = c
		case "submodule-mkdir":
			mode = filemode.Submodule
			dest.mkdirErr = io.ErrClosedPipe
		case "submodule":
			mode = filemode.Submodule
		case "mkdir":
			dest.mkdirErr = io.ErrClosedPipe
		case "blob":
			blobErr = io.ErrClosedPipe
		case "link-read":
			mode = filemode.Symlink
			reader.readErr = io.ErrClosedPipe
		case "link-close":
			mode = filemode.Symlink
			reader.closeErr = io.ErrClosedPipe
		case "symlink":
			mode = filemode.Symlink
			dest.symlinkErr = io.ErrClosedPipe
		case "create":
			dest.createErr = io.ErrClosedPipe
		case "copy":
			dest.file = checkoutBuffer{writeErr: io.ErrClosedPipe}
		case "read-close":
			reader.closeErr = io.ErrClosedPipe
		case "write-close":
			dest.file = checkoutBuffer{closeErr: io.ErrClosedPipe}
		}
		err := materializeEntries(ctx, dest, []GitTreeEntry{{Path: "file", Mode: mode}}, func(plumbing.Hash) (io.ReadCloser, error) { return reader, blobErr })
		if (err != nil) != (kind != "submodule") {
			t.Fatal(kind, err)
		}
	}
}
func TestMaterializeUntrustedTrees(t *testing.T) {
	for _, kind := range []string{"metadata", "missing-blob", "link-escape"} {
		paths, f := fixtureGit(t)
		store, err := OpenGitStore(paths.BareDir)
		if err != nil {
			t.Fatal(err)
		}
		name := "missing"
		if kind == "metadata" {
			name = ".git"
		}
		tree := encodeObject(t, store.storage, &object.Tree{Entries: []object.TreeEntry{{Name: name, Mode: filemode.Regular, Hash: plumbing.NewHash(strings.Repeat("1", 40))}}})
		commit := encodeObject(t, store.storage, &object.Commit{TreeHash: tree})
		if kind == "link-escape" {
			if _, err := MaterializeCurrentCheckout(context.Background(), paths, f.Next); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(paths.CurrentDir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if err = root.Symlink(t.TempDir(), "outside"); err != nil {
				t.Fatal(err)
			}
			err = materializeEntries(context.Background(), checkoutRoot{root}, []GitTreeEntry{{Path: "outside/escape", Mode: filemode.Regular}}, func(plumbing.Hash) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("x")), nil })
			if err == nil {
				t.Fatal("escaped rooted checkout")
			}
			continue
		}
		if _, err = MaterializeCurrentCheckout(context.Background(), paths, commit.String()); err == nil {
			t.Fatal(kind)
		}
	}
}

func TestMaterializeEmptyTree(t *testing.T) {
	paths, _ := fixtureGit(t)
	store, err := OpenGitStore(paths.BareDir)
	if err != nil {
		t.Fatal(err)
	}
	tree := encodeObject(t, store.storage, &object.Tree{})
	commit := encodeObject(t, store.storage, &object.Commit{TreeHash: tree})
	result, err := MaterializeCurrentCheckout(context.Background(), paths, commit.String())
	if err != nil || result.Revision != commit.String() {
		t.Fatal(result, err)
	}
	entries, err := os.ReadDir(paths.CurrentDir)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}
