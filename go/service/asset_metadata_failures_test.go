package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"
)

func TestMetadataBudgetReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/asset-metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Budgets []struct {
			Scenario    string
			Files       map[string]string
			Directories []string
			Arguments   map[string]any
			Policy      access.EffectivePolicy
			Expected    map[string]any
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixture.Budgets {
		t.Run(tc.Scenario, func(t *testing.T) {
			c, _ := rebaseTest(t)
			c.Queue.Paths.CurrentDir = t.TempDir()
			files := map[string][]byte{}
			for path, encoded := range tc.Files {
				b, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					t.Fatal(err)
				}
				files[path] = b
			}
			installAssetFiles(t, c.Queue.Paths.CurrentDir, files)
			for _, path := range tc.Directories {
				if err := os.MkdirAll(filepath.Join(c.Queue.Paths.CurrentDir, path), 0700); err != nil {
					t.Fatal(err)
				}
			}
			data, options, err := c.assetMetadata(context.Background(), ProposalActor{Policy: tc.Policy}, metadataOptions(tc.Arguments), fakeRepo(nil), defaultMetadataIO())
			var result any
			if err != nil {
				result, err = FailureEnvelope(err)
			} else {
				result, err = c.Queue.successEnvelope(data, options, fakeRepo(nil))
			}
			if err != nil || !reflect.DeepEqual(jsonNormal(result), jsonNormal(tc.Expected)) {
				t.Fatal(result, tc.Expected, err)
			}
		})
	}
}
func TestAssetMetadataFailurePaths(t *testing.T) {
	for _, scenario := range []string{"paths", "revision", "entry", "assets-link", "concept-file", "kinds", "kind-stat", "versions", "detail-path", "detail-load", "manifest", "timestamp", "zip-missing", "zip-stat", "skill-root", "latest-load", "latest-digest"} {
		t.Run(scenario, func(t *testing.T) {
			c, actor, _ := realApplyTest(t)
			actor.Policy.Roles = []string{"reader"}
			root := c.Queue.Paths.CurrentDir
			files, _ := assetGetFixture(t)
			installAssetFiles(t, root, files)
			o := metadataOptions(map[string]any{"path_prefix": "/public/"})
			ops := defaultMetadataIO()
			ops.timestamp = func(context.Context, repository.GitRepositoryPaths, string, string) (time.Time, error) {
				return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil
			}
			repo := fakeRepo(nil)
			meta := filepath.Join(root, ".assets/12345678/docs/1.10.0.json")
			zip := filepath.Join(root, ".assets/12345678/docs/1.10.0.zip")
			switch scenario {
			case "paths":
				if err := os.Symlink("/outside", filepath.Join(root, "public/link.md")); err != nil {
					t.Fatal(err)
				}
			case "revision":
				repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
			case "entry":
				if err := os.WriteFile(filepath.Join(root, "public/a.md"), []byte("bad"), 0600); err != nil {
					t.Fatal(err)
				}
			case "assets-link":
				os.Rename(filepath.Join(root, ".assets"), filepath.Join(root, "saved"))
				if err := os.Symlink("saved", filepath.Join(root, ".assets")); err != nil {
					t.Fatal(err)
				}
			case "concept-file":
				os.RemoveAll(filepath.Join(root, ".assets/12345678"))
				os.WriteFile(filepath.Join(root, ".assets/12345678"), nil, 0600)
			case "kinds":
				os.Mkdir(filepath.Join(root, ".assets/12345678/INVALID"), 0700)
			case "kind-stat":
				original := ops.lstat
				ops.lstat = func(path string) (fs.FileInfo, error) {
					if path == filepath.Join(root, ".assets/12345678/docs") {
						return nil, io.ErrClosedPipe
					}
					return original(path)
				}
			case "versions":
				os.WriteFile(filepath.Join(root, ".assets/12345678/docs/bad.json"), nil, 0600)
			case "detail-path":
				if _, _, err := c.assetVersionMetadata(context.Background(), "bad", "docs", "1.0.0", "sha", "main", false, 1, ops); err == nil {
					t.Fatal("bad identity")
				}
				return
			case "detail-load":
				os.WriteFile(meta, []byte("{}"), 0600)
			case "manifest":
				var m map[string]any
				json.Unmarshal(files["/.assets/12345678/docs/1.10.0.json"], &m)
				m["manifest"] = nil
				b, _ := json.Marshal(m)
				os.WriteFile(meta, b, 0600)
			case "timestamp":
				ops.timestamp = func(context.Context, repository.GitRepositoryPaths, string, string) (time.Time, error) {
					return time.Time{}, io.ErrClosedPipe
				}
			case "zip-missing":
				os.Remove(zip)
			case "zip-stat":
				original := ops.stat
				ops.stat = func(path string) (fs.FileInfo, error) {
					if path == zip {
						return nil, io.ErrClosedPipe
					}
					return original(path)
				}
			case "skill-root":
				os.Rename(filepath.Join(root, ".assets/12345678/docs"), filepath.Join(root, ".assets/12345678/skill"))
				for _, v := range []string{"1.9.0", "1.10.0"} {
					p := filepath.Join(root, ".assets/12345678/skill/"+v+".json")
					b, _ := os.ReadFile(p)
					var m map[string]any
					json.Unmarshal(b, &m)
					m["asset_kind"] = "skill"
					b, _ = json.Marshal(m)
					os.WriteFile(p, b, 0600)
				}
			case "latest-load":
				kind, version := "docs", "9.0.0"
				o.AssetKind = &kind
				o.Version = &version
				os.WriteFile(meta, []byte("{}"), 0600)
			case "latest-digest":
				kind, version := "docs", "9.0.0"
				o.AssetKind = &kind
				o.Version = &version
				var m map[string]any
				json.Unmarshal(files["/.assets/12345678/docs/1.10.0.json"], &m)
				m["zip_sha256"] = 1
				b, _ := json.Marshal(m)
				os.WriteFile(meta, b, 0600)
			}
			if _, _, err := c.assetMetadata(context.Background(), actor, o, repo, ops); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
}
