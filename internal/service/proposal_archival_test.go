package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/pyjson"
	"github.com/rcarmo/memento/internal/repository"
)

func archiveDB(t *testing.T, root string) *sql.DB {
	t.Helper()
	db, err := control.Connect(context.Background(), filepath.Join(root, "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE index_state(key TEXT,value TEXT); CREATE TABLE concepts(id TEXT,path TEXT); CREATE TABLE links(source_id TEXT,target_path TEXT,raw_target TEXT,resolution_state TEXT); INSERT INTO index_state VALUES('index_revision','main'),('status','ready');`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
func archivePolicy() access.EffectivePolicy {
	return access.EffectivePolicy{Principal: "author", Roles: []string{"curator"}, ReadPrefixes: []string{"/public/"}, WritePrefixes: []string{"/public/"}, ProtectedReadPrefixes: []string{"/private/"}}
}
func archiveRepo() proposalRepository {
	r := fakeRepo(nil)
	r.read = func(string, string) (repository.BundleEntry, error) {
		return repository.BundleEntry{Document: repository.ConceptDocument{Frontmatter: repository.ConceptFrontmatter{ID: "12345678"}}}, nil
	}
	return r
}
func writeArchiveAssets(t *testing.T, root string, versions map[string][]string) {
	t.Helper()
	for kind, list := range versions {
		for _, version := range list {
			dir := filepath.Join(root, ".assets", "12345678", kind)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			raw := fmt.Sprintf(`{"kind":"asset_pack_version","concept_id":"12345678","asset_kind":%q,"version":%q,"zip_sha256":%q,"manifest":{}}`, kind, version, kind+":"+version)
			if err := os.WriteFile(filepath.Join(dir, version+".json"), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestArchivalReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-archival.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Revision string
		Revisions          []string
		States, References [][]string
		Changes            []any
		Policy             access.EffectivePolicy
		Versions           map[string][]string
		Expected           []any
		ErrorType, Error   string
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			root := t.TempDir()
			db := archiveDB(t, root)
			if _, err := db.Exec("DELETE FROM index_state"); err != nil {
				t.Fatal(err)
			}
			for _, row := range c.States {
				if _, err := db.Exec("INSERT INTO index_state VALUES(?,?)", row[0], row[1]); err != nil {
					t.Fatal(err)
				}
			}
			for i, row := range c.References {
				if _, err := db.Exec("INSERT INTO concepts VALUES(?,?)", fmt.Sprint(i), row[0]); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec("INSERT INTO links VALUES(?,?,?,?)", fmt.Sprint(i), "/public/a.md", row[1], row[2]); err != nil {
					t.Fatal(err)
				}
			}
			writeArchiveAssets(t, root, c.Versions)
			if c.Scenario == "destination" {
				target := filepath.Join(root, "trash/public/a.md")
				_ = os.MkdirAll(filepath.Dir(target), 0700)
				_ = os.WriteFile(target, nil, 0600)
			}
			q := ProposalQueue{Paths: repository.GitRepositoryPaths{CurrentDir: root}}
			repo := archiveRepo()
			calls := 0
			repo.main = func(repository.GitRepositoryPaths) (string, error) { r := c.Revisions[calls]; calls++; return r, nil }
			if c.Scenario == "missing" {
				repo.read = func(string, string) (repository.BundleEntry, error) { return repository.BundleEntry{}, os.ErrNotExist }
			}
			changes, err := NormalizeProposalChanges(c.Changes)
			if err != nil {
				t.Fatal(err)
			}
			got, err := q.archivalImpact(context.Background(), c.Policy, changes, c.Revision, filepath.Join(root, "index.sqlite"), repo, openArchivalIndex)
			if c.Error != "" {
				if err == nil {
					t.Fatal("missing failure", got)
				}
				if c.Scenario != "missing" && err.Error() != c.Error {
					t.Fatal(err, c.Error)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(jsonNormal(got), c.Expected) {
				t.Fatal(got, c.Expected, err)
			}
		})
	}
}
func TestVisibleArchivalReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-archival-visible.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Status   control.ProposalStatus
		Base     string `json:"base_revision"`
		Patch    map[string]any
		Policy   access.EffectivePolicy
		Expected []any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	db := archiveDB(t, root)
	_, err = db.Exec(`INSERT INTO concepts VALUES('public','/public/source.md'),('private','/private/source.md'); INSERT INTO links VALUES('public','/public/a.md','target','resolved'),('private','/public/a.md','hidden','resolved')`)
	if err != nil {
		t.Fatal(err)
	}
	q := ProposalQueue{Paths: repository.GitRepositoryPaths{CurrentDir: root}}
	for _, c := range cases {
		patch, err := pyjson.Dumps(c.Patch)
		if err != nil {
			t.Fatal(err)
		}
		r := control.ProposalRecord{Status: c.Status, BaseRevision: c.Base, PatchJSON: patch}
		got, err := q.visibleArchivalImpact(context.Background(), r, c.Policy, filepath.Join(root, "index.sqlite"), archiveRepo())
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Expected) == 1 && c.Expected[0].(map[string]any)["recomputed_for"] != nil {
			report := got[0].(map[string]any)
			if report["revision"] != "main" || report["retained"] != nil || len(report["inbound_references"].([]any)) != 1 {
				t.Fatal(got)
			}
		} else if !reflect.DeepEqual(jsonNormal(got), c.Expected) {
			t.Fatal(got, c.Expected)
		}
	}
}
