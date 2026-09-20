package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
)

func TestProposalReviseReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-revise.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Initial, Trigger, Exception string
		Source                                control.ProposalRecord
		Policy                                access.EffectivePolicy
		Indexes                               []int
		Expected                              string `json:"expected_revision"`
		Intent, Rationale                     *string
		Response                              map[string]any
		Proposals, Assets, Events             []map[string]any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			installRebaseRecord(t, q, c.Source)
			if _, err := q.Proposals.DB.Exec("UPDATE proposals SET proposal_id='source'"); err != nil {
				t.Fatal(err)
			}
			if c.Scenario != "asset-missing" {
				if _, err := q.Proposals.DB.Exec(`INSERT INTO proposal_assets(proposal_id,asset_id,concept_path,asset_kind,version,media_type,sha256,blob_bytes,manifest_json,created_at) VALUES('source','asset','/a.md','docs','1.0.0','application/zip','digest',X'00FF','{}','created')`); err != nil {
					t.Fatal(err)
				}
			}
			root := t.TempDir()
			installMutationFiles(t, root, map[string]string{"/a.md": c.Initial})
			q.Paths.CurrentDir = root
			controls := ProposalControls{Queue: q, MaxConceptBytes: 65536, Random: bytes.NewReader(make([]byte, 32))}
			if c.Trigger != "" {
				if _, err := q.Proposals.DB.Exec(c.Trigger); err != nil {
					t.Fatal(err)
				}
			}
			repo := fakeRepo([]string{"/blocked.md"})
			repo.read = repository.ReadBundleEntry
			client := "client"
			got, err := controls.revise(ctx, ProposalActor{Policy: c.Policy, ClientInstanceID: &client}, "source", c.Indexes, c.Expected, c.Intent, c.Rationale, repo)
			if c.Exception != "" || c.Response["status"] == "error" {
				if err == nil {
					t.Fatal("expected failure", got)
				}
				if c.Response["status"] == "error" && c.Scenario != "asset-missing" && c.Scenario != "payload-failure" && c.Scenario != "preview-failure" && err.Error() != c.Response["message"] {
					t.Fatal(err, c.Response)
				}
			} else if err != nil || !reflect.DeepEqual(jsonNormal(got), c.Response["data"]) {
				t.Fatal(got, err, c.Response)
			}
			for _, table := range []struct {
				query string
				want  []map[string]any
			}{{"SELECT * FROM proposals ORDER BY proposal_id", c.Proposals}, {"SELECT * FROM proposal_assets ORDER BY proposal_id,asset_id", c.Assets}, {"SELECT * FROM proposal_events ORDER BY event_id", c.Events}} {
				rows := tableRows(t, q.Proposals.DB, table.query)
				for _, row := range rows {
					for key, value := range row {
						if b, ok := value.([]byte); ok {
							row[key] = base64.StdEncoding.EncodeToString(b)
						}
					}
				}
				if !reflect.DeepEqual(jsonNormal(rows), jsonNormal(table.want)) {
					t.Fatal(table.query, rows, table.want)
				}
			}
		})
	}
}
