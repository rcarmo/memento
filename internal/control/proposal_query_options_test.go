package control

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestProposalQuerySortValidation(t *testing.T) {
	p := proposalStore(t)
	ctx := context.Background()
	for _, q := range []ProposalQuery{{SortBy: "invalid"}, {SortOrder: "invalid"}, {ExcludeStatuses: []ProposalStatus{"unknown"}}} {
		if _, err := p.List(ctx, q); err == nil {
			t.Fatal("accepted invalid query", q)
		}
	}
}

func BenchmarkProposalListPending(b *testing.B) {
	db, err := Connect(context.Background(), filepath.Join(b.TempDir(), "control.sqlite"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err = Migrate(ctx, db); err != nil {
		b.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO proposals(proposal_id,author_principal,base_revision,intent,patch_json,patch_hash,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		status := Applied
		if i%10 == 0 {
			status = Submitted
		}
		_, err = stmt.Exec(fmt.Sprintf("p%04d", i), "author", "main", "item", `{"changes":[]}`, "hash", status, time.Unix(int64(i), 0).UTC().Format(time.RFC3339), "2026-01-01T00:00:00Z")
		if err != nil {
			b.Fatal(err)
		}
	}
	_ = stmt.Close()
	if err = tx.Commit(); err != nil {
		b.Fatal(err)
	}
	benchmarkPendingQuery(b, db)
}
func benchmarkPendingQuery(b *testing.B, db *sql.DB) {
	p := Proposals{DB: db}
	limit := 21
	q := ProposalQuery{Pending: true, SortBy: "created_at", SortOrder: "desc", Limit: &limit}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		records, err := p.List(context.Background(), q)
		if err != nil || len(records) != limit {
			b.Fatal(len(records), err)
		}
	}
}
