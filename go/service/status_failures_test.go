package service

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/access"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/derived"
	"github.com/rcarmo/memento/go/repository"
)

type failingStatusIndex struct {
	ReadIndex
	result derived.StatusSnapshot
	err    error
}

func (f failingStatusIndex) Status(context.Context, access.EffectivePolicy) (derived.StatusSnapshot, error) {
	return f.result, f.err
}
func TestStatusFailurePaths(t *testing.T) {
	previous := BuildVersion
	BuildVersion = "1.0.0"
	metadata, err := NewModelsOffMetadata("compact")
	BuildVersion = previous
	if err != nil || metadata.ServiceVersion != "1.0.0" {
		t.Fatal(metadata, err)
	}
	if _, err := modelsOffMetadata("compact", []byte("{")); err == nil {
		t.Fatal("invalid defaults")
	}
	if _, err := NewModelsOffMetadata("bad"); err == nil {
		t.Fatal("surface")
	}
	for _, scenario := range []string{"config", "index", "snapshot", "revision", "refresh", "visibility"} {
		t.Run(scenario, func(t *testing.T) {
			c, actor, _ := realApplyTest(t)
			ctx := context.Background()
			var err error
			c.Metadata, err = NewModelsOffMetadata("compact")
			if err != nil {
				t.Fatal(err)
			}
			c.Index = failingStatusIndex{result: derived.StatusSnapshot{State: derived.IndexState{Status: "ready", RepoRevision: "main", IndexRevision: "main"}}}
			repo := fakeRepo(nil)
			switch scenario {
			case "config":
				c.Metadata = nil
			case "index":
				c.Index = nil
			case "snapshot":
				c.Index = failingStatusIndex{err: io.ErrClosedPipe}
			case "revision":
				repo.main = func(repository.GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
			case "refresh":
				c.Queue.Proposals.DB.Close()
			case "visibility":
				c.Queue.Proposals.DB.Exec("UPDATE proposals SET base_revision='main',patch_json='{}',expires_at=NULL")
			}
			if _, _, err := c.status(ctx, actor, repo); err == nil {
				t.Fatal("expected failure")
			}
		})
	}
	c, actor, _ := realApplyTest(t)
	c.Metadata, _ = NewModelsOffMetadata("compact")
	index := &derived.Index{Path: filepath.Join(t.TempDir(), "derived.sqlite")}
	c.Index = index
	if err := index.Rebuild(context.Background(), c.Queue.Paths.CurrentDir, "ready"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.statusWithList(context.Background(), actor, defaultProposalRepository(), func(context.Context, control.ProposalQuery) ([]control.ProposalRecord, error) {
		return nil, io.ErrClosedPipe
	}); err != io.ErrClosedPipe {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := c.Status(cancelled, actor); err != context.Canceled {
		t.Fatal(err)
	}
}
