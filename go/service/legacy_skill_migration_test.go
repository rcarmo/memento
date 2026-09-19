package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"github.com/rcarmo/memento/go/assets"
	"github.com/rcarmo/memento/go/repository"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLegacySkill(t *testing.T, root, name, version, body string) {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	file, _ := archive.Create("SKILL.md")
	_, _ = file.Write([]byte(body))
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	pack, err := assets.ValidateSkillPack(name, version, body, buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	metadata := map[string]any{"kind": "skill_pack_version", "source_proposal_id": "12345678-abcd", "accepted_by": "curator", "manifest": pack.Manifest}
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "skills", ".versions", name)
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, version+".md"), []byte("---skill-pack-json\n"+string(raw)+"\n---\n"+body), 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, version+".zip"), buffer.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestMigrateLegacySkillPacks(t *testing.T) {
	root := t.TempDir()
	writeLegacySkill(t, root, "demo", "1.0.0", "# Demo\n")
	changed, err := MigrateLegacySkillPacks(root)
	if err != nil || !containsString(changed, "/skills/demo.md") || !containsString(changed, "/.assets/12345678-abcd/skill/1.0.0.json") {
		t.Fatal(changed, err)
	}
	document, err := repository.ParseConceptFile(filepath.Join(root, "skills", "demo.md"))
	if err != nil || strings.TrimSpace(document.Body) != "# Demo" || !containsString(document.Frontmatter.Tags, "skill") {
		t.Fatal(document, err)
	}
	metadata, err := os.ReadFile(filepath.Join(root, ".assets", "12345678-abcd", "skill", "1.0.0.json"))
	if err != nil || !bytes.Contains(metadata, []byte(`"accepted_by": "curator"`)) {
		t.Fatal(string(metadata), err)
	}
	if _, err = os.Stat(filepath.Join(root, "skills", ".versions", "demo", "1.0.0.md")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	changed, err = MigrateLegacySkillPacks(root)
	if err != nil || len(changed) != 0 {
		t.Fatal(changed, err)
	}
}
func TestMigrateLegacySkillExistingConcept(t *testing.T) {
	root := t.TempDir()
	writeLegacySkill(t, root, "demo", "1.0.0", "# New\n")
	front, err := repository.NewConceptFrontmatter("Existing", "concept", "test")
	if err != nil {
		t.Fatal(err)
	}
	front.Tags = []string{"keep"}
	front.SetCopiedStatus("active")
	raw, err := repository.SerializeConcept(repository.ConceptDocument{Frontmatter: front, Body: "old"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "skills", "demo.md")
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = MigrateLegacySkillPacks(root); err != nil {
		t.Fatal(err)
	}
	doc, err := repository.ParseConceptFile(path)
	if err != nil || doc.Frontmatter.ID != front.ID || !containsString(doc.Frontmatter.Tags, "keep") || !containsString(doc.Frontmatter.Tags, "skill") {
		t.Fatal(doc, err)
	}
}
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func TestLegacySkillParseFailures(t *testing.T) {
	root := t.TempDir()
	if changed, err := MigrateLegacySkillPacks(root); err != nil || len(changed) != 0 {
		t.Fatal(changed, err)
	}
	path := filepath.Join(root, "bad.md")
	for _, text := range []string{"bad", "---skill-pack-json\n{\n---\nx", "---skill-pack-json\n{}\n---\nx", "---skill-pack-json\n{\"kind\":\"skill_pack_version\",\"manifest\":{\"file_count\":1,\"entries\":[]}}\n---\nx"} {
		if err := os.WriteFile(path, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := parseLegacySkillDocument(path, defaultLegacySkillIO()); err == nil {
			t.Fatal(text)
		}
	}
}
