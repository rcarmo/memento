package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcarmo/memento/internal/control"
)

func transactionTest(t *testing.T) (*TransactionManager, TransactionRequest) {
	t.Helper()
	ctx := context.Background()
	paths := bootstrapPaths(t)
	stamp := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	boot, err := bootstrapRepository(ctx, paths, "", stamp, defaultBootstrapIO())
	if err != nil {
		t.Fatal(err)
	}
	db, err := control.Connect(ctx, filepath.Join(filepath.Dir(paths.BareDir), "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = control.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	m := &TransactionManager{Operations: control.Operations{DB: db, Now: func() time.Time { return stamp }}, Paths: paths, Now: func() time.Time { return stamp }}
	request := TransactionRequest{Operation: control.OperationRequest{OpID: "transaction-one", Principal: "agent", IdempotencyKey: "key", ToolName: "memory_write", RequestJSON: `{"synthetic":true}`}, ExpectedRevision: boot.Revision, CommitMessage: "synthetic transaction", AuthorName: "Synthetic Agent", AuthorEmail: "test@example.invalid"}
	return m, request
}
func syntheticMutation(_ context.Context, path string) ([]string, error) {
	if err := os.WriteFile(filepath.Join(path, "one.md"), []byte("committed\n"), 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(path, "ignored.md"), []byte("not committed\n"), 0600); err != nil {
		return nil, err
	}
	return []string{"/one.md"}, nil
}
func TestTransactionCheckpointReference(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/parity/repository-transactions.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Checkpoint, Error, Base, Main string
		Seen                          []string
		Worktree                      *string
		Before, After                 control.OperationRecord
		Recovery                      []RecoveryRecord
		Files                         []string
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, c := range fixtures {
		t.Run(c.Checkpoint, func(t *testing.T) {
			m, request := transactionTest(t)
			seen := []string{}
			m.Checkpoint = func(name string) error {
				seen = append(seen, name)
				if name == c.Checkpoint {
					return &CheckpointError{Name: name}
				}
				return nil
			}
			_, err := m.Apply(context.Background(), request, syntheticMutation)
			if c.Error != "" {
				var checkpoint *CheckpointError
				if !errors.As(err, &checkpoint) || err.Error() != c.Checkpoint {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(seen, c.Seen) {
				t.Fatal(seen, c.Seen)
			}
			before, err := m.Operations.Get(context.Background(), request.Operation.OpID)
			if err != nil || !reflect.DeepEqual(before, c.Before) {
				t.Fatal(before, c.Before, err)
			}
			main, err := GetMainRevision(m.Paths)
			if err != nil || main != c.Main || request.ExpectedRevision != c.Base {
				t.Fatal(main, c.Main, err)
			}
			work, err := ResolveWorktreeRevision(filepath.Join(m.Paths.WorktreesDir, request.Operation.OpID))
			if err != nil {
				t.Fatal(err)
			}
			expectedWork := ""
			if c.Worktree != nil {
				expectedWork = *c.Worktree
			}
			if work != expectedWork {
				t.Fatal(work, expectedWork)
			}
			m.Operations.DB.Close()
			db, err := control.Connect(context.Background(), filepath.Join(filepath.Dir(m.Paths.BareDir), "control.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			m.Operations.DB = db
			recovered, err := m.RecoverStartup(context.Background())
			if err != nil || !reflect.DeepEqual(recovered, c.Recovery) {
				t.Fatal(recovered, c.Recovery, err)
			}
			after, err := m.Operations.Get(context.Background(), request.Operation.OpID)
			if err != nil || !reflect.DeepEqual(after, c.After) {
				t.Fatal(after, c.After, err)
			}
			entries, err := os.ReadDir(m.Paths.CurrentDir)
			if err != nil {
				t.Fatal(err)
			}
			names := []string{}
			for _, entry := range entries {
				names = append(names, entry.Name())
			}
			if !reflect.DeepEqual(names, c.Files) {
				t.Fatal(names, c.Files)
			}
			m.Checkpoint = nil
			result, err := m.Apply(context.Background(), request, syntheticMutation)
			if err != nil {
				t.Fatal(err)
			}
			if result.Replayed != (after.State == control.Succeeded) {
				t.Fatal(result)
			}
		})
	}
}
func TestTransactionConcurrencyConflictAndRetry(t *testing.T) {
	m, request := transactionTest(t)
	ctx := context.Background()
	var workers sync.WaitGroup
	results := make(chan TransactionResult, 8)
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result, err := m.Apply(ctx, request, syntheticMutation)
			if err != nil {
				t.Error(err)
			} else {
				results <- result
			}
		}()
	}
	workers.Wait()
	close(results)
	applied := 0
	for result := range results {
		if !result.Replayed {
			applied++
		}
	}
	if applied != 1 {
		t.Fatal(applied)
	}
	bad := request
	bad.Operation.OpID = "two"
	bad.Operation.IdempotencyKey = "two"
	if _, err := m.Apply(ctx, bad, syntheticMutation); err == nil {
		t.Fatal("stale expected revision")
	}
	m, request = transactionTest(t)
	_, err := m.Apply(ctx, request, func(context.Context, string) ([]string, error) {
		return nil, &MutationError{Class: "ValueError", Message: "synthetic"}
	})
	if err == nil {
		t.Fatal("mutation error")
	}
	record, err := m.Operations.Get(ctx, request.Operation.OpID)
	if err != nil || record.State != control.Failed || *record.ErrorClass != "ValueError" {
		t.Fatal(record, err)
	}
	result, err := m.Apply(ctx, request, syntheticMutation)
	if err != nil || result.Replayed {
		t.Fatal(result, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = m.Apply(canceled, request, syntheticMutation); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestTransactionInjectedFailures(t *testing.T) {
	for _, kind := range []string{"create-op", "main", "running-db", "stale-db", "worktree", "commit", "staged-error", "staged-dirty", "publish-error", "publish-conflict", "publish-conflict-db", "materialize", "derived", "succeeded-db", "cleanup", "failed-db", "uncertain"} {
		t.Run(kind, func(t *testing.T) {
			m, request := transactionTest(t)
			git := defaultTransactionGit()
			ctx := context.Background()
			switch kind {
			case "create-op":
				m.Operations.DB.Close()
			case "main":
				git.main = func(GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
			case "running-db", "stale-db":
				m.Checkpoint = func(string) error { m.Operations.DB.Close(); return nil }
				if kind == "stale-db" {
					request.ExpectedRevision = "stale"
				}
			case "worktree":
				git.create = func(context.Context, GitRepositoryPaths, string, string) (Worktree, error) {
					return Worktree{}, io.ErrClosedPipe
				}
			case "commit":
				git.commit = func(context.Context, Worktree, []string, string, GitCommitIdentity, GitCommitIdentity) (StagedCommit, error) {
					return StagedCommit{}, io.ErrClosedPipe
				}
			case "staged-error":
				git.staged = func(string) ([]string, error) { return nil, io.ErrClosedPipe }
			case "staged-dirty":
				git.staged = func(string) ([]string, error) { return []string{"/uncommitted"}, nil }
			case "publish-error":
				git.publish = func(GitRepositoryPaths, string, string) (bool, error) { return false, io.ErrClosedPipe }
			case "publish-conflict", "publish-conflict-db":
				git.publish = func(GitRepositoryPaths, string, string) (bool, error) {
					if kind == "publish-conflict-db" {
						m.Operations.DB.Close()
					}
					return false, nil
				}
			case "materialize":
				git.materialize = func(context.Context, GitRepositoryPaths, string) (MaterializedCheckout, error) {
					return MaterializedCheckout{}, io.ErrClosedPipe
				}
			case "derived":
				m.DerivedUpdate = func(context.Context, string, string, []string) error { return io.ErrClosedPipe }
			case "succeeded-db":
				m.Checkpoint = func(name string) error {
					if name == "derived_updated" {
						m.Operations.DB.Close()
					}
					return nil
				}
			case "cleanup":
				git.remove = func(GitRepositoryPaths, string) error { return io.ErrClosedPipe }
			case "failed-db":
				git.main = func(GitRepositoryPaths) (string, error) { m.Operations.DB.Close(); return "", io.ErrClosedPipe }
			case "uncertain":
				publish := git.publish
				git.publish = func(p GitRepositoryPaths, base, next string) (bool, error) {
					ok, err := publish(p, base, next)
					if err != nil {
						return ok, err
					}
					return ok, &PublishOutcomeError{Revision: next, Cause: io.ErrClosedPipe}
				}
			}
			_, err := m.apply(ctx, request, syntheticMutation, git)
			if err == nil {
				t.Fatal("failure ignored")
			}
			_ = err.Error()
			if kind == "uncertain" {
				record, err := m.Operations.Get(ctx, request.Operation.OpID)
				if err != nil || record.State != control.Running {
					t.Fatal(record, err)
				}
				recovered, err := m.RecoverStartup(ctx)
				if err != nil || len(recovered) != 1 || recovered[0].Classification != "published" {
					t.Fatal(recovered, err)
				}
			}
		})
	}
}
func TestTransactionReplayVariants(t *testing.T) {
	m, request := transactionTest(t)
	ctx := context.Background()
	_, err := m.Apply(ctx, request, syntheticMutation)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"{}", "null", `{"changed_paths":"not a list"}`, `{"changed_paths":[42]}`, "{"} {
		_, _ = m.Operations.DB.Exec("UPDATE operations SET result_json=?,base_revision=NULL", raw)
		result, err := m.Apply(ctx, request, syntheticMutation)
		bad := raw == "{" || strings.Contains(raw, "42")
		if (err != nil) != bad {
			t.Fatal(raw, result, err)
		}
		if !bad && (!result.Replayed || len(result.ChangedPaths) != 0 || result.BaseRevision != request.ExpectedRevision) {
			t.Fatal(result)
		}
	}
	if (m.now()).IsZero() {
		t.Fatal("injected clock")
	}
	m.Now = nil
	if (m.now()).IsZero() {
		t.Fatal("real clock")
	}
}
func TestRecoveryInjectedFailuresAndConflict(t *testing.T) {
	for _, kind := range []string{"main", "list", "resolve", "diff", "mark-success", "conflict", "mark-conflict", "remove", "materialize", "derived"} {
		t.Run(kind, func(t *testing.T) {
			m, request := transactionTest(t)
			ctx := context.Background()
			m.Checkpoint = func(name string) error {
				if name == "publication_complete" {
					return &CheckpointError{Name: name}
				}
				return nil
			}
			_, err := m.Apply(ctx, request, syntheticMutation)
			if err == nil {
				t.Fatal("checkpoint")
			}
			git := defaultTransactionGit()
			switch kind {
			case "main":
				git.main = func(GitRepositoryPaths) (string, error) { return "", io.ErrClosedPipe }
			case "list":
				m.Operations.DB.Close()
			case "resolve":
				git.resolve = func(string) (string, error) { return "", io.ErrClosedPipe }
			case "diff":
				git.diff = func(context.Context, GitRepositoryPaths, string, string) ([]string, error) {
					return nil, io.ErrClosedPipe
				}
			case "mark-success":
				git.diff = func(context.Context, GitRepositoryPaths, string, string) ([]string, error) {
					m.Operations.DB.Close()
					return nil, nil
				}
			case "conflict", "mark-conflict":
				git.resolve = func(string) (string, error) {
					if kind == "mark-conflict" {
						m.Operations.DB.Close()
					}
					return "another", nil
				}
			case "remove":
				git.remove = func(GitRepositoryPaths, string) error { return io.ErrClosedPipe }
			case "materialize":
				git.materialize = func(context.Context, GitRepositoryPaths, string) (MaterializedCheckout, error) {
					return MaterializedCheckout{}, io.ErrClosedPipe
				}
			case "derived":
				m.DerivedUpdate = func(context.Context, string, string, []string) error { return io.ErrClosedPipe }
			}
			recovered, err := m.recover(ctx, git)
			if kind == "conflict" {
				if err != nil || len(recovered) != 1 || recovered[0].Classification != "conflict" || recovered[0].Revision != nil {
					t.Fatal(recovered, err)
				}
			} else if err == nil {
				t.Fatal(kind)
			}
		})
	}
	m, request := transactionTest(t)
	calls := 0
	m.DerivedUpdate = func(context.Context, string, string, []string) error { calls++; return nil }
	if _, err := m.Apply(context.Background(), request, syntheticMutation); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RecoverStartup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}

func TestTransactionLockAliasAndMissingCWD(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "alias")
	real := filepath.Join(root, "real")
	_ = os.Mkdir(real, 0700)
	_ = os.Symlink(real, link)
	a, err := transactionLock(GitRepositoryPaths{BareDir: real})
	if err != nil {
		t.Fatal(err)
	}
	b, err := transactionLock(GitRepositoryPaths{BareDir: link})
	if err != nil || a != b {
		t.Fatal("aliases not serialized", err)
	}
	// No concurrent test changes cwd: isolate the deleted-working-directory case.
	if os.Getenv("MEMENTO_GO_TRANSACTION_CWD_TEST") == "1" {
		cwd, err := os.MkdirTemp("", "memento-cwd-")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
		if err = os.Remove(cwd); err != nil {
			t.Fatal(err)
		}
		m := &TransactionManager{Paths: GitRepositoryPaths{BareDir: "relative"}}
		if _, err = m.Apply(context.Background(), TransactionRequest{}, nil); err == nil {
			t.Fatal("missing cwd accepted")
		}
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^TestTransactionLockAliasAndMissingCWD$")
	command.Env = append(os.Environ(), "MEMENTO_GO_TRANSACTION_CWD_TEST=1")
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatal(err, string(raw))
	}
}

func TestTransactionLockResolutionError(t *testing.T) {
	get := func(paths GitRepositoryPaths) (*sync.Mutex, error) {
		return transactionLockAt(paths, func(string) (string, error) { return "", io.ErrClosedPipe })
	}
	m := &TransactionManager{}
	if _, err := m.applyWithLock(context.Background(), TransactionRequest{}, nil, get); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}

func TestTransactionUpdateFailureKeepsOriginalIdentity(t *testing.T) {
	m, request := transactionTest(t)
	ctx := context.Background()
	if _, err := m.Operations.DB.Exec(`CREATE TRIGGER fail_success BEFORE UPDATE ON operations WHEN NEW.state='succeeded' BEGIN SELECT RAISE(ABORT, 'synthetic failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Apply(ctx, request, syntheticMutation); err == nil {
		t.Fatal("success write failure ignored")
	}
	record, err := m.Operations.Get(ctx, request.Operation.OpID)
	if err != nil || record.State != control.Failed {
		t.Fatal(record, err)
	}
	if _, err = os.Stat(filepath.Join(m.Paths.WorktreesDir, request.Operation.OpID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("original worktree leaked", err)
	}
	m, request = transactionTest(t)
	if _, err = m.Apply(ctx, request, func(context.Context, string) ([]string, error) { panic("synthetic") }); err == nil {
		t.Fatal("panic accepted")
	}
	record, err = m.Operations.Get(ctx, request.Operation.OpID)
	if err != nil || record.State != control.Failed {
		t.Fatal(record, err)
	}
}

func TestTransactionProcessDeathRecovery(t *testing.T) {
	if raw := os.Getenv("MEMENTO_GO_CRASH_TRANSACTION"); raw != "" {
		var config struct {
			Paths      GitRepositoryPaths
			Request    TransactionRequest
			Checkpoint string
		}
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			t.Fatal(err)
		}
		db, err := control.Connect(context.Background(), filepath.Join(filepath.Dir(config.Paths.BareDir), "control.sqlite"))
		if err != nil {
			t.Fatal(err)
		}
		stamp := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
		m := &TransactionManager{Paths: config.Paths, Operations: control.Operations{DB: db, Now: func() time.Time { return stamp }}, Now: func() time.Time { return stamp }}
		m.Checkpoint = func(name string) error {
			if name == config.Checkpoint {
				if _, err = os.Stdout.WriteString("ready\n"); err != nil {
					return err
				}
				_, _ = io.ReadFull(os.Stdin, make([]byte, 1))
				return errors.New("parent did not kill checkpoint process")
			}
			return nil
		}
		if _, err = m.Apply(context.Background(), config.Request, syntheticMutation); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, checkpoint := range []string{"commit_created", "publication_complete"} {
		t.Run(checkpoint, func(t *testing.T) {
			m, request := transactionTest(t)
			m.Operations.DB.Close()
			raw, err := json.Marshal(struct {
				Paths      GitRepositoryPaths
				Request    TransactionRequest
				Checkpoint string
			}{m.Paths, request, checkpoint})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(os.Args[0], "-test.run=^TestTransactionProcessDeathRecovery$")
			command.Env = append(os.Environ(), "MEMENTO_GO_CRASH_TRANSACTION="+string(raw))
			stdin, err := command.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			stdout, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
			ready := make(chan error, 1)
			go func() {
				signal := make([]byte, 6)
				_, err := io.ReadFull(stdout, signal)
				if err == nil && string(signal) != "ready\n" {
					err = fmt.Errorf("unexpected signal: %q", signal)
				}
				ready <- err
			}()
			select {
			case err = <-ready:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("checkpoint not reached")
			}
			if err = command.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			_ = command.Wait()
			db, err := control.Connect(context.Background(), filepath.Join(filepath.Dir(m.Paths.BareDir), "control.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			m.Operations.DB = db
			recovered, err := m.RecoverStartup(context.Background())
			want := "retryable"
			if checkpoint == "publication_complete" {
				want = "published"
			}
			if err != nil || len(recovered) != 1 || recovered[0].Classification != want {
				t.Fatal(recovered, err)
			}
			result, err := m.Apply(context.Background(), request, syntheticMutation)
			if err != nil || result.Replayed != (want == "published") {
				t.Fatal(result, err)
			}
		})
	}
}

func TestControlTransactionLock(t *testing.T) {
	ctx := context.Background()
	paths := GitRepositoryPaths{BareDir: filepath.Join(t.TempDir(), "repo.git")}
	injected := errors.New("lock lookup failed")
	if err := withTransactionLock(ctx, paths, func() error { t.Fatal("callback"); return nil }, func(GitRepositoryPaths) (*sync.Mutex, error) { return nil, injected }); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := WithTransactionLock(canceled, paths, func() error { t.Fatal("cancelled callback"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := WithTransactionLock(ctx, paths, func() error { return injected }); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	lock, err := transactionLock(paths)
	if err != nil {
		t.Fatal(err)
	}
	lock.Lock()
	started := make(chan struct{})
	called := make(chan struct{})
	go func() {
		close(started)
		_ = WithTransactionLock(ctx, paths, func() error { close(called); return nil })
	}()
	<-started
	select {
	case <-called:
		t.Fatal("control lock does not share Apply lock")
	case <-time.After(10 * time.Millisecond):
	}
	lock.Unlock()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("control lock stuck")
	}
}

func TestTryTransactionLockErrors(t *testing.T) {
	ctx := context.Background()
	paths := GitRepositoryPaths{}
	cause := errors.New("probe failure")
	if ok, err := tryTransactionLock(ctx, paths, func() error { return nil }, func(GitRepositoryPaths) (*sync.Mutex, error) { return nil, cause }); ok || !errors.Is(err, cause) {
		t.Fatal(ok, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if ok, err := tryTransactionLock(canceled, paths, func() error { t.Fatal("cancelled callback"); return nil }, func(GitRepositoryPaths) (*sync.Mutex, error) { return &sync.Mutex{}, nil }); !ok || !errors.Is(err, context.Canceled) {
		t.Fatal(ok, err)
	}
	lock := &sync.Mutex{}
	lock.Lock()
	if ok, err := tryTransactionLock(ctx, paths, func() error { t.Fatal("locked callback"); return nil }, func(GitRepositoryPaths) (*sync.Mutex, error) { return lock, nil }); ok || err != nil {
		t.Fatal(ok, err)
	}
	lock.Unlock()
	if ok, err := TryTransactionLock(ctx, GitRepositoryPaths{BareDir: t.TempDir()}, func() error { return cause }); !ok || !errors.Is(err, cause) {
		t.Fatal(ok, err)
	}
}
