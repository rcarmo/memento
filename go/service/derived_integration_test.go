package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/derived"
)

func TestModelsOffDerivedPublicationAndArchival(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	actor.Policy.Roles = []string{"proposer", "curator"}
	indexPath := filepath.Join(t.TempDir(), "derived.sqlite")
	db, err := derived.Connect(ctx, indexPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	index := &derived.Index{Path: indexPath}
	if err = index.Rebuild(ctx, c.Queue.Paths.CurrentDir, base); err != nil {
		t.Fatal(err)
	}
	c.DerivedIndexPath = indexPath
	c.DerivedUpdate = index.UpdatePaths
	first, err := c.Apply(ctx, actor, "proposal", base, "patch-key")
	if err != nil {
		t.Fatal(err)
	}
	var indexedBody, indexedRevision string
	if err = db.QueryRow("SELECT body,repo_revision FROM concepts WHERE path='/a.md'").Scan(&indexedBody, &indexedRevision); err != nil || indexedBody != "changed" || indexedRevision != first.Revision {
		t.Fatal(indexedBody, indexedRevision, err)
	}
	var hits int
	if err = db.QueryRow("SELECT COUNT(*) FROM concept_fts WHERE concept_fts MATCH 'changed'").Scan(&hits); err != nil || hits != 1 {
		t.Fatal(hits, err)
	}
	proposal, err := c.Propose(ctx, actor, "archive", first.Revision, []any{map[string]any{"kind": "trash", "path": "/a.md"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload := proposal["proposal"].(map[string]any)
	reports := payload["archival_impact"].([]any)
	if len(reports) != 1 {
		t.Fatal(reports)
	}
	id := payload["proposal_id"].(string)
	if _, err = c.Review(ctx, actor, id, "approve", nil, "review-key"); err != nil {
		t.Fatal(err)
	}
	archived, err := c.Apply(ctx, actor, id, first.Revision, "archive-key")
	if err != nil {
		t.Fatal(err)
	}
	var path string
	if err = db.QueryRow("SELECT path,repo_revision FROM concepts WHERE id='12345678'").Scan(&path, &indexedRevision); err != nil || path != "/trash/a.md" || indexedRevision != archived.Revision {
		t.Fatal(path, indexedRevision, err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM concept_fts WHERE concept_fts MATCH 'changed'").Scan(&hits); err != nil || hits != 1 {
		t.Fatal(hits, err)
	}
	var state string
	if err = db.QueryRow("SELECT value FROM index_state WHERE key='index_revision'").Scan(&state); err != nil || state != archived.Revision {
		t.Fatal(state, err)
	}
	replay, err := c.Apply(ctx, actor, id, first.Revision, "archive-key")
	if err != nil || replay.Data["replayed"] != true || replay.Revision != archived.Revision {
		t.Fatal(replay, err)
	}
}
