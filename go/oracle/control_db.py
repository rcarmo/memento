"""Control schema and v5 migration fixtures on disposable databases only."""

from __future__ import annotations

import base64
import importlib
import sqlite3
import tempfile
from pathlib import Path
from typing import Any

LEGACY = """CREATE TABLE skill_pack_proposals (
proposal_id TEXT PRIMARY KEY,author_principal TEXT NOT NULL,base_revision TEXT NOT NULL,
skill_name TEXT NOT NULL,version TEXT NOT NULL,skill_md TEXT NOT NULL,zip_sha256 TEXT NOT NULL,
zip_bytes BLOB NOT NULL,manifest_json TEXT NOT NULL,permission_summary_json TEXT NOT NULL,
status TEXT NOT NULL,rationale TEXT,reviewed_by TEXT,review_comment TEXT,
applied_operation_id TEXT,applied_revision TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,expires_at TEXT)
"""


def snapshot(connection: sqlite3.Connection) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for table in ["service_state", "proposals", "proposal_assets"]:
        rows = []
        if (
            connection.execute(
                "SELECT name FROM sqlite_master WHERE type='table' AND name=?", (table,)
            ).fetchone()
            is None
        ):
            result[table] = None
            continue
        for row in connection.execute(f"SELECT * FROM {table}").fetchall():
            item = dict(row)
            if table == "service_state":
                item["updated_at"] = "<clock>"
            if "blob_bytes" in item:
                item["blob_bytes"] = base64.b64encode(item["blob_bytes"]).decode()
            rows.append(item)
        result[table] = rows
    result["schema"] = [
        {"type": row["type"], "name": row["name"], "table": row["tbl_name"], "sql": row["sql"]}
        for row in connection.execute(
            "SELECT type,name,tbl_name,sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name"
        )
    ]
    return result


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.control.db")
    cases = []
    for mode in [
        "fresh",
        "1",
        "2",
        "3",
        "4",
        "6",
        "7",
        "8",
        "9",
        "10",
        "future",
        "5",
        "5-skip",
        "5-invalid",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-control-oracle-") as directory:
            path = Path(directory) / "control.sqlite"
            connection = module.connect_control_db(path)
            if mode != "fresh":
                for statement in module.MIGRATIONS_V1:
                    connection.execute(statement)
                version = "5" if mode.startswith("5") else mode
                connection.execute(
                    "INSERT INTO service_state VALUES('schema_version',?,'original')", (version,)
                )
                if mode.startswith("5"):
                    connection.execute(LEGACY)
                    for proposal_id, name, manifest in [
                        ("legacy-one", "demo-ß-skill", '{"z": "日本", "a": 1.0}'),
                        ("legacy-two", "second-skill", "{" if mode == "5-invalid" else "{}"),
                    ]:
                        connection.execute(
                            "INSERT INTO skill_pack_proposals VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
                            (
                                proposal_id,
                                "agent",
                                "base",
                                name,
                                "1.0",
                                "# Synthetic\n",
                                "abc",
                                b"synthetic zip",
                                manifest,
                                "{}",
                                "submitted",
                                "reason",
                                None,
                                None,
                                None,
                                None,
                                "2026-09-18T00:00:00Z",
                                "2026-09-18T00:00:00Z",
                                None,
                            ),
                        )
                    if mode == "5-skip":
                        connection.execute(
                            "INSERT INTO proposals(proposal_id,author_principal,base_revision,intent,patch_json,patch_hash,status,created_at,updated_at) VALUES('legacy-one','agent','base','keep-me','{}','hash','rejected','old','old')"
                        )
                connection.commit()
            connection.execute("PRAGMA wal_checkpoint(TRUNCATE)")
            connection.close()
            before = path.read_bytes()
            connection = module.connect_control_db(path)
            error = ""
            try:
                module.migrate_control_db(connection)
                module.migrate_control_db(connection)  # repeat migration is stable
            except Exception as exc:
                error = type(exc).__name__
            result = snapshot(connection)
            connection.close()
            cases.append(
                {
                    "mode": mode,
                    "database": base64.b64encode(before).decode(),
                    "expected": result,
                    "error": error,
                }
            )
    return {"schema_version": module.SCHEMA_VERSION, "cases": cases}
