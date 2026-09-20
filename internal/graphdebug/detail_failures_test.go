package graphdebug

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestDetailFailureStages(t *testing.T) {
	for _, failure := range []int{1, 3, 5, 6, 7, 8} {
		s, policy := freshFixtureService(t)
		base := s.open
		calls := 0
		s.open = func(ctx context.Context, path string) (*sql.DB, error) {
			calls++
			if calls == failure {
				return nil, context.Canceled
			}
			return base(ctx, path)
		}
		if _, err := s.Detail(context.Background(), "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", policy, 1, 10, 10); err == nil {
			t.Fatal(failure, calls)
		}
	}
}
func TestProposalSummaryFailures(t *testing.T) {
	if snapshotFaultRegistered.CompareAndSwap(false, true) {
		sql.Register("graphdebug-fault", snapshotFaultDriver{})
	}
	s := NewSnapshotService("", "", "control")
	for _, mode := range []string{"query", "scan", "rows"} {
		s.open = faultOpen(t, mode)
		if _, err := s.Proposals(context.Background(), "/a", nil, 10); err == nil {
			t.Fatal(mode)
		}
	}
	s.open = faultOpen(t, "ok")
	got, err := s.Proposals(context.Background(), "/a", nil, 1)
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
}
func TestDetailPreviewAndParse(t *testing.T) {
	s, policy := freshFixtureService(t)
	detail, err := s.Detail(context.Background(), "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", policy, 2, 10, 10)
	if err != nil || !detail.PreviewTruncated || len([]rune(detail.Preview)) != 2 {
		t.Fatal(detail, err)
	}
	_ = os.WriteFile(s.RepositoryRoot+"/a.md", []byte("bad"), 0600)
	if _, err = s.Detail(context.Background(), "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d", policy, 10, 10, 10); err == nil {
		t.Fatal("parse")
	}
}
