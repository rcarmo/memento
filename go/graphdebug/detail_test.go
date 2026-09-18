package graphdebug

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/access"
)

func TestDetailFixture(t *testing.T) {
	s, policy := freshFixtureService(t)
	got, err := s.Detail(context.Background(), "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", policy, 4000, 12000, 500)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Detail Detail `json:"detail"`
	}
	raw, _ := os.ReadFile("../testdata/parity/graph-snapshot-foundation.json")
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(fixture.Detail)
	if string(a) != string(b) {
		t.Fatal(string(a), string(b))
	}
}
func TestAssetFailures(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".assets", "id", "docs")
	_ = os.MkdirAll(dir, 0700)
	file := filepath.Join(dir, "one.json")
	_ = os.WriteFile(file, []byte(`{}`), 0600)
	originalRead, originalStat := readAssetFile, statAsset
	defer func() { readAssetFile, statAsset = originalRead, originalStat }()
	readAssetFile = func(string) ([]byte, error) { return nil, context.Canceled }
	if got := NewSnapshotService(root, "", "").Assets("id", 10); len(got) != 0 {
		t.Fatal(got)
	}
	readAssetFile = originalRead
	statAsset = func(string) (os.FileInfo, error) { return nil, context.Canceled }
	if got := NewSnapshotService(root, "", "").Assets("id", 10); len(got) != 0 {
		t.Fatal(got)
	}
}
func TestAssetAndProposalBranches(t *testing.T) {
	root := t.TempDir()
	s := NewSnapshotService(root, "", fixtureControlDB(t))
	dir := filepath.Join(root, ".assets", "id", "docs")
	_ = os.MkdirAll(dir, 0700)
	_ = os.WriteFile(filepath.Join(dir, "bad.json"), []byte("bad"), 0600)
	_ = os.WriteFile(filepath.Join(dir, "fallback.json"), []byte(`{}`), 0600)
	_ = os.Mkdir(filepath.Join(dir, "unreadable.json"), 0700)
	assets := s.Assets("id", 1)
	if len(assets) != 1 || assets[0].AssetKind != "docs" || assets[0].Version != "fallback" || assets[0].SourceProposalID != nil {
		t.Fatal(assets)
	}
	policy := access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}}
	proposals, err := s.Proposals(context.Background(), "/a.md", &policy, 1)
	if err != nil || len(proposals) != 1 || proposals[0].AppliedRevision != nil {
		t.Fatal(proposals, err)
	}
	other := policy
	other.Principal = "other"
	hidden, err := s.Proposals(context.Background(), "/a.md", &other, 10)
	if err != nil || len(hidden) != 0 {
		t.Fatal(hidden, err)
	}
	if containsPath([]string{"/a"}, "/b") {
		t.Fatal("contains")
	}
	missing := NewSnapshotService("", "", filepath.Join(t.TempDir(), "missing"))
	if _, err = missing.Proposals(context.Background(), "/", nil, 1); err == nil {
		t.Fatal("open")
	}
}
func TestDetailGuards(t *testing.T) {
	s, policy := freshFixtureService(t)
	if _, err := s.Detail(context.Background(), "missing", policy, 1, 10, 10); err == nil {
		t.Fatal("unknown")
	}
	db, _ := sql.Open("sqlite", s.DerivedDBPath)
	_, _ = db.Exec("UPDATE concepts SET path='/missing.md' WHERE id='5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d'")
	db.Close()
	if _, err := s.Detail(context.Background(), "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", policy, 1, 10, 10); err == nil {
		t.Fatal("file")
	}
}
