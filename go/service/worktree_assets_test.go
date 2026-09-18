package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/go/access"
)

func TestWorktreeAssetReference(t *testing.T) {
	raw, err := os.ReadFile("../testdata/parity/worktree-assets.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario       string
		Initial, Files map[string]string
		Changes        []any
		Adapted        []ProposalChange
		Blob           string `json:"blob_base64"`
		Changed        []string
		ErrorType      string `json:"error_type"`
		Error          string
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err = decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			root := t.TempDir()
			installMutationFiles(t, root, c.Initial)
			blob, err := base64.StdEncoding.DecodeString(c.Blob)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = q.Proposals.DB.Exec(`INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES('proposal','asset','/different.md','different','9.0.0','application/zip',?,?,'{}','created')`, strings.Repeat("a", 64), blob); err != nil {
				t.Fatal(err)
			}
			m := WorktreeMutator{Proposals: q.Proposals, MaxConceptBytes: 65536, Now: func() time.Time { return time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC) }}
			changes, err := NormalizeProposalChanges(c.Changes)
			if err != nil {
				t.Fatal(err)
			}
			adapted := AdaptExistingAssetConcepts(root, changes)
			if !reflect.DeepEqual(jsonNormal(adapted), jsonNormal(c.Adapted)) {
				t.Fatal(adapted, c.Adapted)
			}
			id := "proposal"
			proposalID := &id
			if c.Scenario == "no-proposal" {
				proposalID = nil
			}
			var changed []string
			if strings.HasPrefix(c.Scenario, "adapt") && c.Scenario != "adapt" {
				changed = []string{}
			} else {
				changed, err = m.ApplyChanges(ctx, root, adapted, "actor", access.EffectivePolicy{}, proposalID)
				if err == nil && c.Scenario == "duplicate" {
					_, err = m.ApplyChanges(ctx, root, adapted, "actor", access.EffectivePolicy{}, proposalID)
				}
			}
			if c.ErrorType != "" {
				if err == nil {
					t.Fatal("missing error", c.Error)
				}
				if c.ErrorType == "ConflictError" || c.ErrorType == "ServiceError" {
					if err.Error() != c.Error {
						t.Fatal(err, c.Error)
					}
				}
			} else if err != nil || !reflect.DeepEqual(changed, c.Changed) {
				t.Fatal(changed, c.Changed, err)
			}
			files, _ := mutationSnapshot(t, root)
			encoded := map[string]string{}
			for name, text := range files {
				encoded[name] = base64.StdEncoding.EncodeToString([]byte(text))
			}
			if !reflect.DeepEqual(encoded, c.Files) {
				t.Fatal(encoded, c.Files)
			}
		})
	}
}
func TestAggregateMutationPathsAndFailures(t *testing.T) {
	ctx := context.Background()
	root, m := mutationTest(t)
	raw := []any{map[string]any{"kind": "create", "path": "/public/new.md", "concept_type": "concept", "title": "New", "body": "new"}, map[string]any{"kind": "patch", "path": "/public/new.md", "body": "updated"}, map[string]any{"kind": "rename", "path": "/public/a.md", "new_path": "/public/moved.md"}, map[string]any{"kind": "trash", "path": "/public/ref.md"}}
	changes, err := NormalizeProposalChanges(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.ApplyChanges(ctx, root, changes, "actor", archivePolicy(), nil)
	want := []string{"/public/a.md", "/public/moved.md", "/public/new.md", "/public/ref.md", "/trash/public/ref.md"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	if got, err = m.ApplyChanges(ctx, root, nil, "actor", archivePolicy(), nil); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if _, err = m.ApplyChanges(ctx, root, []ProposalChange{{"kind": "unknown"}}, "actor", archivePolicy(), nil); err == nil {
		t.Fatal("unknown mutation")
	}
	q, _ := queueTest(t)
	m.Proposals = q.Proposals
	change := ProposalChange{"path": "/public/new.md", "asset_id": "asset", "zip_sha256": strings.Repeat("a", 64), "manifest": map[string]any{"invalid": make(chan int)}}
	if _, err = q.Proposals.DB.Exec(`INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES('proposal','asset','/a.md','docs','1.0.0','application/zip',?,X'00','{}','created')`, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = m.ApplyAssetPack(ctx, root, change, "actor", "proposal"); err == nil {
		t.Fatal("unserializable manifest")
	}
}
