"""Capture proposal visibility reads and whole-archive-before-file integrity."""

from __future__ import annotations

import base64
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


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    pack: Any = importlib.import_module("memento.skill_packs")
    authz: Any = importlib.import_module("memento.authz")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    cases = []
    for scenario in [
        "detailed",
        "summary",
        "unknown-view",
        "missing-target",
        "invalid-target",
        "stale",
        "reader",
        "curator-only",
        "denied",
        "metadata",
        "file",
        "binary",
        "offset",
        "limit",
        "missing-file",
        "missing-asset",
        "stored-denied",
        "bad-manifest",
        "bad-sha",
        "bad-bytes",
        "metadata-bad-bytes",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-get-") as directory:
            root = Path(directory)
            initial = "---\nid: '12345678'\ntype: concept\ntitle: Title\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\nupdated_by: original\n---\nbody\n"
            if scenario != "missing-target":
                (root / "a.md").write_text("invalid" if scenario == "invalid-target" else initial)
            connection = db.connect_control_db(root / "control.sqlite")
            db.migrate_control_db(connection)
            output = io.BytesIO()
            with zipfile.ZipFile(output, "w") as archive:
                info = zipfile.ZipInfo("file.txt", date_time=(2026, 1, 1, 0, 0, 0))
                archive.writestr(info, b"hello\xff" if scenario == "binary" else b"hello world")
            blob = output.getvalue()
            validated = pack.validate_asset_pack(asset_kind="docs", version="1.0.0", zip_bytes=blob)
            manifest = "{" if scenario == "bad-manifest" else validated.manifest.model_dump_json()
            digest = "a" * 64 if scenario == "bad-sha" else validated.manifest.sha256
            if scenario in {"bad-bytes", "metadata-bad-bytes"}:
                blob += b"tampered"
            asset_input = proposals.ProposalAssetInput(
                asset_id="asset",
                concept_path="/private/a.md" if scenario == "stored-denied" else "/a.md",
                asset_kind="docs",
                version="1.0.0",
                media_type="application/zip",
                sha256=digest,
                blob_bytes=blob,
                manifest_json=manifest,
            )
            proposals.create_proposal(
                connection,
                proposal_id="proposal",
                author_principal="author",
                client_instance_id=None,
                base_revision="base" if scenario == "stale" else "main",
                intent="intent",
                rationale=None,
                patch={"changes": [{"kind": "patch", "path": "/a.md", "body": "new"}]},
                assets=[asset_input],
            )
            connection.execute(
                "UPDATE proposals SET created_at='created',updated_at='updated',expires_at=NULL"
            )
            connection.commit()
            record = proposals.get_proposal(connection, "proposal")
            roles = (
                ("reader",)
                if scenario == "reader"
                else ("curator",)
                if scenario == "curator-only"
                else ("proposer",)
            )
            policy = authz.EffectivePolicy(
                "author", roles, () if scenario == "denied" else ("/",), (), ("/private/",)
            )
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection,
                repo_paths=SimpleNamespace(current_dir=root),
                config=SimpleNamespace(limits=SimpleNamespace(max_concept_bytes=65536)),
            )
            runtime._policy = lambda _, policy=policy: policy
            runtime._now = lambda: datetime(2026, 9, 18, tzinfo=UTC)
            is_asset = scenario not in {
                "detailed",
                "summary",
                "unknown-view",
                "missing-target",
                "invalid-target",
                "stale",
                "reader",
                "curator-only",
                "denied",
            }
            file = (
                None
                if scenario in {"metadata", "metadata-bad-bytes"}
                else "missing"
                if scenario == "missing-file"
                else "file.txt"
            )
            item: dict[str, Any] = {
                "scenario": scenario,
                "record": asdict(record),
                "initial": initial,
                "policy": asdict(policy),
                "asset": {**asdict(asset_input), "blob_bytes": base64.b64encode(blob).decode()},
                "is_asset": is_asset,
                "file": file,
                "offset": -1 if scenario == "offset" else 0,
                "limit": 0 if scenario == "limit" else 5,
            }
            with (
                patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
                patch.object(service, "get_main_revision", lambda _: "main"),
                patch.object(service, "diff_main_paths", lambda *args, **kwargs: ("/a.md",)),
            ):
                try:
                    if is_asset:
                        result = runtime.memory_proposal_asset_get(
                            None,
                            proposal_id="proposal",
                            asset_id="missing" if scenario == "missing-asset" else "asset",
                            file_path=file,
                            offset=item["offset"],
                            limit=item["limit"],
                        )
                    else:
                        result = runtime.memory_proposal_get(
                            None,
                            proposal_id="proposal",
                            view="summary"
                            if scenario == "summary"
                            else "unknown"
                            if scenario == "unknown-view"
                            else "detailed",
                        )
                    item["response"] = result.model_dump(mode="json")
                except Exception as exc:
                    item["exception"] = type(exc).__name__
            item["after"] = asdict(proposals.get_proposal(connection, "proposal"))
            cases.append(item)
            connection.close()
    return cases
