"""Capture selected-change revision, copied assets and post-commit failures."""

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


class Clock(datetime):
    @classmethod
    def now(cls, tz: Any = None) -> Clock:
        return cls(2026, 9, 18, tzinfo=UTC)


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    authz: Any = importlib.import_module("memento.authz")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    cases = []
    for scenario in [
        "subset",
        "duplicates",
        "empty",
        "negative",
        "range",
        "blocked",
        "wrong-revision",
        "current",
        "rejected",
        "expired",
        "reader",
        "denied",
        "trash",
        "asset-only",
        "body-only",
        "asset-pair",
        "asset-missing",
        "skill-tag",
        "overrides",
        "empty-overrides",
        "insert-failure",
        "asset-failure",
        "commit-failure",
        "preview-failure",
        "payload-failure",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-revise-") as directory:
            root = Path(directory) / "current"
            root.mkdir()
            initial = "---\nid: '12345678'\ntype: concept\ntitle: Title\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: original\n---\nbody\n"
            (root / "a.md").write_text(initial)
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            changes: list[dict[str, Any]] = [
                {"kind": "patch", "path": "/a.md", "body": "new"},
                {"kind": "patch", "path": "/blocked.md", "body": "blocked"},
            ]
            selected = [0]
            if scenario == "duplicates":
                selected = [0, 0]
            elif scenario == "empty":
                selected = []
            elif scenario == "negative":
                selected = [-1]
            elif scenario == "range":
                selected = [2]
            elif scenario == "blocked":
                selected = [1]
            elif scenario == "trash":
                changes = [{"kind": "trash", "path": "/a.md"}]
            if scenario.startswith("asset") or scenario in {
                "body-only",
                "skill-tag",
                "commit-failure",
            }:
                changes = [
                    {"kind": "patch", "path": "/a.md", "body": "new"},
                    {
                        "kind": "attach_asset_pack",
                        "path": "/a.md",
                        "asset_kind": "skill" if scenario == "skill-tag" else "docs",
                        "version": "1.0.0",
                        "asset_id": "asset",
                        "zip_sha256": "digest",
                        "manifest": {},
                    },
                ]
                selected = (
                    [1]
                    if scenario == "asset-only"
                    else [0]
                    if scenario == "body-only"
                    else [1, 0, 1]
                )
            if scenario == "preview-failure":
                changes[0]["path"] = "/missing.md"
            with patch.object(proposals, "datetime", Clock):
                proposals.create_proposal(
                    connection,
                    proposal_id="source",
                    author_principal="author",
                    client_instance_id="original-client",
                    base_revision="main" if scenario == "current" else "base",
                    intent="original",
                    rationale="original",
                    patch={"changes": changes},
                    assets=[
                        proposals.ProposalAssetInput(
                            asset_id="asset",
                            concept_path="/a.md",
                            asset_kind="docs",
                            version="1.0.0",
                            media_type="application/zip",
                            sha256="digest",
                            blob_bytes=b"\x00\xff",
                            manifest_json="{}",
                        )
                    ],
                )
            connection.execute(
                "UPDATE proposals SET created_at='created',updated_at='updated',status=?,reviewed_by='reviewer',review_comment='retain',expires_at=?",
                (
                    "rejected" if scenario == "rejected" else "approved",
                    "2000-01-01T00:00:00Z" if scenario == "expired" else "2099-01-01T00:00:00Z",
                ),
            )
            connection.execute("UPDATE proposal_assets SET created_at='created'")
            if scenario == "asset-missing":
                connection.execute("DELETE FROM proposal_assets")
            connection.commit()
            source = proposals.get_proposal(connection, "source")
            trigger = ""
            if scenario == "insert-failure":
                trigger = "CREATE TRIGGER blocked BEFORE INSERT ON proposals BEGIN SELECT RAISE(ABORT,'synthetic'); END"
            elif scenario == "asset-failure":
                trigger = "CREATE TRIGGER blocked BEFORE INSERT ON proposal_assets BEGIN SELECT RAISE(ABORT,'synthetic'); END"
            elif scenario == "commit-failure":
                trigger = "CREATE TABLE parent(id INTEGER PRIMARY KEY);CREATE TABLE child(id INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER blocked AFTER INSERT ON proposals BEGIN INSERT INTO child VALUES(1);END;"
            elif scenario == "payload-failure":
                trigger = "CREATE TRIGGER blocked AFTER INSERT ON proposals BEGIN UPDATE proposals SET patch_json='{' WHERE proposal_id=NEW.proposal_id;END"
            if trigger:
                connection.executescript(trigger)
            policy = authz.EffectivePolicy(
                "curator",
                ("reader",) if scenario == "reader" else ("curator",),
                ("/",),
                () if scenario == "denied" else ("/",),
                (),
            )
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection,
                repo_paths=SimpleNamespace(current_dir=root),
                config=SimpleNamespace(limits=SimpleNamespace(max_concept_bytes=65536)),
            )
            runtime._policy = lambda _, policy=policy: policy
            runtime._now = Clock.now
            intent = (
                "custom"
                if scenario == "overrides"
                else ""
                if scenario == "empty-overrides"
                else None
            )
            rationale = (
                "reason"
                if scenario == "overrides"
                else ""
                if scenario == "empty-overrides"
                else None
            )
            item: dict[str, Any] = {
                "scenario": scenario,
                "initial": initial,
                "source": asdict(source),
                "policy": asdict(policy),
                "indexes": selected,
                "expected_revision": "wrong" if scenario == "wrong-revision" else "main",
                "intent": intent,
                "rationale": rationale,
                "trigger": trigger,
            }
            with (
                patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
                patch.object(service, "get_main_revision", lambda _: "main"),
                patch.object(service, "diff_main_paths", lambda *args, **kwargs: ("/blocked.md",)),
                patch.object(service, "uuid4", lambda: "00000000-0000-4000-8000-000000000000"),
                patch.object(proposals, "datetime", Clock),
            ):
                try:
                    item["response"] = runtime.memory_proposal_revise(
                        SimpleNamespace(
                            principal=SimpleNamespace(name="curator"), client_instance_id="client"
                        ),
                        proposal_id="source",
                        selected_change_indexes=tuple(selected),
                        expected_revision=item["expected_revision"],
                        intent=intent,
                        rationale=rationale,
                    ).model_dump(mode="json")
                except Exception as exc:
                    item["exception"] = type(exc).__name__
            item["proposals"] = [
                dict(row)
                for row in connection.execute("SELECT * FROM proposals ORDER BY proposal_id")
            ]
            item["assets"] = [
                {
                    key: base64.b64encode(value).decode() if isinstance(value, bytes) else value
                    for key, value in dict(row).items()
                }
                for row in connection.execute(
                    "SELECT * FROM proposal_assets ORDER BY proposal_id,asset_id"
                )
            ]
            item["events"] = [
                dict(row)
                for row in connection.execute("SELECT * FROM proposal_events ORDER BY event_id")
            ]
            cases.append(item)
            connection.close()
    return cases
