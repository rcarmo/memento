package service

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/go/repository"
)

func TestReviseFailuresAndPublic(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"missing", "refresh", "main", "conflicts", "changes"} {
		t.Run(stage, func(t *testing.T) {
			c, actor := rebaseTest(t)
			actor.Policy.Roles = []string{"curator"}
			id := "proposal"
			if stage == "missing" {
				id = "missing"
			}
			repo := fakeRepo(nil)
			calls := 0
			repo.main = func(repository.GitRepositoryPaths) (string, error) {
				calls++
				if stage == "refresh" || (stage == "main" && calls == 2) || (stage == "conflicts" && calls == 3) {
					return "", io.ErrClosedPipe
				}
				return "main", nil
			}
			if stage == "changes" {
				if _, err := c.Queue.Proposals.DB.Exec(`CREATE TRIGGER corrupt AFTER UPDATE ON proposals BEGIN UPDATE proposals SET patch_json='{"changes":null}' WHERE proposal_id=NEW.proposal_id; END`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := c.revise(ctx, actor, id, []int{0}, "main", nil, nil, repo); err == nil {
				t.Fatal("missing failure")
			}
		})
	}
	c, actor, base := realApplyTest(t)
	actor.Policy.Roles = []string{"curator"}
	if _, err := c.Queue.Proposals.DB.Exec("UPDATE proposals SET status='needs_rebase'"); err != nil {
		t.Fatal(err)
	}
	got, err := c.Revise(ctx, actor, "proposal", []int{0, 0}, base, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload := got["proposal"].(map[string]any)
	if payload["author_principal"] != actor.Policy.Principal || payload["proposal_id"] == "proposal" {
		t.Fatal(got)
	}
	source, err := c.Queue.Proposals.Get(ctx, "proposal")
	if err != nil || source.ReviewedBy == nil || *source.ReviewedBy != "reviewer" {
		t.Fatal(source, err)
	}
	denied := actor
	denied.Policy.WritePrefixes = nil
	if _, err := c.createRevised(ctx, denied, "proposal", base, []ProposalChange{{"kind": "patch", "path": "/a.md"}}, []int{0}, nil, nil, fakeRepo(nil)); err == nil {
		t.Fatal("write auth")
	}
	c.Random = bytes.NewReader(nil)
	if _, err := c.createRevised(ctx, actor, "proposal", base, nil, nil, nil, nil, fakeRepo(nil)); err == nil {
		t.Fatal("random id")
	}
	// Asset-only changes with no same-path body edits are independently selectable.
	changes := []ProposalChange{{"kind": "patch", "path": "/unrelated.md", "body": "x"}, {"kind": "attach_asset_pack", "path": "/a.md", "asset_id": "a"}}
	if err := validateSelectedAssetPairs(changes, []int{1}); err != nil {
		t.Fatal(err)
	}
	changes = append(changes, ProposalChange{"kind": "patch", "path": "/a.md", "body": "x"})
	if err := validateSelectedAssetPairs(changes, []int{0}); err != nil {
		t.Fatal("unselected pair", err)
	}
	if _, err := selectedIndexes([]int{3, 2, 3}, 4); err != nil {
		t.Fatal(err)
	}
	if got, err := selectedIndexes([]int{3, 2, 3}, 4); err != nil || !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatal(got, err)
	}
}
