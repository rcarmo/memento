package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/repository"
)

func TestManifestDispatchReference(t *testing.T) {
	testToolReference(t, "manifest-tool-dispatch.json", manifestToolDefinitions)
}
func manifestTestItem() map[string]any {
	return map[string]any{"name": "a", "local_path": "a.md", "memento_path": "/public/a.md", "local_updated_at": "2026-01-01T00:00:00Z", "local_bytes": json.Number("9"), "local_body_sha256": "0" + string(bytes.Repeat([]byte{'0'}, 63))}
}
func TestManifestTimestampReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/manifest-timestamps.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Value           any
		Expected, Error string
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		got, err := manifestTimestamp(tc.Value)
		if tc.Error != "" {
			if err == nil || err.Error() != tc.Error {
				t.Fatal(tc.Value, got, err, tc.Error)
			}
		} else if err != nil || modelTimestamp(got) != tc.Expected {
			t.Fatal(tc.Value, got, err, tc.Expected)
		}
	}
	now := time.Now()
	if got, err := manifestTimestamp(now); err != nil || !got.Equal(now.UTC().Truncate(time.Microsecond)) {
		t.Fatal(got, err)
	}
}
func TestCompareManifestReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/compare-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Files map[string]string
		Cases []struct {
			Arguments map[string]any
			Policy    access.EffectivePolicy
			Extra     int
			Expected  map[string]any
		}
		Summaries []struct {
			Value    any
			Expected map[string]any
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	c, _ := rebaseTest(t)
	root := t.TempDir()
	c.Queue.Paths.CurrentDir = root
	files := map[string][]byte{}
	for path, encoded := range fixture.Files {
		value, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = value
	}
	installAssetFiles(t, root, files)
	for i, tc := range fixture.Cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			for n := range tc.Extra {
				path := filepath.Join(root, fmt.Sprintf("public/z%03d.md", n))
				if err := os.WriteFile(path, files["/public/empty.md"], 0600); err != nil {
					t.Fatal(err)
				}
				defer os.Remove(path)
			}
			args := tc.Arguments
			match, _ := args["match"].(map[string]any)
			include, _ := args["include_asset_metadata"].(bool)
			data, options, err := c.compareManifest(context.Background(), ProposalActor{Policy: tc.Policy}, args["path_prefix"].(string), args["items"].([]any), match, include, fakeRepo(nil))
			var result any
			if err != nil {
				result, err = FailureEnvelope(err)
			} else {
				result, err = c.Queue.successEnvelope(data, options, fakeRepo(nil))
			}
			if err != nil || !reflect.DeepEqual(jsonNormal(result), jsonNormal(tc.Expected)) {
				t.Fatal(args, result, tc.Expected, err)
			}
		})
	}
	for _, tc := range fixture.Summaries {
		if got := manifestAssetSummary(tc.Value); !reflect.DeepEqual(jsonNormal(got), jsonNormal(tc.Expected)) {
			t.Fatal(got, tc.Expected)
		}
	}
}
func TestManifestFailuresAndTruthiness(t *testing.T) {
	c, actor, _ := realApplyTest(t)
	actor.Policy.Roles = []string{"reader"}
	files, _ := assetGetFixture(t)
	installAssetFiles(t, c.Queue.Paths.CurrentDir, files)
	items := []any{manifestTestItem()}
	run := func(items []any) error {
		_, _, err := c.compareManifest(context.Background(), actor, "/public/", items, nil, false, fakeRepo(nil))
		return err
	}
	denied := actor
	denied.Policy.ProtectedReadPrefixes = []string{"/public/blocked/"}
	item := manifestTestItem()
	item["memento_path"] = "/public/blocked/a.md"
	if _, _, err := c.compareManifest(context.Background(), denied, "/public/", []any{item}, nil, false, fakeRepo(nil)); err == nil {
		t.Fatal("target authorisation")
	}
	if err := run([]any{1}); err == nil {
		t.Fatal("bad mapping")
	}
	c.inventoryPolicy = func(context.Context) (access.EffectivePolicy, error) {
		return access.EffectivePolicy{}, io.ErrClosedPipe
	}
	if err := run(items); err != io.ErrClosedPipe {
		t.Fatal(err)
	}
	c.inventoryPolicy = func(context.Context) (access.EffectivePolicy, error) {
		return access.EffectivePolicy{Principal: "reader", Roles: []string{"proposer"}}, nil
	}
	if err := run(items); err == nil {
		t.Fatal("nested policy not checked")
	}
	c.inventoryPolicy = nil
	path := filepath.Join(c.Queue.Paths.CurrentDir, "public/empty.md")
	if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(items); err == nil {
		t.Fatal("inventory failure")
	}
	for _, tc := range []struct {
		value any
		want  bool
	}{{nil, false}, {false, false}, {true, true}, {"", false}, {"x", true}, {json.Number("0.0"), false}, {json.Number("-3"), true}, {json.Number("bad"), true}, {[]any{}, false}, {[]any{1}, true}, {map[string]any{}, false}, {map[string]any{"x": 1}, true}, {struct{}{}, true}} {
		if manifestTruthy(tc.value) != tc.want {
			t.Fatal(tc)
		}
	}
	if _, err := manifestTimestamp("0001-01-01T00:00+01"); err == nil {
		t.Fatal("UTC overflow")
	}
	if _, err := manifestTimestamp("2026-01-01T"); err == nil {
		t.Fatal("empty time")
	}
	if _, err := manifestTimestamp("2026-0101T00:00Z"); err == nil {
		t.Fatal("mixed date")
	}
	if _, err := manifestTimestamp("202601-01T00:00Z"); err == nil {
		t.Fatal("mixed date")
	}
	repo := fakeRepo(nil)
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	installAssetFiles(t, c.Queue.Paths.CurrentDir, files)
	if _, _, err := c.compareManifest(context.Background(), actor, "/public/", items, nil, false, repo); err != io.ErrClosedPipe {
		t.Fatal(err)
	}
}

func FuzzManifestTimestamp(f *testing.F) {
	for _, text := range []string{"", "2026-01-01T00:00:00Z", "2026W011🍀12:34:56.1234567+0130", "2026-01-01T00:00:00+00:00:00.5", "0001-01-01T00:00+01", "bad"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 2048 {
			return
		}
		got, err := manifestTimestamp(text)
		if err != nil {
			return
		}
		if got.Year() < 1 || got.Year() > 9999 || got.Nanosecond()%1000 != 0 {
			t.Fatal(got)
		}
		again, err := manifestTimestamp(modelTimestamp(got))
		if err != nil || !again.Equal(got) {
			t.Fatal(got, again, err)
		}
	})
}
