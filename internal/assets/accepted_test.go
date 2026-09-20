package assets

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/pyjson"
)

func TestAcceptedAssetReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/accepted-assets.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Concept  string
		Manifest Manifest
		ZIP      []byte `json:"zip"`
		Writes   []struct {
			Version  string
			Created  *string
			Paths    []string
			Metadata string
			ZIP      []byte `json:"zip"`
		}
		Versions, Kinds []string
		Resolved        []struct {
			Version         *string
			Expected, Error string
		}
		Paths []struct {
			Concept, Kind, Version, Error string
			Expected                      []string
		}
		Retention []struct {
			Versions []string
			Keep     int
			Expected [][]string
			Error    string
		}
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, c := range f.Writes {
		v := AcceptedVersion{ConceptID: f.Concept, ConceptPath: "/public/café.md", AssetKind: "docs", Version: c.Version, ZIPBytes: f.ZIP, Manifest: f.Manifest, AcceptedBy: "synthetic-agent", SourceProposalID: "proposal"}
		if c.Created != nil {
			stamp, err := time.Parse(time.RFC3339Nano, *c.Created)
			if err != nil {
				t.Fatal(err)
			}
			v.CreatedAt = &stamp
		}
		paths, err := WriteAssetVersion(root, v)
		if err != nil || !reflect.DeepEqual(paths, c.Paths) {
			t.Fatal(paths, c.Paths, err)
		}
		metadata, err := os.ReadFile(filepath.Join(root, paths[0][1:]))
		if err != nil || string(metadata) != c.Metadata {
			t.Fatalf("%s != %s err %v", metadata, c.Metadata, err)
		}
		blob, err := os.ReadFile(filepath.Join(root, paths[1][1:]))
		if err != nil || !reflect.DeepEqual(blob, c.ZIP) {
			t.Fatal(err)
		}
		loaded, err := LoadAssetMetadata(root, f.Concept, "docs", c.Version)
		expected, _ := pyjson.Parse(c.Metadata)
		if err != nil || !reflect.DeepEqual(loaded, expected) {
			t.Fatal(loaded, expected, err)
		}
		if _, err = WriteAssetVersion(root, v); err == nil {
			t.Fatal("version overwrite")
		}
	}
	versions, err := ListAssetVersions(root, f.Concept, "docs")
	if err != nil || !reflect.DeepEqual(versions, f.Versions) {
		t.Fatal(versions, err)
	}
	kinds, err := ListAssetKinds(root, f.Concept)
	if err != nil || !reflect.DeepEqual(kinds, f.Kinds) {
		t.Fatal(kinds, err)
	}
	for _, c := range f.Resolved {
		got, err := ResolveAssetVersion(root, f.Concept, "docs", c.Version)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(err, c.Error)
			}
		} else if err != nil || got != c.Expected {
			t.Fatal(got, err)
		}
	}
	for _, c := range f.Paths {
		a, b, err := AssetVersionPaths(c.Concept, c.Kind, c.Version)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(err, c.Error)
			}
		} else if err != nil || !reflect.DeepEqual([]string{a, b}, c.Expected) {
			t.Fatal(a, b, err)
		}
	}
	for _, c := range f.Retention {
		kept, dropped, err := RetentionPartition(c.Versions, c.Keep)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(err, c.Error)
			}
		} else if err != nil || !reflect.DeepEqual([][]string{kept, dropped}, c.Expected) {
			t.Fatal(kept, dropped, c.Expected, err)
		}
	}
}
func acceptedVersion(t *testing.T) AcceptedVersion {
	t.Helper()
	raw := syntheticZIP(t)
	pack, err := ValidateAssetPack("docs", "1.0.0", raw, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return AcceptedVersion{ConceptID: "12345678", ConceptPath: "/a.md", AssetKind: "docs", Version: "1.0.0", ZIPBytes: raw, Manifest: pack.Manifest}
}
func TestAcceptedFilesystemSafety(t *testing.T) {
	v := acceptedVersion(t)
	for _, kind := range []string{"parent-link", "parent-file", "kind-link", "metadata-link", "metadata-dir", "metadata-big", "metadata-invalid", "root-link", "bad-version-file", "bad-kind", "kind-file"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			_, err := WriteAssetVersion(root, v)
			if err != nil {
				t.Fatal(err)
			}
			meta, _, _ := AssetVersionPaths(v.ConceptID, v.AssetKind, v.Version)
			meta = filepath.Join(root, meta[1:])
			dir := filepath.Dir(meta)
			switch kind {
			case "parent-link":
				_ = os.RemoveAll(filepath.Join(root, ".assets"))
				_ = os.Symlink(t.TempDir(), filepath.Join(root, ".assets"))
			case "parent-file":
				_ = os.RemoveAll(filepath.Join(root, ".assets"))
				_ = os.WriteFile(filepath.Join(root, ".assets"), nil, 0600)
			case "kind-link":
				_ = os.RemoveAll(dir)
				_ = os.Symlink(t.TempDir(), dir)
			case "metadata-link":
				_ = os.Remove(meta)
				_ = os.Symlink(filepath.Join(t.TempDir(), "outside"), meta)
			case "metadata-dir":
				_ = os.Remove(meta)
				_ = os.Mkdir(meta, 0700)
			case "metadata-big":
				_ = os.WriteFile(meta, make([]byte, MaxMetadataBytes+1), 0600)
			case "metadata-invalid":
				_ = os.WriteFile(meta, []byte(`{"kind":"bad"}`), 0600)
			case "root-link":
				link := filepath.Join(t.TempDir(), "root")
				_ = os.Symlink(root, link)
				root = link
			case "bad-version-file":
				_ = os.WriteFile(filepath.Join(dir, "bad.json"), nil, 0600)
			case "bad-kind":
				_ = os.Mkdir(filepath.Join(filepath.Dir(dir), "BAD"), 0700)
			case "kind-file":
				_ = os.WriteFile(filepath.Join(filepath.Dir(dir), "not-kind"), nil, 0600)
			}
			if kind == "bad-version-file" {
				if _, err = ListAssetVersions(root, v.ConceptID, v.AssetKind); err == nil {
					t.Fatal(kind)
				}
				return
			}
			if kind == "kind-file" {
				if got, err := ListAssetKinds(root, v.ConceptID); err != nil || len(got) != 1 {
					t.Fatal(got, err)
				}
				return
			}
			if kind == "kind-link" || kind == "bad-kind" {
				if _, err = ListAssetKinds(root, v.ConceptID); err == nil {
					t.Fatal(kind)
				}
				return
			}
			if _, err = LoadAssetMetadata(root, v.ConceptID, v.AssetKind, v.Version); err == nil {
				t.Fatal(kind)
			}
		})
	}
	root := t.TempDir()
	if got, err := ListAssetKinds(root, "bad"); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if got, err := ListAssetKinds(root, v.ConceptID); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if got, err := ListAssetVersions(root, v.ConceptID, v.AssetKind); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if _, err := ResolveAssetVersion(root, v.ConceptID, v.AssetKind, nil); err == nil {
		t.Fatal("missing kind")
	}
	if _, err := ListAssetVersions(root, v.ConceptID, "BAD"); err == nil {
		t.Fatal("bad kind")
	}
	if _, err := ResolveAssetVersion(root, "bad", "docs", nil); err == nil {
		t.Fatal("bad id")
	}
	if _, err := LoadAssetMetadata(root, v.ConceptID, v.AssetKind, v.Version); err == nil {
		t.Fatal("missing metadata")
	}
}

func TestAcceptedWriteErrors(t *testing.T) {
	for _, kind := range []string{"path", "stat", "directory", "root-stat", "root-file", "root-open", "mkdir", "metadata", "zip", "json"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			v := acceptedVersion(t)
			ops := defaultAcceptedIO()
			switch kind {
			case "path":
				v.ConceptID = "bad"
			case "stat":
				ops.lstat = func(string) (fs.FileInfo, error) { return nil, os.ErrPermission }
			case "directory":
				orig := ops.lstat
				ops.lstat = func(p string) (fs.FileInfo, error) {
					if strings.HasSuffix(p, ".assets") {
						return nil, os.ErrPermission
					}
					return orig(p)
				}
			case "root-stat":
				orig := ops.lstat
				ops.lstat = func(p string) (fs.FileInfo, error) {
					if p == root {
						return nil, os.ErrPermission
					}
					return orig(p)
				}
			case "root-file":
				root = filepath.Join(root, "file")
				_ = os.WriteFile(root, nil, 0600)
				orig := ops.lstat
				ops.lstat = func(p string) (fs.FileInfo, error) {
					if p != root {
						return nil, os.ErrNotExist
					}
					return orig(p)
				}
			case "root-open":
				ops.openRoot = func(string) (*os.Root, error) { return nil, os.ErrPermission }
			case "mkdir":
				ops.mkdir = func(*os.Root, string) error { return os.ErrPermission }
			case "metadata", "zip":
				orig := ops.write
				ops.write = func(r *os.Root, p string, b []byte) error {
					if kind == "metadata" || strings.HasSuffix(p, ".zip") {
						return io.ErrClosedPipe
					}
					return orig(r, p, b)
				}
			case "json":
				v.AcceptedBy = "\xff"
			}
			if _, err := writeAssetVersion(root, v, ops); err == nil {
				t.Fatal(kind)
			}
		})
	}
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root.Close()
	if err = writeAcceptedFile(root, "x", nil); err == nil {
		t.Fatal("closed root")
	}
	for _, writer := range []acceptedWriter{badAcceptedWriter{writeErr: io.ErrClosedPipe}, badAcceptedWriter{short: true}, badAcceptedWriter{closeErr: io.ErrClosedPipe}} {
		if err := writeAcceptedBytes(writer, []byte("value")); err == nil {
			t.Fatal("write failure")
		}
	}
}

type badAcceptedWriter struct {
	writeErr, closeErr error
	short              bool
}

func (w badAcceptedWriter) Write(p []byte) (int, error) {
	if w.short {
		return 1, nil
	}
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return len(p), nil
}
func (w badAcceptedWriter) Close() error { return w.closeErr }
func TestAcceptedReadErrors(t *testing.T) {
	v := acceptedVersion(t)
	root := t.TempDir()
	_, err := WriteAssetVersion(root, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"stat", "open", "file-open"} {
		ops := defaultAcceptedIO()
		switch kind {
		case "stat":
			orig := ops.lstat
			ops.lstat = func(p string) (fs.FileInfo, error) {
				if strings.HasSuffix(p, ".json") {
					return nil, os.ErrPermission
				}
				return orig(p)
			}
		case "open":
			ops.openRoot = func(string) (*os.Root, error) { return nil, os.ErrPermission }
		case "file-open":
			ops.openRoot = func(path string) (*os.Root, error) {
				r, err := os.OpenRoot(path)
				if err == nil {
					r.Close()
				}
				return r, err
			}
		}
		if _, err = loadAssetMetadata(root, v.ConceptID, v.AssetKind, v.Version, ops); err == nil {
			t.Fatal(kind)
		}
	}
	if _, err = LoadAssetMetadata(root, "bad", v.AssetKind, v.Version); err == nil {
		t.Fatal("bad id")
	}
	for _, raw := range []string{"[]", "{}", "{"} {
		if _, err := readAssetMetadata(strings.NewReader(raw)); err == nil {
			t.Fatal(raw)
		}
	}
	if _, err := readAssetMetadata(failedReader{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	for _, which := range []string{"kinds", "versions"} {
		ops := defaultAcceptedIO()
		ops.readDir = func(string) ([]os.DirEntry, error) { return nil, os.ErrPermission }
		if which == "kinds" {
			_, err = listAssetKinds(root, v.ConceptID, ops)
		} else {
			_, err = listAssetVersions(root, v.ConceptID, v.AssetKind, ops)
		}
		if err == nil {
			t.Fatal(which)
		}
	}
	if _, err := ListAssetKinds(filepath.Join(root, "file"), v.ConceptID); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptedDirectoryFailureAndVersionTies(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".assets"), nil, 0600)
	if _, err := ListAssetKinds(root, "12345678"); err == nil {
		t.Fatal("invalid parent")
	}
	// Unicode decimal digits are accepted by the source regex, and can yield
	// equal numeric versions. Input-order ties remain stable for version lists.
	got, err := sortedVersions([]string{"1.0.0", "1.0.0", "1.1٢.0", "1.12.0"}, false)
	if err != nil || !reflect.DeepEqual(got, []string{"1.0.0", "1.0.0", "1.1٢.0", "1.12.0"}) {
		t.Fatal(got, err)
	}
}
