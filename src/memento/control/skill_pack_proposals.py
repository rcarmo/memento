from __future__ import annotations

import json
import sqlite3
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from enum import StrEnum
from uuid import uuid4

from memento.skill_packs import SkillPackManifest


class SkillPackProposalStatus(StrEnum):
    SUBMITTED = "submitted"
    APPROVED = "approved"
    REJECTED = "rejected"
    APPLIED = "applied"
    STALE = "stale"
    EXPIRED = "expired"


@dataclass(frozen=True, slots=True)
class SkillPackProposalRecord:
    proposal_id: str
    author_principal: str
    base_revision: str
    skill_name: str
    version: str
    rationale: str | None
    skill_md: str
    zip_sha256: str
    zip_bytes: bytes
    manifest: SkillPackManifest
    status: SkillPackProposalStatus
    reviewed_by: str | None
    review_comment: str | None
    applied_operation_id: str | None
    applied_revision: str | None
    created_at: str
    updated_at: str
    expires_at: str | None


def create_skill_pack_proposal(
    connection: sqlite3.Connection,
    *,
    author_principal: str,
    base_revision: str,
    skill_name: str,
    version: str,
    rationale: str | None,
    skill_md: str,
    zip_bytes: bytes,
    manifest: SkillPackManifest,
    ttl_days: int = 30,
) -> SkillPackProposalRecord:
    now = datetime.now(tz=UTC).replace(microsecond=0)
    proposal_id = str(uuid4())
    expires_at = (now + timedelta(days=ttl_days)).isoformat().replace("+00:00", "Z")
    with connection:
        existing = connection.execute(
            "SELECT proposal_id FROM skill_pack_proposals "
            "WHERE skill_name = ? AND version = ? AND status IN ('submitted', 'approved')",
            (skill_name, version),
        ).fetchone()
        if existing is not None:
            raise ValueError(f"active skill pack proposal already exists: {skill_name} {version}")
        connection.execute(
            "INSERT INTO skill_pack_proposals("
            "proposal_id, author_principal, base_revision, skill_name, version, rationale, "
            "skill_md, zip_sha256, zip_bytes, manifest_json, status, created_at, updated_at, expires_at"
            ") VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
            (
                proposal_id,
                author_principal,
                base_revision,
                skill_name,
                version,
                rationale,
                skill_md,
                manifest.sha256,
                zip_bytes,
                manifest.model_dump_json(),
                SkillPackProposalStatus.SUBMITTED.value,
                now.isoformat().replace("+00:00", "Z"),
                now.isoformat().replace("+00:00", "Z"),
                expires_at,
            ),
        )
    return get_skill_pack_proposal(connection, proposal_id)


def get_skill_pack_proposal(
    connection: sqlite3.Connection, proposal_id: str
) -> SkillPackProposalRecord:
    row = connection.execute(
        "SELECT * FROM skill_pack_proposals WHERE proposal_id = ?", (proposal_id,)
    ).fetchone()
    if row is None:
        raise KeyError(f"unknown skill pack proposal: {proposal_id}")
    return _record(row)


def list_skill_pack_proposals(
    connection: sqlite3.Connection, *, status: str | None = None
) -> tuple[SkillPackProposalRecord, ...]:
    if status is None:
        rows = connection.execute(
            "SELECT * FROM skill_pack_proposals ORDER BY created_at, proposal_id"
        ).fetchall()
    else:
        SkillPackProposalStatus(status)
        rows = connection.execute(
            "SELECT * FROM skill_pack_proposals WHERE status = ? ORDER BY created_at, proposal_id",
            (status,),
        ).fetchall()
    return tuple(_record(row) for row in rows)


def set_skill_pack_proposal_status(
    connection: sqlite3.Connection,
    proposal_id: str,
    *,
    status: SkillPackProposalStatus,
    reviewed_by: str | None = None,
    review_comment: str | None = None,
    applied_operation_id: str | None = None,
    applied_revision: str | None = None,
) -> SkillPackProposalRecord:
    now = datetime.now(tz=UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    with connection:
        connection.execute(
            "UPDATE skill_pack_proposals SET status=?, reviewed_by=COALESCE(?, reviewed_by), "
            "review_comment=COALESCE(?, review_comment), applied_operation_id=COALESCE(?, applied_operation_id), "
            "applied_revision=COALESCE(?, applied_revision), updated_at=? WHERE proposal_id=?",
            (
                status.value,
                reviewed_by,
                review_comment,
                applied_operation_id,
                applied_revision,
                now,
                proposal_id,
            ),
        )
    return get_skill_pack_proposal(connection, proposal_id)


def _record(row: sqlite3.Row) -> SkillPackProposalRecord:
    return SkillPackProposalRecord(
        proposal_id=str(row["proposal_id"]),
        author_principal=str(row["author_principal"]),
        base_revision=str(row["base_revision"]),
        skill_name=str(row["skill_name"]),
        version=str(row["version"]),
        rationale=row["rationale"],
        skill_md=str(row["skill_md"]),
        zip_sha256=str(row["zip_sha256"]),
        zip_bytes=bytes(row["zip_bytes"]),
        manifest=SkillPackManifest.model_validate(json.loads(row["manifest_json"])),
        status=SkillPackProposalStatus(row["status"]),
        reviewed_by=row["reviewed_by"],
        review_comment=row["review_comment"],
        applied_operation_id=row["applied_operation_id"],
        applied_revision=row["applied_revision"],
        created_at=str(row["created_at"]),
        updated_at=str(row["updated_at"]),
        expires_at=row["expires_at"],
    )
