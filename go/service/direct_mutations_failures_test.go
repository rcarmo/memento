package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
)

func TestDirectMutationFailures(t *testing.T) {
	c, actor, base := realApplyTest(t)
	ctx := context.Background()
	for _, raw := range []map[string]any{{"kind": "trash", "path": "/a.md"}, {"kind": "create", "path": "/new.md"}} {
		if _, _, err := c.CommitConceptChange(ctx, actor, raw, base, "key"); err == nil {
			t.Fatal("invalid direct change")
		}
	}
	change, err := directChange(map[string]any{"kind": "patch", "path": "/a.md", "body": "new"})
	if err != nil {
		t.Fatal(err)
	}
	never := func(context.Context, repository.TransactionRequest, repository.MutationCallback) (repository.TransactionResult, error) {
		t.Error("unexpected transaction")
		return repository.TransactionResult{}, nil
	}
	change["body"] = string([]byte{255})
	if _, _, err = c.commitConceptChange(ctx, actor, change, base, "key", fakeRepo(nil), never, defaultMutationIO()); err == nil {
		t.Fatal("JSON")
	}
	change["body"] = "new"
	c.Random = bytes.NewReader(nil)
	if _, _, err = c.commitConceptChange(ctx, actor, change, base, "key", fakeRepo(nil), never, defaultMutationIO()); err == nil {
		t.Fatal("random")
	}
	c.Random = nil
	for _, failure := range []error{io.ErrClosedPipe, &repository.BundleError{Message: "bad"}, &repository.FrontmatterError{Message: "bad"}, os.ErrNotExist} {
		ops := defaultMutationIO()
		ops.read = func(string, string) (repository.BundleEntry, error) { return repository.BundleEntry{}, failure }
		_, err := c.directMutationWarnings([]ProposalChange{change}, ops)
		if failure == io.ErrClosedPipe && !errors.Is(err, failure) || failure != io.ErrClosedPipe && err != nil {
			t.Fatal(err)
		}
	}
	root := c.Queue.Paths.CurrentDir
	directory := filepath.Join(root, ".assets/12345678")
	if err = os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(directory, "INVALID"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = c.directMutationWarnings([]ProposalChange{change}, defaultMutationIO()); err == nil {
		t.Fatal("invalid kind")
	}
	os.Remove(filepath.Join(directory, "INVALID"))
	if err = os.Mkdir(filepath.Join(directory, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = c.directMutationWarnings([]ProposalChange{change}, defaultMutationIO()); err != nil {
		t.Fatal("empty asset kind", err)
	}
	if err = os.WriteFile(filepath.Join(directory, "docs/bad.json"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = c.directMutationWarnings([]ProposalChange{change}, defaultMutationIO()); err == nil {
		t.Fatal("invalid version")
	}
}
func TestPublicDirectCommitPostPublicationFailureAndReplay(t *testing.T) {
	ctx := context.Background()
	c, actor, base := realApplyTest(t)
	c.ChangedConcepts = func(context.Context, access.EffectivePolicy, []string) error { return io.ErrClosedPipe }
	raw := map[string]any{"kind": "patch", "path": "/a.md", "body": "published"}
	if _, _, err := c.CommitConceptChange(ctx, actor, raw, base, "key"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	op, err := (control.Operations{DB: c.Queue.Proposals.DB}).ByIdempotency(ctx, actor.Policy.Principal, "key")
	if err != nil || op == nil || op.State != control.Succeeded {
		t.Fatal(op, err)
	}
	c.ChangedConcepts = nil
	// Expected revision is absent from direct mutation request hash. Same change
	// replays after publication even with a different expected revision.
	data, options, err := c.CommitConceptChange(ctx, actor, raw, "other-revision", "key")
	if err != nil || data["replayed"] != true || *options.OperationID != op.OpID {
		t.Fatal(data, options, err)
	}
	denied := actor
	denied.Policy.WritePrefixes = nil
	if _, _, err = c.CommitConceptChange(ctx, denied, raw, base, "key"); err == nil {
		t.Fatal("revoked replay")
	}
	// The asset-parity guard is checked before replay too.
	directory := filepath.Join(c.Queue.Paths.CurrentDir, ".assets/12345678/docs")
	if err = os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "1.0.0.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = c.CommitConceptChange(ctx, actor, raw, base, "key"); err == nil {
		t.Fatal("asset-bound replay")
	}
}
