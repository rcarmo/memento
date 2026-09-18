"""Real staging-store HTTP route ordering, tickets and public response fields."""

from __future__ import annotations

import base64
import importlib
import io
import json
import tempfile
import zipfile
from datetime import UTC, datetime, timedelta
from pathlib import Path
from typing import Any
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.staging_http")
    staging: Any = importlib.import_module("memento.staged_assets")
    db: Any = importlib.import_module("memento.control.db")
    config: Any = importlib.import_module("memento.config")
    data = io.BytesIO()
    with zipfile.ZipFile(data, "w") as archive:
        archive.writestr(zipfile.ZipInfo("file.txt", date_time=(2026, 1, 1, 0, 0, 0)), "synthetic")
    blob = data.getvalue()
    stamp = datetime(2026, 9, 18, tzinfo=UTC)
    commands: list[dict[str, Any]] = [
        {"method": "GET", "path": "/other"},
        {"method": "GET", "path": "/assets/staging"},
        {"method": "POST", "path": "/assets/staging", "headers": {"authorization": "reader"}},
        {"method": "POST", "path": "/assets/staging/upload", "headers": {}},
        {
            "method": "POST",
            "path": "/assets/staging/upload",
            "headers": {"content-type": "application/zip"},
        },
        {
            "method": "POST",
            "path": "/assets/staging",
            "headers": {"authorization": "actor", "content-type": "Application/Zip"},
        },
        {
            "method": "POST",
            "path": "/assets/staging?ignored=1",
            "headers": {
                "authorization": "actor",
                "content-type": " application/zip ; charset=binary",
                "x-memento-asset-kind": "docs",
                "x-memento-asset-version": "1.0.0",
                "idempotency-key": "inline",
            },
            "body": "zip",
        },
        {
            "method": "POST",
            "path": "/assets/staging",
            "headers": {
                "authorization": "actor",
                "content-type": "application/zip",
                "x-memento-asset-kind": "docs",
                "x-memento-asset-version": "1.0.0",
                "idempotency-key": "inline",
            },
            "body": "zip",
        },
        {
            "method": "GET",
            "path": "/assets/staging/00000000-0000-4000-8000-000000000000",
            "headers": {"authorization": "actor"},
        },
        {
            "method": "GET",
            "path": "/assets/staging/00000000-0000-4000-8000-000000000000",
            "headers": {"authorization": "other"},
        },
        {"method": "PUT", "path": "/assets/staging", "headers": {"authorization": "actor"}},
        {"method": "GET", "path": "/assets/staging/", "headers": {"authorization": "actor"}},
        {
            "method": "POST",
            "path": "/assets/staging/upload",
            "headers": {"content-type": "application/zip", "x-memento-upload-ticket": "ticket"},
            "body": "zip",
        },
        {
            "method": "POST",
            "path": "/assets/staging/upload?x=y",
            "headers": {
                "authorization": "reader",
                "content-type": "application/zip",
                "x-memento-upload-ticket": "ticket",
            },
            "body": "zip",
        },
        {
            "method": "POST",
            "path": "/assets/staging/upload",
            "headers": {"content-type": "application/zip", "x-memento-upload-ticket": "ticket"},
            "body": "bad",
        },
        {"method": "GET", "path": "/assets/staging/%30", "headers": {"authorization": "actor"}},
        {"method": "GET", "path": "/assets/staging/upload", "headers": {}},
        {
            "method": "GET",
            "path": "/assets/staging/00000000-0000-4000-8000-000000000000",
            "headers": {"authorization": "actor"},
            "advance": 25,
        },
        {
            "method": "POST",
            "path": "/assets/staging/upload",
            "headers": {"content-type": "application/zip", "x-memento-upload-ticket": "ticket"},
            "body": "zip",
        },
    ]
    cases = []
    with tempfile.TemporaryDirectory(prefix="memento-staging-http-") as directory:
        connection = db.connect_control_db(Path(directory) / "control.sqlite")
        db.migrate_control_db(connection)
        store = staging.StagedAssetStore(connection)
        clock = [stamp]
        ids = iter([f"00000000-0000-4000-8000-{i:012d}" for i in range(10)])
        calls = []

        def authenticate(headers: Any) -> Any:
            calls.append(dict(headers))
            name = headers.get("authorization")
            return (
                config.Principal(
                    name=name, roles=("reader",) if name == "reader" else ("proposer",)
                )
                if name in {"reader", "actor", "other"}
                else None
            )

        handler = module.AssetStagingHTTPHandler(store, authenticate)
        with (
            patch.object(staging, "_now", lambda: clock[0]),
            patch.object(staging, "uuid4", lambda: next(ids)),
            patch.object(staging.secrets, "token_urlsafe", lambda _: "ticket"),
        ):
            store.begin_upload(
                principal="actor", idempotency_key="ticket-key", asset_kind="docs", version="1.0.0"
            )
            ticket_row = dict(connection.execute("SELECT * FROM asset_upload_tickets").fetchone())
            for command in commands:
                if command.get("advance"):
                    clock[0] += timedelta(hours=command["advance"])
                before = len(calls)
                body = (
                    blob
                    if command.get("body") == "zip"
                    else b"bad"
                    if command.get("body") == "bad"
                    else b""
                )
                response = handler.handle(
                    method=command["method"],
                    path=command["path"],
                    headers=command.get("headers", {}),
                    body=body,
                )
                expected = (
                    None
                    if response is None
                    else {
                        "status": response.status,
                        "content_type": response.content_type,
                        "headers": response.headers,
                        "body": json.loads(response.body),
                    }
                )
                cases.append({**command, "expected": expected, "auth_calls": len(calls) - before})
        connection.close()
    return {"zip": base64.b64encode(blob).decode(), "ticket": ticket_row, "cases": cases}
