package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
)

func TestMetadataDispatchReference(t *testing.T) {
	testToolReference(t, "metadata-tool-dispatch.json", metadataToolDefinitions)
}
func metadataOptions(args map[string]any) AssetMetadataOptions {
	o := AssetMetadataOptions{Limit: 20, VersionLimit: 5, FileLimit: 50, IncludeFiles: false}
	for key, target := range map[string]**string{"id_or_path": &o.IDOrPath, "path_prefix": &o.PathPrefix, "asset_kind": &o.AssetKind, "version": &o.Version, "cursor": &o.Cursor} {
		if value, ok := args[key].(string); ok {
			*target = &value
		}
	}
	for key, target := range map[string]*int{"limit": &o.Limit, "version_limit": &o.VersionLimit, "file_limit": &o.FileLimit} {
		if value, ok := args[key].(json.Number); ok {
			n, _ := value.Int64()
			*target = int(n)
		}
	}
	if v, ok := args["include_files"]; ok {
		o.IncludeFiles = v
	}
	return o
}
func TestAssetMetadataReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/asset-metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Files map[string]string
		Cases []struct {
			Arguments map[string]any
			Policy    access.EffectivePolicy
			Mutation  *struct {
				Path, Base64 string
				Files        map[string]string
			}
			Expected map[string]any
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = d.Decode(&f); err != nil {
		t.Fatal(err)
	}
	c, _ := rebaseTest(t)
	root := t.TempDir()
	c.Queue.Paths.CurrentDir = root
	files := map[string][]byte{}
	for path, encoded := range f.Files {
		b, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = b
	}
	installAssetFiles(t, root, files)
	if err := os.Mkdir(filepath.Join(root, ".assets/12345678/empty"), 0700); err != nil {
		t.Fatal(err)
	}
	ops := defaultMetadataIO()
	ops.timestamp = func(context.Context, repository.GitRepositoryPaths, string, string) (time.Time, error) {
		return time.Date(2026, 1, 2, 2, 4, 5, 0, time.UTC), nil
	}
	for i, tc := range f.Cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if tc.Mutation != nil {
				mutations := map[string]string{}
				if tc.Mutation.Path != "" {
					mutations[tc.Mutation.Path] = tc.Mutation.Base64
				}
				for k, v := range tc.Mutation.Files {
					mutations[k] = v
				}
				for path, encoded := range mutations {
					b, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						t.Fatal(err)
					}
					installAssetFiles(t, root, map[string][]byte{path: b})
					defer func(path string) {
						if b, ok := files[path]; ok {
							installAssetFiles(t, root, map[string][]byte{path: b})
						} else {
							os.Remove(filepath.Join(root, path))
						}
					}(path)
				}
			}
			data, options, err := c.assetMetadata(context.Background(), ProposalActor{Policy: tc.Policy}, metadataOptions(tc.Arguments), fakeRepo(nil), ops)
			var result any
			if err != nil {
				result, err = FailureEnvelope(err)
			} else {
				result, err = c.Queue.successEnvelope(data, options, fakeRepo(nil))
			}
			if err != nil || !reflect.DeepEqual(jsonNormal(result), jsonNormal(tc.Expected)) {
				t.Fatal(tc.Arguments, result, tc.Expected, err)
			}
		})
	}
}
