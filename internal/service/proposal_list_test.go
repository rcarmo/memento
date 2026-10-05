package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testCursor() proposalCursor {
	var p proposalCursor
	for i := range p.key {
		p.key[i] = byte(i)
	}
	return p
}
func TestFernetReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-fernet.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Data, Token string }
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	p := testCursor()
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i)
	}
	for _, c := range cases {
		data, err := base64.StdEncoding.DecodeString(c.Data)
		if err != nil {
			t.Fatal(err)
		}
		token, err := p.encrypt(data, time.Unix(1789689600, 0), bytes.NewReader(iv))
		if err != nil || token != c.Token {
			t.Fatal(token, c.Token, err)
		}
		got, err := p.decrypt(c.Token)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal(got, data, err)
		}
		for _, mutated := range []string{token[:2] + "!\n" + token[2:], token + "=="} {
			if got, err = p.decrypt(mutated); err != nil || !bytes.Equal(got, data) {
				t.Fatal(mutated, err)
			}
		}
	}
}

func TestFernetDecryptReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-fernet-decrypt.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Token, Expected string
		Error           bool
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	codec := testCursor()
	for _, c := range cases {
		got, err := codec.decrypt(c.Token)
		if c.Error {
			if err == nil {
				t.Fatal("accepted", c.Token)
			}
		} else if err != nil || base64.StdEncoding.EncodeToString(got) != c.Expected {
			t.Fatal(got, err, c)
		}
	}
}

// Replay the original records/visibility/errors, explicitly asking for historical
// all-status ascending order. The new API fills visible pages, so empty legacy
// pages are collapsed; encrypted cursor payloads now seal a sort tuple.
func TestProposalListReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-list.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario string
		Before   []map[string]any
		Steps    []struct {
			Policy         access.EffectivePolicy
			Revision       string
			Status, Cursor *string
			Limit          int
			Response       map[string]any
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Scenario, func(t *testing.T) {
			q, _ := queueTest(t)
			restoreSubmitRows(t, q.Proposals.DB, map[string][]map[string]any{"proposals": tc.Before})
			c := ProposalControls{Queue: q}
			first := tc.Steps[0]
			status := first.Status
			if status == nil {
				status = listText("all")
			}
			repo := fakeRepo(nil)
			repo.main = func(repository.GitRepositoryPaths) (string, error) { return first.Revision, nil }
			got, err := c.listProposals(context.Background(), ProposalActor{Policy: first.Policy}, status, first.Limit, first.Cursor, repo, ProposalListOptions{SortOrder: "asc"})
			if first.Response["status"] == "error" {
				if err == nil || err.Error() != first.Response["message"] {
					t.Fatal(err, first.Response)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(tc.Scenario, "scope-") {
				token, ok := got["next_cursor"].(string)
				if !ok {
					t.Fatal("missing visible continuation")
				}
				second := tc.Steps[1]
				filter := second.Status
				if filter == nil {
					filter = listText("all")
				}
				repo.main = func(repository.GitRepositoryPaths) (string, error) { return second.Revision, nil }
				if _, err = c.listProposals(context.Background(), ProposalActor{Policy: second.Policy}, filter, second.Limit, &token, repo, ProposalListOptions{SortOrder: "asc"}); err == nil {
					t.Fatal("changed scope accepted")
				}
				return
			}
			want := []string{}
			for _, step := range tc.Steps {
				if data, ok := step.Response["data"].(map[string]any); ok {
					want = append(want, listedProposalIDs(data)...)
				}
			}
			ids := listedProposalIDs(got)
			for n := 0; got["next_cursor"] != nil; n++ {
				if n > 20 {
					t.Fatal("unbounded cursor")
				}
				token := got["next_cursor"].(string)
				got, err = c.listProposals(context.Background(), ProposalActor{Policy: first.Policy}, status, first.Limit, &token, repo, ProposalListOptions{SortOrder: "asc"})
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, listedProposalIDs(got)...)
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatal(ids, want)
			}
		})
	}
}
