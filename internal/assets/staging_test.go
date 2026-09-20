package assets

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/control"
)

type sequentialRandom struct{ position int }

func (r *sequentialRandom) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r.position + i)
	}
	r.position += len(p)
	return len(p), nil
}
func stagingStore(t *testing.T) (*StagingStore, *time.Time) {
	t.Helper()
	db, err := control.Connect(context.Background(), filepath.Join(t.TempDir(), "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = control.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO proposals(proposal_id,author_principal,base_revision,intent,patch_json,patch_hash,status,created_at,updated_at) VALUES('proposal','agent','base','test','{}','hash','submitted','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	return &StagingStore{DB: db, Now: func() time.Time { return clock }, Random: &sequentialRandom{}}, &clock
}
func normal(value any) any {
	raw, _ := json.Marshal(value)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}
func TestStagingReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/staged-assets.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Packs map[string][]byte
		Cases []struct {
			Input struct {
				Action, Principal, Key, Version, Pack string
				IDs                                   []string
				Hours                                 int
				Ready                                 bool
			}
			Payload             map[string]any
			Blob                []byte
			Replayed            bool
			Token, State, Error string
			AssetID             *string `json:"asset_id"`
		}
		Rows map[string][]map[string]any
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	s, clock := stagingStore(t)
	ctx := context.Background()
	id, token := "", ""
	for i, c := range f.Cases {
		principal := c.Input.Principal
		if principal == "" {
			principal = "agent"
		}
		var staged StagedAsset
		var replay bool
		var ticket UploadTicket
		var err error
		switch c.Input.Action {
		case "advance":
			*clock = clock.Add(time.Duration(c.Input.Hours) * time.Hour)
			continue
		case "put":
			staged, replay, err = s.Put(ctx, principal, c.Input.Key, "asset", "1.0.0", f.Packs[c.Input.Pack])
		case "get":
			staged, err = s.Get(ctx, principal, id, c.Input.Ready)
		case "consume":
			ids := []string{}
			for _, value := range c.Input.IDs {
				ids = append(ids, strings.ReplaceAll(value, "{{ASSET}}", id))
			}
			err = s.Consume(ctx, principal, ids, "proposal")
		case "begin":
			version := c.Input.Version
			if version == "" {
				version = "1.0.0"
			}
			var created string
			ticket, created, err = s.BeginUpload(ctx, principal, c.Input.Key, "asset", version)
			if err == nil {
				token = created
				if token != c.Token {
					t.Fatal(i, "token mismatch")
				}
			}
		case "status":
			ticket, err = s.TicketStatus(ctx, principal, c.Input.Key)
		case "upload":
			staged, replay, err = s.PutWithTicket(ctx, token, f.Packs[c.Input.Pack])
		}
		if c.Error != "" {
			if err == nil || err.Error() != c.Error {
				t.Fatal(i, err, c.Error)
			}
			continue
		}
		if err != nil {
			t.Fatal(i, err)
		}
		if c.Payload != nil {
			if c.Input.Action != "get" {
				id = staged.StagedAssetID
			}
			if !reflect.DeepEqual(normal(staged.PublicPayload()), c.Payload) || !reflect.DeepEqual(staged.BlobBytes, c.Blob) || replay != c.Replayed {
				t.Fatal(i, staged, c, replay)
			}
		}
		if c.State != "" {
			state, err := ticket.State(*clock)
			if err != nil || state != c.State {
				t.Fatal(i, state, err)
			}
			if c.Input.Action == "status" && !reflect.DeepEqual(ticket.StagedAssetID, c.AssetID) {
				t.Fatal(i, ticket, c.AssetID)
			}
		}
	}
	for table, want := range f.Rows {
		rows, err := s.DB.Query("SELECT * FROM " + table + " ORDER BY 1")
		if err != nil {
			t.Fatal(err)
		}
		columns, _ := rows.Columns()
		got := []map[string]any{}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err = rows.Scan(pointers...); err != nil {
				t.Fatal(err)
			}
			row := map[string]any{}
			for i, k := range columns {
				v := values[i]
				if data, ok := v.([]byte); ok {
					v = base64.StdEncoding.EncodeToString(data)
				}
				row[k] = v
			}
			got = append(got, row)
		}
		rows.Close()
		if !reflect.DeepEqual(normal(got), normal(want)) {
			t.Fatal(table, got, want)
		}
	}
}
