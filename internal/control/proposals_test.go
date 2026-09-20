package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

func ptr[T any](v T) *T { return &v }
func proposalStore(t *testing.T) Proposals {
	ops, _ := newOperations(t)
	return Proposals(ops)
}
func proposalInput(id string) ProposalRequest {
	return ProposalRequest{ProposalID: id, AuthorPrincipal: "alice", BaseRevision: "base", Intent: "synthetic", Rationale: ptr("because"), Patch: map[string]any{"changes": []any{}, "z": "日本", "n": 1.0}, Assets: []ProposalAssetInput{{AssetID: "asset-one", ConceptPath: "/public/a.md", AssetKind: "skill", Version: "1.0", MediaType: "application/zip", SHA256: "synthetic", BlobBytes: []byte("synthetic\x00\xff"), ManifestJSON: `{ "name": "日本" }`}}}
}
func TestProposalReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/control-proposals.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Created []ProposalRecord
		Cleared ProposalRecord
		Queries []struct {
			Status                *ProposalStatus
			Unresolved, Effective bool
			Expected              []ProposalRecord
			Error                 string
			Input                 map[string]any
		}
		Assets []ProposalAssetRecord
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	p := proposalStore(t)
	ctx := context.Background()
	statuses := []ProposalStatus{Draft, Submitted, Approved, Rejected, Applied, Stale, NeedsRebase, Conflicted, Expired}
	for i, status := range statuses {
		input := proposalInput(fmt.Sprint("proposal-", i))
		if i%2 != 0 {
			input.AuthorPrincipal = "bob"
		}
		if i%3 == 0 {
			input.ExpiresInDays = ptr(-1)
		}
		record, err := p.Create(ctx, input)
		if err != nil || record.Status != Submitted {
			t.Fatal(record, err)
		}
		record, err = p.UpdateStatus(ctx, input.ProposalID, ProposalStatusUpdate{Status: status, ReviewedBy: ptr("reviewer"), ReviewComment: ptr("comment")})
		if err != nil || !reflect.DeepEqual(record, fixture.Created[i]) {
			t.Fatal(record, fixture.Created[i], err)
		}
	}
	cleared, err := p.UpdateStatus(ctx, "proposal-0", ProposalStatusUpdate{Status: Submitted})
	if err != nil || !reflect.DeepEqual(cleared, fixture.Cleared) {
		t.Fatal(cleared, fixture.Cleared, err)
	}
	for i, c := range fixture.Queries {
		q := ProposalQuery{Status: c.Status, Unresolved: c.Unresolved}
		if c.Effective {
			q.CurrentRevision = ptr("changed")
			q.Now = ptr("2026-10-19T00:00:00Z")
		}
		for key, value := range c.Input {
			switch key {
			case "author_principal":
				q.AuthorPrincipal = ptr(value.(string))
			case "cursor":
				q.Cursor = ptr(value.(string))
			case "limit":
				q.Limit = ptr(int(value.(float64)))
			case "status":
				q.Status = ptr(ProposalStatus(value.(string)))
			case "now":
				q.Now = ptr(value.(string))
			}
		}
		got, err := p.List(ctx, q)
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(i, err, c.Error)
			}
		} else if err != nil || !reflect.DeepEqual(got, c.Expected) {
			t.Fatal(i, got, c.Expected, err)
		}
	}
	assets, err := p.ListAssets(ctx, ProposalAssetQuery{})
	if err != nil || !reflect.DeepEqual(assets, fixture.Assets) {
		t.Fatal(assets, fixture.Assets, err)
	}
	got, err := p.GetAsset(ctx, "proposal-0", "asset-one")
	if err != nil || !reflect.DeepEqual(got, assets[0]) {
		t.Fatal(got, err)
	}
	filtered, err := p.ListAssets(ctx, ProposalAssetQuery{ProposalID: ptr("proposal-0"), ConceptPath: ptr("/public/a.md"), AssetKind: ptr("skill")})
	if err != nil || len(filtered) != 1 {
		t.Fatal(filtered, err)
	}
	if patch, err := cleared.Patch(); err != nil || patch["z"] != "日本" {
		t.Fatal(patch, err)
	}
	if manifest, err := got.Manifest(); err != nil || manifest["name"] != "日本" {
		t.Fatal(manifest, err)
	}
}
func TestProposalTransactionsAndErrors(t *testing.T) {
	p := proposalStore(t)
	ctx := context.Background()
	input := proposalInput("rollback")
	input.Assets = append(input.Assets, input.Assets[0])
	if _, err := p.Create(ctx, input); err == nil {
		t.Fatal("duplicate asset")
	}
	if _, err := p.Get(ctx, "rollback"); err == nil {
		t.Fatal("partial proposal persisted")
	}
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	input = proposalInput("outer")
	input.Assets = nil
	if _, err = p.CreateInTx(ctx, tx, input); err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback()
	if _, err = p.Get(ctx, "outer"); err == nil {
		t.Fatal("outer rollback failed")
	}
	input = proposalInput("one")
	record, err := p.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Create(ctx, input); err == nil {
		t.Fatal("duplicate proposal")
	}
	if _, err = p.UpdateStatus(ctx, "missing", ProposalStatusUpdate{Status: Approved}); err == nil || err.Error() != "unknown proposal: missing" {
		t.Fatal(err)
	}
	if _, err = p.GetAsset(ctx, "one", "missing"); err == nil || err.Error() != "unknown proposal asset: one/missing" {
		t.Fatal(err)
	}
	if _, err = p.UpdateStatus(ctx, "one", ProposalStatusUpdate{Status: "invalid"}); err == nil {
		t.Fatal("invalid status")
	}
	for _, bad := range []string{"{", "[]"} {
		record.PatchJSON = bad
		if _, err = record.Patch(); err == nil {
			t.Fatal(bad)
		}
		asset := ProposalAssetRecord{ProposalAssetInput: ProposalAssetInput{ManifestJSON: bad}}
		if _, err = asset.Manifest(); err == nil {
			t.Fatal(bad)
		}
	}
	input = proposalInput("bad")
	input.Patch["bad"] = make(chan int)
	if _, err = p.Create(ctx, input); err == nil {
		t.Fatal("JSON error")
	}
	for _, days := range []int{4000000, -4000000, 3652058, -3652058} {
		input = proposalInput("bad")
		input.ExpiresInDays = &days
		if _, err = p.Create(ctx, input); err == nil {
			t.Fatal("overflow", days)
		}
	}
	if (Proposals{}).now().IsZero() {
		t.Fatal("clock")
	}
	_, _ = p.DB.Exec("UPDATE proposals SET status='invalid' WHERE proposal_id='one'")
	if _, err = p.Get(ctx, "one"); err == nil {
		t.Fatal("invalid row")
	}
	for _, scan := range []func(operationRows) error{func(rows operationRows) error { _, err := scanProposals(rows); return err }, func(rows operationRows) error { _, err := scanProposalAssets(rows); return err }} {
		if err := scan(&failedRows{fail: "scan"}); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
	}
	p.DB.Close()
	if _, err = p.Create(ctx, input); err == nil {
		t.Fatal("closed create")
	}
	if _, err = p.List(ctx, ProposalQuery{Cursor: ptr("one")}); err == nil {
		t.Fatal("closed cursor")
	}
	if _, err = p.List(ctx, ProposalQuery{}); err == nil {
		t.Fatal("closed list")
	}
	if _, err = p.ListAssets(ctx, ProposalAssetQuery{}); err == nil {
		t.Fatal("closed assets")
	}
}
func TestProposalStatusFieldsAndFailures(t *testing.T) {
	p := proposalStore(t)
	ctx := context.Background()
	input := proposalInput("one")
	if _, err := p.Create(ctx, input); err != nil {
		t.Fatal(err)
	}
	operations := Operations{DB: p.DB}
	if _, err := operations.Create(ctx, OperationRequest{OpID: "applied", Principal: "p", IdempotencyKey: "k", ToolName: "write", RequestJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpdateStatus(ctx, "one", ProposalStatusUpdate{Status: Applied, AppliedOperationID: ptr("applied"), AppliedRevision: ptr("revision")}); err != nil {
		t.Fatal(err)
	}
	record, err := p.UpdateStatus(ctx, "one", ProposalStatusUpdate{Status: Submitted})
	if err != nil || record.AppliedOperationID != nil || record.AppliedRevision != nil {
		t.Fatal(record, err)
	}
	if _, err = p.UpdateStatus(ctx, "one", ProposalStatusUpdate{Status: Applied, AppliedOperationID: ptr("nonexistent")}); err == nil {
		t.Fatal("foreign key")
	}
}

func TestProposalCommitFailureAndEmptyAsset(t *testing.T) {
	p := proposalStore(t)
	ctx := context.Background()
	input := proposalInput("empty-blob")
	input.Assets[0].BlobBytes = nil
	record, err := p.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := p.GetAsset(ctx, record.ProposalID, "asset-one")
	if err != nil || len(asset.BlobBytes) != 0 {
		t.Fatal(asset, err)
	}
	// Deferred FK failure occurs at Commit after proposal and asset insertion.
	_, err = p.DB.Exec(`CREATE TABLE deferred_probe(proposal_id TEXT REFERENCES proposals(proposal_id) DEFERRABLE INITIALLY DEFERRED); CREATE TRIGGER fail_commit AFTER INSERT ON proposals BEGIN INSERT INTO deferred_probe VALUES('missing'); END`)
	if err != nil {
		t.Fatal(err)
	}
	input = proposalInput("commit-failure")
	if _, err = p.Create(ctx, input); err == nil {
		t.Fatal("deferred FK ignored")
	}
	if _, err = p.Get(ctx, "commit-failure"); err == nil {
		t.Fatal("partial commit")
	}
}
