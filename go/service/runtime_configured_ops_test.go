package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/rcarmo/memento/go/execute"
	"github.com/rcarmo/memento/go/umcp"
)

func TestDefaultConfiguredRegistrationFailures(t *testing.T) {
	ctx := context.Background()
	ops := defaultModelsOffBuildOps()
	jobs, _ := jobsTest(t)
	server := umcp.NewServer("x")
	runtime := &Runtime{DB: jobs.Controls.Queue.Proposals.DB}
	options := ModelsOffRuntimeOptions{Surface: "standard", DeepAnswers: DefaultDeepAnswersConfig()}
	closed, _ := answerStoreTest(t)
	_ = closed.DB.Close()
	runtime.DB = closed.DB
	if err := ops.registerConfigured(ctx, runtime, jobs, server, options, nil); err == nil {
		t.Fatal("migrate")
	}
	runtime.DB = jobs.Controls.Queue.Proposals.DB
	options.Surface = "bad"
	if err := ops.registerConfigured(ctx, runtime, jobs, umcp.NewServer("x"), options, nil); err == nil {
		t.Fatal("catalog")
	}
	options.Surface = "standard"
	old := executeFactory
	t.Cleanup(func() { executeFactory = old })
	executeFactory = func(execute.Limits) (*execute.Factory, error) { return nil, errors.New("endpoint") }
	if err := ops.registerConfigured(ctx, runtime, jobs, umcp.NewServer(filepath.Base(t.TempDir())), options, nil); err == nil {
		t.Fatal("endpoint")
	}
}
