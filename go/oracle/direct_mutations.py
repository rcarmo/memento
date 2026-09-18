"""Direct curator create/patch/rename: hashes, warnings and post-commit previews."""

from __future__ import annotations

import base64
import importlib
import tempfile
import threading
from dataclasses import asdict
from datetime import UTC, datetime
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
    cases = []
    for scenario in [
        "create",
        "create-defaults",
        "patch",
        "patch-noop",
        "rename",
        "reader",
        "denied",
        "invalid-path",
        "trash-path",
        "asset-body",
        "asset-title",
        "missing",
        "invalid-frontmatter",
        "invalid-status",
        "transaction-failure",
        "expected-revision",
        "replay",
        "tracking-failure",
        "preview-failure",
        "refresh-failure",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-direct-") as directory:
            root = Path(directory) / "current"
            root.mkdir()
            files = {
                name: encoded
                for name, encoded in source.items()
                if not name.startswith("/.assets/") or scenario.startswith("asset-")
            }
            if scenario == "invalid-frontmatter":
                files["/public/a.md"] = base64.b64encode(b"not a concept").decode()
            for path, encoded in files.items():
                target = root / path[1:]
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(base64.b64decode(encoded))
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            policy = authz.EffectivePolicy(
                "actor",
                ("reader",) if scenario == "reader" else ("curator",),
                ("/",),
                () if scenario == "denied" else ("/",),
                (),
            )
            method = "patch"
            args: dict[str, Any] = {
                "path": "/public/a.md",
                "body": "new",
                "expected_revision": "main",
                "idempotency_key": "key",
            }
            if scenario.startswith("create"):
                method = "create"
                args.update(path="/public/new.md", concept_type="concept", title="New")
                if scenario == "create":
                    args.update(description="desc", tags=["z", "a", "z"], aliases=["alias"])
            if scenario == "rename":
                method = "rename"
                args.pop("body")
                args["new_path"] = "/public/renamed.md"
            if scenario == "patch-noop":
                args.pop("body")
            if scenario == "patch":
                args.update(tags=["z", "a", "z"], aliases=[], status="deprecated")
            if scenario == "asset-title":
                args.pop("body")
                args["title"] = "Changed"
            if scenario == "invalid-status":
                args["status"] = "bad"
            if scenario == "invalid-path":
                args["path"] = "/a.txt"
            if scenario == "trash-path":
                args["path"] = "/trash/a.md"
            if scenario == "missing":
                args["path"] = "/missing.md"
            if scenario == "expected-revision":
                args["expected_revision"] = "old"
            state: Any = SimpleNamespace(revision="main", requests=[], recorded=[], refreshes=0)
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection,
                repo_paths=SimpleNamespace(current_dir=root),
                config=SimpleNamespace(
                    limits=SimpleNamespace(max_concept_bytes=65536),
                    intelligent_tiers=SimpleNamespace(
                        hot_working_memory=SimpleNamespace(enabled=False)
                    ),
                ),
            )
            runtime._now = lambda: datetime(2026, 9, 18, tzinfo=UTC)
            runtime._policy = lambda _, policy=policy: policy

            def refresh(state: Any = state, scenario: str = scenario) -> None:
                state.refreshes += 1
                if scenario == "refresh-failure":
                    raise service.ConflictError("synthetic refresh failure")

            def record(
                policy: Any,
                paths: Any,
                state: Any = state,
                scenario: str = scenario,
                refresh: Any = refresh,
            ) -> None:
                refresh()
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
                    return SimpleNamespace(
                        operation=existing,
                        result_revision=existing.result_revision,
                        changed_paths=("/public/a.md",),
                        replayed=True,
                    )
                operation = operations.create_operation(connection, request.operation)
                if scenario == "transaction-failure":
                    raise service.ConflictError("synthetic transaction failure")
                if request.expected_revision != state.revision:
                    raise service.ConflictError("synthetic expected revision mismatch")
                changed = mutate(root)
                state.revision = "result"
                operation = operations.mark_operation_succeeded(
                    connection,
                    operation.op_id,
                    result_revision="result",
                    result={"changed_paths": changed},
                )
                if scenario == "preview-failure":
                    (root / "public/a.md").write_text("not a concept")
                return SimpleNamespace(
                    operation=operation,
                    result_revision="result",
                    changed_paths=changed,
                    replayed=False,
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
                for _ in range(2 if scenario == "replay" else 1):
                    try:
                        result = getattr(runtime, "memory_" + method)(context, **args).model_dump(
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
                    operation = operations.get_operation_by_idempotency(
                        connection, principal="actor", idempotency_key="key"
                    )
                    steps.append(
                        {
                            "expected": result,
                            "exception": exception,
                            "files": after,
                            "operation": asdict(operation) if operation else None,
                            "requests": list(state.requests),
                            "recorded": list(state.recorded),
                            "refreshes": state.refreshes,
                        }
                    )
            cases.append(
                {
                    "scenario": scenario,
                    "method": method,
                    "arguments": args,
                    "policy": asdict(policy),
                    "files": files,
                    "steps": steps,
                }
            )
            connection.close()
    return cases
