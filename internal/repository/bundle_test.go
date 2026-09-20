package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPythonBundleFixtures(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/repository-bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Generated []struct {
			Entries []struct {
				Path, Body string
				Metadata   map[string]any
			}
			Indexes map[string]string
			Log     string
		}
		Files map[string]string
		Scans []struct {
			Tree        string
			Filtered    bool
			Paths, Scan []string
			Error       string
			AuditError  string `json:"audit_error"`
			Audit       RepositoryAudit
			OK          bool
		}
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixtures.Generated {
		bundle := RepositoryBundle{Root: "/synthetic", Entries: []BundleEntry{}}
		for _, entry := range c.Entries {
			m, err := ValidateConceptMetadata(entry.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			bundle.Entries = append(bundle.Entries, BundleEntry{BundlePath: entry.Path, Document: ConceptDocument{Frontmatter: m, Body: entry.Body}})
		}
		if got := GenerateDirectoryIndexes(bundle); !reflect.DeepEqual(got, c.Indexes) {
			t.Fatal(got, c.Indexes)
		}
		if got := GenerateRootLog(bundle); got != c.Log {
			t.Fatalf("log %q != %q", got, c.Log)
		}
	}
	for _, c := range fixtures.Scans {
		t.Run(c.Tree+fmt.Sprint(c.Filtered), func(t *testing.T) {
			root := t.TempDir()
			for path, content := range fixtures.Files {
				target := filepath.Join(root, path[1:])
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for path, content := range map[string]string{"index.md": "reserved", "log.md": "reserved", ".assets/bad.md": "reserved"} {
				target := filepath.Join(root, path)
				_ = os.MkdirAll(filepath.Dir(target), 0700)
				if err := os.WriteFile(target, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch c.Tree {
			case "symlink-file":
				err = os.Symlink(filepath.Join(root, "a.md"), filepath.Join(root, "linked.md"))
			case "symlink-dir":
				err = os.Symlink(filepath.Join(root, "nested"), filepath.Join(root, "linked"))
			case "directory-md":
				err = os.Mkdir(filepath.Join(root, "folder.md"), 0700)
			case "invalid-concept":
				err = os.WriteFile(filepath.Join(root, "bad.md"), []byte("invalid"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			filter := BundleFilter{}
			if c.Filtered {
				filter.IncludePath = func(path string) bool {
					return path != "/linked.md" && path != "/bad.md" && path != "/folder.md" && path != "/hidden.md"
				}
				filter.IncludeDirectory = func(path string) bool { return path != "/nested/deep/" }
			}
			paths, pathErr := ListBundlePaths(root, filter)
			if c.Paths != nil && !reflect.DeepEqual(paths, c.Paths) {
				t.Fatal(paths, c.Paths, pathErr)
			}
			bundle, scanErr := ScanBundle(root, filter)
			if c.Error != "" {
				if errorText(scanErr) != c.Error {
					t.Fatal(scanErr, c.Error)
				}
			} else {
				if pathErr != nil || scanErr != nil {
					t.Fatal(pathErr, scanErr)
				}
				got := []string{}
				for _, entry := range bundle.Entries {
					got = append(got, entry.BundlePath)
				}
				if !reflect.DeepEqual(got, c.Scan) {
					t.Fatal(got, c.Scan)
				}
			}
			audit, err := AuditRepository(root, filter.IncludePath)
			if c.AuditError != "" {
				if errorText(err) != c.AuditError {
					t.Fatal(err, c.AuditError)
				}
			} else if err != nil || !reflect.DeepEqual(audit, c.Audit) || audit.OK() != c.OK {
				t.Fatal(err, audit, c.Audit)
			}
			entry, err := ReadBundleEntry(root, "/a.md")
			if err != nil || entry.BundlePath != "/a.md" {
				t.Fatal(entry, err)
			}
		})
	}
}

func TestBundleFailuresAndHelpers(t *testing.T) {
	root := t.TempDir()
	if _, err := ReadBundleEntry(root, "/missing.md"); err == nil {
		t.Fatal("missing entry")
	}
	path := filepath.Join(root, "bad.md")
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBundleEntry(root, "/bad.md"); err == nil {
		t.Fatal("bad entry")
	}
	if _, err := AuditRepository(root, nil); err == nil {
		t.Fatal("bad audit")
	}
	_, err := ScanBundle(root, BundleFilter{})
	var bundleError *BundleError
	if !errors.As(err, &bundleError) || bundleError.Unwrap() == nil {
		t.Fatal(err)
	}
	if _, err = (RepositoryBundle{}).Get("/missing"); errorText(err) != "unknown bundle path: /missing" {
		t.Fatal(err)
	}
	paths, err := listBundlePaths(root, BundleFilter{}, func(string) ([]os.DirEntry, error) { return nil, fs.ErrPermission })
	if err != nil || len(paths) != 0 {
		t.Fatal(paths, err)
	}
	if _, err = auditBundle(RepositoryBundle{Entries: []BundleEntry{{BundlePath: "/bad", Document: ConceptDocument{}}}}, nil); err == nil {
		t.Fatal("invalid serialization")
	}
	if got := pythonTimestamp(time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("offset", 3600))); got != "2026-01-01T00:00:00+01:00" {
		t.Fatal(got)
	}
	if !(RepositoryAudit{}).OK() {
		t.Fatal("empty audit")
	}
}

func TestBundleFilesystemRacesAndCasefoldTie(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "x.md"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := scanBundle(root, BundleFilter{}, func(string) (ConceptDocument, error) { return ConceptDocument{}, fs.ErrPermission }); !errors.Is(err, fs.ErrPermission) {
		t.Fatal(err)
	}
	bundle := RepositoryBundle{Entries: []BundleEntry{{BundlePath: "/A.md", Document: ConceptDocument{Frontmatter: ConceptFrontmatter{Title: "Same"}}}, {BundlePath: "/a.md", Document: ConceptDocument{Frontmatter: ConceptFrontmatter{Title: "same"}}}}}
	// Source set iteration ties vary with PYTHONHASHSEED. The Go port chooses
	// lexical order for ties; this unresolved byte-parity gap is documented.
	got := GenerateDirectoryIndexes(bundle)["/"]
	if got != "# Index\n\n- [Same](/A.md)\n- [same](/a.md)\n" {
		t.Fatal(got)
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "dir"), filepath.Join(root, "link.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	paths, err := ListBundlePaths(root, BundleFilter{IncludeDirectory: func(string) bool { return true }})
	if err != nil || !reflect.DeepEqual(paths, []string{"/x.md"}) {
		t.Fatal(paths, err)
	}
}
