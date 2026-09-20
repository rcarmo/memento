package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/rcarmo/memento/internal/control"
	"github.com/rcarmo/memento/internal/repository"
	"strconv"
	"time"
)

type dreamRunOps struct {
	revision   func(repository.GitRepositoryPaths) (string, error)
	scan       func(string, repository.BundleFilter) (repository.RepositoryBundle, error)
	previous   func(context.Context, *sql.DB, string) (*string, error)
	diff       func(context.Context, repository.GitRepositoryPaths, string, string) ([]string, error)
	claim      func(context.Context, string, string, *string) (control.SchedulerClaim, error)
	upsert     func(context.Context, string, []control.DetectedSignal) ([]control.DreamSignal, error)
	actionable func(context.Context) ([]control.DreamSignal, error)
	list       func(context.Context) ([]control.DreamSignal, error)
	finish     func(context.Context, string, string, *string, int, int, []control.ModelAttempt, *string) (control.SchedulerRunRecord, error)
	setState   func(context.Context, *sql.DB, string, string) error
}

func (r *Runtime) defaultDreamRunOps() dreamRunOps {
	s := control.Scheduler{DB: r.DB}
	signals := control.Signals{DB: r.DB}
	return dreamRunOps{repository.GetMainRevision, repository.ScanBundle, control.GetServiceState, repository.DiffMainPaths, s.Claim, signals.Upsert, signals.Actionable, signals.List, s.Finish, control.SetServiceState}
}
func (r *Runtime) RunDream(ctx context.Context, mode string, now time.Time) (map[string]any, error) {
	return r.runDream(ctx, mode, now, r.defaultDreamRunOps())
}
func (r *Runtime) runDream(ctx context.Context, mode string, now time.Time, ops dreamRunOps) (map[string]any, error) {
	if mode == "" {
		mode = r.Dream.Mode
	}
	if mode == "disabled" {
		return map[string]any{"ok": true, "mode": mode, "state": "disabled"}, nil
	}
	if mode != "report_only" && mode != "propose" {
		return nil, errors.New("dream mode must be disabled, report_only, or propose")
	}
	if mode == "propose" && r.ModelClient == nil {
		return nil, errors.New("dream propose requires a configured model provider")
	}
	started := time.Now()
	revision, err := ops.revision(r.Paths.Repository)
	if err != nil {
		return nil, err
	}
	bundle, err := ops.scan(r.Paths.Repository.CurrentDir, repository.BundleFilter{})
	if err != nil {
		return nil, err
	}
	if quiet := DreamQuietUntil(bundle, now.Unix(), r.Dream.QuietPeriodSeconds); quiet != nil {
		return map[string]any{"ok": true, "mode": mode, "state": "quiet_period", "repo_revision": revision, "quiet_until": *quiet}, nil
	}
	window := strconv.FormatInt(now.Unix()/int64(r.Dream.IntervalSeconds), 10)
	claim, err := ops.claim(ctx, "dream", window, &revision)
	if err != nil {
		var conflict *control.SchedulerConflictError
		if errors.As(err, &conflict) {
			return map[string]any{"ok": true, "mode": mode, "state": "skipped_overlap", "repo_revision": revision, "window_key": window}, nil
		}
		return nil, err
	}
	if !claim.Created {
		return map[string]any{"ok": true, "mode": mode, "state": "skipped_duplicate_window", "repo_revision": revision, "window_key": window, "run_id": claim.Record.RunID}, nil
	}
	failed := func(cause error) error {
		message := cause.Error()
		_, finishErr := ops.finish(ctx, claim.Record.RunID, "failed", &revision, 0, 0, nil, &message)
		return errors.Join(cause, finishErr)
	}
	previous, err := ops.previous(ctx, r.DB, "last_dream_revision")
	if err != nil {
		return nil, failed(err)
	}
	changed := map[string]bool{}
	if previous != nil && *previous != revision {
		paths, diffErr := ops.diff(ctx, r.Paths.Repository, *previous, revision)
		if diffErr != nil {
			return nil, failed(diffErr)
		}
		for _, path := range paths {
			changed[path] = true
		}
	}
	prior := ""
	if previous != nil {
		prior = *previous
	}
	assetPaths, err := acceptedBundleAssets(bundle)
	if err != nil {
		return nil, failed(err)
	}
	detections := detectDreamSignals(bundle, revision, prior, changed, r.Dream.Scanner, assetPaths)
	if len(detections) > r.Dream.Budgets.MaxSignalsPerRun {
		detections = detections[:r.Dream.Budgets.MaxSignalsPerRun]
	}
	signals, err := ops.upsert(ctx, revision, detections)
	if err != nil {
		return nil, failed(err)
	}
	actionable, err := ops.actionable(ctx)
	if err != nil {
		return nil, failed(err)
	}
	proposalCount := 0
	var modelChain []control.ModelAttempt
	if mode == "propose" {
		remaining := time.Duration(r.Dream.Budgets.MaxRuntimeSeconds*float64(time.Second)) - time.Since(started)
		proposalCount, modelChain, err = r.generateDreamProposal(ctx, actionable, revision, remaining, now)
		if err != nil {
			return nil, failed(err)
		}
	}
	if _, err = ops.finish(ctx, claim.Record.RunID, "succeeded", &revision, len(signals), proposalCount, modelChain, nil); err != nil {
		return nil, err
	}
	if err = ops.setState(ctx, r.DB, "last_dream_revision", revision); err != nil {
		return nil, err
	}
	all, err := ops.list(ctx)
	if err != nil {
		return nil, err
	}
	payload := []any{}
	for _, signal := range all {
		payload = append(payload, map[string]any{"type": signal.SignalType, "status": signal.Status, "dedupe_key": signal.DedupeKey, "entities": append([]string{}, signal.EntityRefs...)})
	}
	return map[string]any{"ok": true, "mode": mode, "state": "succeeded", "run_id": claim.Record.RunID, "window_key": window, "repo_revision": revision, "signal_count": len(signals), "actionable_signal_count": len(actionable), "proposal_count": proposalCount, "signals": payload}, nil
}
