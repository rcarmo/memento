package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const dreamDraft = `{"intent":"maintain","rationale":" because ","consulted_concepts":[{"id":"a","path":"/a.md","revision":"r","title":"A"}],"contradictions":[],"reciprocal_links":[],"changes":[{"kind":"patch","path":"/a.md","body":"updated"}]}`

func TestDreamProposalParseValidate(t *testing.T) {
	draft, err := ParseDreamProposal(dreamDraft)
	if err != nil || draft.Intent != "maintain" || draft.Rationale != "because" || len(draft.Changes) != 1 {
		t.Fatal(draft, err)
	}
	root := t.TempDir()
	if err = os.WriteFile(filepath.Join(root, "a.md"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = ValidateDreamProposal(root, "r", draft, DreamProposalLimits{20, 32000, 6}); err != nil {
		t.Fatal(err)
	}
	defaults, err := ParseDreamProposal(`{"rationale":"x","changes":[{"kind":"create","path":"/b.md","concept_type":"concept","title":"B","body":"body"}]}`)
	if err != nil || defaults.Intent != "model proposal" || len(defaults.Consulted) != 0 {
		t.Fatal(defaults, err)
	}
}
func TestDreamProposalFailures(t *testing.T) {
	bad := []string{`{`, `{} {}`, `{"extra":1}`, `{"changes":{}}`, `{"consulted_concepts":{}}`, `{"consulted_concepts":[1]}`, `{"consulted_concepts":[{}]}`, `{"consulted_concepts":[{"id":1,"path":"/a","revision":"r","title":"A"}]}`, `{"contradictions":{}}`, `{"reciprocal_links":{}}`, `{"changes":[{"kind":"bad"}]}`}
	for _, raw := range bad {
		if _, err := ParseDreamProposal(raw); err == nil {
			t.Fatal(raw)
		}
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.md"), []byte("x"), 0600)
	base, _ := ParseDreamProposal(dreamDraft)
	cases := []DreamProposalDraft{base, base, base, base, base, base, base}
	cases[0].Rationale = ""
	cases[1].Changes = nil
	cases[2].Changes = append(cases[2].Changes, cases[2].Changes...)
	cases[3].Consulted = append(cases[3].Consulted, cases[3].Consulted[0])
	cases[4].Consulted[0]["revision"] = "wrong"
	cases[5].Changes[0]["kind"] = "rename"
	cases[6].Changes[0]["body"] = strings.Repeat("x", 10)
	limits := []DreamProposalLimits{{20, 100, 6}, {20, 100, 6}, {1, 100, 6}, {20, 100, 1}, {20, 100, 6}, {20, 100, 6}, {20, 5, 6}}
	for i, draft := range cases {
		if err := ValidateDreamProposal(root, "r", draft, limits[i]); err == nil {
			t.Fatal(i)
		}
	}
	base, _ = ParseDreamProposal(dreamDraft)
	base.Consulted = append(base.Consulted, map[string]any{"id": "b", "path": "/a.md", "revision": "r", "title": "B"})
	if err := ValidateDreamProposal(root, "r", base, DreamProposalLimits{20, 100, 1}); err == nil {
		t.Fatal("consulted limit")
	}
	base, _ = ParseDreamProposal(dreamDraft)
	base.Consulted = append(base.Consulted, map[string]any{"id": "a", "path": "/a.md", "revision": "r", "title": "A"})
	if err := ValidateDreamProposal(root, "r", base, DreamProposalLimits{20, 100, 6}); err == nil {
		t.Fatal("duplicate citation")
	}
	base, _ = ParseDreamProposal(dreamDraft)
	base.Changes = []map[string]any{{"kind": "rename", "path": "/a.md"}}
	if err := ValidateDreamProposal(root, "r", base, DreamProposalLimits{20, 100, 6}); err == nil {
		t.Fatal("rename")
	}
	base, _ = ParseDreamProposal(dreamDraft)
	base.Changes = []map[string]any{{"kind": "patch", "path": "/a.md", "body": strings.Repeat("x", 10)}}
	if err := ValidateDreamProposal(root, "r", base, DreamProposalLimits{20, 5, 6}); err == nil {
		t.Fatal("body")
	}
	base, _ = ParseDreamProposal(dreamDraft)
	base.Changes[0]["path"] = "relative"
	if err := ValidateDreamProposal(root, "r", base, DreamProposalLimits{20, 100, 6}); err == nil {
		t.Fatal("path")
	}
}
func TestStoreDreamProposal(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = t.TempDir()
	runtime, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(ctx)
	signals := control.Signals{DB: runtime.DB, Now: func() time.Time { return time.Unix(1, 0) }, UUID: func() string { return "signal" }}
	if _, err = signals.Upsert(ctx, "r", []control.DetectedSignal{{SignalType: "orphan", DedupeKey: "key", Evidence: map[string]any{}}}); err != nil {
		t.Fatal(err)
	}
	draft, err := ParseDreamProposal(dreamDraft)
	if err != nil {
		t.Fatal(err)
	}
	record, err := StoreDreamProposal(ctx, runtime.DB, "r", draft, []string{"key"}, time.Unix(2, 0), func() string { return "proposal" })
	if err != nil || record.AuthorPrincipal != "dream" || record.Status != control.Submitted {
		t.Fatal(record, err)
	}
	items, err := signals.List(ctx)
	if err != nil || items[0].Status != "proposed" {
		t.Fatal(items, err)
	}
	patch, err := record.Patch()
	if err != nil || !reflect.DeepEqual(patch["dream_signal_keys"], []any{"key"}) {
		t.Fatal(patch, err)
	}
	before, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil {
		t.Fatal(err)
	}
	after, err := repository.GetMainRevision(runtime.Paths.Repository)
	if err != nil || before != after {
		t.Fatal(before, after, err)
	}
	draft.Intent = "nil-id"
	generated, genErr := StoreDreamProposal(ctx, runtime.DB, "r", draft, nil, time.Unix(3, 0), nil)
	if genErr != nil || generated.ProposalID == "" {
		t.Fatal(generated, genErr)
	}
	if _, err = StoreDreamProposal(ctx, runtime.DB, "r", draft, nil, time.Unix(2, 0), func() string { return "proposal" }); err == nil {
		t.Fatal("duplicate")
	}
}
func TestStoreDreamProposalRollback(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if err = control.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER fail_signal_proposed BEFORE UPDATE ON dream_signals BEGIN SELECT RAISE(FAIL,'signal'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO dream_signals(signal_id,signal_type,entity_refs_json,severity,repo_revision,dedupe_key,status,evidence_hash,evidence_json,first_detected_at,last_detected_at) VALUES('s','x','[]','x','r','key','open','x','{}','x','x')`); err != nil {
		t.Fatal(err)
	}
	draft, _ := ParseDreamProposal(dreamDraft)
	if _, err = StoreDreamProposal(ctx, db, "r", draft, []string{"key"}, time.Unix(2, 0), func() string { return "p" }); err == nil {
		t.Fatal("failure")
	}
	records, err := (control.Proposals{DB: db}).List(ctx, control.ProposalQuery{})
	if err != nil || len(records) != 0 {
		t.Fatal(records, err)
	}
	_ = errors.Is(err, sql.ErrNoRows)
}
