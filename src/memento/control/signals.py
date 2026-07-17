from __future__ import annotations

import hashlib
import json
import sqlite3
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any
from uuid import uuid4


@dataclass(frozen=True, slots=True)
class DreamSignal:
    signal_id: str
    signal_type: str
    entity_refs: tuple[str, ...]
    severity: str
    repo_revision: str
    dedupe_key: str
    status: str
    evidence_hash: str
    evidence_json: str
    first_detected_at: str
    last_detected_at: str
    resolved_revision: str | None


@dataclass(frozen=True, slots=True)
class DetectedSignal:
    signal_type: str
    entity_refs: tuple[str, ...]
    severity: str
    dedupe_key: str
    evidence: dict[str, Any]

    @property
    def evidence_hash(self) -> str:
        return hashlib.sha256(
            json.dumps(self.evidence, sort_keys=True, separators=(",", ":")).encode("utf-8")
        ).hexdigest()


def utcnow() -> str:
    return datetime.now(tz=UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def list_signals(connection: sqlite3.Connection) -> tuple[DreamSignal, ...]:
    rows = connection.execute(
        "SELECT * FROM dream_signals ORDER BY signal_type, dedupe_key"
    ).fetchall()
    return tuple(_row_to_signal(row) for row in rows)


def upsert_detected_signals(
    connection: sqlite3.Connection,
    *,
    repo_revision: str,
    detections: list[DetectedSignal],
) -> tuple[DreamSignal, ...]:
    now = utcnow()
    by_key = {signal.dedupe_key: signal for signal in list_signals(connection)}
    seen_keys = {item.dedupe_key for item in detections}
    with connection:
        for detection in detections:
            evidence_json = json.dumps(detection.evidence, sort_keys=True)
            existing = by_key.get(detection.dedupe_key)
            if existing is None:
                connection.execute(
                    """
                    INSERT INTO dream_signals(
                        signal_id, signal_type, entity_refs_json, severity, repo_revision,
                        dedupe_key, status, evidence_hash, evidence_json,
                        first_detected_at, last_detected_at, resolved_revision
                    ) VALUES(?, ?, ?, ?, ?, ?, 'open', ?, ?, ?, ?, NULL)
                    """,
                    (
                        str(uuid4()),
                        detection.signal_type,
                        json.dumps(list(detection.entity_refs)),
                        detection.severity,
                        repo_revision,
                        detection.dedupe_key,
                        detection.evidence_hash,
                        evidence_json,
                        now,
                        now,
                    ),
                )
                continue
            status = existing.status
            resolved_revision = existing.resolved_revision
            if (
                existing.status in {"resolved", "ignored"}
                and existing.evidence_hash != detection.evidence_hash
            ):
                status = "open"
                resolved_revision = None
            connection.execute(
                """
                UPDATE dream_signals
                SET signal_type = ?, entity_refs_json = ?, severity = ?, repo_revision = ?,
                    status = ?, evidence_hash = ?, evidence_json = ?, last_detected_at = ?,
                    resolved_revision = ?
                WHERE dedupe_key = ?
                """,
                (
                    detection.signal_type,
                    json.dumps(list(detection.entity_refs)),
                    detection.severity,
                    repo_revision,
                    status,
                    detection.evidence_hash,
                    evidence_json,
                    now,
                    resolved_revision,
                    detection.dedupe_key,
                ),
            )
        for existing in by_key.values():
            if existing.dedupe_key in seen_keys:
                continue
            if existing.status in {"open", "acknowledged", "proposed"}:
                connection.execute(
                    "UPDATE dream_signals SET status = 'resolved', resolved_revision = ? WHERE dedupe_key = ?",
                    (repo_revision, existing.dedupe_key),
                )
    return list_signals(connection)


def actionable_signals(connection: sqlite3.Connection) -> tuple[DreamSignal, ...]:
    rows = connection.execute(
        "SELECT * FROM dream_signals WHERE status IN ('open', 'acknowledged') ORDER BY signal_type, dedupe_key"
    ).fetchall()
    return tuple(_row_to_signal(row) for row in rows)


def mark_signals_status(
    connection: sqlite3.Connection, *, dedupe_keys: tuple[str, ...], status: str
) -> None:
    if not dedupe_keys:
        return
    placeholders = ", ".join("?" for _ in dedupe_keys)
    with connection:
        connection.execute(
            f"UPDATE dream_signals SET status = ? WHERE dedupe_key IN ({placeholders})",
            (status, *dedupe_keys),
        )


def set_service_state(connection: sqlite3.Connection, *, key: str, value: str) -> None:
    with connection:
        connection.execute(
            """
            INSERT INTO service_state(key, value, updated_at) VALUES(?, ?, datetime('now'))
            ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=datetime('now')
            """,
            (key, value),
        )


def get_service_state(connection: sqlite3.Connection, *, key: str) -> str | None:
    row = connection.execute("SELECT value FROM service_state WHERE key = ?", (key,)).fetchone()
    return None if row is None else str(row["value"])


def _row_to_signal(row: sqlite3.Row) -> DreamSignal:
    return DreamSignal(
        signal_id=row["signal_id"],
        signal_type=row["signal_type"],
        entity_refs=tuple(str(item) for item in json.loads(row["entity_refs_json"])),
        severity=row["severity"],
        repo_revision=row["repo_revision"],
        dedupe_key=row["dedupe_key"],
        status=row["status"],
        evidence_hash=row["evidence_hash"],
        evidence_json=row["evidence_json"],
        first_detected_at=row["first_detected_at"],
        last_detected_at=row["last_detected_at"],
        resolved_revision=row["resolved_revision"],
    )
