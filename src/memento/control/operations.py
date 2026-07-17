from __future__ import annotations

import hashlib
import json
import sqlite3
from dataclasses import dataclass
from datetime import UTC, datetime
from enum import StrEnum


class OperationState(StrEnum):
    QUEUED = "queued"
    RUNNING = "running"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CONFLICT = "conflict"
    RECOVERING = "recovering"


class IdempotencyConflictError(RuntimeError):
    """Raised when the same principal/key pair is reused for another request."""


@dataclass(frozen=True, slots=True)
class OperationRequest:
    op_id: str
    principal: str
    idempotency_key: str
    tool_name: str
    request_json: str
    client_instance_id: str | None = None
    mcp_session_id: str | None = None
    source_chat: str | None = None

    @property
    def request_hash(self) -> str:
        return hashlib.sha256(self.request_json.encode("utf-8")).hexdigest()


@dataclass(frozen=True, slots=True)
class OperationRecord:
    op_id: str
    principal: str
    idempotency_key: str
    tool_name: str
    request_hash: str
    state: OperationState
    request_json: str
    base_revision: str | None
    result_revision: str | None
    result_json: str | None
    error_class: str | None
    error_message: str | None

    @property
    def replay_payload(self) -> dict[str, object] | None:
        if self.result_json is None:
            return None
        payload = json.loads(self.result_json)
        if not isinstance(payload, dict):
            return None
        return {str(key): value for key, value in payload.items()}


def _utcnow() -> str:
    return datetime.now(tz=UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def create_operation(connection: sqlite3.Connection, request: OperationRequest) -> OperationRecord:
    existing = connection.execute(
        "SELECT * FROM operations WHERE principal = ? AND idempotency_key = ?",
        (request.principal, request.idempotency_key),
    ).fetchone()
    if existing is not None:
        if existing["request_hash"] != request.request_hash:
            raise IdempotencyConflictError("idempotency key already used for a different request")
        return operation_from_row(existing)
    with connection:
        connection.execute(
            """
            INSERT INTO operations(
                op_id, idempotency_key, principal, client_instance_id, mcp_session_id,
                source_chat, tool_name, request_hash, state, request_json, created_at
            ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                request.op_id,
                request.idempotency_key,
                request.principal,
                request.client_instance_id,
                request.mcp_session_id,
                request.source_chat,
                request.tool_name,
                request.request_hash,
                OperationState.QUEUED,
                request.request_json,
                _utcnow(),
            ),
        )
    return get_operation(connection, request.op_id)


def get_operation_by_idempotency(
    connection: sqlite3.Connection, principal: str, idempotency_key: str
) -> OperationRecord | None:
    row = connection.execute(
        "SELECT * FROM operations WHERE principal = ? AND idempotency_key = ?",
        (principal, idempotency_key),
    ).fetchone()
    return operation_from_row(row) if row is not None else None


def get_operation(connection: sqlite3.Connection, op_id: str) -> OperationRecord:
    row = connection.execute("SELECT * FROM operations WHERE op_id = ?", (op_id,)).fetchone()
    if row is None:
        raise KeyError(op_id)
    return operation_from_row(row)


def list_interrupted_operations(connection: sqlite3.Connection) -> tuple[OperationRecord, ...]:
    rows = connection.execute(
        "SELECT * FROM operations WHERE state IN (?, ?, ?) ORDER BY created_at, op_id",
        (OperationState.QUEUED, OperationState.RUNNING, OperationState.RECOVERING),
    ).fetchall()
    return tuple(operation_from_row(row) for row in rows)


def mark_operation_running(
    connection: sqlite3.Connection, op_id: str, *, base_revision: str
) -> OperationRecord:
    return _update_operation(
        connection,
        op_id,
        state=OperationState.RUNNING,
        base_revision=base_revision,
        started_at=_utcnow(),
    )


def mark_operation_succeeded(
    connection: sqlite3.Connection,
    op_id: str,
    *,
    result_revision: str,
    result: dict[str, object],
) -> OperationRecord:
    return _update_operation(
        connection,
        op_id,
        state=OperationState.SUCCEEDED,
        result_revision=result_revision,
        result_json=json.dumps(result, sort_keys=True),
        error_class=None,
        error_message=None,
        finished_at=_utcnow(),
    )


def mark_operation_conflict(
    connection: sqlite3.Connection, op_id: str, *, error_message: str
) -> OperationRecord:
    return _update_operation(
        connection,
        op_id,
        state=OperationState.CONFLICT,
        error_class="conflict",
        error_message=error_message,
        finished_at=_utcnow(),
    )


def mark_operation_failed(
    connection: sqlite3.Connection, op_id: str, *, error_class: str, error_message: str
) -> OperationRecord:
    return _update_operation(
        connection,
        op_id,
        state=OperationState.FAILED,
        error_class=error_class,
        error_message=error_message,
        finished_at=_utcnow(),
    )


def _update_operation(
    connection: sqlite3.Connection, op_id: str, **values: object
) -> OperationRecord:
    assignments = ", ".join(f"{column} = ?" for column in values)
    parameters = [*values.values(), op_id]
    with connection:
        connection.execute(f"UPDATE operations SET {assignments} WHERE op_id = ?", parameters)
    return get_operation(connection, op_id)


def operation_from_row(row: sqlite3.Row) -> OperationRecord:
    return OperationRecord(
        op_id=row["op_id"],
        principal=row["principal"],
        idempotency_key=row["idempotency_key"],
        tool_name=row["tool_name"],
        request_hash=row["request_hash"],
        state=OperationState(row["state"]),
        request_json=row["request_json"],
        base_revision=row["base_revision"],
        result_revision=row["result_revision"],
        result_json=row["result_json"],
        error_class=row["error_class"],
        error_message=row["error_message"],
    )
