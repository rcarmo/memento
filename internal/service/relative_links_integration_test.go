package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/derived"
	"github.com/rcarmo/memento/internal/repository"
)

func writeLinkAssets(t *testing.T, root string) {
	t.Helper()
	digest := strings.Repeat("a", 64)
	manifest := assets.Manifest{SHA256: digest, FileCount: 2, Entries: []assets.ManifestEntry{{Path: "references/guide.md", SHA256: digest, MediaType: "text/markdown"}, {Path: "target.md", SHA256: digest, MediaType: "text/markdown"}}}
	_, err := assets.WriteAssetVersion(root, assets.AcceptedVersion{ConceptID: "12345678", ConceptPath: "/public/a.md", AssetKind: "skill", Version: "1.0.0", Manifest: manifest, ZIPBytes: []byte("accepted archive")})
	if err != nil {
		t.Fatal(err)
	}
}
func TestRelativeLinksAcrossIndexAuditDreamAndMove(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	body := "[target](target.md#p) [up](../public/target.md) [self](#part) [resource](references/guide.md#x) [missing](references/missing.md) [net](//host/a) [scheme](custom:foo)"
	installMutationFiles(t, root, map[string]string{
		"/public/a.md":      strings.Replace(mutationConcept, "[target](/public/a.md)", body, 1),
		"/public/target.md": strings.ReplaceAll(mutationConcept, "12345678", "87654321"),
		"/public/ref.md":    strings.Replace(strings.ReplaceAll(mutationConcept, "12345678", "11111111"), "[target](/public/a.md)", "[relative](a.md#part) [absolute](/public/a.md) [self](#local)", 1),
	})
	writeLinkAssets(t, root)
	bundle, err := repository.ScanBundle(root, repository.BundleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	paths, err := acceptedBundleAssets(bundle)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := repository.AuditBundleWithAssets(bundle, nil, paths)
	if err != nil || len(audit.Issues) != 1 || audit.Issues[0].Message != "broken link to /public/references/missing.md" {
		t.Fatal(audit, err)
	}
	signals := detectDreamSignals(bundle, "r", "", nil, DreamScannerConfig{OversizedBodyChars: 10000, OversizedTopLevelSections: 100, MaxOversizedCandidates: 1, DuplicateSimilarityThreshold: 1}, paths)
	broken := 0
	for _, signal := range signals {
		if signal.SignalType == "broken_link" {
			broken++
			if signal.Evidence["target_path"] != "/public/references/missing.md" {
				t.Fatal(signal)
			}
		}
	}
	if broken != 1 {
		t.Fatal(signals)
	}
	db, err := derived.Connect(ctx, filepath.Join(t.TempDir(), "derived.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := derived.ContentStore{DB: db}
	if err = store.Rebuild(ctx, root, "r"); err != nil {
		t.Fatal(err)
	}
	var state string
	var target any
	if err = db.QueryRow("SELECT resolution_state,target_id FROM links WHERE raw_target='references/guide.md#x'").Scan(&state, &target); err != nil || state != "asset" || target != nil {
		t.Fatal(state, target, err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM concepts").Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM links WHERE resolution_state='broken'").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	// Asset-only updates re-evaluate manifests even without Markdown changes.
	if err = store.UpdatePaths(ctx, root, "r2", []string{"/.assets/12345678/skill/1.0.0.json"}); err != nil {
		t.Fatal(err)
	}
	m := WorktreeMutator{MaxConceptBytes: 65536}
	changed, err := m.Rename(root, normalizedMutation(t, map[string]any{"kind": "rename", "path": "/public/a.md", "new_path": "/public/deep/moved.md"}), "actor", archivePolicy())
	if err != nil || len(changed) != 4 {
		t.Fatal(changed, err)
	}
	moved, err := repository.ReadBundleEntry(root, "/public/deep/moved.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{"[target](../target.md#p)", "[self](#part)", "[resource](references/guide.md#x)", "[missing](../references/missing.md)"} {
		if !strings.Contains(moved.Document.Body, link) {
			t.Fatal(link, moved.Document.Body)
		}
	}
	ref, err := repository.ReadBundleEntry(root, "/public/ref.md")
	if err != nil || !strings.Contains(ref.Document.Body, "[relative](deep/moved.md#part)") {
		t.Fatal(ref, err)
	}
	if err = store.UpdatePaths(ctx, root, "r3", changed); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT resolution_state FROM links WHERE raw_target='references/guide.md#x'").Scan(&state); err != nil || state != "asset" {
		t.Fatal(state, err)
	}
	// Corrupt manifests fail closed in index, audit and Dream rather than turning
	// every accepted file into a spurious maintenance proposal.
	if err = os.WriteFile(filepath.Join(root, ".assets/12345678/skill/1.0.0.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = store.UpdatePaths(ctx, root, "bad", nil); err == nil {
		t.Fatal("accepted corrupt manifest")
	}
}

func TestRelativeLinksRuntimeManifestFailures(t *testing.T) {
	ctx := context.Background()
	runtime, _, _ := contractRuntime(t, "compact", false, false)
	installMutationFiles(t, runtime.Paths.Repository.CurrentDir, map[string]string{"/public/a.md": mutationConcept})
	root := runtime.Paths.Repository.CurrentDir
	writeLinkAssets(t, root)
	if _, err := runtime.AuditRepository(ctx, nil); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(root, ".assets/12345678/skill/1.0.0.json")
	if err := os.WriteFile(meta, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.AuditRepository(ctx, nil); err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatal("audit manifest failure", err)
	}
	runtime.Dream = DefaultDreamConfig()
	runtime.Dream.Mode = "report_only"
	runtime.Dream.QuietPeriodSeconds = 0
	if _, err := runtime.RunDream(ctx, "report_only", time.Unix(1000000, 0)); err == nil {
		t.Fatal("Dream ignored manifest corruption")
	}
	if err := os.Remove(filepath.Join(root, "private/b.md")); err != nil {
		t.Fatal(err)
	}
	m := WorktreeMutator{MaxConceptBytes: 65536}
	if _, err := m.Rename(root, normalizedMutation(t, map[string]any{"kind": "rename", "path": "/public/a.md", "new_path": "/public/new.md"}), "actor", archivePolicy()); err == nil || !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatal("rename manifest failure", err)
	}
}

func TestRelativeLinksAuditSerializationError(t *testing.T) {
	runtime, _, _ := contractRuntime(t, "compact", false, false)
	// Omitted status retains the source enum default, which parses but cannot be
	// serialized by the compatibility serializer. Audit must propagate that error.
	text := strings.Replace(mutationConcept, "status: active\n", "", 1)
	installMutationFiles(t, runtime.Paths.Repository.CurrentDir, map[string]string{"/public/a.md": text})
	if _, err := runtime.AuditRepository(context.Background(), nil); err == nil {
		t.Fatal("audit serializer error ignored")
	}
}
