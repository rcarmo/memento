package repository

import (
	"errors"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/cases"
)

type BundleError struct {
	Message string
	Cause   error
}

func (e *BundleError) Error() string { return e.Message }
func (e *BundleError) Unwrap() error { return e.Cause }

type BundleEntry struct {
	BundlePath string          `json:"bundle_path"`
	Document   ConceptDocument `json:"document"`
}
type RepositoryBundle struct {
	Root    string
	Entries []BundleEntry
}
type AuditIssue struct {
	BundlePath string `json:"bundle_path"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}
type RepositoryAudit struct {
	Issues           []AuditIssue      `json:"issues"`
	GeneratedIndexes map[string]string `json:"generated_indexes"`
	GeneratedLog     string            `json:"generated_log"`
}

func (a RepositoryAudit) OK() bool { return len(a.Issues) == 0 }
func (b RepositoryBundle) Get(bundlePath string) (BundleEntry, error) {
	for _, entry := range b.Entries {
		if entry.BundlePath == bundlePath {
			return entry, nil
		}
	}
	return BundleEntry{}, &BundleError{Message: "unknown bundle path: " + bundlePath}
}

type BundleFilter struct {
	IncludePath      func(string) bool
	IncludeDirectory func(string) bool
}

// ListBundlePaths preserves the reference's distinction between rglob and the
// directory-pruned os.walk path. Symlink directories are never descended;
// matching file/dir targets are validated only after filtering and sorting.
// Directory read errors are suppressed by both Python iterators. This is not a
// race-safe open API: each later operation must enforce containment separately.
func ListBundlePaths(root string, filter BundleFilter) ([]string, error) {
	return listBundlePaths(root, filter, os.ReadDir)
}
func listBundlePaths(root string, filter BundleFilter, readDir func(string) ([]os.DirEntry, error)) ([]string, error) {
	paths := []string{}
	var walk func(string, string)
	walk = func(directory, prefix string) {
		entries, err := readDir(directory)
		if err != nil {
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			path := prefix + "/" + entry.Name()
			isDirectory := entry.IsDir()
			if entry.Type()&os.ModeSymlink != 0 {
				// os.walk puts symlinks to directories in directory_names,
				// even though neither source iterator follows those links.
				if info, err := os.Stat(joinPath(directory, entry.Name())); err == nil {
					isDirectory = info.IsDir()
				}
			}
			if strings.HasSuffix(entry.Name(), ".md") && (filter.IncludeDirectory == nil || !isDirectory) {
				paths = append(paths, path)
			}
			if !entry.IsDir() {
				continue
			}
			if filter.IncludeDirectory != nil && (IsReservedBundlePath(path+"/") || !filter.IncludeDirectory(path+"/")) {
				continue
			}
			walk(joinPath(directory, entry.Name()), path)
		}
	}
	walk(pythonPath(root), "")
	sort.Strings(paths)
	out := []string{}
	for _, path := range paths {
		if IsReservedBundlePath(path) || (filter.IncludePath != nil && !filter.IncludePath(path)) {
			continue
		}
		if _, err := ValidateRepositoryReadPath(root, path); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, nil
}
func ScanBundle(root string, filter BundleFilter) (RepositoryBundle, error) {
	return scanBundle(root, filter, ParseConceptFile)
}
func scanBundle(root string, filter BundleFilter, parseFile func(string) (ConceptDocument, error)) (RepositoryBundle, error) {
	paths, err := ListBundlePaths(root, filter)
	if err != nil {
		return RepositoryBundle{}, err
	}
	bundle := RepositoryBundle{Root: root, Entries: []BundleEntry{}}
	for _, path := range paths {
		document, err := parseFile(joinPath(root, strings.TrimPrefix(path, "/")))
		if err != nil {
			var frontmatter *FrontmatterError
			if errors.As(err, &frontmatter) {
				return RepositoryBundle{}, &BundleError{Message: "invalid concept at " + path, Cause: err}
			}
			return RepositoryBundle{}, err
		}
		bundle.Entries = append(bundle.Entries, BundleEntry{BundlePath: path, Document: document})
	}
	return bundle, nil
}
func ReadBundleEntry(root, bundlePath string) (BundleEntry, error) {
	safe, err := ValidateRepositoryReadPath(root, bundlePath)
	if err != nil {
		return BundleEntry{}, err
	}
	document, err := ParseConceptFile(safe.AbsolutePath)
	if err != nil {
		return BundleEntry{}, err
	}
	return BundleEntry{BundlePath: bundlePath, Document: document}, nil
}
func escapeMarkdown(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	for _, r := range "[]()*_`#>|" {
		value = strings.ReplaceAll(value, string(r), `\`+string(r))
	}
	return value
}
func GenerateDirectoryIndexes(bundle RepositoryBundle) map[string]string {
	directories := map[string]bool{"/": true}
	for _, entry := range bundle.Entries {
		parts := strings.Split(strings.TrimPrefix(entry.BundlePath, "/"), "/")
		for i := 1; i < len(parts); i++ {
			directories["/"+strings.Join(parts[:i], "/")+"/"] = true
		}
	}
	out := map[string]string{}
	for directory := range directories {
		out[directory] = renderDirectoryIndex(bundle, directory)
	}
	return out
}
func renderDirectoryIndex(bundle RepositoryBundle, directory string) string {
	children := map[string]bool{}
	prefix := strings.TrimPrefix(directory, "/")
	for _, entry := range bundle.Entries {
		path := strings.TrimPrefix(entry.BundlePath, "/")
		remainder := path
		if prefix != "" {
			withSlash := strings.TrimRight(prefix, "/") + "/"
			if !strings.HasPrefix(path, withSlash) {
				continue
			}
			remainder = strings.TrimPrefix(path, withSlash)
		}
		first, _, nested := strings.Cut(remainder, "/")
		if nested {
			child := strings.TrimPrefix(strings.TrimRight(prefix, "/")+"/"+first, "/")
			children["["+first+"](/"+child+"/index.md)"] = true
		} else {
			// Python's bundle.get chooses the first duplicate path, not the current
			// entry. Scans normally produce unique paths, but in-memory bundles may not.
			selected, _ := bundle.Get("/" + path)
			children["["+escapeMarkdown(selected.Document.Frontmatter.Title)+"](/"+path+")"] = true
		}
	}
	values := make([]string, 0, len(children))
	for child := range children {
		values = append(values, child)
	}
	fold := cases.Fold()
	sort.Slice(values, func(i, j int) bool {
		a, b := fold.String(values[i]), fold.String(values[j])
		if a == b {
			return values[i] < values[j]
		}
		return a < b
	})
	if len(values) == 0 {
		return "# Index\n\n_Empty directory._\n"
	}
	return "# Index\n\n- " + strings.Join(values, "\n- ") + "\n"
}
func pythonTimestamp(t time.Time) string {
	text := t.Format("2006-01-02T15:04:05")
	if t.Nanosecond() != 0 {
		text += t.Format(".000000")
	}
	_, offset := t.Zone()
	if offset == 0 {
		return text + "Z"
	}
	return text + t.Format("-07:00")
}
func GenerateRootLog(bundle RepositoryBundle) string {
	entries := append([]BundleEntry{}, bundle.Entries...)
	fold := cases.Fold()
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.Document.Frontmatter.UpdatedAt.Equal(b.Document.Frontmatter.UpdatedAt) {
			return a.Document.Frontmatter.UpdatedAt.After(b.Document.Frontmatter.UpdatedAt)
		}
		x, y := fold.String(a.Document.Frontmatter.Title), fold.String(b.Document.Frontmatter.Title)
		if x != y {
			return x > y
		}
		return a.BundlePath > b.BundlePath
	})
	lines := []string{"# Mutation Log", "", "This file is generated deterministically from concept metadata.", ""}
	for _, entry := range entries {
		m := entry.Document.Frontmatter
		lines = append(lines, "- "+pythonTimestamp(m.UpdatedAt)+" — ["+escapeMarkdown(m.Title)+"]("+entry.BundlePath+") — "+escapeMarkdown(m.UpdatedBy))
	}
	return strings.Join(lines, "\n") + "\n"
}
func AuditRepository(root string, includePath func(string) bool) (RepositoryAudit, error) {
	bundle, err := ScanBundle(root, BundleFilter{IncludePath: includePath})
	if err != nil {
		return RepositoryAudit{}, err
	}
	return auditBundle(bundle, includePath)
}
func auditBundle(bundle RepositoryBundle, includePath func(string) bool) (RepositoryAudit, error) {
	issues := []AuditIssue{}
	seen := map[string]string{}
	paths := map[string]bool{}
	for _, entry := range bundle.Entries {
		paths[entry.BundlePath] = true
	}
	for _, entry := range bundle.Entries {
		id := entry.Document.Frontmatter.ID
		if previous, ok := seen[id]; ok {
			issues = append(issues, AuditIssue{BundlePath: entry.BundlePath, Code: "duplicate_id", Message: "concept id " + id + " already used by " + previous})
		} else {
			seen[id] = entry.BundlePath
		}
		for _, link := range ExtractStructuralLinks(entry.Document.Body) {
			if !strings.HasPrefix(link.Href, "/") {
				continue
			}
			target, _, _ := strings.Cut(link.Href, "#")
			if IsReservedBundlePath(target) || (includePath != nil && !includePath(target)) {
				continue
			}
			if !paths[target] {
				issues = append(issues, AuditIssue{BundlePath: entry.BundlePath, Code: "broken_link", Message: "broken link to " + target})
			}
		}
		// Serializer guarantees a final newline by construction. The Python audit's
		// unreachable missing-newline branch needs no fabricated Go error path.
		if _, err := SerializeConcept(entry.Document); err != nil {
			return RepositoryAudit{}, err
		}
	}
	return RepositoryAudit{Issues: issues, GeneratedIndexes: GenerateDirectoryIndexes(bundle), GeneratedLog: GenerateRootLog(bundle)}, nil
}
