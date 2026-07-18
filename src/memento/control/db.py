from __future__ import annotations

import hashlib
import json
import sqlite3
from pathlib import Path

SCHEMA_VERSION = "6"

MIGRATIONS_V1 = (
    """
    CREATE TABLE IF NOT EXISTS operations (
        op_id TEXT PRIMARY KEY,
        idempotency_key TEXT NOT NULL,
        principal TEXT NOT NULL,
        client_instance_id TEXT,
        mcp_session_id TEXT,
        source_chat TEXT,
        tool_name TEXT NOT NULL,
        request_hash TEXT NOT NULL,
        base_revision TEXT,
        result_revision TEXT,
        state TEXT NOT NULL,
        request_json TEXT NOT NULL,
        result_json TEXT,
        error_class TEXT,
        error_message TEXT,
        created_at TEXT NOT NULL,
        started_at TEXT,
        finished_at TEXT,
        UNIQUE(principal, idempotency_key)
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS proposals (
        proposal_id TEXT PRIMARY KEY,
        author_principal TEXT NOT NULL,
        client_instance_id TEXT,
        base_revision TEXT NOT NULL,
        intent TEXT NOT NULL,
        rationale TEXT,
        patch_json TEXT NOT NULL,
        patch_hash TEXT NOT NULL,
        status TEXT NOT NULL,
        reviewed_by TEXT,
        review_comment TEXT,
        applied_operation_id TEXT,
        applied_revision TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        expires_at TEXT,
        FOREIGN KEY(applied_operation_id) REFERENCES operations(op_id)
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS scheduler_runs (
        run_id TEXT PRIMARY KEY,
        job_name TEXT NOT NULL,
        window_key TEXT NOT NULL,
        base_revision TEXT,
        end_revision TEXT,
        state TEXT NOT NULL,
        signal_count INTEGER NOT NULL DEFAULT 0,
        proposal_count INTEGER NOT NULL DEFAULT 0,
        model_chain_json TEXT,
        started_at TEXT NOT NULL,
        finished_at TEXT,
        error_message TEXT,
        UNIQUE(job_name, window_key)
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS service_state (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL,
        updated_at TEXT NOT NULL
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS dream_signals (
        signal_id TEXT PRIMARY KEY,
        signal_type TEXT NOT NULL,
        entity_refs_json TEXT NOT NULL,
        severity TEXT NOT NULL,
        repo_revision TEXT NOT NULL,
        dedupe_key TEXT NOT NULL UNIQUE,
        status TEXT NOT NULL,
        evidence_hash TEXT NOT NULL,
        evidence_json TEXT NOT NULL,
        first_detected_at TEXT NOT NULL,
        last_detected_at TEXT NOT NULL,
        resolved_revision TEXT
    )
    """,
    "CREATE INDEX IF NOT EXISTS idx_dream_signals_status ON dream_signals(status, signal_type)",
)

MIGRATIONS_V6 = (
    """
    CREATE TABLE IF NOT EXISTS proposal_assets (
        proposal_id TEXT NOT NULL,
        asset_id TEXT NOT NULL,
        concept_path TEXT NOT NULL,
        asset_kind TEXT NOT NULL,
        version TEXT NOT NULL,
        media_type TEXT NOT NULL,
        sha256 TEXT NOT NULL,
        blob_bytes BLOB NOT NULL,
        manifest_json TEXT NOT NULL,
        created_at TEXT NOT NULL,
        PRIMARY KEY (proposal_id, asset_id),
        FOREIGN KEY(proposal_id) REFERENCES proposals(proposal_id) ON DELETE CASCADE
    )
    """,
    "CREATE INDEX IF NOT EXISTS idx_proposal_assets_concept ON proposal_assets(concept_path, asset_kind, version)",
    "CREATE INDEX IF NOT EXISTS idx_proposal_assets_created ON proposal_assets(created_at, proposal_id)",
)


class MigrationError(RuntimeError):
    """Raised when control-plane schema migration fails."""


def connect_control_db(path: Path) -> sqlite3.Connection:
    path.parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(path)
    connection.row_factory = sqlite3.Row
    connection.execute("PRAGMA journal_mode=WAL")
    connection.execute("PRAGMA foreign_keys=ON")
    return connection


def migrate_control_db(connection: sqlite3.Connection) -> None:
    with connection:
        for statement in MIGRATIONS_V1:
            connection.execute(statement)
        schema_row = connection.execute(
            "SELECT value FROM service_state WHERE key = 'schema_version'"
        ).fetchone()
        if schema_row is None:
            for statement in MIGRATIONS_V6:
                connection.execute(statement)
            connection.execute(
                "INSERT INTO service_state(key, value, updated_at) "
                "VALUES('schema_version', ?, datetime('now'))",
                (SCHEMA_VERSION,),
            )
            return
        current_version = schema_row["value"]
        if current_version == SCHEMA_VERSION:
            for statement in MIGRATIONS_V6:
                connection.execute(statement)
            return
        if current_version in {"1", "2", "3", "4"}:
            for statement in MIGRATIONS_V6:
                connection.execute(statement)
            connection.execute(
                "UPDATE service_state SET value = ?, updated_at = datetime('now') WHERE key = 'schema_version'",
                (SCHEMA_VERSION,),
            )
            return
        if current_version == "5":
            _migrate_v5_to_v6(connection)
            connection.execute(
                "UPDATE service_state SET value = ?, updated_at = datetime('now') WHERE key = 'schema_version'",
                (SCHEMA_VERSION,),
            )
            return
        raise MigrationError(f"unsupported control schema version: {current_version}")


def _migrate_v5_to_v6(connection: sqlite3.Connection) -> None:
    for statement in MIGRATIONS_V6:
        connection.execute(statement)
    rows = connection.execute(
        "SELECT * FROM skill_pack_proposals ORDER BY created_at, proposal_id"
    ).fetchall()
    for row in rows:
        proposal_id = str(row["proposal_id"])
        existing = connection.execute(
            "SELECT proposal_id FROM proposals WHERE proposal_id = ?", (proposal_id,)
        ).fetchone()
        if existing is not None:
            continue
        concept_path = f"/skills/{row['skill_name']}.md"
        asset_id = _skill_pack_asset_id(str(row["skill_name"]), str(row["version"]))
        patch = {
            "changes": [
                {
                    "kind": "create",
                    "path": concept_path,
                    "concept_type": "concept",
                    "title": str(row["skill_name"]).replace("-", " ").title(),
                    "body": str(row["skill_md"]),
                    "description": f"Versioned agent skill {row['skill_name']}.",
                    "tags": ["skill"],
                    "aliases": [],
                },
                {
                    "kind": "attach_asset_pack",
                    "path": concept_path,
                    "asset_id": asset_id,
                    "asset_kind": "skill",
                    "zip_sha256": str(row["zip_sha256"]),
                    "version": str(row["version"]),
                    "manifest": json.loads(str(row["manifest_json"])),
                },
            ]
        }
        patch_json = json.dumps(patch, sort_keys=True)
        patch_hash = hashlib.sha256(patch_json.encode("utf-8")).hexdigest()
        connection.execute(
            """
            INSERT INTO proposals(
                proposal_id, author_principal, client_instance_id, base_revision,
                intent, rationale, patch_json, patch_hash, status,
                reviewed_by, review_comment, applied_operation_id, applied_revision,
                created_at, updated_at, expires_at
            ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                proposal_id,
                str(row["author_principal"]),
                None,
                str(row["base_revision"]),
                f"Attach skill asset {row['skill_name']} {row['version']}",
                row["rationale"],
                patch_json,
                patch_hash,
                str(row["status"]),
                row["reviewed_by"],
                row["review_comment"],
                row["applied_operation_id"],
                row["applied_revision"],
                str(row["created_at"]),
                str(row["updated_at"]),
                row["expires_at"],
            ),
        )
        connection.execute(
            """
            INSERT INTO proposal_assets(
                proposal_id, asset_id, concept_path, asset_kind, version,
                media_type, sha256, blob_bytes, manifest_json, created_at
            ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                proposal_id,
                asset_id,
                concept_path,
                "skill",
                str(row["version"]),
                "application/zip",
                str(row["zip_sha256"]),
                bytes(row["zip_bytes"]),
                str(row["manifest_json"]),
                str(row["created_at"]),
            ),
        )


def _skill_pack_asset_id(skill_name: str, version: str) -> str:
    return f"skill-pack:{skill_name}:{version}"
