"""Capture operation reconciliation: writer probe, ownership and scoped replay."""

from __future__ import annotations

import importlib
import json
import tempfile
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    module: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    operations: Any = importlib.import_module("memento.control.operations")
    proposals: Any = importlib.import_module("memento.control.proposals")
    authz: Any = importlib.import_module("memento.authz")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    cases = []
    for scenario in [
        "missing",
        "writer",
        "late-insert",
        "no-selector",
        "two-selectors",
        "id-missing",
        "foreign",
        "reader",
        "proposer-write",
        "proposer-rebase",
        "control-review",
        "control-denied",
        "control-missing",
        "queued",
        "running",
        "recovering",
        "succeeded",
        "conflict",
        "failed-before",
        "failed-no-base",
        "failed-advanced",
        "unicode-error",
        "bad-replay",
        "nonlist-paths",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-operation-") as directory:
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            state = (
                scenario
                if scenario in {"queued", "running", "recovering", "succeeded", "conflict"}
                else "failed"
                if scenario.startswith("failed") or scenario == "unicode-error"
                else "succeeded"
            )
            method = (
                "memory_proposal_rebase"
                if scenario == "proposer-rebase"
                else "memory_proposal_review"
                if scenario.startswith("control")
                else "memory_patch"
            )
            replay: dict[str, Any] = (
                {"proposal_id": "proposal"}
                if "proposal_" in method
                else {
                    "changed_paths": [
                        "/public/a.md",
                        "/private/a.md",
                        "/read-only/a.md",
                        1,
                        None,
                        "/public/a.md",
                    ]
                }
            )
            if scenario == "nonlist-paths":
                replay["changed_paths"] = "bad"

            def insert(
                connection: Any = connection,
                method: str = method,
                scenario: str = scenario,
                state: str = state,
                replay: dict[str, Any] = replay,
            ) -> None:
                operations.create_operation(
                    connection,
                    operations.OperationRequest(
                        op_id="op",
                        principal="other" if scenario == "foreign" else "actor",
                        idempotency_key="key",
                        tool_name=method,
                        request_json="{}",
                    ),
                )
                connection.execute(
                    "UPDATE operations SET state=?,base_revision=?,result_revision=?,result_json=?,error_class=?,error_message=?",
                    (
                        state,
                        None if scenario == "failed-no-base" else "base",
                        "result" if state == "succeeded" else None,
                        "{" if scenario == "bad-replay" else json.dumps(replay, sort_keys=True),
                        "failure" if state == "failed" else None,
                        "😀" * 510
                        if scenario == "unicode-error"
                        else "synthetic"
                        if state == "failed"
                        else None,
                    ),
                )
                connection.execute("UPDATE operations SET created_at='2026-09-18T00:00:00Z'")
                connection.commit()

            if scenario not in {"missing", "writer", "late-insert", "id-missing"}:
                insert()
            if scenario in {"proposer-rebase", "control-review", "control-denied"}:
                proposals.create_proposal(
                    connection,
                    proposal_id="proposal",
                    author_principal="actor",
                    client_instance_id=None,
                    base_revision="base",
                    intent="keep",
                    rationale=None,
                    patch={
                        "changes": [
                            {
                                "kind": "patch",
                                "path": "/private/a.md"
                                if scenario == "control-denied"
                                else "/public/a.md",
                            }
                        ]
                    },
                )
            connection.execute(
                "UPDATE proposals SET created_at='created',updated_at='updated',expires_at='2099-01-01T00:00:00Z'"
            )
            connection.commit()
            roles = (
                ("reader",)
                if scenario == "reader"
                else ("proposer",)
                if scenario.startswith("proposer")
                else ("curator",)
            )
            policy = authz.EffectivePolicy(
                "actor", roles, ("/public/", "/read-only/"), ("/public/",), ()
            )
            runtime = object.__new__(module.MemoryService)
            runtime._deps = SimpleNamespace(control_connection=connection, repo_paths=None)
            runtime._policy = lambda _, policy=policy: policy

            class Lock:
                def acquire(
                    self, blocking: bool = True, scenario: str = scenario, insert: Any = insert
                ) -> bool:
                    if scenario == "late-insert":
                        insert()
                    return scenario != "writer"

                def release(self) -> None:
                    pass

            key = None if scenario in {"no-selector", "id-missing", "foreign"} else "key"
            id = (
                "missing"
                if scenario == "id-missing"
                else "op"
                if scenario in {"foreign", "two-selectors"}
                else None
            )
            revision = "advanced" if scenario == "failed-advanced" else "base"
            item: dict[str, Any] = {
                "scenario": scenario,
                "policy": {
                    "principal": policy.principal,
                    "roles": policy.roles,
                    "read_prefixes": policy.read_prefixes,
                    "write_prefixes": policy.write_prefixes,
                    "protected_read_prefixes": policy.protected_read_prefixes,
                },
                "key": key,
                "id": id,
                "revision": revision,
                "before": [dict(row) for row in connection.execute("SELECT * FROM operations")],
                "proposals": [dict(row) for row in connection.execute("SELECT * FROM proposals")],
            }
            with (
                patch.object(transactions, "_transaction_lock", lambda _: Lock()),
                patch.object(module, "get_main_revision", lambda _, revision=revision: revision),
            ):
                try:
                    item["expected"] = runtime.memory_operation_get(
                        None, idempotency_key=key, operation_id=id
                    ).model_dump(mode="json")
                except Exception as exc:
                    item["exception"] = type(exc).__name__
            item["after"] = [dict(row) for row in connection.execute("SELECT * FROM operations")]
            cases.append(item)
            connection.close()
    return cases
