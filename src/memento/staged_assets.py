from __future__ import annotations

import hashlib
import secrets
import sqlite3
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import Any
from uuid import uuid4

from memento.repository.asset_packs import validate_asset_kind
from memento.skill_packs import SkillPackManifest, parse_stable_semver, validate_asset_pack

STAGING_TTL_HOURS = 24
UPLOAD_TICKET_TTL_HOURS = 1


class StagedAssetError(ValueError):
    pass


@dataclass(frozen=True, slots=True)
class UploadTicket:
    principal: str
    idempotency_key: str
    asset_kind: str
    version: str
    expires_at: str
    staged_asset_id: str | None
    consumed_at: str | None

    @property
    def state(self) -> str:
        if self.staged_asset_id is not None:
            return "uploaded"
        if _parse_timestamp(self.expires_at) <= _now():
            return "expired"
        return "pending"


@dataclass(frozen=True, slots=True)
class StagedAsset:
    staged_asset_id: str
    principal: str
    asset_kind: str
    version: str
    media_type: str
    sha256: str
    blob_bytes: bytes
    manifest: SkillPackManifest
    state: str
    proposal_id: str | None
    created_at: str
    expires_at: str
    consumed_at: str | None

    def public_payload(self) -> dict[str, Any]:
        return {
            "staged_asset_id": self.staged_asset_id,
            "asset_kind": self.asset_kind,
            "version": self.version,
            "media_type": self.media_type,
            "sha256": self.sha256,
            "manifest": self.manifest.model_dump(mode="json"),
            "state": self.state,
            "proposal_id": self.proposal_id,
            "created_at": self.created_at,
            "expires_at": self.expires_at,
            "consumed_at": self.consumed_at,
        }


def _now() -> datetime:
    return datetime.now(tz=UTC).replace(microsecond=0)


def _timestamp(value: datetime) -> str:
    return value.isoformat().replace("+00:00", "Z")


def _parse_timestamp(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


class StagedAssetStore:
    def __init__(self, connection: sqlite3.Connection) -> None:
        self._connection = connection

    def begin_upload(
        self,
        *,
        principal: str,
        idempotency_key: str,
        asset_kind: str,
        version: str,
    ) -> tuple[UploadTicket, str]:
        if not idempotency_key.strip():
            raise StagedAssetError("idempotency_key is required")
        existing = self._connection.execute(
            "SELECT * FROM asset_upload_tickets WHERE principal=? AND idempotency_key=?",
            (principal, idempotency_key),
        ).fetchone()
        if existing is not None:
            ticket = self._ticket_from_row(existing)
            if ticket.asset_kind != asset_kind or ticket.version != version:
                raise StagedAssetError("idempotency key was already used for a different upload")
            raise StagedAssetError(f"upload ticket already issued: {ticket.state}")
        parse_stable_semver(version)
        validate_asset_kind(asset_kind)
        now = _now()
        raw_token = "memento_upload_" + secrets.token_urlsafe(32)
        with self._connection:
            self._connection.execute(
                """
                INSERT INTO asset_upload_tickets(
                    token_digest,principal,idempotency_key,asset_kind,version,
                    created_at,expires_at,staged_asset_id,consumed_at
                ) VALUES(?,?,?,?,?,?,?,?,?)
                """,
                (
                    hashlib.sha256(raw_token.encode()).hexdigest(),
                    principal,
                    idempotency_key,
                    asset_kind,
                    version,
                    _timestamp(now),
                    _timestamp(now + timedelta(hours=UPLOAD_TICKET_TTL_HOURS)),
                    None,
                    None,
                ),
            )
        return self.ticket_status(principal=principal, idempotency_key=idempotency_key), raw_token

    def ticket_status(self, *, principal: str, idempotency_key: str) -> UploadTicket:
        row = self._connection.execute(
            "SELECT * FROM asset_upload_tickets WHERE principal=? AND idempotency_key=?",
            (principal, idempotency_key),
        ).fetchone()
        if row is None:
            raise StagedAssetError("upload ticket not found")
        return self._ticket_from_row(row)

    def put_with_ticket(self, *, raw_token: str, zip_bytes: bytes) -> tuple[StagedAsset, bool]:
        digest = hashlib.sha256(raw_token.encode()).hexdigest()
        row = self._connection.execute(
            "SELECT * FROM asset_upload_tickets WHERE token_digest=?", (digest,)
        ).fetchone()
        if row is None:
            raise StagedAssetError("upload ticket not found")
        ticket = self._ticket_from_row(row)
        if ticket.state == "expired":
            raise StagedAssetError("upload ticket expired")
        if ticket.staged_asset_id is not None:
            staged = self.get(principal=ticket.principal, staged_asset_id=ticket.staged_asset_id)
            if staged.sha256 != hashlib.sha256(zip_bytes).hexdigest():
                raise StagedAssetError("upload ticket was already used for a different asset")
            return staged, True
        with self._connection:
            staged, replayed = self.put(
                principal=ticket.principal,
                idempotency_key=f"ticket:{ticket.idempotency_key}",
                asset_kind=ticket.asset_kind,
                version=ticket.version,
                zip_bytes=zip_bytes,
                manage_transaction=False,
            )
            now = _timestamp(_now())
            updated = self._connection.execute(
                """
                UPDATE asset_upload_tickets SET staged_asset_id=?,consumed_at=?
                WHERE token_digest=? AND staged_asset_id IS NULL
                """,
                (staged.staged_asset_id, now, digest),
            )
            if updated.rowcount != 1:
                raise StagedAssetError("upload ticket could not be consumed")
        return staged, replayed

    def put(
        self,
        *,
        principal: str,
        idempotency_key: str,
        asset_kind: str,
        version: str,
        zip_bytes: bytes,
        manage_transaction: bool = True,
    ) -> tuple[StagedAsset, bool]:
        if not idempotency_key.strip():
            raise StagedAssetError("Idempotency-Key header is required")
        self.expire()
        existing = self._connection.execute(
            "SELECT * FROM staged_assets WHERE principal=? AND idempotency_key=?",
            (principal, idempotency_key),
        ).fetchone()
        if existing is not None:
            staged = self._from_row(existing)
            candidate = validate_asset_pack(
                asset_kind=asset_kind, version=version, zip_bytes=zip_bytes
            )
            if (
                staged.asset_kind != asset_kind
                or staged.version != version
                or staged.sha256 != candidate.manifest.sha256
            ):
                raise StagedAssetError("idempotency key was already used for a different asset")
            return staged, True
        pack = validate_asset_pack(asset_kind=asset_kind, version=version, zip_bytes=zip_bytes)
        now = _now()
        staged = StagedAsset(
            staged_asset_id=str(uuid4()),
            principal=principal,
            asset_kind=asset_kind,
            version=version,
            media_type="application/zip",
            sha256=pack.manifest.sha256,
            blob_bytes=zip_bytes,
            manifest=pack.manifest,
            state="ready",
            proposal_id=None,
            created_at=_timestamp(now),
            expires_at=_timestamp(now + timedelta(hours=STAGING_TTL_HOURS)),
            consumed_at=None,
        )

        def insert() -> None:
            self._connection.execute(
                """
                INSERT INTO staged_assets(
                    staged_asset_id,principal,idempotency_key,asset_kind,version,media_type,
                    sha256,blob_bytes,manifest_json,state,proposal_id,created_at,expires_at,consumed_at
                ) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
                """,
                (
                    staged.staged_asset_id,
                    principal,
                    idempotency_key,
                    asset_kind,
                    version,
                    staged.media_type,
                    staged.sha256,
                    zip_bytes,
                    staged.manifest.model_dump_json(),
                    staged.state,
                    None,
                    staged.created_at,
                    staged.expires_at,
                    None,
                ),
            )

        if manage_transaction:
            with self._connection:
                insert()
        else:
            insert()
        return staged, False

    def get(
        self, *, principal: str, staged_asset_id: str, require_ready: bool = False
    ) -> StagedAsset:
        self.expire()
        row = self._connection.execute(
            "SELECT * FROM staged_assets WHERE staged_asset_id=? AND principal=?",
            (staged_asset_id, principal),
        ).fetchone()
        if row is None:
            raise StagedAssetError("staged asset not found")
        staged = self._from_row(row)
        if require_ready and staged.state != "ready":
            raise StagedAssetError(f"staged asset is not ready: {staged.state}")
        return staged

    def consume(
        self,
        *,
        principal: str,
        staged_asset_ids: tuple[str, ...],
        proposal_id: str,
        manage_transaction: bool = True,
    ) -> None:
        if not staged_asset_ids:
            return
        now = _timestamp(_now())

        def update() -> None:
            for staged_asset_id in staged_asset_ids:
                updated = self._connection.execute(
                    """
                    UPDATE staged_assets SET state='consumed',proposal_id=?,consumed_at=?,blob_bytes=X''
                    WHERE staged_asset_id=? AND principal=? AND state='ready'
                    """,
                    (proposal_id, now, staged_asset_id, principal),
                )
                if updated.rowcount != 1:
                    raise StagedAssetError("staged asset could not be consumed")

        if manage_transaction:
            with self._connection:
                update()
        else:
            update()

    def expire(self) -> int:
        now = _timestamp(_now())
        with self._connection:
            result = self._connection.execute(
                """
                UPDATE staged_assets SET state='expired',blob_bytes=X''
                WHERE state='ready' AND expires_at<=?
                """,
                (now,),
            )
        return result.rowcount

    @staticmethod
    def _ticket_from_row(row: sqlite3.Row) -> UploadTicket:
        return UploadTicket(
            principal=str(row["principal"]),
            idempotency_key=str(row["idempotency_key"]),
            asset_kind=str(row["asset_kind"]),
            version=str(row["version"]),
            expires_at=str(row["expires_at"]),
            staged_asset_id=(str(row["staged_asset_id"]) if row["staged_asset_id"] else None),
            consumed_at=str(row["consumed_at"]) if row["consumed_at"] else None,
        )

    @staticmethod
    def _from_row(row: sqlite3.Row) -> StagedAsset:
        state = str(row["state"])
        if state == "ready" and _parse_timestamp(str(row["expires_at"])) <= _now():
            state = "expired"
        return StagedAsset(
            staged_asset_id=str(row["staged_asset_id"]),
            principal=str(row["principal"]),
            asset_kind=str(row["asset_kind"]),
            version=str(row["version"]),
            media_type=str(row["media_type"]),
            sha256=str(row["sha256"]),
            blob_bytes=bytes(row["blob_bytes"]),
            manifest=SkillPackManifest.model_validate_json(str(row["manifest_json"])),
            state=state,
            proposal_id=str(row["proposal_id"]) if row["proposal_id"] else None,
            created_at=str(row["created_at"]),
            expires_at=str(row["expires_at"]),
            consumed_at=str(row["consumed_at"]) if row["consumed_at"] else None,
        )
