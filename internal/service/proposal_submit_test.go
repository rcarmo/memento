package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/rcarmo/memento/internal/access"
	"github.com/rcarmo/memento/internal/assets"
	"github.com/rcarmo/memento/internal/repository"
)

func restoreSubmitRows(t *testing.T, db *sql.DB, tables map[string][]map[string]any) {
	t.Helper()
	if _, err := db.Exec("DELETE FROM proposals"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"proposals", "proposal_assets", "staged_assets", "proposal_events"} {
		for _, row := range tables[table] {
			keys := []string{}
			for key := range row {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			values := []any{}
			marks := []string{}
			for _, key := range keys {
				v := row[key]
				if key == "blob_bytes" {
					var err error
					v, err = base64.StdEncoding.DecodeString(v.(string))
					if err != nil {
						t.Fatal(err)
					}
				}
				values = append(values, v)
				marks = append(marks, "?")
			}
			if _, err := db.Exec("INSERT INTO "+table+"("+strings.Join(keys, ",")+") VALUES("+strings.Join(marks, ",")+")", values...); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func submitSnapshot(t *testing.T, db *sql.DB) map[string][]map[string]any {
	t.Helper()
	result := map[string][]map[string]any{}
	for _, table := range []string{"proposals", "proposal_assets", "staged_assets", "proposal_events"} {
		rows := tableRows(t, db, "SELECT * FROM "+table+" ORDER BY rowid")
		for _, row := range rows {
			for key, value := range row {
				if b, ok := value.([]byte); ok {
					row[key] = base64.StdEncoding.EncodeToString(b)
				}
			}
		}
		result[table] = rows
	}
	return result
}
func TestProposalSubmitReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-submit.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Scenario, Initial, Base, Trigger, Exception string
		Changes                                     []any
		Policy                                      access.EffectivePolicy
		HasStaging                                  bool `json:"has_staging"`
		Before, After                               map[string][]map[string]any
		Response                                    map[string]any
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Scenario, func(t *testing.T) {
			ctx := context.Background()
			q, _ := queueTest(t)
			restoreSubmitRows(t, q.Proposals.DB, c.Before)
			root := t.TempDir()
			installMutationFiles(t, root, map[string]string{"/a.md": c.Initial})
			q.Paths.CurrentDir = root
			random := make([]byte, 48)
			random[31] = 1
			random[47] = 2
			controls := ProposalControls{Queue: q, MaxConceptBytes: 65536, Random: bytes.NewReader(random)}
			if c.Scenario == "size" {
				controls.MaxConceptBytes = 1
			}
			if c.HasStaging {
				controls.Staging = &assets.StagingStore{DB: q.Proposals.DB, Now: q.Now}
			}
			if c.Trigger != "" {
				if _, err := q.Proposals.DB.Exec(c.Trigger); err != nil {
					t.Fatal(err)
				}
			}
			why, client := "why", "client"
			repo := fakeRepo(nil)
			repo.read = repository.ReadBundleEntry
			got, err := controls.propose(ctx, ProposalActor{Policy: c.Policy, ClientInstanceID: &client}, "intent", c.Base, c.Changes, &why, repo)
			if c.Exception != "" || c.Response["status"] == "error" {
				if err == nil {
					t.Fatal("missing failure", got)
				}
				if c.Response["status"] == "error" && c.Scenario != "invalid" && c.Scenario != "payload-failure" {
					if err.Error() != c.Response["message"] {
						t.Fatal(err, c.Response)
					}
				}
			} else if err != nil || !reflect.DeepEqual(jsonNormal(got), jsonNormal(c.Response["data"])) {
				t.Fatal(got, err, c.Response)
			}
			after := submitSnapshot(t, q.Proposals.DB)
			if !reflect.DeepEqual(jsonNormal(after), jsonNormal(c.After)) {
				for table, want := range c.After {
					if !reflect.DeepEqual(jsonNormal(after[table]), jsonNormal(want)) {
						t.Error(table, fmt.Sprint(after[table]), fmt.Sprint(want))
					}
				}
				t.FailNow()
			}
		})
	}
}
func TestProposalZIPBase64(t *testing.T) {
	for _, raw := range []string{"", "Zm9v", "Zg==", "Zh==", "Zm8="} {
		got, err := decodeProposalZIP(raw)
		if err != nil {
			t.Fatal(raw, err)
		}
		if raw != "" && len(got) == 0 {
			t.Fatal(raw)
		}
	}
	for _, raw := range []string{"=", "====", "Zm9v=", "Zm9v====", "Zg===", "Zg=", "Zg==a", "Z g==", "Zg==\n", "Zg==\r", "💀"} {
		if _, err := decodeProposalZIP(raw); err == nil {
			t.Fatal(raw)
		}
	}
	// Encoded-size guard counts code points before decoding, matching Python.
	if _, err := decodeProposalZIP(strings.Repeat("a", ((assets.MaxZIPBytes+2)/3)*4+1)); err == nil {
		t.Fatal("oversize")
	}
}

func TestProposalBase64Reference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/proposal-base64.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input, Expected string
		Error           bool
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		got, err := decodeProposalZIP(c.Input)
		if c.Error {
			if err == nil {
				t.Fatalf("accepted %q", c.Input)
			}
		} else if err != nil || base64.StdEncoding.EncodeToString(got) != c.Expected {
			t.Fatalf("%q: %x %v want %s", c.Input, got, err, c.Expected)
		}
	}
}
