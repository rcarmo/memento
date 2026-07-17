from __future__ import annotations

import json
import sqlite3
from dataclasses import dataclass
from datetime import datetime, timezone
from uuid import uuid4

from memento.answers import ModelAttempt


@dataclass(frozen=True, slots=True)
class SchedulerRunRecord:
    run_id: str
    job_name: str
    window_key: str
    base_revision: str | None
    end_revision: str | None
    state: str
    signal_count: int
    proposal_count: int
    model_chain: tuple[ModelAttempt, ...]
    started_at: str
    finished_at: str | None
    error_message: str | None


class SchedulerConflictError(RuntimeError):
    """Raised when a scheduler window is already running or completed."""


@dataclass(frozen=True, slots=True)
class SchedulerClaim:
    created: bool
    record: SchedulerRunRecord


def utcnow() -> str:
    return datetime.now(tz=timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def claim_scheduler_run(
    connection: sqlite3.Connection,
    *,
    job_name: str,
    window_key: str,
    base_revision: str | None,
) -> SchedulerClaim:
    running = connection.execute(
        "SELECT * FROM scheduler_runs WHERE job_name = ? AND state = 'running' LIMIT 1",
        (job_name,),
    ).fetchone()
    if running is not None:
        raise SchedulerConflictError(f"job {job_name} is already running")
    existing = connection.execute(
        "SELECT * FROM scheduler_runs WHERE job_name = ? AND window_key = ?",
        (job_name, window_key),
    ).fetchone()
    if existing is not None:
        return SchedulerClaim(created=False, record=_row_to_run(existing))
    run_id = str(uuid4())
    started_at = utcnow()
    with connection:
        connection.execute(
            """
            INSERT INTO scheduler_runs(
                run_id, job_name, window_key, base_revision, state, started_at
            ) VALUES(?, ?, ?, ?, 'running', ?)
            """,
            (run_id, job_name, window_key, base_revision, started_at),
        )
    return SchedulerClaim(created=True, record=get_scheduler_run(connection, run_id))


def get_scheduler_run(connection: sqlite3.Connection, run_id: str) -> SchedulerRunRecord:
    row = connection.execute("SELECT * FROM scheduler_runs WHERE run_id = ?", (run_id,)).fetchone()
    if row is None:
        raise KeyError(run_id)
    return _row_to_run(row)


def finish_scheduler_run(
    connection: sqlite3.Connection,
    run_id: str,
    *,
    state: str,
    end_revision: str | None,
    signal_count: int,
    proposal_count: int,
    model_chain: tuple[ModelAttempt, ...] = (),
    error_message: str | None = None,
) -> SchedulerRunRecord:
    with connection:
        connection.execute(
            """
            UPDATE scheduler_runs
            SET state = ?, end_revision = ?, signal_count = ?, proposal_count = ?,
                model_chain_json = ?, error_message = ?, finished_at = ?
            WHERE run_id = ?
            """,
            (
                state,
                end_revision,
                signal_count,
                proposal_count,
                json.dumps([item.model_dump(mode="json") for item in model_chain]),
                error_message,
                utcnow(),
                run_id,
            ),
        )
    return get_scheduler_run(connection, run_id)


def _row_to_run(row: sqlite3.Row) -> SchedulerRunRecord:
    chain = json.loads(row["model_chain_json"] or "[]")
    attempts = tuple(
        ModelAttempt(model=str(item), outcome="success")
        if isinstance(item, str)
        else ModelAttempt.model_validate(item)
        for item in chain
    )
    return SchedulerRunRecord(
        run_id=row["run_id"],
        job_name=row["job_name"],
        window_key=row["window_key"],
        base_revision=row["base_revision"],
        end_revision=row["end_revision"],
        state=row["state"],
        signal_count=int(row["signal_count"]),
        proposal_count=int(row["proposal_count"]),
        model_chain=attempts,
        started_at=row["started_at"],
        finished_at=row["finished_at"],
        error_message=row["error_message"],
    )
