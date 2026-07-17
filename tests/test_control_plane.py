from __future__ import annotations

import sqlite3
from collections.abc import Generator
from pathlib import Path

import pytest

from memento.control.checkpoints import CheckpointError, CheckpointHook, FailAtCheckpoint
from memento.control.db import connect_control_db, migrate_control_db
from memento.control.operations import (
    IdempotencyConflictError,
    OperationRequest,
    OperationState,
    create_operation,
    get_operation,
)
from memento.repository.git import GitRepositoryPaths, bootstrap_repository, get_main_revision
from memento.repository.lease import WriterLeaseError, acquire_writer_lease
from memento.repository.transactions import (
    TransactionConflictError,
    TransactionManager,
    TransactionRequest,
)


@pytest.fixture()
def control_connection(tmp_path: Path) -> Generator[sqlite3.Connection, None, None]:
    connection = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(connection)
    yield connection
    connection.close()


@pytest.fixture()
def repo_paths(tmp_path: Path) -> GitRepositoryPaths:
    seed = tmp_path / "seed"
    (seed / "instances").mkdir(parents=True)
    (seed / "instances" / "smith.md").write_text(
        "# Smith\n",
        encoding="utf-8",
    )
    paths = GitRepositoryPaths(
        bare_dir=tmp_path / "repo.git",
        current_dir=tmp_path / "current",
        worktrees_dir=tmp_path / "worktrees",
    )
    bootstrap_repository(paths, seed)
    return paths


def test_control_db_migrations_enable_wal_and_v1_tables(
    control_connection: sqlite3.Connection,
) -> None:
    mode = control_connection.execute("PRAGMA journal_mode").fetchone()[0]
    foreign_keys = control_connection.execute("PRAGMA foreign_keys").fetchone()[0]
    tables = {
        row[0]
        for row in control_connection.execute(
            "SELECT name FROM sqlite_master WHERE type = 'table'"
        ).fetchall()
    }
    schema_version = control_connection.execute(
        "SELECT value FROM service_state WHERE key = 'schema_version'"
    ).fetchone()[0]

    assert mode.lower() == "wal"
    assert foreign_keys == 1
    assert {"operations", "proposals", "scheduler_runs", "service_state", "dream_signals"} <= tables
    assert schema_version == "3"


def test_idempotency_replays_per_principal_and_rejects_payload_conflicts(
    control_connection: sqlite3.Connection,
) -> None:
    request = OperationRequest(
        op_id="op-1",
        principal="smith",
        idempotency_key="idem-1",
        tool_name="memory_patch",
        request_json='{"title":"Smith"}',
    )
    first = create_operation(control_connection, request)
    replay = create_operation(control_connection, request)

    assert first.op_id == replay.op_id
    assert replay.state is OperationState.QUEUED

    other_principal = create_operation(
        control_connection,
        OperationRequest(
            op_id="op-2",
            principal="flint",
            idempotency_key="idem-1",
            tool_name="memory_patch",
            request_json='{"title":"Smith"}',
        ),
    )
    assert other_principal.op_id == "op-2"

    with pytest.raises(IdempotencyConflictError):
        create_operation(
            control_connection,
            OperationRequest(
                op_id="op-3",
                principal="smith",
                idempotency_key="idem-1",
                tool_name="memory_patch",
                request_json='{"title":"Changed"}',
            ),
        )


def test_writer_lease_reports_contention(tmp_path: Path) -> None:
    lock_path = tmp_path / "locks" / "writer.lock"
    first = acquire_writer_lease(lock_path, owner="writer-a")
    try:
        with pytest.raises(WriterLeaseError, match="writer-a"):
            acquire_writer_lease(lock_path, owner="writer-b")
    finally:
        first.release()


def test_transaction_pipeline_commits_and_materializes_current(
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
) -> None:
    manager = TransactionManager(control_connection, repo_paths)
    expected_revision = get_main_revision(repo_paths)

    result = manager.apply(
        TransactionRequest(
            operation=OperationRequest(
                op_id="op-success",
                principal="smith",
                idempotency_key="idem-success",
                tool_name="memory_patch",
                request_json='{"path":"/instances/smith.md"}',
            ),
            expected_revision=expected_revision,
            commit_message="memory: update smith",
            author_name="Rui Carmo",
            author_email="rui.carmo@gmail.com",
        ),
        lambda worktree: _write_file(
            worktree,
            "/instances/smith.md",
            "# Smith\n\nUpdated.\n",
        ),
    )

    assert result.replayed is False
    assert result.base_revision == expected_revision
    assert result.changed_paths == ("/instances/smith.md",)
    assert (
        (repo_paths.current_dir / "instances" / "smith.md")
        .read_text(encoding="utf-8")
        .endswith("Updated.\n")
    )
    assert get_operation(control_connection, "op-success").state is OperationState.SUCCEEDED


def test_transaction_pipeline_rejects_stale_revision(
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
) -> None:
    manager = TransactionManager(control_connection, repo_paths)

    with pytest.raises(TransactionConflictError):
        manager.apply(
            TransactionRequest(
                operation=OperationRequest(
                    op_id="op-stale",
                    principal="smith",
                    idempotency_key="idem-stale",
                    tool_name="memory_patch",
                    request_json='{"path":"/instances/smith.md"}',
                ),
                expected_revision="deadbeef",
                commit_message="memory: stale update",
                author_name="Rui Carmo",
                author_email="rui.carmo@gmail.com",
            ),
            lambda worktree: _write_file(
                worktree,
                "/instances/smith.md",
                "# Smith\n",
            ),
        )

    assert get_operation(control_connection, "op-stale").state is OperationState.CONFLICT


def test_transaction_pipeline_stages_exact_paths_only(
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
) -> None:
    manager = TransactionManager(control_connection, repo_paths)

    result = manager.apply(
        TransactionRequest(
            operation=OperationRequest(
                op_id="op-exact",
                principal="smith",
                idempotency_key="idem-exact",
                tool_name="memory_patch",
                request_json='{"path":"/instances/smith.md"}',
            ),
            expected_revision=get_main_revision(repo_paths),
            commit_message="memory: exact paths",
            author_name="Rui Carmo",
            author_email="rui.carmo@gmail.com",
        ),
        _mutate_with_extra_unstaged_file,
    )

    assert result.changed_paths == ("/instances/smith.md",)
    assert not (repo_paths.current_dir / "instances" / "ignored.md").exists()


def test_startup_recovery_classifies_publication_after_crash(
    control_connection: sqlite3.Connection,
    repo_paths: GitRepositoryPaths,
) -> None:
    manager = TransactionManager(
        control_connection,
        repo_paths,
        checkpoints=CheckpointHook(callback=FailAtCheckpoint("publication_complete")),
    )

    with pytest.raises(CheckpointError):
        manager.apply(
            TransactionRequest(
                operation=OperationRequest(
                    op_id="op-crash",
                    principal="smith",
                    idempotency_key="idem-crash",
                    tool_name="memory_patch",
                    request_json='{"path":"/instances/smith.md"}',
                ),
                expected_revision=get_main_revision(repo_paths),
                commit_message="memory: crash after publication",
                author_name="Rui Carmo",
                author_email="rui.carmo@gmail.com",
            ),
            lambda worktree: _write_file(
                worktree,
                "/instances/smith.md",
                "# Smith\n\nPublished before crash.\n",
            ),
        )

    pending = get_operation(control_connection, "op-crash")
    assert pending.state is OperationState.RUNNING

    recovery = TransactionManager(control_connection, repo_paths).recover_startup()

    assert recovery[0].classification == "published"
    assert get_operation(control_connection, "op-crash").state is OperationState.SUCCEEDED
    assert (
        (repo_paths.current_dir / "instances" / "smith.md")
        .read_text(encoding="utf-8")
        .endswith("Published before crash.\n")
    )


def _write_file(worktree: Path, bundle_path: str, content: str) -> tuple[str, ...]:
    relative_path = bundle_path.removeprefix("/")
    target = worktree / relative_path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")
    return (bundle_path,)


def _mutate_with_extra_unstaged_file(worktree: Path) -> tuple[str, ...]:
    _write_file(worktree, "/instances/smith.md", "# Smith\n\nExact staging.\n")
    _write_file(worktree, "/instances/ignored.md", "# Ignored\n")
    return ("/instances/smith.md",)
