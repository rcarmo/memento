package graphdebug

import (
	"context"
	"database/sql"
	"github.com/rcarmo/memento/go/access"
	_ "modernc.org/sqlite"
	"path/filepath"
	"reflect"
	"testing"
)

func proposalDB(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "control.sqlite")
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	_, e = db.Exec(`CREATE TABLE proposals(proposal_id TEXT,status TEXT,patch_json TEXT,author_principal TEXT);INSERT INTO proposals VALUES('1','draft','{"changes":[{"path":"/public/a.md"},{"new_path":"/public/b.md"},{"concept_path":"/public/a.md"}]}','reader'),('2','applied','{"path":"/public/a.md"}','other'),('3','approved','{"path":"/private/x.md"}','reader'),('4','submitted','bad','reader')`)
	if e != nil {
		t.Fatal(e)
	}
	return path
}
func TestProposalPaths(t *testing.T) {
	got := proposalPaths(`{"path":"/b","nested":[{"new_path":"/a"},{"concept_path":1}]}`)
	if !reflect.DeepEqual(got, []string{"/a", "/b"}) {
		t.Fatal(got)
	}
	if got = proposalPaths("bad"); got == nil || len(got) != 0 {
		t.Fatal(got)
	}
}
func TestProposalCounts(t *testing.T) {
	ctx := context.Background()
	s := NewSnapshotService("", "", proposalDB(t))
	counts, e := s.ProposalCounts(ctx, nil)
	if e != nil || counts["/public/a.md"] != (ProposalCount{2, 1}) || counts["/private/x.md"].Pending != 1 {
		t.Fatal(counts, e)
	}
	reader := access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/"}, ProtectedReadPrefixes: []string{"/private/"}}
	counts, e = s.ProposalCounts(ctx, &reader)
	if e != nil || len(counts) != 2 || counts["/public/a.md"].Total != 1 {
		t.Fatal(counts, e)
	}
	curator := reader
	curator.Roles = []string{"curator"}
	counts, e = s.ProposalCounts(ctx, &curator)
	if e != nil || counts["/public/a.md"].Total != 2 {
		t.Fatal(counts, e)
	}
}
func TestProposalCountFailures(t *testing.T) {
	if snapshotFaultRegistered.CompareAndSwap(false, true) {
		sql.Register("graphdebug-fault", snapshotFaultDriver{})
	}
	s := NewSnapshotService("", "", "control")
	s.open = func(context.Context, string) (*sql.DB, error) { return nil, context.Canceled }
	if _, err := s.ProposalCounts(context.Background(), nil); err == nil {
		t.Fatal("open")
	}
	for _, mode := range []string{"query", "scan", "rows"} {
		s.open = faultOpen(t, mode)
		if _, err := s.ProposalCounts(context.Background(), nil); err == nil {
			t.Fatal(mode)
		}
	}
	s.open = faultOpen(t, "ok")
	counts, err := s.ProposalCounts(context.Background(), nil)
	if err != nil || counts["/a"].Pending != 1 {
		t.Fatal(counts, err)
	}
}
func TestProposalVisible(t *testing.T) {
	p := access.EffectivePolicy{Principal: "reader", Roles: []string{"reader"}, ReadPrefixes: []string{"/public/"}}
	if proposalVisible("other", []string{"/public/a"}, &p) || proposalVisible("reader", nil, &p) || proposalVisible("reader", []string{"/private/a"}, &p) || !proposalVisible("reader", []string{"/public/a"}, &p) {
		t.Fatal("visibility")
	}
}
