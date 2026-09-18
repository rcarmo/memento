"""Capture rebase results and complete transaction state from the pinned service."""

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
    for scenario in [
        "clean",
        "wrong-revision",
        "conflict",
        "same",
        "draft",
        "status-submitted",
        "status-stale",
        "status-needs_rebase",
        "status-conflicted",
        "status-rejected",
        "status-applied",
        "status-expired",
        "expired",
        "trash",
        "reader",
        "other",
        "curator",
        "skill-disk",
        "skill-missing-tag",
        "skill-patch",
        "skill-clear",
        "replay-after-advance",
        "replay-key-conflict",
        "replay-denied",
        "event-failure",
        "update-failure",
        "journal-failure",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-rebase-") as directory:
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            changes: list[dict[str, Any]] = [
                {"kind": "patch", "path": "/public/a.md", "body": "proposed"}
            ]
            if scenario == "trash":
                changes = [{"kind": "trash", "path": "/public/a.md"}]
            if scenario.startswith("skill"):
                changes = [
                    {
                        "kind": "attach_asset_pack",
                        "path": "/public/a.md",
                        "asset_kind": "skill",
                        "version": "1.0.0",
                        "asset_id": "asset",
                        "zip_sha256": "digest",
                        "manifest": {},
                    }
                ]
                if scenario in {"skill-patch", "skill-clear"}:
                    changes.append(
                        {
                            "kind": "patch",
                            "path": "/public/a.md",
                            "tags": ["skill"] if scenario == "skill-patch" else [],
                        }
                    )
            proposals.create_proposal(
                connection,
                proposal_id="proposal",
                author_principal="author",
                client_instance_id="original-client",
                base_revision="main" if scenario == "same" else "base",
                intent="keep",
                rationale="keep",
                patch={"changes": changes, "context": "retained"},
            )
            connection.execute(
                "UPDATE proposals SET status=?,created_at='created',updated_at='updated',expires_at=?,reviewed_by='reviewer',review_comment='keep'",
                (
                    scenario.removeprefix("status-")
                    if scenario.startswith("status-")
                    else "draft"
                    if scenario == "draft"
                    else "approved",
                    "2000-01-01T00:00:00Z" if scenario == "expired" else "2099-01-01T00:00:00Z",
                ),
            )
            # Retained opaque bytes prove rebasing does not recreate asset records.
            connection.execute(
                "INSERT INTO proposal_assets(asset_id,proposal_id,concept_path,asset_kind,version,sha256,blob_bytes,manifest_json,media_type,created_at) VALUES('asset','proposal','/public/a.md','docs','1.0.0','digest',X'0001FF','{}','application/zip','created')"
            )
            connection.commit()
            before = asdict(proposals.get_proposal(connection, "proposal"))
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection,
                repo_paths=SimpleNamespace(current_dir=Path(directory)),
            )
            runtime._now = lambda: datetime(2026, 9, 18, 0, 0, 0, 123456, tzinfo=UTC)
            principal = "other" if scenario in {"other", "curator"} else "author"
            roles = (
                ("reader",)
                if scenario == "reader"
                else ("curator",)
                if scenario == "curator"
                else ("proposer",)
            )
            policy = authz.EffectivePolicy(principal, roles, ("/public/",), ("/public/",), ())
            state = SimpleNamespace(policy=policy, revision="main")
            runtime._policy = lambda _, state=state: state.policy
            context = SimpleNamespace(
                principal=SimpleNamespace(name=principal), client_instance_id="client"
            )
            revision = "main"
            changed = ["/public/a.md"] if scenario == "conflict" else []
            tags = () if scenario == "skill-missing-tag" else ("skill",)
            trigger = ""
            if scenario == "event-failure":
                trigger = "CREATE TRIGGER blocked BEFORE INSERT ON proposal_events WHEN NEW.action='rebase' BEGIN SELECT RAISE(ABORT,'synthetic'); END"
            elif scenario == "update-failure":
                trigger = "CREATE TRIGGER blocked BEFORE UPDATE ON proposals WHEN NEW.status='submitted' BEGIN SELECT RAISE(ABORT,'synthetic'); END"
            elif scenario == "journal-failure":
                trigger = "CREATE TRIGGER blocked BEFORE INSERT ON operations BEGIN SELECT RAISE(ABORT,'synthetic'); END"
            if trigger:
                connection.execute(trigger)
            steps = []
            with (
                patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
                patch.object(service, "get_main_revision", lambda _, state=state: state.revision),
                patch.object(
                    service,
                    "diff_main_paths",
                    lambda *args, changed=changed, **kwargs: tuple(changed),
                ),
                patch.object(
                    service,
                    "read_bundle_entry",
                    lambda *args, tags=tags: SimpleNamespace(
                        document=SimpleNamespace(
                            frontmatter=SimpleNamespace(id="12345678", tags=tags)
                        )
                    ),
                ),
                patch.object(service, "uuid4", lambda: "00000000-0000-4000-8000-000000000000"),
            ):
                for step in range(2 if scenario.startswith("replay") or scenario == "clean" else 1):
                    expected = "wrong" if scenario == "wrong-revision" else "main"
                    if step and scenario == "replay-after-advance":
                        revision = "next"
                        state.revision = revision
                        connection.execute("UPDATE proposals SET status='applied'")
                        connection.commit()
                    if step and scenario == "replay-key-conflict":
                        expected = "different"
                    if step and scenario == "replay-denied":
                        policy = authz.EffectivePolicy(principal, roles, ("/public/",), (), ())
                        state.policy = policy
                    item: dict[str, Any] = {
                        "expected_revision": expected,
                        "revision": revision,
                        "policy": asdict(policy),
                    }
                    try:
                        response = runtime.memory_proposal_rebase(
                            context,
                            proposal_id="proposal",
                            expected_revision=expected,
                            idempotency_key="key",
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
                    item["assets"] = [
                        dict(row)
                        for row in connection.execute(
                            "SELECT asset_id,proposal_id,concept_path,asset_kind,version,sha256,hex(blob_bytes) AS bytes_hex,manifest_json,media_type,created_at FROM proposal_assets"
                        )
                    ]
                    steps.append(item)
            cases.append(
                {
                    "scenario": scenario,
                    "before": before,
                    "changed": changed,
                    "tags": tags,
                    "trigger": trigger,
                    "steps": steps,
                }
            )
            connection.close()
    return cases
