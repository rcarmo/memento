"""Exercise apply policy against real stores/mutations and a synthetic publisher."""

from __future__ import annotations

import importlib
import tempfile
import threading
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    operations: Any = importlib.import_module("memento.control.operations")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    schema: Any = importlib.import_module("memento.repository.schema")
    authz: Any = importlib.import_module("memento.authz")
    stamp = datetime(2026, 9, 18, tzinfo=UTC)
    initial = str(
        frontmatter.serialize_concept(
            schema.ConceptDocument(
                frontmatter=schema.ConceptFrontmatter(
                    id="12345678",
                    type="concept",
                    title="Title",
                    status="active",
                    created_at=stamp,
                    updated_at=stamp,
                    updated_by="original",
                ),
                body="original",
            )
        )
    )
    cases = []
    for scenario in [
        "apply",
        "denied",
        "reader",
        "stale-clean",
        "stale-conflict",
        "draft",
        "rejected",
        "expired",
        "applied",
        "expected-revision",
        "transaction-failure",
        "update-failure",
        "refresh-failure",
        "replay",
        "replay-conflict",
        "replay-denied",
        "replay-no-state",
        "preview-failure",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-apply-") as directory:
            root = Path(directory) / "current"
            root.mkdir()
            (root / "a.md").write_text(initial)
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            changes = [{"kind": "patch", "path": "/a.md", "body": "new"}]
            if scenario == "preview-failure":
                changes = [{"kind": "patch", "path": "/a.md", "title": ""}]
            proposals.create_proposal(
                connection,
                proposal_id="proposal",
                author_principal="author",
                client_instance_id="client",
                base_revision="base" if scenario.startswith("stale") else "main",
                intent="synthetic",
                rationale="keep",
                patch={"changes": changes},
            )
            connection.execute(
                "UPDATE proposals SET status=?,created_at='created',updated_at='updated',expires_at=?,reviewed_by='reviewer',review_comment='keep'",
                (
                    scenario if scenario in {"draft", "rejected", "applied"} else "approved",
                    "2000-01-01T00:00:00Z" if scenario == "expired" else "2099-01-01T00:00:00Z",
                ),
            )
            connection.commit()
            before = asdict(proposals.get_proposal(connection, "proposal"))
            policy = authz.EffectivePolicy(
                "author",
                ("reader",) if scenario == "reader" else ("curator",),
                ("/",),
                () if scenario == "denied" else ("/",),
                (),
            )
            state = SimpleNamespace(
                policy=policy,
                revision="main",
                changed=["/a.md"] if scenario == "stale-conflict" else [],
                transaction_calls=0,
            )
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection,
                repo_paths=SimpleNamespace(current_dir=root),
                config=SimpleNamespace(limits=SimpleNamespace(max_concept_bytes=65536)),
            )
            runtime._now = lambda: stamp
            runtime._policy = lambda _, state=state: state.policy
            runtime._record_changed_concepts = lambda *args, runtime=runtime: (
                runtime._refresh_proposal_queue()
            )
            trigger = (
                "CREATE TRIGGER blocked BEFORE UPDATE ON proposals WHEN NEW.status='applied' BEGIN SELECT RAISE(ABORT,'synthetic'); END"
                if scenario == "update-failure"
                else ""
            )
            if trigger:
                connection.execute(trigger)

            def apply(
                request: Any,
                mutate: Any,
                state: Any = state,
                connection: Any = connection,
                root: Path = root,
                scenario: str = scenario,
            ) -> Any:
                state.transaction_calls += 1
                operation = operations.create_operation(connection, request.operation)
                if scenario == "transaction-failure":
                    raise service.ConflictError("synthetic transaction failure")
                if request.expected_revision != state.revision:
                    raise service.ConflictError("synthetic expected revision mismatch")
                changed = mutate(root)
                state.revision = "result"
                state.changed = list(changed)
                operation = operations.mark_operation_succeeded(
                    connection,
                    operation.op_id,
                    result_revision="result",
                    result={"changed_paths": changed},
                )
                if scenario == "refresh-failure":
                    connection.execute(
                        "INSERT INTO proposals SELECT 'broken',author_principal,client_instance_id,'base',intent,rationale,'{',patch_hash,'approved',NULL,NULL,NULL,NULL,created_at,updated_at,expires_at FROM proposals WHERE proposal_id='proposal'"
                    )
                    connection.commit()
                return SimpleNamespace(
                    operation=operation,
                    changed_paths=changed,
                    result_revision="result",
                    replayed=False,
                )

            runtime._deps.transaction_manager = SimpleNamespace(apply=apply)
            context = SimpleNamespace(
                principal=SimpleNamespace(name="author"),
                client_instance_id="client",
                mcp_session_id="session",
                source_chat="chat",
            )
            steps = []
            with (
                patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
                patch.object(service, "get_main_revision", lambda _, state=state: state.revision),
                patch.object(
                    service,
                    "diff_main_paths",
                    lambda *args, state=state, **kwargs: tuple(state.changed),
                ),
                patch.object(service, "uuid4", lambda: "00000000-0000-4000-8000-000000000000"),
                patch.object(proposals, "utcnow", lambda: "2026-09-18T00:00:00Z"),
            ):
                for step in range(2 if scenario.startswith("replay") else 1):
                    expected = (
                        "wrong"
                        if scenario == "expected-revision"
                        or (step and scenario == "replay-conflict")
                        else "main"
                    )
                    if step and scenario == "replay-denied":
                        state.policy = authz.EffectivePolicy("author", ("curator",), ("/",), (), ())
                    if step and scenario == "replay-no-state":
                        connection.execute("UPDATE operations SET state='failed'")
                        connection.commit()
                    item: dict[str, Any] = {"expected": expected, "policy": asdict(state.policy)}
                    try:
                        item["response"] = runtime.memory_proposal_apply(
                            context,
                            proposal_id="proposal",
                            expected_revision=expected,
                            idempotency_key="key",
                        ).model_dump(mode="json")
                    except Exception as exc:
                        item["exception"] = type(exc).__name__
                    item["after"] = asdict(proposals.get_proposal(connection, "proposal"))
                    item["operation"] = (
                        asdict(operations.get_operation_by_idempotency(connection, "author", "key"))
                        if state.transaction_calls
                        else None
                    )
                    item["revision"] = state.revision
                    item["changed"] = state.changed
                    item["transaction_calls"] = state.transaction_calls
                    steps.append(item)
            cases.append(
                {
                    "scenario": scenario,
                    "initial": initial,
                    "before": before,
                    "trigger": trigger,
                    "steps": steps,
                }
            )
            connection.close()
    return cases
