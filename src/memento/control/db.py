from __future__ import annotations

import sqlite3
from pathlib import Path

SCHEMA_VERSION = "2"

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
            connection.execute(
                "INSERT INTO service_state(key, value, updated_at) "
                "VALUES('schema_version', ?, datetime('now'))",
                (SCHEMA_VERSION,),
            )
            return
        if schema_row["value"] == SCHEMA_VERSION:
            return
        if schema_row["value"] == "1":
            connection.execute(
                "UPDATE service_state SET value = ?, updated_at = datetime('now') WHERE key = 'schema_version'",
                (SCHEMA_VERSION,),
            )
            return
        raise MigrationError(f"unsupported control schema version: {schema_row['value']}")
