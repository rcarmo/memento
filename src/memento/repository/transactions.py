from __future__ import annotations

import sqlite3
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from memento.control.checkpoints import CheckpointError, CheckpointHook
from memento.control.operations import (
    OperationRecord,
    OperationRequest,
    OperationState,
    create_operation,
    list_interrupted_operations,
    mark_operation_conflict,
    mark_operation_failed,
    mark_operation_running,
    mark_operation_succeeded,
)
from memento.repository.git import (
    GitRepositoryPaths,
    commit_exact_paths,
    create_operation_worktree,
    exact_staged_paths,
    get_main_revision,
    materialize_current_checkout,
    publish_main_compare_and_swap,
    remove_operation_worktree,
    resolve_worktree_revision,
)


class TransactionConflictError(RuntimeError):
    """Raised when an optimistic concurrency check fails."""


MutationCallback = Callable[[Path], tuple[str, ...]]


@dataclass(frozen=True, slots=True)
class TransactionRequest:
    operation: OperationRequest
    expected_revision: str
    commit_message: str
    author_name: str
    author_email: str


@dataclass(frozen=True, slots=True)
class TransactionResult:
    operation: OperationRecord
    base_revision: str
    result_revision: str
    changed_paths: tuple[str, ...]
    materialized_path: Path
    replayed: bool = False


@dataclass(frozen=True, slots=True)
class RecoveryRecord:
    op_id: str
    state: OperationState
    classification: str
    revision: str | None


class TransactionManager:
    def __init__(
        self,
        connection: sqlite3.Connection,
        paths: GitRepositoryPaths,
        *,
        checkpoints: CheckpointHook | None = None,
    ) -> None:
        self._connection = connection
        self._paths = paths
        self._checkpoints = checkpoints or CheckpointHook()

    def apply(self, request: TransactionRequest, mutate: MutationCallback) -> TransactionResult:
        operation = create_operation(self._connection, request.operation)
        if operation.state is OperationState.SUCCEEDED and operation.result_revision is not None:
            payload = operation.replay_payload or {}
            replayed_paths = payload.get("changed_paths", ())
            if not isinstance(replayed_paths, list):
                replayed_paths = []
            return TransactionResult(
                operation=operation,
                base_revision=operation.base_revision or request.expected_revision,
                result_revision=operation.result_revision,
                changed_paths=tuple(str(path) for path in replayed_paths),
                materialized_path=self._paths.current_dir,
                replayed=True,
            )
        cleanup_worktree = True
        try:
            self._checkpoints.hit("operation_inserted")
            base_revision = get_main_revision(self._paths)
            if request.expected_revision != base_revision:
                conflict = mark_operation_conflict(
                    self._connection,
                    request.operation.op_id,
                    error_message=(
                        "expected revision "
                        f"{request.expected_revision} does not match {base_revision}"
                    ),
                )
                raise TransactionConflictError(conflict.error_message or "revision conflict")
            operation = mark_operation_running(
                self._connection,
                request.operation.op_id,
                base_revision=base_revision,
            )
            worktree = create_operation_worktree(
                self._paths,
                op_id=request.operation.op_id,
                base_revision=base_revision,
            )
            self._checkpoints.hit("worktree_created")
            changed_paths = tuple(sorted(mutate(worktree.path)))
            self._checkpoints.hit("mutation_applied")
            commit = commit_exact_paths(
                worktree,
                changed_paths=changed_paths,
                message=request.commit_message,
                author_name=request.author_name,
                author_email=request.author_email,
            )
            if exact_staged_paths(worktree.path):
                raise RuntimeError("staging area must be clean after commit")
            self._checkpoints.hit("commit_created")
            published = publish_main_compare_and_swap(
                self._paths,
                base_revision=base_revision,
                new_revision=commit.revision,
            )
            if not published:
                mark_operation_conflict(
                    self._connection,
                    request.operation.op_id,
                    error_message="repository head moved before publication",
                )
                raise TransactionConflictError("repository head moved before publication")
            self._checkpoints.hit("publication_complete")
            materialized = materialize_current_checkout(self._paths, revision=commit.revision)
            self._checkpoints.hit("current_materialized")
            operation = mark_operation_succeeded(
                self._connection,
                request.operation.op_id,
                result_revision=commit.revision,
                result={"changed_paths": list(commit.changed_paths)},
            )
            self._checkpoints.hit("operation_completed")
            return TransactionResult(
                operation=operation,
                base_revision=base_revision,
                result_revision=commit.revision,
                changed_paths=commit.changed_paths,
                materialized_path=materialized.path,
            )
        except TransactionConflictError:
            raise
        except CheckpointError:
            cleanup_worktree = False
            raise
        except Exception as exc:
            mark_operation_failed(
                self._connection,
                request.operation.op_id,
                error_class=exc.__class__.__name__,
                error_message=str(exc),
            )
            raise
        finally:
            if cleanup_worktree:
                remove_operation_worktree(self._paths, request.operation.op_id)

    def recover_startup(self) -> tuple[RecoveryRecord, ...]:
        head_revision = get_main_revision(self._paths)
        recovered: list[RecoveryRecord] = []
        for operation in list_interrupted_operations(self._connection):
            worktree_path = self._paths.worktrees_dir / operation.op_id
            worktree_revision = resolve_worktree_revision(worktree_path)
            if worktree_revision == head_revision:
                updated = mark_operation_succeeded(
                    self._connection,
                    operation.op_id,
                    result_revision=head_revision,
                    result={"changed_paths": _worktree_tracked_paths(worktree_path)},
                )
                recovered.append(
                    RecoveryRecord(
                        op_id=updated.op_id,
                        state=updated.state,
                        classification="published",
                        revision=head_revision,
                    )
                )
            elif operation.base_revision is not None and operation.base_revision != head_revision:
                updated = mark_operation_conflict(
                    self._connection,
                    operation.op_id,
                    error_message="interrupted operation is stale after startup recovery",
                )
                recovered.append(
                    RecoveryRecord(
                        op_id=updated.op_id,
                        state=updated.state,
                        classification="conflict",
                        revision=None,
                    )
                )
            else:
                recovered.append(
                    RecoveryRecord(
                        op_id=operation.op_id,
                        state=operation.state,
                        classification="retryable",
                        revision=worktree_revision,
                    )
                )
            remove_operation_worktree(self._paths, operation.op_id)
        materialize_current_checkout(self._paths, revision=head_revision)
        return tuple(recovered)


def _worktree_tracked_paths(worktree_path: Path) -> list[str]:
    if not worktree_path.exists():
        return []
    tracked: list[str] = []
    for path in sorted(worktree_path.rglob("*.md")):
        tracked.append("/" + path.relative_to(worktree_path).as_posix())
    return tracked
