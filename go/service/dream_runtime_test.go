package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rcarmo/memento/go/control"
	"github.com/rcarmo/memento/go/repository"
	"testing"
	"time"
)

func dreamOps() dreamRunOps {
	return dreamRunOps{revision: func(repository.GitRepositoryPaths) (string, error) { return "r2", nil }, scan: func(string, repository.BundleFilter) (repository.RepositoryBundle, error) {
		return repository.RepositoryBundle{}, nil
	}, previous: func(context.Context, *sql.DB, string) (*string, error) { value := "r1"; return &value, nil }, diff: func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
		return []string{"/a"}, nil
	}, claim: func(context.Context, string, string, *string) (control.SchedulerClaim, error) {
		return control.SchedulerClaim{Created: true, Record: control.SchedulerRunRecord{RunID: "run"}}, nil
	}, upsert: func(context.Context, string, []control.DetectedSignal) ([]control.DreamSignal, error) {
		return []control.DreamSignal{{DedupeKey: "x"}}, nil
	}, actionable: func(context.Context) ([]control.DreamSignal, error) {
		return []control.DreamSignal{{DedupeKey: "x"}}, nil
	}, list: func(context.Context) ([]control.DreamSignal, error) {
		return []control.DreamSignal{{SignalType: "orphan", Status: "open", DedupeKey: "x", EntityRefs: []string{"a"}}}, nil
	}, finish: func(context.Context, string, string, *string, int, int, []control.ModelAttempt, *string) (control.SchedulerRunRecord, error) {
		return control.SchedulerRunRecord{}, nil
	}, setState: func(context.Context, *sql.DB, string, string) error { return nil }}
}
func TestDreamRealRuntime(t *testing.T) {
	ctx := context.Background()
	var config RuntimeConfig
	config.Repository.RootPath = t.TempDir()
	dream := DefaultDreamConfig()
	dream.Mode = "report_only"
	dream.QuietPeriodSeconds = 0
	runtime, _, err := BuildModelsOffRuntime(ctx, config, ModelsOffRuntimeOptions{Surface: "standard", Tokens: []BearerPrincipal{}, Dream: dream})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := runtime.RunDream(ctx, "report_only", time.Unix(21600, 0))
	if err != nil || payload["state"] != "succeeded" {
		t.Fatal(payload, err)
	}
	duplicate, err := runtime.RunDream(ctx, "report_only", time.Unix(21600, 0))
	if err != nil || duplicate["state"] != "skipped_duplicate_window" {
		t.Fatal(duplicate, err)
	}
	_ = runtime.Close(ctx)
}
func TestDreamRunStates(t *testing.T) {
	ctx := context.Background()
	runtime := &Runtime{Dream: DefaultDreamConfig()}
	now := time.Unix(21600, 0).UTC()
	payload, err := runtime.runDream(ctx, "disabled", now, dreamOps())
	if err != nil || payload["state"] != "disabled" {
		t.Fatal(payload, err)
	}
	if _, err = runtime.runDream(ctx, "propose", now, dreamOps()); err == nil {
		t.Fatal("propose")
	}
	if _, err = runtime.runDream(ctx, "invalid", now, dreamOps()); err == nil {
		t.Fatal("invalid")
	}
	ops := dreamOps()
	stamp := now.Add(-time.Second)
	ops.scan = func(string, repository.BundleFilter) (repository.RepositoryBundle, error) {
		return repository.RepositoryBundle{Entries: []repository.BundleEntry{dreamEntry("/a", "a", "A", "active", "", stamp)}}, nil
	}
	payload, err = runtime.runDream(ctx, "report_only", now, ops)
	if err != nil || payload["state"] != "quiet_period" {
		t.Fatal(payload, err)
	}
	ops = dreamOps()
	ops.claim = func(context.Context, string, string, *string) (control.SchedulerClaim, error) {
		return control.SchedulerClaim{}, &control.SchedulerConflictError{JobName: "dream"}
	}
	runtime.Dream.QuietPeriodSeconds = 0
	payload, err = runtime.runDream(ctx, "report_only", now, ops)
	if err != nil || payload["state"] != "skipped_overlap" {
		t.Fatal(payload, err)
	}
	ops = dreamOps()
	ops.claim = func(context.Context, string, string, *string) (control.SchedulerClaim, error) {
		return control.SchedulerClaim{Record: control.SchedulerRunRecord{RunID: "old"}}, nil
	}
	payload, err = runtime.runDream(ctx, "report_only", now, ops)
	if err != nil || payload["state"] != "skipped_duplicate_window" || payload["run_id"] != "old" {
		t.Fatal(payload, err)
	}
}
func TestDreamProposeFailureFinishing(t *testing.T) {
	ctx, runtime, revision := configuredDreamRuntime(t)
	defer runtime.Close(ctx)
	runtime.ModelClient = &stubModelClient{err: errors.New("model")}
	ops := dreamOps()
	ops.revision = func(repository.GitRepositoryPaths) (string, error) { return revision, nil }
	var failed bool
	ops.finish = func(_ context.Context, _ string, state string, _ *string, _ int, _ int, _ []control.ModelAttempt, _ *string) (control.SchedulerRunRecord, error) {
		failed = state == "failed"
		return control.SchedulerRunRecord{}, nil
	}
	if _, err := runtime.runDream(ctx, "propose", time.Unix(21600, 0), ops); err == nil || !failed {
		t.Fatal(err, failed)
	}
}

func TestDreamRunSuccessAndFailures(t *testing.T) {
	ctx := context.Background()
	runtime := &Runtime{Dream: DefaultDreamConfig()}
	runtime.Dream.Mode = "report_only"
	runtime.Dream.QuietPeriodSeconds = 0
	runtime.Dream.Budgets.MaxSignalsPerRun = 1
	now := time.Unix(21600, 0)
	ops := dreamOps()
	stamp := time.Unix(1, 0)
	ops.scan = func(string, repository.BundleFilter) (repository.RepositoryBundle, error) {
		return repository.RepositoryBundle{Entries: []repository.BundleEntry{dreamEntry("/a", "a", "A", "active", "[x](/x) [y](/y)", stamp)}}, nil
	}
	upsertCount := 0
	ops.upsert = func(_ context.Context, _ string, d []control.DetectedSignal) ([]control.DreamSignal, error) {
		upsertCount = len(d)
		return []control.DreamSignal{{DedupeKey: "x"}}, nil
	}
	payload, err := runtime.runDream(ctx, "", now, ops)
	if err != nil || payload["state"] != "succeeded" || payload["proposal_count"] != 0 || upsertCount > 1 || len(payload["signals"].([]any)) != 1 {
		t.Fatal(payload, upsertCount, err)
	}
	boom := errors.New("boom")
	for _, stage := range []string{"revision", "scan", "claim", "previous", "diff", "upsert", "actionable", "finish", "state", "list"} {
		ops = dreamOps()
		failed := false
		ops.finish = func(_ context.Context, _ string, state string, _ *string, _ int, _ int, _ []control.ModelAttempt, _ *string) (control.SchedulerRunRecord, error) {
			if state == "failed" {
				failed = true
			}
			if stage == "finish" && state == "succeeded" {
				return control.SchedulerRunRecord{}, boom
			}
			return control.SchedulerRunRecord{}, nil
		}
		switch stage {
		case "revision":
			ops.revision = func(repository.GitRepositoryPaths) (string, error) { return "", boom }
		case "scan":
			ops.scan = func(string, repository.BundleFilter) (repository.RepositoryBundle, error) {
				return repository.RepositoryBundle{}, boom
			}
		case "claim":
			ops.claim = func(context.Context, string, string, *string) (control.SchedulerClaim, error) {
				return control.SchedulerClaim{}, boom
			}
		case "previous":
			ops.previous = func(context.Context, *sql.DB, string) (*string, error) { return nil, boom }
		case "diff":
			ops.diff = func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error) {
				return nil, boom
			}
		case "upsert":
			ops.upsert = func(context.Context, string, []control.DetectedSignal) ([]control.DreamSignal, error) {
				return nil, boom
			}
		case "actionable":
			ops.actionable = func(context.Context) ([]control.DreamSignal, error) { return nil, boom }
		case "state":
			ops.setState = func(context.Context, *sql.DB, string, string) error { return boom }
		case "list":
			ops.list = func(context.Context) ([]control.DreamSignal, error) { return nil, boom }
		}
		if _, err = runtime.runDream(ctx, "report_only", now, ops); !errors.Is(err, boom) {
			t.Fatal(stage, err)
		}
		postClaim := stage == "previous" || stage == "diff" || stage == "upsert" || stage == "actionable"
		if postClaim && !failed {
			t.Fatal(stage, "not failed")
		}
	}
}
