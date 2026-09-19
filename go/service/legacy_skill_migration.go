package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/repository"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type legacySkillMetadata struct {
	Kind             string          `json:"kind"`
	SourceProposalID string          `json:"source_proposal_id"`
	AcceptedBy       string          `json:"accepted_by"`
	Manifest         assets.Manifest `json:"manifest"`
}
type legacySkillIO struct {
	readDir      func(string) ([]os.DirEntry, error)
	readFile     func(string) ([]byte, error)
	mkdirAll     func(string, os.FileMode) error
	writeFile    func(string, []byte, os.FileMode) error
	remove       func(string) error
	parseConcept func(string) (repository.ConceptDocument, error)
	serialize    func(repository.ConceptDocument) (string, error)
	writeAsset   func(string, assets.AcceptedVersion) ([]string, error)
	now          func() time.Time
}

func defaultLegacySkillIO() legacySkillIO {
	return legacySkillIO{os.ReadDir, os.ReadFile, os.MkdirAll, os.WriteFile, os.Remove, repository.ParseConceptFile, repository.SerializeCopiedConcept, assets.WriteAssetVersion, time.Now}
}
func MigrateLegacySkillPacks(worktree string) ([]string, error) {
	return migrateLegacySkillPacks(worktree, defaultLegacySkillIO())
}
func migrateLegacySkillPacks(worktree string, ops legacySkillIO) ([]string, error) {
	root := filepath.Join(worktree, "skills", ".versions")
	entries, err := ops.readDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	for _, skillEntry := range entries {
		if !skillEntry.IsDir() {
			continue
		}
		name := skillEntry.Name()
		versions, err := legacySkillVersions(filepath.Join(root, name), ops)
		if err != nil {
			return nil, err
		}
		if len(versions) == 0 {
			continue
		}
		latestMeta, latestBody, err := parseLegacySkillDocument(filepath.Join(root, name, versions[len(versions)-1]+".md"), ops)
		if err != nil {
			return nil, err
		}
		conceptPath := "/skills/" + name + ".md"
		conceptFile := filepath.Join(worktree, strings.TrimPrefix(conceptPath, "/"))
		conceptID := latestMeta.SourceProposalID
		if conceptID == "" {
			conceptID = "legacy-" + name
		}
		document := legacySkillConcept(conceptFile, name, conceptID, latestBody, ops)
		rendered, err := ops.serialize(document)
		if err != nil {
			return nil, err
		}
		if err = ops.mkdirAll(filepath.Dir(conceptFile), 0755); err != nil {
			return nil, err
		}
		if err = ops.writeFile(conceptFile, []byte(rendered), 0644); err != nil {
			return nil, err
		}
		changed[conceptPath] = true
		for _, version := range versions {
			metadataPath := filepath.Join(root, name, version+".md")
			zipPath := filepath.Join(root, name, version+".zip")
			metadata, _, err := parseLegacySkillDocument(metadataPath, ops)
			if err != nil {
				return nil, err
			}
			zipBytes, err := ops.readFile(zipPath)
			if err != nil {
				return nil, err
			}
			accepted := metadata.AcceptedBy
			if accepted == "" {
				accepted = "memento-migration"
			}
			source := metadata.SourceProposalID
			if source == "" {
				source = "legacy"
			}
			paths, err := ops.writeAsset(worktree, assets.AcceptedVersion{ConceptID: conceptID, ConceptPath: conceptPath, AssetKind: "skill", Version: version, ZIPBytes: zipBytes, Manifest: metadata.Manifest, AcceptedBy: accepted, SourceProposalID: source})
			if err != nil {
				return nil, err
			}
			for _, path := range paths {
				changed[path] = true
			}
			if err = ops.remove(metadataPath); err != nil {
				return nil, err
			}
			if err = ops.remove(zipPath); err != nil {
				return nil, err
			}
			changed["/skills/.versions/"+name+"/"+version+".md"] = true
			changed["/skills/.versions/"+name+"/"+version+".zip"] = true
		}
	}
	out := make([]string, 0, len(changed))
	for path := range changed {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}
func legacySkillVersions(directory string, ops legacySkillIO) ([]string, error) {
	entries, err := ops.readDir(directory)
	if err != nil {
		return nil, err
	}
	versions := []string{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		version := strings.TrimSuffix(entry.Name(), ".md")
		if _, err := assets.ParseStableSemver(version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool {
		a, _ := assets.ParseStableSemver(versions[i])
		b, _ := assets.ParseStableSemver(versions[j])
		for k := 0; k < 3; k++ {
			if compare := a[k].Cmp(b[k]); compare != 0 {
				return compare < 0
			}
		}
		return versions[i] < versions[j]
	})
	return versions, nil
}
func parseLegacySkillDocument(path string, ops legacySkillIO) (legacySkillMetadata, string, error) {
	raw, err := ops.readFile(path)
	if err != nil {
		return legacySkillMetadata{}, "", err
	}
	const prefix = "---skill-pack-json\n"
	const separator = "\n---\n"
	text := string(raw)
	if !strings.HasPrefix(text, prefix) || !strings.Contains(text, separator) {
		return legacySkillMetadata{}, "", errors.New("invalid skill pack metadata document")
	}
	parts := strings.SplitN(strings.TrimPrefix(text, prefix), separator, 2)
	var metadata legacySkillMetadata
	if json.Unmarshal([]byte(parts[0]), &metadata) != nil {
		return metadata, "", errors.New("invalid skill pack metadata JSON")
	}
	if metadata.Kind != "skill_pack_version" {
		return metadata, "", errors.New("invalid skill pack metadata kind")
	}
	if metadata.Manifest.FileCount != len(metadata.Manifest.Entries) {
		return metadata, "", errors.New("invalid skill pack manifest")
	}
	return metadata, parts[1], nil
}
func legacySkillConcept(path, name, id, body string, ops legacySkillIO) repository.ConceptDocument {
	if existing, err := ops.parseConcept(path); err == nil {
		tags := append([]string{}, existing.Frontmatter.Tags...)
		found := false
		for _, tag := range tags {
			found = found || tag == "skill"
		}
		if !found {
			tags = append(tags, "skill")
			sort.Strings(tags)
		}
		existing.Frontmatter.Tags = tags
		existing.Body = body
		return existing
	}
	title := cases.Title(language.English).String(strings.ReplaceAll(name, "-", " "))
	description := fmt.Sprintf("Versioned agent skill %s.", name)
	stamp := ops.now().UTC().Truncate(time.Second)
	front := repository.ConceptFrontmatter{SchemaVersion: 1, ID: id, Type: "concept", Title: title, Status: "active", Description: &description, Aliases: []string{}, Tags: []string{"skill"}, SourceRefs: []string{}, Supersedes: []string{}, CreatedAt: stamp, UpdatedAt: stamp, UpdatedBy: "memento-migration"}
	front.SetCopiedStatus("active")
	return repository.ConceptDocument{Frontmatter: front, Body: body}
}
