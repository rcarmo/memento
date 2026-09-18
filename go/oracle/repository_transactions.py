"""Record every transaction checkpoint against disposable Git/SQLite stores."""

from __future__ import annotations

import importlib
import os
import tempfile
from dataclasses import asdict
from pathlib import Path
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    git: Any = importlib.import_module("memento.repository.git")
    db: Any = importlib.import_module("memento.control.db")
    ops: Any = importlib.import_module("memento.control.operations")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    checkpoints: Any = importlib.import_module("memento.control.checkpoints")
    results = []
    for checkpoint in [
        "",
        "operation_inserted",
        "worktree_created",
        "mutation_applied",
        "commit_created",
        "publication_complete",
        "current_materialized",
        "derived_updated",
        "operation_completed",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-transaction-oracle-") as directory:
            root = Path(directory)
            paths = git.GitRepositoryPaths(root / "repo.git", root / "current", root / "worktrees")
            env = {
                "GIT_AUTHOR_DATE": "2026-09-18T00:00:00+00:00",
                "GIT_COMMITTER_DATE": "2026-09-18T00:00:00+00:00",
                "GIT_CONFIG_NOSYSTEM": "1",
                "GIT_CONFIG_GLOBAL": "/dev/null",
            }
            with (
                patch.dict(os.environ, env),
                patch.object(ops, "_utcnow", lambda: "2026-09-18T00:00:00Z"),
            ):
                base = git.bootstrap_repository(paths).revision
                connection = db.connect_control_db(root / "control.sqlite")
                db.migrate_control_db(connection)
                hook = checkpoints.CheckpointHook(checkpoints.FailAtCheckpoint(checkpoint))
                manager = transactions.TransactionManager(connection, paths, checkpoints=hook)
                request = transactions.TransactionRequest(
                    operation=ops.OperationRequest(
                        op_id="transaction-one",
                        principal="agent",
                        idempotency_key="key",
                        tool_name="memory_write",
                        request_json='{"synthetic":true}',
                    ),
                    expected_revision=base,
                    commit_message="synthetic transaction",
                    author_name="Synthetic Agent",
                    author_email="test@example.invalid",
                )

                def mutate(path: Path) -> tuple[str, ...]:
                    (path / "one.md").write_text("committed\n")
                    (path / "ignored.md").write_text("not committed\n")
                    return ("/one.md",)

                error = ""
                try:
                    manager.apply(request, mutate)
                except checkpoints.CheckpointError:
                    error = "CheckpointError"
                before = asdict(ops.get_operation(connection, "transaction-one"))
                main = git.get_main_revision(paths)
                worktree = git.resolve_worktree_revision(paths.worktrees_dir / "transaction-one")
                connection.close()
                connection = db.connect_control_db(root / "control.sqlite")
                recovery = transactions.TransactionManager(connection, paths).recover_startup()
                after = asdict(ops.get_operation(connection, "transaction-one"))
                files = sorted(path.name for path in paths.current_dir.iterdir())
                results.append(
                    {
                        "checkpoint": checkpoint,
                        "error": error,
                        "seen": hook.seen,
                        "base": base,
                        "main": main,
                        "worktree": worktree,
                        "before": before,
                        "recovery": [asdict(row) for row in recovery],
                        "after": after,
                        "files": files,
                    }
                )
                connection.close()
    return results
