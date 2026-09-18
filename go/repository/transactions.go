package repository

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rcarmo/memento/go/control"
)

type CheckpointError struct{ Name string }

func (e *CheckpointError) Error() string { return e.Name }

type TransactionConflictError struct{ Message string }

func (e *TransactionConflictError) Error() string { return e.Message }

// MutationError supplies the source diagnostic class at a callback boundary.
// Generic Go errors otherwise use their concrete Go type name.
type MutationError struct{ Class, Message string }

func (e *MutationError) Error() string { return e.Message }

type TransactionRequest struct {
	Operation                                                control.OperationRequest
	ExpectedRevision, CommitMessage, AuthorName, AuthorEmail string
}
type TransactionResult struct {
	Operation                    control.OperationRecord
	BaseRevision, ResultRevision string
	ChangedPaths                 []string
	MaterializedPath             string
	Replayed                     bool
}
type RecoveryRecord struct {
	OpID           string                 `json:"op_id"`
	State          control.OperationState `json:"state"`
	Classification string                 `json:"classification"`
	Revision       *string                `json:"revision"`
}
type MutationCallback func(context.Context, string) ([]string, error)
type DerivedUpdateCallback func(context.Context, string, string, []string) error

type TransactionManager struct {
	Operations    control.Operations
	Paths         GitRepositoryPaths
	Checkpoint    func(string) error
	DerivedUpdate DerivedUpdateCallback
	Now           func() time.Time
}

var transactionLocks = struct {
	sync.Mutex
	values map[string]*sync.Mutex
}{values: map[string]*sync.Mutex{}}

func transactionLock(paths GitRepositoryPaths) (*sync.Mutex, error) {
	return transactionLockAt(paths, filepath.Abs)
}
func transactionLockAt(paths GitRepositoryPaths, abs func(string) (string, error)) (*sync.Mutex, error) {
	absolute, err := abs(paths.BareDir)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		absolute = resolved
	}
	transactionLocks.Lock()
	defer transactionLocks.Unlock()
	lock := transactionLocks.values[absolute]
	if lock == nil {
		lock = &sync.Mutex{}
		transactionLocks.values[absolute] = lock
	}
	return lock, nil
}

// WithTransactionLock serialises service control mutations with Apply/Recover
// for this repository. The caller must hold WriterLease across processes. Like
// Apply, the callback must not recursively acquire this non-reentrant lock.
func WithTransactionLock(ctx context.Context, paths GitRepositoryPaths, fn func() error) error {
	return withTransactionLock(ctx, paths, fn, transactionLock)
}
func withTransactionLock(ctx context.Context, paths GitRepositoryPaths, fn func() error, getLock func(GitRepositoryPaths) (*sync.Mutex, error)) error {
	lock, err := getLock(paths)
	if err != nil {
		return err
	}
	lock.Lock()
	defer lock.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	return fn()
}

func (m *TransactionManager) hit(name string) error {
	if m.Checkpoint != nil {
		return m.Checkpoint(name)
	}
	return nil
}
func (m *TransactionManager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Apply serialises one repository in-process; the caller must hold WriterLease
// for cross-process exclusion. Callbacks must not recursively call Apply: unlike
// Python's RLock, Go mutex ownership is not associated with a goroutine.
func (m *TransactionManager) Apply(ctx context.Context, request TransactionRequest, mutate MutationCallback) (TransactionResult, error) {
	return m.applyWithLock(ctx, request, mutate, transactionLock)
}
func (m *TransactionManager) applyWithLock(ctx context.Context, request TransactionRequest, mutate MutationCallback, getLock func(GitRepositoryPaths) (*sync.Mutex, error)) (TransactionResult, error) {
	lock, err := getLock(m.Paths)
	if err != nil {
		return TransactionResult{}, err
	}
	lock.Lock()
	defer lock.Unlock()
	if err = ctx.Err(); err != nil {
		return TransactionResult{}, err
	}
	return m.ApplyUnderLock(ctx, request, mutate)
}

// ApplyUnderLock composes service policy checks and publication under one
// caller-held WithTransactionLock. The caller must also hold WriterLease.
// Calling Apply instead would recursively acquire the non-reentrant Go mutex.
func (m *TransactionManager) ApplyUnderLock(ctx context.Context, request TransactionRequest, mutate MutationCallback) (TransactionResult, error) {
	return m.apply(ctx, request, mutate, defaultTransactionGit())
}

type transactionGit struct {
	main        func(GitRepositoryPaths) (string, error)
	create      func(context.Context, GitRepositoryPaths, string, string) (Worktree, error)
	commit      func(context.Context, Worktree, []string, string, GitCommitIdentity, GitCommitIdentity) (StagedCommit, error)
	staged      func(string) ([]string, error)
	publish     func(GitRepositoryPaths, string, string) (bool, error)
	materialize func(context.Context, GitRepositoryPaths, string) (MaterializedCheckout, error)
	remove      func(GitRepositoryPaths, string) error
	resolve     func(string) (string, error)
	diff        func(context.Context, GitRepositoryPaths, string, string) ([]string, error)
}

func defaultTransactionGit() transactionGit {
	return transactionGit{GetMainRevision, CreateOperationWorktree, CommitExactPaths, ExactStagedPaths, PublishMainCompareAndSwap, MaterializeCurrentCheckout, RemoveOperationWorktree, ResolveWorktreeRevision, DiffMainPaths}
}
func (m *TransactionManager) apply(ctx context.Context, request TransactionRequest, mutate MutationCallback, git transactionGit) (result TransactionResult, err error) {
	operation, err := m.Operations.Create(ctx, request.Operation)
	if err != nil {
		return result, err
	}
	operationID := operation.OpID
	if operation.State == control.Succeeded && operation.ResultRevision != nil {
		payload, err := operation.ReplayPayload()
		if err != nil {
			return result, err
		}
		changed := []string{}
		if paths, ok := payload["changed_paths"].([]any); ok {
			for _, path := range paths {
				value, ok := path.(string)
				if !ok {
					return result, &GitError{Message: "non-string replay path is not supported"}
				}
				changed = append(changed, value)
			}
		}
		base := request.ExpectedRevision
		if operation.BaseRevision != nil && *operation.BaseRevision != "" {
			base = *operation.BaseRevision
		}
		return TransactionResult{Operation: operation, BaseRevision: base, ResultRevision: *operation.ResultRevision, ChangedPaths: changed, MaterializedPath: m.Paths.CurrentDir, Replayed: true}, nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = &MutationError{Class: "RuntimeError", Message: fmt.Sprintf("transaction callback panic: %v", recovered)}
		}
		var checkpoint *CheckpointError
		var conflict *TransactionConflictError
		var uncertain *PublishOutcomeError
		if errors.As(err, &checkpoint) {
			return
		}
		// Source checkpoint failures retain worktrees. Go adds the same retention
		// for an explicitly indeterminate ref durability result, so recovery can
		// inspect the original detached HEAD instead of losing publication evidence.
		if errors.As(err, &uncertain) {
			return
		}
		if err != nil && !errors.As(err, &conflict) {
			class := fmt.Sprintf("%T", err)
			var mutation *MutationError
			if errors.As(err, &mutation) {
				class = mutation.Class
			}
			if _, updateErr := m.Operations.MarkFailed(context.WithoutCancel(ctx), operationID, class, err.Error()); updateErr != nil {
				err = updateErr
			}
		}
		if cleanupErr := git.remove(m.Paths, operationID); cleanupErr != nil {
			err = cleanupErr
		}
	}()
	if err = m.hit("operation_inserted"); err != nil {
		return result, err
	}
	base, err := git.main(m.Paths)
	if err != nil {
		return result, err
	}
	if request.ExpectedRevision != base {
		message := "expected revision " + request.ExpectedRevision + " does not match " + base
		if _, err = m.Operations.MarkConflict(ctx, operation.OpID, message); err != nil {
			return result, err
		}
		return result, &TransactionConflictError{Message: message}
	}
	operation, err = m.Operations.MarkRunning(ctx, operation.OpID, base)
	if err != nil {
		return result, err
	}
	worktree, err := git.create(ctx, m.Paths, operation.OpID, base)
	if err != nil {
		return result, err
	}
	if err = m.hit("worktree_created"); err != nil {
		return result, err
	}
	changed, err := mutate(ctx, worktree.Path)
	if err != nil {
		return result, err
	}
	sort.Strings(changed)
	if err = m.hit("mutation_applied"); err != nil {
		return result, err
	}
	identity := GitCommitIdentity{Name: request.AuthorName, Email: request.AuthorEmail, When: m.now()}
	commit, err := git.commit(ctx, worktree, changed, request.CommitMessage, identity, identity)
	if err != nil {
		return result, err
	}
	staged, err := git.staged(worktree.Path)
	if err != nil {
		return result, err
	}
	if len(staged) > 0 {
		return result, &MutationError{Class: "RuntimeError", Message: "staging area must be clean after commit"}
	}
	if err = m.hit("commit_created"); err != nil {
		return result, err
	}
	published, err := git.publish(m.Paths, base, commit.Revision)
	if err != nil {
		return result, err
	}
	if !published {
		message := "repository head moved before publication"
		if _, err = m.Operations.MarkConflict(ctx, operation.OpID, message); err != nil {
			return result, err
		}
		return result, &TransactionConflictError{Message: message}
	}
	if err = m.hit("publication_complete"); err != nil {
		return result, err
	}
	checkout, err := git.materialize(ctx, m.Paths, commit.Revision)
	if err != nil {
		return result, err
	}
	if err = m.hit("current_materialized"); err != nil {
		return result, err
	}
	if m.DerivedUpdate != nil {
		if err = m.DerivedUpdate(ctx, checkout.Path, commit.Revision, commit.ChangedPaths); err != nil {
			return result, err
		}
	}
	if err = m.hit("derived_updated"); err != nil {
		return result, err
	}
	operation, err = m.Operations.MarkSucceeded(ctx, operation.OpID, commit.Revision, map[string]any{"changed_paths": commit.ChangedPaths})
	if err != nil {
		return result, err
	}
	if err = m.hit("operation_completed"); err != nil {
		return result, err
	}
	return TransactionResult{Operation: operation, BaseRevision: base, ResultRevision: commit.Revision, ChangedPaths: commit.ChangedPaths, MaterializedPath: checkout.Path}, nil
}
func (m *TransactionManager) RecoverStartup(ctx context.Context) ([]RecoveryRecord, error) {
	return m.recover(ctx, defaultTransactionGit())
}
func (m *TransactionManager) recover(ctx context.Context, git transactionGit) ([]RecoveryRecord, error) {
	head, err := git.main(m.Paths)
	if err != nil {
		return nil, err
	}
	operations, err := m.Operations.Interrupted(ctx)
	if err != nil {
		return nil, err
	}
	recovered := []RecoveryRecord{}
	for _, operation := range operations {
		revision, err := git.resolve(filepath.Join(m.Paths.WorktreesDir, operation.OpID))
		if err != nil {
			return nil, err
		}
		record := RecoveryRecord{OpID: operation.OpID, State: operation.State, Classification: "retryable"}
		if revision != "" {
			record.Revision = &revision
		}
		if revision == head && operation.BaseRevision != nil && revision != *operation.BaseRevision {
			changed, err := git.diff(ctx, m.Paths, *operation.BaseRevision, head)
			if err != nil {
				return nil, err
			}
			updated, err := m.Operations.MarkSucceeded(ctx, operation.OpID, head, map[string]any{"changed_paths": changed})
			if err != nil {
				return nil, err
			}
			record.State = updated.State
			record.Classification = "published"
			record.Revision = &head
		} else if operation.BaseRevision != nil && *operation.BaseRevision != head {
			updated, err := m.Operations.MarkConflict(ctx, operation.OpID, "interrupted operation is stale after startup recovery")
			if err != nil {
				return nil, err
			}
			record.State = updated.State
			record.Classification = "conflict"
			record.Revision = nil
		}
		recovered = append(recovered, record)
		if err = git.remove(m.Paths, operation.OpID); err != nil {
			return nil, err
		}
	}
	checkout, err := git.materialize(ctx, m.Paths, head)
	if err != nil {
		return nil, err
	}
	if m.DerivedUpdate != nil {
		if err = m.DerivedUpdate(ctx, checkout.Path, head, []string{}); err != nil {
			return nil, err
		}
	}
	return recovered, nil
}
