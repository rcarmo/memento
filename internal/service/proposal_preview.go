package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/rcarmo/memento/internal/repository"
)

// PreviewChanges renders the source preview, not a dry-run validation of writes.
// Actor/time remain unchanged in patch previews; create IDs are placeholders.
func (m WorktreeMutator) PreviewChanges(root string, changes []ProposalChange) (string, error) {
	return m.previewChanges(root, changes, defaultMutationIO())
}
func (m WorktreeMutator) previewChanges(root string, changes []ProposalChange, ops mutationIO) (string, error) {
	var out strings.Builder
	for _, change := range changes {
		path := change["path"].(string)
		switch change["kind"] {
		case "create":
			stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			meta, err := repository.ValidateConceptMetadata(map[string]any{"schema_version": 1, "id": "<generated>", "type": change["concept_type"], "title": change["title"], "status": "active", "description": change["description"], "tags": change["tags"], "aliases": change["aliases"], "source_refs": []string{}, "supersedes": []string{}, "created_at": stamp, "updated_at": stamp, "updated_by": "<proposal>"})
			if err != nil {
				return "", err
			}
			text, err := m.bounded(repository.ConceptDocument{Frontmatter: meta, Body: change["body"].(string)}, false)
			if err != nil {
				return "", err
			}
			out.WriteString(proposalDiff("", text, path))
		case "patch":
			entry, err := ops.read(root, path)
			if err != nil {
				return "", err
			}
			original, err := repository.SerializeCopiedConcept(entry.Document)
			if err != nil {
				return "", err
			}
			updated := patchDocument(entry.Document, change, entry.Document.Frontmatter.UpdatedBy, entry.Document.Frontmatter.UpdatedAt)
			text, err := m.bounded(updated, true)
			if err != nil {
				return "", err
			}
			out.WriteString(proposalDiff(original, text, path))
		case "trash":
			out.WriteString(fmt.Sprintf("archive %s -> /trash%s; assets and Git history retained; inbound links left unchanged\n", path, path))
		case "rename":
			out.WriteString(fmt.Sprintf("rename %s -> %s\n", path, change["new_path"]))
		case "attach_asset_pack":
			out.WriteString(fmt.Sprintf("attach %s asset %s (%s) to %s\n", change["asset_kind"], change["version"], change["zip_sha256"], path))
		}
	}
	return out.String(), nil
}
func proposalDiff(before, after, path string) string {
	// bytes.Buffer cannot fail. Splitlines must recognise all Python boundaries,
	// not just LF (including CRLF, Unicode separators and control separators).
	text, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: pythonDiffLines(before), B: pythonDiffLines(after), FromFile: "a" + path, ToFile: "b" + path, Context: 3})
	return text
}
func pythonDiffLines(text string) []string {
	lines := []string{}
	start := 0
	skipLF := false
	for i, r := range text {
		if r == '\n' && skipLF {
			skipLF = false
			continue
		}
		skipLF = false
		end := i + len(string(r))
		switch r {
		case '\r':
			if end < len(text) && text[end] == '\n' {
				end++
				skipLF = true
			}
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\x85', '\u2028', '\u2029':
		default:
			continue
		}
		lines = append(lines, text[start:end])
		start = end
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}
func patchDocument(document repository.ConceptDocument, change ProposalChange, actor string, stamp time.Time) repository.ConceptDocument {
	meta := &document.Frontmatter
	if value := change["title"]; value != nil {
		meta.Title = value.(string)
	}
	if value := change["description"]; value != nil {
		text := value.(string)
		meta.Description = &text
	}
	if value := change["status"]; value != nil {
		meta.SetCopiedStatus(value.(string))
	}
	if value := change["tags"]; value != nil {
		meta.Tags = value.([]string)
	}
	if value := change["aliases"]; value != nil {
		meta.Aliases = value.([]string)
	}
	meta.UpdatedAt = stamp
	meta.UpdatedBy = actor
	if value := change["body"]; value != nil {
		document.Body = value.(string)
	}
	return document
}
