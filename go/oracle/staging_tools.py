"""Direct staging MCP wrappers: role source, one-time ticket/status semantics."""

from __future__ import annotations

import asyncio
import base64
import importlib
import io
import tempfile
import zipfile
from datetime import UTC, datetime, timedelta
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    server: Any = importlib.import_module("memento.server")
    service: Any = importlib.import_module("memento.service")
    config: Any = importlib.import_module("memento.config")
    db: Any = importlib.import_module("memento.control.db")
    staging: Any = importlib.import_module("memento.staged_assets")
    output = io.BytesIO()
    with zipfile.ZipFile(output, "w") as archive:
        archive.writestr(zipfile.ZipInfo("file.txt", date_time=(2026, 1, 1, 0, 0, 0)), "synthetic")
    blob = output.getvalue()
    cases: list[dict[str, Any]] = []
    commands = [
        ("begin", "reader", "one", "docs", "1.0.0"),
        ("status", "reader", "one", "docs", "1.0.0"),
        ("unavailable", "actor", "one", "docs", "1.0.0"),
        ("unavailable_status", "actor", "one", "docs", "1.0.0"),
        ("status", "actor", "missing", "docs", "1.0.0"),
        ("begin", "actor", "", "docs", "1.0.0"),
        ("begin", "actor", "bad", "docs", "latest"),
        ("begin", "actor", "one", "docs", "1.0.0"),
        ("status", "actor", "one", "docs", "1.0.0"),
        ("begin", "actor", "one", "docs", "1.0.0"),
        ("begin", "actor", "one", "docs", "2.0.0"),
        ("upload", "actor", "one", "docs", "1.0.0"),
        ("status", "actor", "one", "docs", "1.0.0"),
        ("status", "other", "one", "docs", "1.0.0"),
        ("expire", "actor", "one", "docs", "1.0.0"),
        ("status", "actor", "one", "docs", "1.0.0"),
        ("begin", "actor", "one", "docs", "1.0.0"),
        ("begin", "actor", "two", "docs", "1.0.0"),
        ("expire", "actor", "two", "docs", "1.0.0"),
        ("status", "actor", "two", "docs", "1.0.0"),
        ("begin", "actor", "two", "docs", "1.0.0"),
    ]
    with tempfile.TemporaryDirectory(prefix="memento-staging-tools-") as directory:
        connection = db.connect_control_db(Path(directory) / "control.sqlite")
        db.migrate_control_db(connection)
        store = staging.StagedAssetStore(connection)
        runtime = object.__new__(server.MementoMCPServer)
        memory = object.__new__(service.MemoryService)
        memory._deps = SimpleNamespace(repo_paths=None, staged_asset_store=store)
        runtime._service = memory
        clock = [datetime(2026, 9, 18, tzinfo=UTC)]
        token = "memento_upload_" + base64.urlsafe_b64encode(bytes(32)).decode().rstrip("=")
        tokens = iter(
            base64.urlsafe_b64encode(bytes([i]) * 32).decode().rstrip("=") for i in range(2)
        )
        with (
            patch.object(staging, "_now", lambda: clock[0]),
            patch.object(staging.secrets, "token_urlsafe", lambda _: next(tokens)),
            patch.object(staging, "uuid4", lambda: "00000000-0000-4000-8000-000000000000"),
            patch.object(service, "get_main_revision", lambda _: "main"),
        ):
            for action, principal, key, kind, version in commands:
                if action == "upload":
                    store.put_with_ticket(raw_token=token, zip_bytes=blob)
                    cases.append({"action": action})
                    continue
                if action == "expire":
                    clock[0] += timedelta(hours=25)
                    cases.append({"action": action})
                    continue
                roles = ("reader",) if principal == "reader" else ("proposer",)
                runtime._context = lambda principal=principal, roles=roles: SimpleNamespace(
                    principal=config.Principal(name=principal, roles=roles)
                )
                memory._deps.staged_asset_store = (
                    None if action.startswith("unavailable") else store
                )
                result = asyncio.run(
                    runtime.tool_memory_asset_stage_status(key)
                    if action in {"status", "unavailable_status"}
                    else runtime.tool_memory_asset_stage_begin(kind, version, key)
                )
                cases.append(
                    {
                        "action": action,
                        "principal": principal,
                        "roles": roles,
                        "key": key,
                        "asset_kind": kind,
                        "version": version,
                        "expected": result,
                    }
                )
        connection.close()
    return {"zip": base64.b64encode(blob).decode(), "cases": cases}
