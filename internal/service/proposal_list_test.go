package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/repository"
)

func testCursor() proposalCursor {
	var p proposalCursor
	for i := range p.key {
		p.key[i] = byte(i)
	}
	return p
}
func TestProposalListReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-list.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario              string
		Before, After, Events []map[string]any
		Steps                 []struct {
			Policy         access.EffectivePolicy
			Revision       string
			Status, Cursor *string
			Limit          int
			Response       map[string]any
			DecodedNext    any `json:"decoded_next"`
		}
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			restoreSubmitRows(t, q.Proposals.DB, map[string][]map[string]any{"proposals": c.Before})
			codec := testCursor()
			controls := ProposalControls{Queue: q, cursor: &codec, Random: bytes.NewReader(make([]byte, 1024))}
			for _, step := range c.Steps {
				repo := fakeRepo(nil)
				repo.main = func(repository.GitRepositoryPaths) (string, error) { return step.Revision, nil }
				got, err := controls.listProposals(ctx, ProposalActor{Policy: step.Policy}, step.Status, step.Limit, step.Cursor, repo)
				if step.Response["status"] == "error" {
					if err == nil || err.Error() != step.Response["message"] {
						t.Fatal(got, err, step.Response)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				expected := step.Response["data"].(map[string]any)
				if token, ok := got["next_cursor"].(string); ok {
					raw, err := codec.decrypt(token)
					if err != nil {
						t.Fatal(err)
					}
					var decoded any
					if err = json.Unmarshal(raw, &decoded); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(decoded, step.DecodedNext) {
						t.Fatal(decoded, step.DecodedNext)
					}
					got["next_cursor"] = expected["next_cursor"]
				} else if expected["next_cursor"] != nil {
					t.Fatal("missing cursor")
				}
				if !reflect.DeepEqual(jsonNormal(got), expected) {
					t.Fatal(got, expected)
				}
			}
			for _, table := range []struct {
				query string
				want  []map[string]any
			}{{"SELECT * FROM proposals ORDER BY proposal_id", c.After}, {"SELECT * FROM proposal_events ORDER BY event_id", c.Events}} {
				if got := tableRows(t, q.Proposals.DB, table.query); !reflect.DeepEqual(jsonNormal(got), jsonNormal(table.want)) {
					t.Fatal(got, table.want)
				}
			}
		})
	}
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
