"""Capture raw-candidate pagination and Python Fernet interoperability."""

from __future__ import annotations

import base64
import importlib
import json
import tempfile
import threading
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    authz: Any = importlib.import_module("memento.authz")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    crypto: Any = importlib.import_module("cryptography.fernet")
    cipher = crypto.Fernet(base64.urlsafe_b64encode(bytes(range(32))))
    cases = []
    for scenario in [
        "pages",
        "author",
        "reader",
        "no-write",
        "stale",
        "unresolved",
        "expired",
        "invalid-status",
        "limit-zero",
        "bad-cursor",
        "scope-revision",
        "scope-roles",
        "scope-status",
        "scope-reads",
        "scope-writes",
        "scope-protected",
        "scope-principal",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-list-") as directory:
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            for i, (author, path, base, stored_status, expiry) in enumerate(
                [
                    ("author", "/private/0.md", "base", "approved", "2099-01-01T00:00:00Z"),
                    ("other", "/public/1.md", "base", "approved", "2099-01-01T00:00:00Z"),
                    ("author", "/public/2.md", "main", "submitted", "2000-01-01T00:00:00Z"),
                    ("author", "/public/3.md", "base", "submitted", "2099-01-01T00:00:00Z"),
                    ("author", "/public/4.md", "main", "rejected", "2099-01-01T00:00:00Z"),
                ]
            ):
                proposals.create_proposal(
                    connection,
                    proposal_id=f"p{i}",
                    author_principal=author,
                    client_instance_id=None,
                    base_revision=base,
                    intent="synthetic",
                    rationale=None,
                    patch={"changes": [{"kind": "patch", "path": path}]},
                )
                connection.execute(
                    "UPDATE proposals SET status=?,created_at='created',updated_at='updated',expires_at=? WHERE proposal_id=?",
                    (stored_status, expiry, f"p{i}"),
                )
            connection.commit()
            before = [
                dict(row)
                for row in connection.execute("SELECT * FROM proposals ORDER BY proposal_id")
            ]
            policy = authz.EffectivePolicy(
                "author",
                ("reader",)
                if scenario == "reader"
                else ("proposer",)
                if scenario == "author"
                else ("proposer", "curator"),
                ("/",),
                () if scenario == "no-write" else ("/",),
                ("/private/",),
            )
            state = SimpleNamespace(policy=policy, revision="main")
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection,
                repo_paths=SimpleNamespace(current_dir=Path(directory)),
            )
            runtime._policy = lambda _, state=state: state.policy
            runtime._now = lambda: datetime(2026, 9, 18, tzinfo=UTC)
            runtime._proposal_cursor_cipher = SimpleNamespace(
                encrypt=lambda data: cipher._encrypt_from_parts(data, 1789689600, bytes(range(16))),
                decrypt=cipher.decrypt,
            )
            status = (
                scenario
                if scenario in {"stale", "unresolved", "expired"}
                else "bad"
                if scenario == "invalid-status"
                else None
            )
            limit = 0 if scenario == "limit-zero" else 1
            cursor = "bad" if scenario == "bad-cursor" else None
            steps = []
            with (
                patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
                patch.object(service, "get_main_revision", lambda _, state=state: state.revision),
                patch.object(service, "diff_main_paths", lambda *args, **kwargs: ()),
            ):
                for step in range(8):
                    if step and scenario.startswith("scope-"):
                        which = scenario.removeprefix("scope-")
                        if which == "revision":
                            state.revision = "next"
                        elif which == "status":
                            status = "unresolved"
                        else:
                            fields = asdict(state.policy)
                            key = {
                                "roles": "roles",
                                "reads": "read_prefixes",
                                "writes": "write_prefixes",
                                "protected": "protected_read_prefixes",
                                "principal": "principal",
                            }[which]
                            fields[key] = (
                                "someone"
                                if which == "principal"
                                else tuple(reversed(fields[key]))
                                if which == "roles"
                                else ("/public/",)
                            )
                            state.policy = authz.EffectivePolicy(**fields)
                    item: dict[str, Any] = {
                        "policy": asdict(state.policy),
                        "revision": state.revision,
                        "status": status,
                        "limit": limit,
                        "cursor": cursor,
                    }
                    result = runtime.memory_proposal_list(
                        None, status=status, limit=limit, cursor=cursor
                    ).model_dump(mode="json")
                    item["response"] = result
                    if result["status"] != "error":
                        cursor = result["data"]["next_cursor"]
                        item["decoded_next"] = (
                            json.loads(cipher.decrypt(cursor.encode())) if cursor else None
                        )
                    steps.append(item)
                    if result["status"] == "error" or cursor is None:
                        break
            cases.append(
                {
                    "scenario": scenario,
                    "before": before,
                    "steps": steps,
                    "after": [
                        dict(row)
                        for row in connection.execute(
                            "SELECT * FROM proposals ORDER BY proposal_id"
                        )
                    ],
                    "events": [
                        dict(row)
                        for row in connection.execute(
                            "SELECT * FROM proposal_events ORDER BY event_id"
                        )
                    ],
                }
            )
            connection.close()
    return cases


def fernet_fixtures() -> list[dict[str, Any]]:
    crypto: Any = importlib.import_module("cryptography.fernet")
    cipher = crypto.Fernet(base64.urlsafe_b64encode(bytes(range(32))))
    cases = []
    for data in [b"", b"hello", bytes(range(64)), b"a" * 16]:
        token = cipher._encrypt_from_parts(data, 1789689600, bytes(range(16))).decode()
        cases.append({"data": base64.b64encode(data).decode(), "token": token})
    return cases


def decrypt_fixtures() -> list[dict[str, Any]]:
    crypto: Any = importlib.import_module("cryptography.fernet")
    cipher = crypto.Fernet(base64.urlsafe_b64encode(bytes(range(32))))
    token = cipher._encrypt_from_parts(b"payload", 1789689600, bytes([255] * 16)).decode()
    cases = []
    for value in [
        token,
        token + "==",
        token[:3] + "!\n" + token[3:],
        token.replace("-", "+").replace("_", "/"),
        token[:-1],
        token[:-2],
        "",
        "bad",
        "=",
        "💀",
        token[:8] + "A" + token[9:],
    ]:
        item: dict[str, Any] = {"token": value}
        try:
            item["expected"] = base64.b64encode(cipher.decrypt(value.encode("ascii"))).decode()
        except (crypto.InvalidToken, UnicodeError):
            item["error"] = True
        cases.append(item)
    return cases
