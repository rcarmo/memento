"""Deterministic staged asset/ticket lifecycle with synthetic archives and tokens."""

from __future__ import annotations

import base64
import importlib
import tempfile
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from unittest.mock import patch
from uuid import UUID

from access_store import Random
from asset_pack import archive


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.staged_assets")
    db: Any = importlib.import_module("memento.control.db")
    random = Random()
    clock = [datetime(2026, 9, 18, tzinfo=UTC)]
    packs = {
        "one": archive([("doc.md", b"# Synthetic\n", 0o100644)]),
        "two": archive([("doc.md", b"different\n", 0o100644)]),
    }
    commands: list[dict[str, Any]] = [
        {"action": "put", "key": "asset", "pack": "one"},
        {"action": "put", "key": "asset", "pack": "one"},
        {"action": "put", "key": "asset", "pack": "two"},
        {"action": "get", "principal": "other"},
        {"action": "consume", "ids": ["{{ASSET}}", "missing"]},
        {"action": "get", "ready": True},
        {"action": "consume", "ids": ["{{ASSET}}"]},
        {"action": "get", "ready": True},
        {"action": "get"},
        {"action": "put", "key": "asset", "pack": "one"},
        {"action": "begin", "key": "ticket"},
        {"action": "begin", "key": "ticket"},
        {"action": "begin", "key": "ticket", "version": "2.0.0"},
        {"action": "status", "key": "ticket", "principal": "other"},
        {"action": "upload", "pack": "one"},
        {"action": "upload", "pack": "one"},
        {"action": "upload", "pack": "two"},
        {"action": "status", "key": "ticket"},
        {"action": "begin", "key": "never-used"},
        {"action": "advance", "hours": 25},
        {"action": "upload", "pack": "one"},
        {"action": "status", "key": "never-used"},
        {"action": "status", "key": "ticket"},
        {"action": "get"},
        {"action": "put", "key": "ticket:ticket", "pack": "one"},
        {"action": "begin", "key": "never-used"},
        {"action": "put", "key": " ", "pack": "one"},
        {"action": "begin", "key": " "},
    ]
    with tempfile.TemporaryDirectory(prefix="memento-stage-oracle-") as directory:
        connection = db.connect_control_db(Path(directory) / "control.sqlite")
        db.migrate_control_db(connection)
        connection.execute(
            "INSERT INTO proposals(proposal_id,author_principal,base_revision,intent,patch_json,patch_hash,status,created_at,updated_at) VALUES('proposal','agent','base','test','{}','hash','submitted','now','now')"
        )
        connection.commit()
        store = module.StagedAssetStore(connection)
        cases = []
        asset_id = ""
        token = ""
        with (
            patch.object(module, "_now", lambda: clock[0]),
            patch.object(module, "uuid4", lambda: UUID(bytes=random.bytes(16), version=4)),
            patch.object(module.secrets, "token_urlsafe", random.token),
        ):
            for command in commands:
                item: dict[str, Any] = {"input": command}
                principal = command.get("principal", "agent")
                try:
                    action = command["action"]
                    if action == "advance":
                        from datetime import timedelta

                        clock[0] += timedelta(hours=command["hours"])
                    elif action == "put":
                        staged, replayed = store.put(
                            principal=principal,
                            idempotency_key=command["key"],
                            asset_kind="asset",
                            version="1.0.0",
                            zip_bytes=packs[command["pack"]],
                        )
                        asset_id = staged.staged_asset_id
                        item.update(
                            payload=staged.public_payload(),
                            blob=base64.b64encode(staged.blob_bytes).decode(),
                            replayed=replayed,
                        )
                    elif action == "get":
                        staged = store.get(
                            principal=principal,
                            staged_asset_id=asset_id,
                            require_ready=command.get("ready", False),
                        )
                        item.update(
                            payload=staged.public_payload(),
                            blob=base64.b64encode(staged.blob_bytes).decode(),
                        )
                    elif action == "consume":
                        store.consume(
                            principal=principal,
                            staged_asset_ids=tuple(
                                value.replace("{{ASSET}}", asset_id) for value in command["ids"]
                            ),
                            proposal_id="proposal",
                        )
                    elif action == "begin":
                        ticket, token = store.begin_upload(
                            principal=principal,
                            idempotency_key=command["key"],
                            asset_kind="asset",
                            version=command.get("version", "1.0.0"),
                        )
                        item.update(token=token, state=ticket.state)
                    elif action == "status":
                        ticket = store.ticket_status(
                            principal=principal, idempotency_key=command["key"]
                        )
                        item.update(state=ticket.state, asset_id=ticket.staged_asset_id)
                    else:
                        staged, replayed = store.put_with_ticket(
                            raw_token=token, zip_bytes=packs[command["pack"]]
                        )
                        asset_id = staged.staged_asset_id
                        item.update(
                            payload=staged.public_payload(),
                            blob=base64.b64encode(staged.blob_bytes).decode(),
                            replayed=replayed,
                        )
                except Exception as exc:
                    item["error"] = str(exc)
                cases.append(item)
            rows: dict[str, list[dict[str, Any]]] = {}
            for table in ["staged_assets", "asset_upload_tickets"]:
                rows[table] = []
                for row in connection.execute(f"SELECT * FROM {table} ORDER BY 1"):
                    item = dict(row)
                    if "blob_bytes" in item:
                        item["blob_bytes"] = base64.b64encode(item["blob_bytes"]).decode()
                    rows[table].append(item)
        connection.close()
    return {
        "packs": {key: base64.b64encode(value).decode() for key, value in packs.items()},
        "cases": cases,
        "rows": rows,
    }
