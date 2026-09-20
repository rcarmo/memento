package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func TestProposalGetReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-get.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Initial, Exception string
		Record, After                control.ProposalRecord
		Asset                        map[string]any
		Policy                       access.EffectivePolicy
		IsAsset                      bool `json:"is_asset"`
		File                         *string
		Offset, Limit                int64
		Response                     map[string]any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			installRebaseRecord(t, q, c.Record)
			root := t.TempDir()
			q.Paths.CurrentDir = root
			if c.Scenario != "missing-target" {
				text := c.Initial
				if c.Scenario == "invalid-target" {
					text = "invalid"
				}
				installMutationFiles(t, root, map[string]string{"/a.md": text})
			}
			a := c.Asset
			blob, err := base64.StdEncoding.DecodeString(a["blob_bytes"].(string))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = q.Proposals.DB.Exec(`INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES('proposal',?,?,?,?,?,?,?,?, 'created')`, a["asset_id"], a["concept_path"], a["asset_kind"], a["version"], a["media_type"], a["sha256"], blob, a["manifest_json"]); err != nil {
				t.Fatal(err)
			}
			controls := ProposalControls{Queue: q, MaxConceptBytes: 65536}
			actor := ProposalActor{Policy: c.Policy}
			var got map[string]any
			if c.IsAsset {
				id := "asset"
				if c.Scenario == "missing-asset" {
					id = "missing"
				}
				got, err = controls.ProposalAssetGet(ctx, actor, "proposal", id, c.File, c.Offset, c.Limit)
			} else {
				view := "detailed"
				if c.Scenario == "summary" {
					view = "summary"
				}
				if c.Scenario == "unknown-view" {
					view = "unknown"
				}
				got, err = controls.getProposal(ctx, actor, "proposal", view, fakeRepo([]string{"/a.md"}))
			}
			if c.Exception != "" || c.Response["status"] == "error" {
				if err == nil {
					t.Fatal("expected error", got)
				}
				if c.Scenario != "missing-asset" && c.Scenario != "missing-file" && c.Scenario != "bad-manifest" && c.Scenario != "invalid-target" && c.Exception == "" && err.Error() != c.Response["message"] {
					t.Fatal(err, c.Response)
				}
			} else if err != nil || !reflect.DeepEqual(jsonNormal(got), c.Response["data"]) {
				t.Fatal(got, err, c.Response)
			}
			after, err := q.Proposals.Get(ctx, "proposal")
			if err != nil || !reflect.DeepEqual(after, c.After) {
				t.Fatal(after, c.After, err)
			}
		})
	}
}
func TestProposalGetFailures(t *testing.T) {
	ctx := context.Background()
	c, actor, _ := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer", "curator"}
	if _, err := c.GetProposal(ctx, actor, "proposal", "detailed"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetProposal(ctx, actor, "missing", "summary"); err == nil {
		t.Fatal("missing")
	}
	repo := fakeRepo(nil)
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
	if _, err := c.getProposal(ctx, actor, "proposal", "summary", repo); err == nil {
		t.Fatal("refresh")
	}
	for _, view := range []string{"summary", "detailed"} {
		controls, actor := rebaseTest(t)
		actor.Policy.Roles = []string{"proposer"}
		controls.MaxConceptBytes = 65536
		if _, err := controls.Queue.Proposals.DB.Exec(`UPDATE proposals SET base_revision='base',patch_json='{"changes":[]}'`); err != nil {
			t.Fatal(err)
		}
		repo := fakeRepo(nil)
		// Force refreshed Get to return changed malformed state before rendering.
		if view == "detailed" {
			if _, err := controls.Queue.Proposals.DB.Exec(`CREATE TRIGGER malformed AFTER UPDATE ON proposals BEGIN UPDATE proposals SET patch_json='{"changes":null}' WHERE proposal_id=NEW.proposal_id;END`); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := controls.Queue.Proposals.DB.Exec(`DROP TABLE proposal_assets`); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := controls.getProposal(ctx, actor, "proposal", view, repo); err == nil {
			t.Fatal(view)
		}
	}
	// Visible archival recomputation after an otherwise successful summary must
	// fail if its required derived index cannot be opened.
	if _, err := c.Queue.Proposals.DB.Exec(`UPDATE proposals SET patch_json='{"changes":[{"kind":"trash","path":"/a.md"}],"archival_impact":[{"path":"/a.md"}]}'`); err != nil {
		t.Fatal(err)
	}
	c.DerivedIndexPath = filepath.Join(t.TempDir(), "missing.sqlite")
	if _, err := c.getProposal(ctx, actor, "proposal", "summary", fakeRepo(nil)); err == nil {
		t.Fatal("archive")
	}
}

func TestGetPayloadAndAssetFailureBranches(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer", "curator"}
	file := "file.txt"
	reader := actor
	reader.Policy.Roles = []string{"reader"}
	if _, err := c.ProposalAssetGet(ctx, reader, "proposal", "asset", nil, 0, 5); err == nil {
		t.Fatal("role bypass")
	}
	if _, err := c.ProposalAssetGet(ctx, actor, "missing", "asset", nil, 0, 5); err == nil {
		t.Fatal("visibility bypass")
	}
	if _, err := c.Queue.Proposals.DB.Exec(`INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES('proposal','asset','/a.md','docs','1.0.0','application/zip',?,X'00','{}','created')`, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ProposalAssetGet(ctx, actor, "proposal", "asset", &file, 0, 5); err == nil {
		t.Fatal("invalid manifest shape")
	}
	manifest := `{"entries":[{"path":"file.txt","size":0,"media_type":"text/plain","sha256":"` + strings.Repeat("a", 64) + `"}],"file_count":1,"total_uncompressed_bytes":0,"sha256":"` + strings.Repeat("a", 64) + `"}`
	if _, err := c.Queue.Proposals.DB.Exec("UPDATE proposal_assets SET manifest_json=?,blob_bytes=zeroblob(?)", manifest, assets.MaxZIPBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ProposalAssetGet(ctx, actor, "proposal", "asset", &file, 0, 5); err == nil {
		t.Fatal("oversize archive")
	}
	if _, err := c.Queue.Proposals.DB.Exec("DROP TABLE proposal_events"); err != nil {
		t.Fatal(err)
	}
	repo := fakeRepo(nil)
	repo.main = func(repository.GitRepositoryPaths) (string, error) { return base, nil }
	if _, err := c.getProposal(ctx, actor, "proposal", "detailed", repo); err == nil {
		t.Fatal("history query failure")
	}
}
