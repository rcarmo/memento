package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/access"
)

func TestAssetToolDispatchReference(t *testing.T) {
	testToolReference(t, "asset-tool-dispatch.json", assetToolDefinitions)
}
func assetGetFixture(t *testing.T) (map[string][]byte, []struct {
	Arguments map[string]any
	Policy    access.EffectivePolicy
	Mutation  *struct{ Path, Base64 string }
	Expected  map[string]any
}) {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/parity/asset-get.json")
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
	files := map[string][]byte{}
	for path, encoded := range f.Files {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = data
	}
	return files, f.Cases
}
func installAssetFiles(t *testing.T, root string, files map[string][]byte) {
	t.Helper()
	for path, content := range files {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func assetOptions(args map[string]any) AssetGetOptions {
	o := AssetGetOptions{IDOrPath: args["id_or_path"].(string), AssetKind: args["asset_kind"].(string), View: "archive"}
	if value, ok := args["view"].(string); ok {
		o.View = value
	}
	if n, ok := args["offset"].(float64); ok {
		o.Offset = int64(n)
	}
	if n, ok := args["limit"].(float64); ok {
		value := int64(n)
		o.Limit = &value
	}
	if _, ok := args["offset"].(bool); ok {
		o.Offset = -1
	}
	if _, ok := args["limit"].(bool); ok {
		value := int64(0)
		o.Limit = &value
	}
	for key, target := range map[string]**string{"version": &o.Version, "file_path": &o.FilePath, "expected_sha256": &o.ExpectedSHA256} {
		if value, ok := args[key].(string); ok {
			*target = &value
		}
	}
	return o
}
func TestAssetGetReference(t *testing.T) {
	files, cases := assetGetFixture(t)
	c, _ := rebaseTest(t)
	root := t.TempDir()
	c.Queue.Paths.CurrentDir = root
	installAssetFiles(t, root, files)
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if tc.Mutation != nil {
				data, err := base64.StdEncoding.DecodeString(tc.Mutation.Base64)
				if err != nil {
					t.Fatal(err)
				}
				installAssetFiles(t, root, map[string][]byte{tc.Mutation.Path: data})
				defer installAssetFiles(t, root, files)
			}
			data, options, err := c.assetGet(context.Background(), ProposalActor{Policy: tc.Policy}, assetOptions(tc.Arguments), os.Stat, func(path string) (assetSource, error) { return os.Open(path) }, fakeRepo(nil))
			var result any
			if err != nil {
				result, err = FailureEnvelope(err)
			} else {
				result, err = c.Queue.successEnvelope(data, options, fakeRepo(nil))
			}
			if err != nil || !reflect.DeepEqual(jsonNormal(result), tc.Expected) {
				t.Fatal(tc.Arguments, result, tc.Expected, err)
			}
		})
	}
}
