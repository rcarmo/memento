"""Pinned non-model submissions with exact proposal/asset/staging rows."""

from __future__ import annotations

import base64
import binascii
import importlib
import io
import tempfile
import threading
import zipfile
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
    staged: Any = importlib.import_module("memento.staged_assets")
    authz: Any = importlib.import_module("memento.authz")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    stamp = datetime(2026, 9, 18, tzinfo=UTC)
    cases = []
    scenarios = [
        "empty",
        "create",
        "patch",
        "stale",
        "reader",
        "denied",
        "size",
        "invalid",
        "inline",
        "padding",
        "bad-base64",
        "two-inputs",
        "no-input",
        "rename",
        "missing-body",
        "skill",
        "skill-noncanonical",
        "skill-tag",
        "skill-body",
        "staged",
        "staged-denied",
        "staged-version",
        "staged-expired",
        "staged-unavailable",
        "staged-twice",
        "existing",
        "existing-stale",
        "existing-expired",
        "insert-failure",
        "asset-failure",
        "consume-failure",
        "commit-failure",
        "payload-failure",
    ]
    for scenario in scenarios:
        with tempfile.TemporaryDirectory(prefix="memento-submit-") as directory:
            root = Path(directory) / "current"
            root.mkdir()
            initial = "---\nid: '12345678'\ntype: concept\ntitle: Title\nstatus: active\ntags: [skill]\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: original\n---\nbody\n"
            (root / "a.md").write_text(initial)
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            output = io.BytesIO()
            with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_STORED) as archive:
                info = zipfile.ZipInfo("SKILL.md", date_time=(2026, 1, 1, 0, 0, 0))
                archive.writestr(info, "wrong" if scenario == "skill-body" else "body")
            blob = output.getvalue()
            kind = "skill" if scenario.startswith("skill") else "docs"
            is_asset = scenario not in {
                "empty",
                "create",
                "patch",
                "stale",
                "reader",
                "denied",
                "size",
                "invalid",
                "insert-failure",
                "payload-failure",
            }
            asset: dict[str, Any] = {
                "kind": "attach_asset_pack",
                "path": "/a.md",
                "asset_kind": kind,
                "version": "1.0.0",
                "zip_base64": base64.b64encode(blob).decode(),
            }
            changes: list[dict[str, Any]] = (
                [asset]
                if is_asset
                else [
                    {
                        "kind": "create",
                        "path": "/new.md",
                        "concept_type": "concept",
                        "title": "New",
                        "body": "body",
                    }
                ]
            )
            if scenario == "empty":
                changes = []
            elif scenario == "patch":
                changes = [{"kind": "patch", "path": "/a.md", "body": "new"}]
            elif scenario == "invalid":
                changes = [{"kind": "unknown"}]
            elif scenario == "padding":
                # The archive length is not necessarily divisible by 3, so use
                # the exact source decoder output rather than accepting extra =.
                asset["zip_base64"] += "="
            elif scenario == "bad-base64":
                asset["zip_base64"] = "Zm9v\n"
            elif scenario == "two-inputs":
                asset["staged_asset_id"] = "staged"
            elif scenario == "no-input":
                del asset["zip_base64"]
            elif scenario == "rename":
                changes.append({"kind": "rename", "path": "/other.md", "new_path": "/a.md"})
            elif scenario == "missing-body":
                asset["path"] = "/missing.md"
            elif scenario == "skill-noncanonical":
                changes.insert(0, {"kind": "patch", "path": "/a.md", "body": "body\n"})
            elif scenario == "skill-tag":
                changes.insert(0, {"kind": "patch", "path": "/a.md", "tags": []})
            store = staged.StagedAssetStore(connection)
            has_staging = scenario.startswith("staged") or scenario in {
                "asset-failure",
                "consume-failure",
                "commit-failure",
            }
            with (
                patch.object(staged, "_now", lambda: stamp),
                patch.object(staged, "uuid4", lambda: "staged"),
            ):
                if has_staging:
                    upload, _ = store.put(
                        principal="other" if scenario == "staged-denied" else "author",
                        idempotency_key="upload",
                        asset_kind=kind,
                        version="1.0.0",
                        zip_bytes=blob,
                    )
                    del asset["zip_base64"]
                    asset["staged_asset_id"] = upload.staged_asset_id
                    if scenario == "staged-version":
                        asset["version"] = "2.0.0"
                    if scenario == "staged-expired":
                        connection.execute(
                            "UPDATE staged_assets SET expires_at='2000-01-01T00:00:00Z'"
                        )
                        connection.commit()
                    if scenario == "staged-twice":
                        changes.append(dict(asset))
            if scenario.startswith("existing"):
                proposals.create_proposal(
                    connection,
                    proposal_id="existing",
                    author_principal="author",
                    client_instance_id=None,
                    base_revision="old" if scenario == "existing-stale" else "main",
                    intent="old",
                    rationale=None,
                    patch={"changes": []},
                    assets=[
                        proposals.ProposalAssetInput(
                            asset_id="old-asset",
                            concept_path="/a.md",
                            asset_kind=kind,
                            version="1.0.0",
                            media_type="application/zip",
                            sha256="digest",
                            blob_bytes=b"old",
                            manifest_json="{}",
                        )
                    ],
                )
                connection.execute(
                    "UPDATE proposals SET created_at='created',updated_at='updated',expires_at=?",
                    (
                        "2000-01-01T00:00:00Z"
                        if scenario == "existing-expired"
                        else "2099-01-01T00:00:00Z",
                    ),
                )
                connection.commit()
            connection.execute("UPDATE proposal_assets SET created_at='created'")
            connection.commit()
            policy = authz.EffectivePolicy(
                "author",
                ("reader",) if scenario == "reader" else ("proposer",),
                ("/",),
                () if scenario == "denied" else ("/",),
                (),
            )
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection,
                repo_paths=SimpleNamespace(current_dir=root),
                config=SimpleNamespace(
                    limits=SimpleNamespace(max_concept_bytes=1 if scenario == "size" else 65536)
                ),
                staged_asset_store=store
                if has_staging and scenario != "staged-unavailable"
                else None,
            )
            runtime._policy = lambda _, policy=policy: policy
            runtime._now = lambda: stamp
            trigger = ""
            if scenario == "insert-failure":
                trigger = "CREATE TRIGGER blocked BEFORE INSERT ON proposals BEGIN SELECT RAISE(ABORT,'synthetic'); END"
            elif scenario == "asset-failure":
                trigger = "CREATE TRIGGER blocked BEFORE INSERT ON proposal_assets BEGIN SELECT RAISE(ABORT,'synthetic'); END"
            elif scenario == "consume-failure":
                trigger = "CREATE TRIGGER blocked BEFORE UPDATE ON staged_assets WHEN NEW.state='consumed' BEGIN SELECT RAISE(ABORT,'synthetic'); END"
            elif scenario == "commit-failure":
                trigger = "CREATE TABLE parent(id INTEGER PRIMARY KEY);CREATE TABLE child(id INTEGER REFERENCES parent(id) DEFERRABLE INITIALLY DEFERRED);CREATE TRIGGER blocked AFTER INSERT ON proposals BEGIN INSERT INTO child VALUES(1);END;"
            elif scenario == "payload-failure":
                trigger = "CREATE TRIGGER blocked AFTER INSERT ON proposals BEGIN UPDATE proposals SET patch_json='{' WHERE proposal_id=NEW.proposal_id; END"

            def snapshot(connection: Any) -> dict[str, Any]:
                result = {}
                for table in ["proposals", "proposal_assets", "staged_assets", "proposal_events"]:
                    rows = []
                    for row in connection.execute(f"SELECT * FROM {table} ORDER BY rowid"):
                        rows.append(
                            {
                                key: base64.b64encode(value).decode()
                                if isinstance(value, bytes)
                                else value
                                for key, value in dict(row).items()
                            }
                        )
                    result[table] = rows
                return result

            before = snapshot(connection)
            if trigger:
                connection.executescript(trigger)
            ids = iter(
                [
                    "00000000-0000-4000-8000-000000000000",
                    "00000000-0000-4000-8000-000000000001",
                    "00000000-0000-4000-8000-000000000002",
                ]
            )
            item: dict[str, Any] = {
                "scenario": scenario,
                "initial": initial,
                "changes": changes,
                "policy": asdict(policy),
                "base": "old" if scenario == "stale" else "main",
                "before": before,
                "trigger": trigger,
                "has_staging": has_staging and scenario != "staged-unavailable",
            }
            with (
                patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
                patch.object(service, "get_main_revision", lambda _: "main"),
                patch.object(service, "diff_main_paths", lambda *args, **kwargs: ()),
                patch.object(service, "uuid4", lambda ids=ids: next(ids)),
                patch.object(proposals, "utcnow", lambda: "2026-09-18T00:00:00Z"),
                patch.object(proposals, "datetime", Clock),
                patch.object(staged, "_now", lambda: stamp),
            ):
                try:
                    item["response"] = runtime.memory_propose(
                        SimpleNamespace(
                            principal=SimpleNamespace(name="author"), client_instance_id="client"
                        ),
                        intent="intent",
                        base_revision=item["base"],
                        changes=changes,
                        rationale="why",
                    ).model_dump(mode="json")
                except Exception as exc:
                    item["exception"] = type(exc).__name__
            item["after"] = snapshot(connection)
            cases.append(item)
            connection.close()
    return cases


def base64_fixtures() -> list[dict[str, Any]]:
    cases = []
    for data in ["", "Z", "Zg", "Zg=", "Zg==", "Zm8=", "Zm9v", "Zh==", "💀"]:
        for suffix in ["", "=", "==", "===", "====", "a", "\n", "\r", " ", "\t", "=a"]:
            text = data + suffix
            item: dict[str, Any] = {"input": text}
            try:
                item["expected"] = base64.b64encode(base64.b64decode(text, validate=True)).decode()
            except (binascii.Error, ValueError):
                item["error"] = True
            cases.append(item)
    return cases
