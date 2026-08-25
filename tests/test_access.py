from __future__ import annotations

import sqlite3
from pathlib import Path

import pytest

from memento.access import AccessError, AccessStore
from memento.authz import AuthorizationError, authorize_path, resolve_policy
from memento.config import AuthorizationConfig, NamespacePolicy, Principal
from memento.control.db import connect_control_db, migrate_control_db


def store(tmp_path: Path, key: str = "nenhuma") -> tuple[sqlite3.Connection, AccessStore]:
    connection = connect_control_db(tmp_path / "control.sqlite")
    migrate_control_db(connection)
    return connection, AccessStore(connection, key)


def authorization() -> AuthorizationConfig:
    return AuthorizationConfig(
        principals={
            "piclaw-workspace": NamespacePolicy(
                roles=("reader", "proposer", "curator"),
                token_env="TOKEN",
                read_prefixes=("/",),
                write_prefixes=("/projects/",),
            ),
            "work-agent": NamespacePolicy(
                roles=("reader", "proposer"),
                token_env="WORK",
                read_prefixes=("/work/", "/skills/", "/public/"),
                write_prefixes=("/work/",),
            ),
        }
    )


def test_bootstrap_renames_initial_admin_and_authenticates(tmp_path: Path) -> None:
    connection, access = store(tmp_path)
    access.bootstrap(
        authorization(), {"piclaw-workspace": "sandbox-token", "work-agent": "work-token"}
    )
    sandbox = next(item for item in access.list() if item.name == "sandbox")
    assert sandbox.roles == ("admin", "curator", "proposer", "reader")
    sandbox_auth = access.authenticate("sandbox-token")
    work_auth = access.authenticate("work-token")
    assert sandbox_auth is not None and sandbox_auth.name == "sandbox"
    assert work_auth is not None and work_auth.name == "work-agent"
    assert access.authenticate("wrong") is None
    connection.close()


def test_create_returns_one_time_token_and_validates_scope(tmp_path: Path) -> None:
    connection, access = store(tmp_path)
    principal, token = access.create(
        actor="sandbox",
        name="gates",
        roles=("reader", "proposer"),
        read_prefixes=("/work/", "/skills/"),
        write_prefixes=("/work/",),
    )
    assert principal.name == "gates"
    assert token.startswith("memento_")
    authenticated = access.authenticate(token)
    assert authenticated is not None and authenticated.name == "gates"
    rotated = access.rotate(actor="sandbox", name="gates", idempotency_key="rotate-gates-1")
    assert rotated != token
    with pytest.raises(AccessError, match="one-time credential cannot be replayed"):
        access.rotate(actor="sandbox", name="gates", idempotency_key="rotate-gates-1")
    with pytest.raises(AccessError, match="inside a readable"):
        access.create(
            actor="sandbox",
            name="bad",
            roles=("reader",),
            read_prefixes=("/skills/",),
            write_prefixes=("/work/",),
        )
    connection.close()


def test_lifecycle_and_last_admin_guard(tmp_path: Path) -> None:
    connection, access = store(tmp_path)
    access.bootstrap(authorization(), {"piclaw-workspace": "sandbox-token"})
    with pytest.raises(AccessError, match="final enabled admin"):
        access.set_enabled(actor="sandbox", name="sandbox", enabled=False)
    created, _ = access.create(
        actor="sandbox",
        name="second-admin",
        roles=("reader", "admin"),
        read_prefixes=("/",),
        write_prefixes=(),
    )
    assert "admin" in created.roles
    disabled = access.set_enabled(actor="second-admin", name="sandbox", enabled=False)
    assert disabled.enabled is False
    revoked = access.revoke(actor="second-admin", name="sandbox")
    assert revoked.revoked is True
    deleted = access.delete(actor="second-admin", name="sandbox")
    assert deleted.deleted is True
    assert access.authenticate("sandbox-token") is None
    assert [event["action"] for event in access.audit()] == [
        "principal.delete",
        "credential.revoke",
        "principal.disable",
        "principal.create",
    ]
    connection.close()


def test_managed_principal_policy_inherits_protected_namespaces() -> None:
    principal = Principal(name="managed-reader", roles=("reader",))
    authorization = AuthorizationConfig(
        principals={
            "managed-reader": NamespacePolicy(
                roles=("reader",), token_env="MANAGED", read_prefixes=("/",)
            )
        },
        protected_read_prefixes=("/personal/",),
    )
    policy = resolve_policy(authorization, principal)
    with pytest.raises(AuthorizationError):
        authorize_path(policy, "/personal/rui.md", action="read")


def test_master_key_rotation_preserves_credentials(tmp_path: Path) -> None:
    connection, access = store(tmp_path)
    access.bootstrap(authorization(), {"piclaw-workspace": "sandbox-token"})
    access.rotate_master_key("nenhuma", "stronger-key")
    principal = AccessStore(connection, "stronger-key").authenticate("sandbox-token")
    assert principal is not None and principal.name == "sandbox"
    with pytest.raises(AccessError, match="invalid"):
        AccessStore(connection, "nenhuma")
    connection.close()
