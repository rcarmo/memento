package assets

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAcceptedAssetPathsLatestVersion(t *testing.T) {
	root := t.TempDir()

	latest := acceptedVersion(t)
	latest.Version = "1.10.0"
	latest.Manifest = acceptedManifestWithPrefix(latest.Manifest, "latest")

	older := acceptedVersion(t)
	older.Version = "1.9.0"
	older.Manifest = acceptedManifestWithPrefix(older.Manifest, "older")

	images := acceptedVersion(t)
	images.AssetKind = "images"
	images.Version = "2.0.0"
	images.Manifest = acceptedManifestWithPrefix(images.Manifest, "images")

	for _, version := range []AcceptedVersion{latest, older, images} {
		if _, err := WriteAssetVersion(root, version); err != nil {
			t.Fatal(err)
		}
	}

	got, err := AcceptedAssetPaths(root, latest.ConceptID)
	if err != nil {
		t.Fatal(err)
	}

	want := manifestPathSet(latest.Manifest)
	for entry := range manifestPathSet(images.Manifest) {
		want[entry] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
}

func TestAcceptedAssetPathsMetadataAndZIPFailures(t *testing.T) {
	base := acceptedVersion(t)
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string, string)
	}{
		{
			name: "missing metadata",
			mutate: func(t *testing.T, metadataPath, _ string) {
				t.Helper()
				if err := os.Remove(metadataPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "invalid metadata json",
			mutate: func(t *testing.T, metadataPath, _ string) {
				t.Helper()
				if err := os.WriteFile(metadataPath, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "metadata identity mismatch",
			mutate: func(t *testing.T, metadataPath, _ string) {
				t.Helper()
				rewriteAcceptedMetadata(t, metadataPath, func(metadata map[string]any) {
					metadata["concept_id"] = "87654321"
				})
			},
		},
		{
			name: "invalid zip digest metadata",
			mutate: func(t *testing.T, metadataPath, _ string) {
				t.Helper()
				rewriteAcceptedMetadata(t, metadataPath, func(metadata map[string]any) {
					metadata["zip_sha256"] = true
				})
			},
		},
		{
			name: "missing zip",
			mutate: func(t *testing.T, _ string, zipPath string) {
				t.Helper()
				if err := os.Remove(zipPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "zip directory",
			mutate: func(t *testing.T, _ string, zipPath string) {
				t.Helper()
				if err := os.Remove(zipPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(zipPath, 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if _, err := WriteAssetVersion(root, base); err != nil {
				t.Fatal(err)
			}
			metadataPath, zipPath := acceptedAssetStoragePaths(t, root, base)
			tc.mutate(t, metadataPath, zipPath)
			if _, err := AcceptedAssetPaths(root, base.ConceptID); err == nil {
				t.Fatal("expected AcceptedAssetPaths to fail")
			}
		})
	}
}

func TestAcceptedAssetPathsSymlinkSafety(t *testing.T) {
	base := acceptedVersion(t)
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string, string, string)
	}{
		{
			name: "metadata symlink",
			mutate: func(t *testing.T, root, metadataPath, _ string) {
				t.Helper()
				if err := os.Remove(metadataPath); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(t.TempDir(), "outside.json")
				if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, metadataPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "zip symlink",
			mutate: func(t *testing.T, root, _, zipPath string) {
				t.Helper()
				if err := os.Remove(zipPath); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(t.TempDir(), "outside.zip")
				if err := os.WriteFile(target, []byte("zip"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, zipPath); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "kind symlink",
			mutate: func(t *testing.T, root, metadataPath, _ string) {
				t.Helper()
				kindDir := filepath.Dir(metadataPath)
				if err := os.RemoveAll(kindDir); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), kindDir); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "assets parent symlink",
			mutate: func(t *testing.T, root, _, _ string) {
				t.Helper()
				assetsDir := filepath.Join(root, ".assets")
				if err := os.RemoveAll(assetsDir); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), assetsDir); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if _, err := WriteAssetVersion(root, base); err != nil {
				t.Fatal(err)
			}
			metadataPath, zipPath := acceptedAssetStoragePaths(t, root, base)
			tc.mutate(t, root, metadataPath, zipPath)
			if _, err := AcceptedAssetPaths(root, base.ConceptID); err == nil {
				t.Fatal("expected AcceptedAssetPaths to fail")
			}
		})
	}
}

func acceptedManifestWithPrefix(manifest Manifest, prefix string) Manifest {
	clone := manifest
	clone.Entries = make([]ManifestEntry, len(manifest.Entries))
	copy(clone.Entries, manifest.Entries)
	for i := range clone.Entries {
		clone.Entries[i].Path = path.Join(prefix, clone.Entries[i].Path)
	}
	return clone
}

func manifestPathSet(manifest Manifest) map[string]bool {
	paths := make(map[string]bool, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		paths[entry.Path] = true
	}
	return paths
}

func acceptedAssetStoragePaths(t *testing.T, root string, version AcceptedVersion) (string, string) {
	t.Helper()
	metadataPath, zipPath, err := AssetVersionPaths(version.ConceptID, version.AssetKind, version.Version)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, metadataPath[1:]), filepath.Join(root, zipPath[1:])
}

func rewriteAcceptedMetadata(t *testing.T, metadataPath string, mutate func(map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	mutate(metadata)
	raw, err = json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataPath, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAcceptedAssetPathsInvalidManifest(t *testing.T) {
	for _, manifest := range []string{`{}`, `{"entries":[],"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","total_uncompressed_bytes":0,"file_count":0}`} {
		t.Run(manifest, func(t *testing.T) {
			root := t.TempDir()
			v := acceptedVersion(t)
			if _, err := WriteAssetVersion(root, v); err != nil {
				t.Fatal(err)
			}
			meta, _, _ := AssetVersionPaths(v.ConceptID, v.AssetKind, v.Version)
			raw := `{"kind":"asset_pack_version","concept_id":"12345678","asset_kind":"docs","version":"1.0.0","zip_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","manifest":` + manifest + `}`
			if err := os.WriteFile(filepath.Join(root, meta[1:]), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := AcceptedAssetPaths(root, v.ConceptID); err == nil {
				t.Fatal("invalid manifest")
			}
		})
	}
}
