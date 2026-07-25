from __future__ import annotations

import base64
import hashlib
import hmac
import json
import re
import secrets
import sqlite3
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from memento.config import AuthorizationConfig, NamespacePolicy, Principal

_NAME = re.compile(r"^[a-z0-9][a-z0-9-]{0,62}$")
_ROLES = frozenset({"reader", "proposer", "curator", "admin"})


class AccessError(ValueError):
    pass


@dataclass(frozen=True, slots=True)
class ManagedPrincipal:
    name: str
    roles: tuple[str, ...]
    read_prefixes: tuple[str, ...]
    write_prefixes: tuple[str, ...]
    enabled: bool
    revoked: bool
    deleted: bool
    updated_at: str


def _now() -> str:
    return datetime.now(tz=UTC).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def _master_key(value: str) -> bytes:
    if not value:
        raise AccessError("MEMENTO_ADMIN_MASTER_KEY is required")
    return hashlib.scrypt(value.encode(), salt=b"memento-access-v1", n=2**14, r=8, p=1, dklen=32)


def _seal_key(verifier_key: bytes, master_key: str) -> str:
    nonce = secrets.token_bytes(12)
    payload = nonce + AESGCM(_master_key(master_key)).encrypt(
        nonce, verifier_key, b"memento-access"
    )
    return base64.urlsafe_b64encode(payload).decode()


def _open_key(payload: str, master_key: str) -> bytes:
    try:
        raw = base64.urlsafe_b64decode(payload.encode())
        return AESGCM(_master_key(master_key)).decrypt(raw[:12], raw[12:], b"memento-access")
    except Exception as exc:
        raise AccessError("access master key is invalid") from exc


def _digest(verifier_key: bytes, token: str) -> str:
    return hmac.new(verifier_key, token.encode(), hashlib.sha256).hexdigest()


def _normalize_prefixes(values: tuple[str, ...]) -> tuple[str, ...]:
    normalized = tuple(sorted(dict.fromkeys(values)))
    for value in normalized:
        if not value.startswith("/") or not value.endswith("/"):
            raise AccessError("namespace prefixes must start and end with '/'")
    return normalized


def _validate_policy(
    name: str,
    roles: tuple[str, ...],
    read_prefixes: tuple[str, ...],
    write_prefixes: tuple[str, ...],
) -> tuple[tuple[str, ...], tuple[str, ...], tuple[str, ...]]:
    if not _NAME.fullmatch(name):
        raise AccessError("principal name must use lowercase letters, digits, and hyphens")
    normalized_roles = tuple(sorted(dict.fromkeys(roles)))
    if not normalized_roles or not set(normalized_roles) <= _ROLES:
        raise AccessError("principal roles are invalid")
    reads = _normalize_prefixes(read_prefixes)
    writes = _normalize_prefixes(write_prefixes)
    if any(not any(path.startswith(read) for read in reads) for path in writes):
        raise AccessError("every write prefix must be inside a readable prefix")
    return normalized_roles, reads, writes


class AccessStore:
    def __init__(self, connection: sqlite3.Connection, master_key: str) -> None:
        self._connection = connection
        row = connection.execute(
            "SELECT value FROM access_meta WHERE key='verifier_key'"
        ).fetchone()
        if row is None:
            verifier_key = secrets.token_bytes(32)
            with connection:
                connection.execute(
                    "INSERT INTO access_meta(key,value,updated_at) VALUES('verifier_key',?,?)",
                    (_seal_key(verifier_key, master_key), _now()),
                )
        else:
            verifier_key = _open_key(str(row["value"]), master_key)
        self._verifier_key = verifier_key

    def bootstrap(self, authorization: AuthorizationConfig, tokens: dict[str, str]) -> None:
        with self._connection:
            for configured_name, policy in authorization.principals.items():
                name = "sandbox" if configured_name == "piclaw-workspace" else configured_name
                roles = tuple(
                    sorted(set(policy.roles) | ({"admin"} if name == "sandbox" else set()))
                )
                roles, reads, writes = _validate_policy(
                    name, roles, policy.read_prefixes, policy.write_prefixes
                )
                now = _now()
                self._connection.execute(
                    """
                    INSERT INTO access_principals(name,roles_json,read_prefixes_json,write_prefixes_json,enabled,revoked,deleted,created_at,updated_at)
                    VALUES(?,?,?,?,1,0,0,?,?)
                    ON CONFLICT(name) DO NOTHING
                    """,
                    (name, json.dumps(roles), json.dumps(reads), json.dumps(writes), now, now),
                )
                token = tokens.get(configured_name)
                if token:
                    self._connection.execute(
                        """
                        INSERT INTO access_credentials(principal_name,token_digest,created_at,revoked_at)
                        VALUES(?,?,?,NULL)
                        ON CONFLICT(principal_name) DO NOTHING
                        """,
                        (name, _digest(self._verifier_key, token), now),
                    )
            old = self._connection.execute(
                "SELECT name FROM access_principals WHERE name='piclaw-workspace'"
            ).fetchone()
            sandbox = self._connection.execute(
                "SELECT name FROM access_principals WHERE name='sandbox'"
            ).fetchone()
            if old is not None and sandbox is None:
                self._connection.execute(
                    "UPDATE access_principals SET name='sandbox',updated_at=? WHERE name='piclaw-workspace'",
                    (_now(),),
                )

    def authenticate(self, token: str) -> Principal | None:
        digest = _digest(self._verifier_key, token)
        row = self._connection.execute(
            """
            SELECT p.* FROM access_credentials c
            JOIN access_principals p ON p.name=c.principal_name
            WHERE c.token_digest=? AND c.revoked_at IS NULL
              AND p.enabled=1 AND p.revoked=0 AND p.deleted=0
            """,
            (digest,),
        ).fetchone()
        if row is None:
            return None
        return Principal(name=str(row["name"]), roles=tuple(json.loads(row["roles_json"])))

    def policy(self, name: str) -> NamespacePolicy | None:
        row = self._connection.execute(
            "SELECT * FROM access_principals WHERE name=? AND enabled=1 AND revoked=0 AND deleted=0",
            (name,),
        ).fetchone()
        if row is None:
            return None
        return NamespacePolicy(
            roles=tuple(json.loads(row["roles_json"])),
            token_env="MANAGED_ACCESS",
            read_prefixes=tuple(json.loads(row["read_prefixes_json"])),
            write_prefixes=tuple(json.loads(row["write_prefixes_json"])),
        )

    def list(self) -> tuple[ManagedPrincipal, ...]:
        rows = self._connection.execute("SELECT * FROM access_principals ORDER BY name").fetchall()
        return tuple(
            ManagedPrincipal(
                name=str(row["name"]),
                roles=tuple(json.loads(row["roles_json"])),
                read_prefixes=tuple(json.loads(row["read_prefixes_json"])),
                write_prefixes=tuple(json.loads(row["write_prefixes_json"])),
                enabled=bool(row["enabled"]),
                revoked=bool(row["revoked"]),
                deleted=bool(row["deleted"]),
                updated_at=str(row["updated_at"]),
            )
            for row in rows
        )

    def create(
        self,
        *,
        actor: str,
        name: str,
        roles: tuple[str, ...],
        read_prefixes: tuple[str, ...],
        write_prefixes: tuple[str, ...],
        idempotency_key: str | None = None,
    ) -> tuple[ManagedPrincipal, str]:
        roles, reads, writes = _validate_policy(name, roles, read_prefixes, write_prefixes)
        self._claim_idempotency(actor, idempotency_key, "principal.create", name)
        token = "memento_" + secrets.token_urlsafe(32)
        now = _now()
        try:
            with self._connection:
                self._connection.execute(
                    "INSERT INTO access_principals VALUES(?,?,?,?,1,0,0,?,?)",
                    (name, json.dumps(roles), json.dumps(reads), json.dumps(writes), now, now),
                )
                self._connection.execute(
                    "INSERT INTO access_credentials VALUES(?,?,?,NULL)",
                    (name, _digest(self._verifier_key, token), now),
                )
                self._audit(
                    actor,
                    "principal.create",
                    name,
                    {"roles": roles, "read_prefixes": reads, "write_prefixes": writes},
                )
        except sqlite3.IntegrityError as exc:
            raise AccessError("principal already exists") from exc
        return next(item for item in self.list() if item.name == name), token

    def update(
        self,
        *,
        actor: str,
        name: str,
        roles: tuple[str, ...],
        read_prefixes: tuple[str, ...],
        write_prefixes: tuple[str, ...],
    ) -> ManagedPrincipal:
        roles, reads, writes = _validate_policy(name, roles, read_prefixes, write_prefixes)
        current = self._require(name)
        if "admin" in current.roles and "admin" not in roles:
            self._require_other_admin(name)
        with self._connection:
            self._connection.execute(
                "UPDATE access_principals SET roles_json=?,read_prefixes_json=?,write_prefixes_json=?,updated_at=? WHERE name=?",
                (json.dumps(roles), json.dumps(reads), json.dumps(writes), _now(), name),
            )
            self._audit(
                actor,
                "principal.update",
                name,
                {"roles": roles, "read_prefixes": reads, "write_prefixes": writes},
            )
        return self._require(name)

    def rename(self, *, actor: str, name: str, new_name: str) -> ManagedPrincipal:
        current = self._require(name)
        _validate_policy(new_name, current.roles, current.read_prefixes, current.write_prefixes)
        try:
            with self._connection:
                self._connection.execute(
                    "UPDATE access_principals SET name=?,updated_at=? WHERE name=?",
                    (new_name, _now(), name),
                )
                self._audit(actor, "principal.rename", new_name, {"previous_name": name})
        except sqlite3.IntegrityError as exc:
            raise AccessError("principal already exists") from exc
        return self._require(new_name)

    def set_enabled(self, *, actor: str, name: str, enabled: bool) -> ManagedPrincipal:
        current = self._require(name)
        if not enabled and "admin" in current.roles:
            self._require_other_admin(name)
        with self._connection:
            self._connection.execute(
                "UPDATE access_principals SET enabled=?,updated_at=? WHERE name=?",
                (int(enabled), _now(), name),
            )
            self._audit(actor, "principal.enable" if enabled else "principal.disable", name, {})
        return self._require(name)

    def rotate(self, *, actor: str, name: str, idempotency_key: str | None = None) -> str:
        self._require(name)
        self._claim_idempotency(actor, idempotency_key, "credential.rotate", name)
        token = "memento_" + secrets.token_urlsafe(32)
        now = _now()
        with self._connection:
            self._connection.execute(
                "UPDATE access_credentials SET token_digest=?,created_at=?,revoked_at=NULL WHERE principal_name=?",
                (_digest(self._verifier_key, token), now, name),
            )
            self._connection.execute(
                "UPDATE access_principals SET revoked=0,updated_at=? WHERE name=?", (now, name)
            )
            self._audit(actor, "credential.rotate", name, {})
        return token

    def revoke(self, *, actor: str, name: str) -> ManagedPrincipal:
        current = self._require(name)
        if "admin" in current.roles:
            self._require_other_admin(name)
        now = _now()
        with self._connection:
            self._connection.execute(
                "UPDATE access_credentials SET revoked_at=? WHERE principal_name=?", (now, name)
            )
            self._connection.execute(
                "UPDATE access_principals SET revoked=1,enabled=0,updated_at=? WHERE name=?",
                (now, name),
            )
            self._audit(actor, "credential.revoke", name, {})
        return self._require(name)

    def delete(self, *, actor: str, name: str) -> ManagedPrincipal:
        current = self._require(name)
        if current.enabled or not current.revoked:
            raise AccessError("principal must be disabled and revoked before deletion")
        with self._connection:
            self._connection.execute(
                "UPDATE access_principals SET deleted=1,updated_at=? WHERE name=?", (_now(), name)
            )
            self._audit(actor, "principal.delete", name, {})
        return self._require(name)

    def audit(self, limit: int = 50) -> tuple[dict[str, Any], ...]:
        rows = self._connection.execute(
            "SELECT actor,action,target,details_json,created_at FROM access_audit ORDER BY id DESC LIMIT ?",
            (max(1, min(limit, 100)),),
        ).fetchall()
        return tuple(
            {
                "actor": str(row["actor"]),
                "action": str(row["action"]),
                "target": str(row["target"]),
                "details": json.loads(row["details_json"]),
                "created_at": str(row["created_at"]),
            }
            for row in rows
        )

    def _require(self, name: str) -> ManagedPrincipal:
        try:
            return next(item for item in self.list() if item.name == name)
        except StopIteration as exc:
            raise AccessError("principal not found") from exc

    def _require_other_admin(self, name: str) -> None:
        if not any(
            item.name != name
            and item.enabled
            and not item.revoked
            and not item.deleted
            and "admin" in item.roles
            for item in self.list()
        ):
            raise AccessError("the final enabled admin cannot remove its own access")

    def _claim_idempotency(self, actor: str, key: str | None, action: str, target: str) -> None:
        if key is None:
            return
        if not key.strip():
            raise AccessError("idempotency key must not be empty")
        try:
            with self._connection:
                self._connection.execute(
                    "INSERT INTO access_idempotency(actor,idempotency_key,action,target,created_at) VALUES(?,?,?,?,?)",
                    (actor, key, action, target, _now()),
                )
        except sqlite3.IntegrityError as exc:
            raise AccessError(
                "access mutation already completed; one-time credential cannot be replayed"
            ) from exc

    def _audit(self, actor: str, action: str, target: str, details: dict[str, Any]) -> None:
        self._connection.execute(
            "INSERT INTO access_audit(actor,action,target,details_json,created_at) VALUES(?,?,?,?,?)",
            (actor, action, target, json.dumps(details, sort_keys=True), _now()),
        )

    def rotate_master_key(self, old_key: str, new_key: str) -> None:
        row = self._connection.execute(
            "SELECT value FROM access_meta WHERE key='verifier_key'"
        ).fetchone()
        if row is None:
            raise AccessError("access verifier key is not initialized")
        verifier_key = _open_key(str(row["value"]), old_key)
        with self._connection:
            self._connection.execute(
                "UPDATE access_meta SET value=?,updated_at=? WHERE key='verifier_key'",
                (_seal_key(verifier_key, new_key), _now()),
            )
