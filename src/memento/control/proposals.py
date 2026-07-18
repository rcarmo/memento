from __future__ import annotations

import hashlib
import json
import sqlite3
from collections.abc import Iterable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from enum import StrEnum
from typing import Any


class ProposalStatus(StrEnum):
    DRAFT = "draft"
    SUBMITTED = "submitted"
    APPROVED = "approved"
    REJECTED = "rejected"
    APPLIED = "applied"
    STALE = "stale"
    EXPIRED = "expired"


class ProposalTransitionError(RuntimeError):
    """Raised when a proposal state transition is invalid."""


@dataclass(frozen=True, slots=True)
class ProposalRecord:
    proposal_id: str
    author_principal: str
    client_instance_id: str | None
    base_revision: str
    intent: str
    rationale: str | None
    patch_json: str
    patch_hash: str
    status: ProposalStatus
    reviewed_by: str | None
    review_comment: str | None
    applied_operation_id: str | None
    applied_revision: str | None
    created_at: str
    updated_at: str
    expires_at: str | None

    @property
    def patch(self) -> dict[str, Any]:
        value = json.loads(self.patch_json)
        if not isinstance(value, dict):
            raise TypeError("proposal patch must decode to an object")
        return value


@dataclass(frozen=True, slots=True)
class ProposalAssetRecord:
    proposal_id: str
    asset_id: str
    concept_path: str
    asset_kind: str
    version: str
    media_type: str
    sha256: str
    blob_bytes: bytes
    manifest_json: str
    created_at: str

    @property
    def manifest(self) -> dict[str, Any]:
        value = json.loads(self.manifest_json)
        if not isinstance(value, dict):
            raise TypeError("proposal asset manifest must decode to an object")
        return value


@dataclass(frozen=True, slots=True)
class ProposalAssetInput:
    asset_id: str
    concept_path: str
    asset_kind: str
    version: str
    media_type: str
    sha256: str
    blob_bytes: bytes
    manifest_json: str


DEFAULT_PROPOSAL_TTL_DAYS = 30


def utcnow() -> str:
    return datetime.now(tz=UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def create_proposal(
    connection: sqlite3.Connection,
    *,
    proposal_id: str,
    author_principal: str,
    client_instance_id: str | None,
    base_revision: str,
    intent: str,
    rationale: str | None,
    patch: dict[str, Any],
    expires_in_days: int = DEFAULT_PROPOSAL_TTL_DAYS,
    assets: Iterable[ProposalAssetInput] = (),
) -> ProposalRecord:
    patch_json = json.dumps(patch, sort_keys=True)
    patch_hash = hashlib.sha256(patch_json.encode("utf-8")).hexdigest()
    now = utcnow()
    expires_at = (
        (datetime.now(tz=UTC).replace(microsecond=0) + timedelta(days=expires_in_days))
        .isoformat()
        .replace("+00:00", "Z")
    )
    asset_rows = tuple(assets)
    with connection:
        connection.execute(
            """
            INSERT INTO proposals(
                proposal_id, author_principal, client_instance_id, base_revision,
                intent, rationale, patch_json, patch_hash, status,
                created_at, updated_at, expires_at
            ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                proposal_id,
                author_principal,
                client_instance_id,
                base_revision,
                intent,
                rationale,
                patch_json,
                patch_hash,
                ProposalStatus.SUBMITTED.value,
                now,
                now,
                expires_at,
            ),
        )
        _insert_proposal_assets(
            connection, proposal_id=proposal_id, assets=asset_rows, created_at=now
        )
    return get_proposal(connection, proposal_id)


def get_proposal(connection: sqlite3.Connection, proposal_id: str) -> ProposalRecord:
    row = connection.execute(
        "SELECT * FROM proposals WHERE proposal_id = ?", (proposal_id,)
    ).fetchone()
    if row is None:
        raise KeyError(proposal_id)
    return proposal_from_row(row)


def get_proposal_asset(
    connection: sqlite3.Connection, proposal_id: str, asset_id: str
) -> ProposalAssetRecord:
    row = connection.execute(
        "SELECT * FROM proposal_assets WHERE proposal_id = ? AND asset_id = ?",
        (proposal_id, asset_id),
    ).fetchone()
    if row is None:
        raise KeyError((proposal_id, asset_id))
    return proposal_asset_from_row(row)


def list_proposals(
    connection: sqlite3.Connection,
    *,
    status: ProposalStatus | None = None,
    author_principal: str | None = None,
) -> tuple[ProposalRecord, ...]:
    conditions: list[str] = []
    parameters: list[object] = []
    if status is not None:
        conditions.append("status = ?")
        parameters.append(status.value)
    if author_principal is not None:
        conditions.append("author_principal = ?")
        parameters.append(author_principal)
    where = f"WHERE {' AND '.join(conditions)}" if conditions else ""
    rows = connection.execute(
        f"SELECT * FROM proposals {where} ORDER BY created_at, proposal_id", parameters
    ).fetchall()
    return tuple(proposal_from_row(row) for row in rows)


def list_proposal_assets(
    connection: sqlite3.Connection,
    *,
    proposal_id: str | None = None,
    concept_path: str | None = None,
    asset_kind: str | None = None,
) -> tuple[ProposalAssetRecord, ...]:
    conditions: list[str] = []
    parameters: list[object] = []
    if proposal_id is not None:
        conditions.append("proposal_id = ?")
        parameters.append(proposal_id)
    if concept_path is not None:
        conditions.append("concept_path = ?")
        parameters.append(concept_path)
    if asset_kind is not None:
        conditions.append("asset_kind = ?")
        parameters.append(asset_kind)
    where = f"WHERE {' AND '.join(conditions)}" if conditions else ""
    rows = connection.execute(
        f"SELECT * FROM proposal_assets {where} ORDER BY created_at, proposal_id, asset_id",
        parameters,
    ).fetchall()
    return tuple(proposal_asset_from_row(row) for row in rows)


def update_proposal_status(
    connection: sqlite3.Connection,
    proposal_id: str,
    *,
    status: ProposalStatus,
    reviewed_by: str | None = None,
    review_comment: str | None = None,
    applied_operation_id: str | None = None,
    applied_revision: str | None = None,
) -> ProposalRecord:
    current = get_proposal(connection, proposal_id)
    assignments = ["status = ?", "updated_at = ?"]
    parameters: list[object] = [status.value, utcnow()]
    if reviewed_by is not None or current.reviewed_by is not None:
        assignments.append("reviewed_by = ?")
        parameters.append(reviewed_by)
    if review_comment is not None or current.review_comment is not None:
        assignments.append("review_comment = ?")
        parameters.append(review_comment)
    if applied_operation_id is not None or current.applied_operation_id is not None:
        assignments.append("applied_operation_id = ?")
        parameters.append(applied_operation_id)
    if applied_revision is not None or current.applied_revision is not None:
        assignments.append("applied_revision = ?")
        parameters.append(applied_revision)
    parameters.append(proposal_id)
    with connection:
        connection.execute(
            f"UPDATE proposals SET {', '.join(assignments)} WHERE proposal_id = ?",
            parameters,
        )
    return get_proposal(connection, proposal_id)


def proposal_from_row(row: sqlite3.Row) -> ProposalRecord:
    return ProposalRecord(
        proposal_id=row["proposal_id"],
        author_principal=row["author_principal"],
        client_instance_id=row["client_instance_id"],
        base_revision=row["base_revision"],
        intent=row["intent"],
        rationale=row["rationale"],
        patch_json=row["patch_json"],
        patch_hash=row["patch_hash"],
        status=ProposalStatus(row["status"]),
        reviewed_by=row["reviewed_by"],
        review_comment=row["review_comment"],
        applied_operation_id=row["applied_operation_id"],
        applied_revision=row["applied_revision"],
        created_at=row["created_at"],
        updated_at=row["updated_at"],
        expires_at=row["expires_at"],
    )


def proposal_asset_from_row(row: sqlite3.Row) -> ProposalAssetRecord:
    return ProposalAssetRecord(
        proposal_id=str(row["proposal_id"]),
        asset_id=str(row["asset_id"]),
        concept_path=str(row["concept_path"]),
        asset_kind=str(row["asset_kind"]),
        version=str(row["version"]),
        media_type=str(row["media_type"]),
        sha256=str(row["sha256"]),
        blob_bytes=bytes(row["blob_bytes"]),
        manifest_json=str(row["manifest_json"]),
        created_at=str(row["created_at"]),
    )


def _insert_proposal_assets(
    connection: sqlite3.Connection,
    *,
    proposal_id: str,
    assets: Iterable[ProposalAssetInput],
    created_at: str,
) -> None:
    for asset in assets:
        connection.execute(
            """
            INSERT INTO proposal_assets(
                proposal_id, asset_id, concept_path, asset_kind, version,
                media_type, sha256, blob_bytes, manifest_json, created_at
            ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                proposal_id,
                asset.asset_id,
                asset.concept_path,
                asset.asset_kind,
                asset.version,
                asset.media_type,
                asset.sha256,
                asset.blob_bytes,
                asset.manifest_json,
                created_at,
            ),
        )
