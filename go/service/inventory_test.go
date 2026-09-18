package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"
)

func TestInventoryDispatchReference(t *testing.T) {
	testToolReference(t, "inventory-tool-dispatch.json", inventoryToolDefinitions)
}
func TestInventoryReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Files map[string]string
		Cases []struct {
			Arguments map[string]any
			Policy    access.EffectivePolicy
			Mutation  *struct{ Path, Base64 string }
			Expected  map[string]any
		}
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	c, _ := rebaseTest(t)
	root := t.TempDir()
	c.Queue.Paths.CurrentDir = root
	files := map[string][]byte{}
	for path, encoded := range f.Files {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = data
	}
	installAssetFiles(t, root, files)
	for i, tc := range f.Cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if tc.Mutation != nil {
				blob, err := base64.StdEncoding.DecodeString(tc.Mutation.Base64)
				if err != nil {
					t.Fatal(err)
				}
				installAssetFiles(t, root, map[string][]byte{tc.Mutation.Path: blob})
				defer func() {
					if old, ok := files[tc.Mutation.Path]; ok {
						installAssetFiles(t, root, map[string][]byte{tc.Mutation.Path: old})
					} else {
						os.Remove(filepath.Join(root, tc.Mutation.Path))
					}
				}()
			}
			prefix := "/"
			if s, ok := tc.Arguments["path_prefix"].(string); ok {
				prefix = s
			}
			limit := 50
			if n, ok := tc.Arguments["limit"].(float64); ok {
				limit = int(n)
			}
			if b, ok := tc.Arguments["limit"].(bool); ok {
				limit = 0
				if b {
					limit = 1
				}
			}
			var fields []string
			if raw, ok := tc.Arguments["fields"].([]any); ok {
				fields = []string{}
				for _, item := range raw {
					fields = append(fields, item.(string))
				}
			}
			if text, ok := tc.Arguments["fields"].(string); ok {
				fields = []string{}
				for _, r := range text {
					fields = append(fields, string(r))
				}
			}
			var cursor *string
			if s, ok := tc.Arguments["cursor"].(string); ok {
				cursor = &s
			}
			data, options, err := c.inventory(context.Background(), ProposalActor{Policy: tc.Policy}, prefix, fields, limit, cursor, fakeRepo(nil))
			var got any
			if err != nil {
				got, err = FailureEnvelope(err)
			} else {
				got, err = c.Queue.successEnvelope(data, options, fakeRepo(nil))
			}
			if err != nil || !reflect.DeepEqual(jsonNormal(got), tc.Expected) {
				t.Fatal(tc.Arguments, tc.Policy, got, tc.Expected, err)
			}
		})
	}
}
func TestInventoryFailuresAndMetadataSemantics(t *testing.T) {
	c, actor, _ := realApplyTest(t)
	actor.Policy.Roles = []string{"reader"}
	root := c.Queue.Paths.CurrentDir
	files, _ := assetGetFixture(t)
	installAssetFiles(t, root, files)
	repo := fakeRepo(nil)
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, _, err := c.inventory(context.Background(), actor, "/", []string{"path"}, 50, nil, repo); err != io.ErrClosedPipe {
		t.Fatal(err)
	}
	if err := os.Symlink("/outside", filepath.Join(root, "public/unsafe.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.inventory(context.Background(), actor, "/public/", nil, 1, nil, fakeRepo(nil)); err == nil {
		t.Fatal("symlink paths validated before pagination")
	}
	os.Remove(filepath.Join(root, "public/unsafe.md"))
	directory := filepath.Join(root, ".assets/12345678")
	if err := os.Mkdir(filepath.Join(directory, "INVALID"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := c.inventoryAssets("12345678"); err == nil {
		t.Fatal("bad kind")
	}
	os.Remove(filepath.Join(directory, "INVALID"))
	if err := os.Mkdir(filepath.Join(directory, "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := c.inventoryAssets("12345678"); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(directory, "docs/bad.json")
	if err := os.WriteFile(bad, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.inventoryAssets("12345678"); err == nil {
		t.Fatal("bad version")
	}
	os.Remove(bad)
	latest := filepath.Join(directory, "docs/1.10.0.json")
	if err := os.WriteFile(latest, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.inventoryAssets("12345678"); err == nil {
		t.Fatal("bad metadata")
	}
}
