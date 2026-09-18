"""Capture durable review ordering, payloads and transactional rows."""

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
    authz: Any = importlib.import_module("memento.authz")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    cases = []
    scenarios = [
        "current",
        "stale-clean",
        "stale-conflict",
        "expired",
        "applied",
        "denied",
        "no-curator",
        "optional-key",
        "archival",
        "event-failure",
        "update-failure",
        "journal-failure",
        "replay-conflict",
        "replay-denied",
    ]
    for decision in ["approve", "reject", "request_changes", "invalid"]:
        for scenario in scenarios:
            with tempfile.TemporaryDirectory(prefix="memento-review-") as directory:
                connection = db.connect_control_db(Path(directory) / "control.sqlite")
                db.migrate_control_db(connection)
                patch_data: dict[str, Any] = {
                    "changes": [{"kind": "patch", "path": "/public/a.md", "body": "keep"}]
                }
                if scenario == "archival":
                    patch_data = {
                        "changes": [{"kind": "trash", "path": "/public/a.md"}],
                        "archival_impact": [{"path": "/public/a.md"}],
                    }
                proposals.create_proposal(
                    connection,
                    proposal_id="proposal",
                    author_principal="author",
                    base_revision="base" if scenario.startswith("stale") else "main",
                    intent="keep",
                    client_instance_id=None,
                    rationale=None,
                    patch=patch_data,
                )
                connection.execute(
                    "UPDATE proposals SET status=?,created_at='created',updated_at='updated',expires_at=?,reviewed_by='previous',review_comment='previous comment'",
                    (
                        "applied" if scenario == "applied" else "submitted",
                        "2000-01-01T00:00:00Z" if scenario == "expired" else "2099-01-01T00:00:00Z",
                    ),
                )
                connection.commit()
                before = asdict(proposals.get_proposal(connection, "proposal"))
                runtime = object.__new__(service.MemoryService)
                runtime._deps = SimpleNamespace(
                    control_connection=connection,
                    repo_paths=SimpleNamespace(current_dir=Path(directory)),
                )
                runtime._now = lambda: datetime(2026, 9, 18, tzinfo=UTC)
                policy = authz.EffectivePolicy(
                    "curator",
                    ("reader",) if scenario == "no-curator" else ("curator",),
                    ("/public/",),
                    () if scenario == "denied" else ("/public/",),
                    (),
                )
                state = SimpleNamespace(policy=policy, archival_calls=0)
                runtime._policy = lambda _, state=state: state.policy

                def archival(*args: Any, state: Any = state) -> list[Any]:
                    state.archival_calls += 1
                    raise service.ConflictError("archival impact requires a fresh content index")

                runtime._archival_impact = archival
                trigger = ""
                if scenario == "event-failure":
                    trigger = "CREATE TRIGGER blocked BEFORE INSERT ON proposal_events WHEN NEW.action='review' BEGIN SELECT RAISE(ABORT,'synthetic'); END"
                elif scenario == "update-failure":
                    trigger = "CREATE TRIGGER blocked BEFORE UPDATE ON proposals WHEN NEW.reviewed_by='curator' BEGIN SELECT RAISE(ABORT,'synthetic'); END"
                elif scenario == "journal-failure":
                    trigger = "CREATE TRIGGER blocked BEFORE INSERT ON operations BEGIN SELECT RAISE(ABORT,'synthetic'); END"
                if trigger:
                    connection.execute(trigger)
                ids = iter([f"00000000-0000-4000-8000-{i:012d}" for i in range(1, 5)])
                changed = ["/public/a.md"] if scenario == "stale-conflict" else []
                steps = []
                context = SimpleNamespace(
                    principal=SimpleNamespace(name="curator"), client_instance_id="client"
                )
                with (
                    patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
                    patch.object(service, "get_main_revision", lambda _: "main"),
                    patch.object(
                        service, "diff_main_paths", lambda *args, changed=changed, **kwargs: changed
                    ),
                    patch.object(service, "uuid4", lambda ids=ids: next(ids)),
                ):
                    for step in range(
                        2
                        if scenario
                        in {"current", "replay-conflict", "replay-denied", "optional-key"}
                        else 1
                    ):
                        comment = (
                            "changed"
                            if step and scenario == "replay-conflict"
                            else "review comment"
                        )
                        if step and scenario == "replay-denied":
                            state.policy = authz.EffectivePolicy(
                                "curator", ("curator",), ("/public/",), (), ()
                            )
                        key = None if scenario == "optional-key" else "key"
                        item: dict[str, Any] = {
                            "policy": asdict(state.policy),
                            "key": key,
                            "comment": comment,
                        }
                        try:
                            response = runtime.memory_proposal_review(
                                context,
                                proposal_id="proposal",
                                decision=decision,
                                comment=comment,
                                idempotency_key=key,
                            )
                            item["response"] = response.model_dump(mode="json")
                        except Exception as exc:
                            item["exception"] = type(exc).__name__
                        item["after"] = asdict(proposals.get_proposal(connection, "proposal"))
                        item["events"] = [
                            dict(row)
                            for row in connection.execute(
                                "SELECT * FROM proposal_events ORDER BY event_id"
                            )
                        ]
                        item["operations"] = [
                            dict(row)
                            for row in connection.execute("SELECT * FROM operations ORDER BY op_id")
                        ]
                        steps.append(item)
                cases.append(
                    {
                        "scenario": scenario,
                        "decision": decision,
                        "before": before,
                        "changed": changed,
                        "trigger": trigger,
                        "steps": steps,
                    }
                )
                connection.close()
    return cases
