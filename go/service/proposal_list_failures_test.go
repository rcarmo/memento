package service

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"

	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func TestListFailures(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"refresh", "main", "query", "access", "selected-refresh", "cursor-random", "assets", "summary"} {
		t.Run(stage, func(t *testing.T) {
			c, actor := rebaseTest(t)
			actor.Policy.Roles = []string{"proposer", "curator"}
			if _, err := c.Queue.Proposals.DB.Exec("UPDATE proposals SET base_revision='main'"); err != nil {
				t.Fatal(err)
			}
			if stage == "cursor-random" {
				if _, err := c.Queue.Proposals.Create(ctx, control.ProposalRequest{ProposalID: "second", AuthorPrincipal: actor.Policy.Principal, BaseRevision: "main", Intent: "next", Patch: map[string]any{"changes": []any{}}}); err != nil {
					t.Fatal(err)
				}
				codec := testCursor()
				c.cursor = &codec
				c.Random = bytes.NewReader(nil)
			}
			calls := 0
			repo := fakeRepo(nil)
			repo.main = func(repository.GitRepositoryPaths) (string, error) {
				calls++
				if stage == "refresh" || (stage == "main" && calls == 2) || (stage == "selected-refresh" && calls == 3) || (stage == "summary" && calls == 4) {
					return "", io.ErrClosedPipe
				}
				if calls == 2 {
					switch stage {
					case "query":
						c.Queue.Proposals.DB.Close()
					case "access":
						if _, err := c.Queue.Proposals.DB.Exec(`UPDATE proposals SET patch_json='{"changes":null}'`); err != nil {
							t.Fatal(err)
						}
					case "assets":
						if _, err := c.Queue.Proposals.DB.Exec("DROP TABLE proposal_assets"); err != nil {
							t.Fatal(err)
						}
					}
				}
				return "main", nil
			}
			if _, err := c.listProposals(ctx, actor, nil, 1, nil, repo); err == nil {
				t.Fatal("missing failure", stage, calls)
			}
		})
	}
}
func TestListPublicKeyLifetime(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer", "curator"}
	if _, err := c.Queue.Proposals.Create(ctx, control.ProposalRequest{ProposalID: "second", AuthorPrincipal: actor.Policy.Principal, BaseRevision: base, Intent: "second", Patch: map[string]any{"changes": []any{}}}); err != nil {
		t.Fatal(err)
	}
	c.Random = bytes.NewReader(make([]byte, 32+16))
	first, err := c.ListProposals(ctx, actor, nil, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	token := first["next_cursor"].(string)
	next, err := c.ListProposals(ctx, actor, nil, 1, &token)
	if err != nil || next["next_cursor"] != nil {
		t.Fatal(next, err)
	}
	restarted := &ProposalControls{Queue: c.Queue}
	if _, err := restarted.ListProposals(ctx, actor, nil, 1, &token); err == nil {
		t.Fatal("cursor survived new service key")
	}
	// Concurrent initialisation generates a single session key, with no races.
	concurrent := &ProposalControls{Queue: c.Queue}
	var wg sync.WaitGroup
	keys := make(chan proposalCursor, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key, err := concurrent.cursorCipher()
			if err != nil {
				t.Error(err)
				return
			}
			keys <- key
		}()
	}
	wg.Wait()
	close(keys)
	var firstKey *proposalCursor
	for key := range keys {
		if firstKey == nil {
			copy := key
			firstKey = &copy
		} else if key != *firstKey {
			t.Fatal("multiple keys")
		}
	}
	concurrent.Random = nil
	encoded, err := concurrent.encodeProposalCursor(proposalListScope(actor.Policy, nil, base), "proposal")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = concurrent.decodeProposalCursor(encoded, proposalListScope(actor.Policy, nil, base)); err != nil {
		t.Fatal(err)
	}
}
