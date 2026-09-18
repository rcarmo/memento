"""Trash/restore/purge service ordering and disposable-worktree effects."""

from __future__ import annotations

import base64
import importlib
import tempfile
import threading
from dataclasses import asdict
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch

from asset_get import fixtures as asset_fixtures


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    operations: Any = importlib.import_module("memento.control.operations")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    authz: Any = importlib.import_module("memento.authz")
    source = asset_fixtures()["files"]
    # Inbound links deliberately remain pointed at the original concept path.
    source["/public/empty.md"] = base64.b64encode(
        base64.b64decode(source["/public/empty.md"]).replace(b"synthetic", b"[a](/public/a.md)")
    ).decode()
    cases = []
    for scenario in [
        "trash",
        "restore",
        "purge",
        "purge-no-assets",
        "purge-no-zip",
        "purge-unconfirmed",
        "purge-number",
        "purge-null",
        "reader",
        "unconfirmed-reader",
        "denied-read",
        "denied-write",
        "collision-trash",
        "collision-restore",
        "trash-twice",
        "restore-active",
        "purge-active",
        "nested-trash",
        "non-markdown",
        "unsafe",
        "missing",
        "malformed",
        "transaction-failure",
        "expected-revision",
        "replay-trash",
        "replay-restore",
        "replay-purge",
        "replay-different-revision",
        "tracking-failure",
        "refresh-failure",
        "purge-unlink-failure",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-trash-") as directory:
            root = Path(directory) / "current"
            root.mkdir()
            action = (
                "restore"
                if "restore" in scenario
                else "purge"
                if "purge" in scenario or scenario == "unconfirmed-reader"
                else "trash"
            )
            archived = (
                action != "trash"
                and scenario not in {"restore-active", "purge-active"}
                or scenario in {"trash-twice", "nested-trash"}
            )
            files = dict(source)
            if scenario == "purge-no-assets":
                files = {k: v for k, v in files.items() if not k.startswith("/.assets/")}
            if archived:
                files["/trash/public/a.md"] = files.pop("/public/a.md")
            if scenario.startswith("collision"):
                files["/public/a.md"] = source["/public/a.md"]
                files["/trash/public/a.md"] = source["/public/a.md"]
            path = "/trash/public/a.md" if archived else "/public/a.md"
            if scenario == "nested-trash":
                path = "/trash/trash/public/a.md"
                action = "restore"
            if scenario == "non-markdown":
                path = "/public/a.txt"
            if scenario == "unsafe":
                path = "/../a.md"
            if scenario == "missing":
                path = "/missing.md"
            if scenario == "malformed":
                files[path] = base64.b64encode(b"not a concept").decode()
            if scenario == "purge-no-zip":
                files.pop("/.assets/12345678/docs/1.9.0.zip")
            for name, encoded in files.items():
                target = root / name[1:]
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(base64.b64decode(encoded))
            if scenario == "purge-unlink-failure":
                target = root / ".assets/12345678/docs/1.9.0.zip"
                target.unlink()
                target.mkdir()
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            policy = authz.EffectivePolicy(
                "actor",
                ("reader",) if scenario in {"reader", "unconfirmed-reader"} else ("curator",),
                () if scenario == "denied-read" else ("/",),
                () if scenario == "denied-write" else ("/",),
                (),
            )
            confirm: Any = (
                False
                if scenario in {"purge-unconfirmed", "unconfirmed-reader"}
                else 1
                if scenario == "purge-number"
                else None
                if scenario == "purge-null"
                else True
            )
            args = {
                "path": path,
                "expected_revision": "old" if scenario == "expected-revision" else "main",
                "idempotency_key": "key",
            }
            if action == "purge":
                args["confirm"] = confirm
            state: Any = SimpleNamespace(revision="main", requests=[], recorded=[], refreshes=0)
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection, repo_paths=SimpleNamespace(current_dir=root)
            )
            runtime._policy = lambda _, policy=policy: policy

            def record(
                policy: Any, paths: Any, state: Any = state, scenario: str = scenario
            ) -> None:
                state.refreshes += 1
                if scenario == "refresh-failure":
                    raise service.ConflictError("synthetic refresh failure")
                state.recorded.append(list(paths))
                if scenario == "tracking-failure":
                    raise service.ConflictError("synthetic tracking failure")

            runtime._record_changed_concepts = record

            def apply(
                request: Any,
                mutate: Any,
                state: Any = state,
                scenario: str = scenario,
                root: Path = root,
                connection: Any = connection,
            ) -> Any:
                state.requests.append(asdict(request))
                existing = operations.get_operation_by_idempotency(
                    connection, principal="actor", idempotency_key="key"
                )
                if existing is not None:
                    if existing.request_hash != request.operation.request_hash:
                        raise operations.IdempotencyConflictError(
                            "idempotency key already used for a different request"
                        )
                    return SimpleNamespace(
                        operation=existing,
                        result_revision=existing.result_revision,
                        changed_paths=tuple(existing.replay_payload["changed_paths"]),
                        replayed=True,
                    )
                op = operations.create_operation(connection, request.operation)
                if scenario == "transaction-failure":
                    raise service.ConflictError("synthetic transaction failure")
                if request.expected_revision != state.revision:
                    raise service.ConflictError("synthetic expected revision mismatch")
                paths = mutate(root)
                state.revision = "result"
                op = operations.mark_operation_succeeded(
                    connection, op.op_id, result_revision="result", result={"changed_paths": paths}
                )
                return SimpleNamespace(
                    operation=op, result_revision="result", changed_paths=paths, replayed=False
                )

            runtime._deps.transaction_manager = SimpleNamespace(apply=apply)
            context = SimpleNamespace(
                principal=SimpleNamespace(name="actor"),
                client_instance_id="client",
                mcp_session_id="session",
                source_chat="chat",
            )
            steps = []
            with (
                patch.object(service, "get_main_revision", lambda _, state=state: state.revision),
                patch.object(service, "uuid4", lambda: "00000000-0000-4000-8000-000000000000"),
                patch.object(operations, "_utcnow", lambda: "2026-09-18T00:00:00Z"),
                patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
            ):
                for step in range(2 if scenario.startswith("replay") else 1):
                    if step and scenario == "replay-different-revision":
                        args["expected_revision"] = "different"
                    try:
                        result = getattr(runtime, "memory_" + action)(context, **args).model_dump(
                            mode="json"
                        )
                        exception = None
                    except Exception as exc:
                        result = None
                        exception = type(exc).__name__
                    after = {
                        "/" + str(p.relative_to(root)): base64.b64encode(p.read_bytes()).decode()
                        for p in root.rglob("*")
                        if p.is_file()
                    }
                    op = operations.get_operation_by_idempotency(
                        connection, principal="actor", idempotency_key="key"
                    )
                    steps.append(
                        {
                            "arguments": dict(args),
                            "expected": result,
                            "exception": exception,
                            "files": after,
                            "operation": asdict(op) if op else None,
                            "requests": list(state.requests),
                            "recorded": list(state.recorded),
                            "refreshes": state.refreshes,
                        }
                    )
            cases.append(
                {
                    "scenario": scenario,
                    "action": action,
                    "files": files,
                    "policy": asdict(policy),
                    "steps": steps,
                }
            )
            connection.close()
    return cases
