package service

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/control"
)

// The immutable Python proposal-list fixture describes the pre-options contract:
// oldest-first all-status pages (including empty pages for hidden records).
// Explicit tests below cover the revised contract rather than rewriting that oracle.
func listOptionsFixture(t *testing.T) (*ProposalControls, ProposalActor) {
	t.Helper()
	c, actor := rebaseTest(t)
	c.Random = nil
	actor.Policy.Roles = []string{"proposer", "curator"}
	actor.Policy.ReadPrefixes = []string{"/"}
	actor.Policy.WritePrefixes = []string{"/"}
	if _, err := c.Queue.Proposals.DB.Exec("DELETE FROM proposals"); err != nil {
		t.Fatal(err)
	}
	statuses := []control.ProposalStatus{control.Submitted, control.Approved, control.Applied, control.NeedsRebase, control.Conflicted, control.Draft, control.Rejected, control.Expired}
	for i, status := range statuses {
		id := string(rune('a' + i))
		_, err := c.Queue.Proposals.Create(context.Background(), control.ProposalRequest{ProposalID: id, AuthorPrincipal: actor.Policy.Principal, BaseRevision: "main", Intent: id, Patch: map[string]any{"changes": []any{}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = c.Queue.Proposals.DB.Exec("UPDATE proposals SET status=?,created_at=?,updated_at=? WHERE proposal_id=?", status, "2026-09-10T00:00:00Z", "2026-09-11T00:00:00Z", id); err != nil {
			t.Fatal(err)
		}
	}
	return c, actor
}
func listedProposalIDs(result map[string]any) []string {
	ids := []string{}
	for _, raw := range result["proposals"].([]any) {
		ids = append(ids, raw.(map[string]any)["proposal_id"].(string))
	}
	return ids
}
func TestProposalListOptions(t *testing.T) {
	c, actor := listOptionsFixture(t)
	ctx := context.Background()
	for _, tt := range []struct {
		name   string
		status *string
		opts   ProposalListOptions
		want   []string
	}{
		{"default", nil, ProposalListOptions{}, []string{"b", "a"}},
		{"all", listText("all"), ProposalListOptions{}, []string{"h", "g", "f", "e", "d", "c", "b", "a"}},
		{"unresolved", listText("unresolved"), ProposalListOptions{}, []string{"f", "e", "d", "b", "a"}},
		{"applied", listText("applied"), ProposalListOptions{}, []string{"c"}},
		{"asc", nil, ProposalListOptions{SortOrder: "asc"}, []string{"a", "b"}},
		{"exclude", listText("all"), ProposalListOptions{ExcludeStatuses: []string{"applied", "approved", "applied"}}, []string{"h", "g", "f", "e", "d", "a"}},
		{"id", nil, ProposalListOptions{SortBy: "proposal_id"}, []string{"b", "a"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.listProposals(ctx, actor, tt.status, 20, nil, fakeRepo(nil), tt.opts)
			if err != nil || !reflect.DeepEqual(listedProposalIDs(got), tt.want) {
				t.Fatal(got, err, tt.want)
			}
		})
	}
	for _, field := range []string{"created_at", "updated_at", "proposal_id"} {
		for _, order := range []string{"asc", "desc"} {
			t.Run(field+order, func(t *testing.T) {
				opts := ProposalListOptions{SortBy: field, SortOrder: order}
				first, err := c.listProposals(ctx, actor, nil, 1, nil, fakeRepo(nil), opts)
				if err != nil {
					t.Fatal(err)
				}
				token := first["next_cursor"].(string)
				// Updating anchor ordering data must not change the tuple sealed in the cursor.
				second, err := c.listProposals(ctx, actor, nil, 1, &token, fakeRepo(nil), opts)
				if err != nil || second["next_cursor"] != nil {
					t.Fatal(second, err)
				}
				if listedProposalIDs(first)[0] == listedProposalIDs(second)[0] {
					t.Fatal("duplicate")
				}
				changed := opts
				changed.SortOrder = map[string]string{"asc": "desc", "desc": "asc"}[order]
				if _, err = c.listProposals(ctx, actor, nil, 1, &token, fakeRepo(nil), changed); err == nil {
					t.Fatal("ordering cursor accepted")
				}
				changed = opts
				changed.ExcludeStatuses = []string{"applied"}
				if _, err = c.listProposals(ctx, actor, nil, 1, &token, fakeRepo(nil), changed); err == nil {
					t.Fatal("filter cursor accepted")
				}
			})
		}
	}
	for _, opts := range []ProposalListOptions{{SortBy: "created_at;DROP TABLE proposals"}, {SortOrder: "sideways"}, {ExcludeStatuses: []string{"unknown"}}} {
		if _, err := c.listProposals(ctx, actor, nil, 2, nil, fakeRepo(nil), opts); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
	for _, status := range []string{"unknown", "", "accepted"} {
		if _, err := c.listProposals(ctx, actor, &status, 2, nil, fakeRepo(nil)); err == nil {
			t.Fatal("invalid status")
		}
	}
	for _, n := range []int{0, 201} {
		if _, err := c.listProposals(ctx, actor, nil, n, nil, fakeRepo(nil)); err == nil {
			t.Fatal("invalid limit")
		}
	}
}
func listText(s string) *string { return &s }

func TestProposalListVisibilityAndCursorAnchor(t *testing.T) {
	c, actor := listOptionsFixture(t)
	ctx := context.Background()
	// Hide newest pending record and leave a full first visible page with no cursor.
	patch, _ := json.Marshal(map[string]any{"changes": []any{map[string]any{"kind": "patch", "path": "/private/x.md", "body": "secret"}}})
	if _, err := c.Queue.Proposals.DB.Exec("UPDATE proposals SET patch_json=? WHERE proposal_id='b'", string(patch)); err != nil {
		t.Fatal(err)
	}
	actor.Policy.ReadPrefixes = []string{"/public/"}
	actor.Policy.WritePrefixes = []string{"/public/"}
	got, err := c.listProposals(ctx, actor, nil, 1, nil, fakeRepo(nil))
	if err != nil || !reflect.DeepEqual(listedProposalIDs(got), []string{"a"}) || got["next_cursor"] != nil {
		t.Fatal(got, err)
	}
	actor.Policy.ReadPrefixes = []string{"/"}
	actor.Policy.WritePrefixes = []string{"/"}
	got, err = c.listProposals(ctx, actor, listText("all"), 1, nil, fakeRepo(nil))
	if err != nil {
		t.Fatal(err)
	}
	token := got["next_cursor"].(string)
	if _, err = c.Queue.Proposals.DB.Exec("DELETE FROM proposals WHERE proposal_id='h'"); err != nil {
		t.Fatal(err)
	}
	got, err = c.listProposals(ctx, actor, listText("all"), 1, &token, fakeRepo(nil))
	if err != nil || !reflect.DeepEqual(listedProposalIDs(got), []string{"g"}) {
		t.Fatal(got, err)
	}
}

func TestProposalListOptionBoundaryFailures(t *testing.T) {
	c, actor := listOptionsFixture(t)
	ctx := context.Background()
	filter, opts, err := normalizeProposalList(nil, ProposalListOptions{ExcludeStatuses: []string{"stale"}})
	if err != nil || filter != "pending" || !reflect.DeepEqual(opts.ExcludeStatuses, []string{"needs_rebase"}) {
		t.Fatal(filter, opts, err)
	}
	scope := proposalListScope(actor.Policy, &filter, "main")
	scope["sort_by"] = opts.SortBy
	scope["sort_order"] = opts.SortOrder
	scope["exclude_statuses"] = opts.ExcludeStatuses
	token, err := c.encodeProposalCursor(scope, `{"id":""}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.listProposals(ctx, actor, nil, 1, &token, fakeRepo(nil), opts); err == nil {
		t.Fatal("invalid cursor payload")
	}
	if _, _, err = runProposalTool(ctx, c, actor, "memory_proposal_list", map[string]any{"status": "all", "limit": json.Number("20"), "exclude_statuses": []any{true}}); err == nil {
		t.Fatal("invalid exclusion type")
	}
	// Valid exclude items reach normal role authorization, before repository I/O.
	actor.Policy.Roles = nil
	if _, _, err = runProposalTool(ctx, c, actor, "memory_proposal_list", map[string]any{"status": "all", "limit": json.Number("20"), "exclude_statuses": []any{"applied"}}); err == nil {
		t.Fatal("missing role accepted")
	}
}

func TestProposalListDistinctTimestampsAndEquivalentFilters(t *testing.T) {
	c, actor := listOptionsFixture(t)
	ctx := context.Background()
	if _, err := c.Queue.Proposals.DB.Exec("UPDATE proposals SET created_at='2026-09-12T00:00:00Z',updated_at='2026-09-10T00:00:00Z' WHERE proposal_id='a'"); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		field, order string
		want         []string
	}{
		{"created_at", "desc", []string{"a", "b"}}, {"created_at", "asc", []string{"b", "a"}},
		{"updated_at", "desc", []string{"b", "a"}}, {"updated_at", "asc", []string{"a", "b"}},
	} {
		got, err := c.listProposals(ctx, actor, nil, 20, nil, fakeRepo(nil), ProposalListOptions{SortBy: tt.field, SortOrder: tt.order})
		if err != nil || !reflect.DeepEqual(listedProposalIDs(got), tt.want) {
			t.Fatal(tt, got, err)
		}
	}
	opts := ProposalListOptions{ExcludeStatuses: []string{"rejected", "applied", "applied"}}
	first, err := c.listProposals(ctx, actor, nil, 1, nil, fakeRepo(nil), opts)
	if err != nil {
		t.Fatal(err)
	}
	token := first["next_cursor"].(string)
	// The sealed tuple must survive a changed creation value on the anchor.
	if _, err = c.Queue.Proposals.DB.Exec("UPDATE proposals SET created_at='2026-09-01T00:00:00Z' WHERE proposal_id='a'"); err != nil {
		t.Fatal(err)
	}
	opts.ExcludeStatuses = []string{"applied", "rejected"}
	second, err := c.listProposals(ctx, actor, nil, 1, &token, fakeRepo(nil), opts)
	if err != nil || !reflect.DeepEqual(listedProposalIDs(second), []string{"b"}) {
		t.Fatal(second, err)
	}
}

func TestMCPContractProposalListOptions(t *testing.T) {
	runtime, server, _ := contractRuntime(t, "compact", false, false)
	ctx := context.Background()
	// Only synthetic proposal rows in the disposable runtime's control database.
	p := control.Proposals{DB: runtime.DB}
	revision := contractTool(t, server, "actor", "memory_status", nil)["repo_revision"].(string)
	for i, status := range []control.ProposalStatus{control.Submitted, control.Approved, control.Applied, control.NeedsRebase, control.Conflicted} {
		id := string(rune('a' + i))
		if _, err := p.Create(ctx, control.ProposalRequest{ProposalID: id, AuthorPrincipal: "actor", BaseRevision: revision, Intent: "fixture", Patch: map[string]any{"changes": []any{}}}); err != nil {
			t.Fatal(err)
		}
		if _, err := runtime.DB.Exec("UPDATE proposals SET status=?,created_at='2026-01-01T00:00:00Z' WHERE proposal_id=?", status, id); err != nil {
			t.Fatal(err)
		}
	}
	status := contractTool(t, server, "actor", "memory_status", nil)["data"].(map[string]any)
	if status["proposal_backlog"] != float64(2) || status["proposal_unresolved"] != float64(4) {
		t.Fatal(status)
	}
	for _, tt := range []struct {
		args map[string]any
		want []string
	}{
		{map[string]any{}, []string{"b", "a"}},
		{map[string]any{"status": "all", "exclude_statuses": []any{"applied", "needs_rebase", "conflicted", "approved"}, "sort_by": "updated_at", "sort_order": "asc"}, []string{"a"}},
	} {
		body := contractTool(t, server, "actor", "memory_execute", map[string]any{"operations": []any{map[string]any{"op": "proposal_list", "args": tt.args}}})
		data := body["data"].(map[string]any)["returns"].(map[string]any)["result"].(map[string]any)
		if !reflect.DeepEqual(listedProposalIDs(data), tt.want) {
			t.Fatal(data, tt.want)
		}
	}
}
