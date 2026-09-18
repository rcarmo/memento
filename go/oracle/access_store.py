"""Managed access lifecycle and wrapped-key interoperability, synthetic keys only."""

from __future__ import annotations

import base64
import importlib
import tempfile
from dataclasses import asdict
from pathlib import Path
from typing import Any
from unittest.mock import patch


class Random:
    def __init__(self) -> None:
        self.position = 0

    def bytes(self, size: int) -> bytes:
        raw = bytes((self.position + i) % 256 for i in range(size))
        self.position += size
        return raw

    def token(self, size: int) -> str:
        return base64.urlsafe_b64encode(self.bytes(size)).decode().rstrip("=")


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.access")
    config: Any = importlib.import_module("memento.config")
    control: Any = importlib.import_module("memento.control.db")
    random = Random()
    commands: list[dict[str, Any]] = [
        {"action": "authenticate", "token": "wrong"},
        {"action": "authenticate", "token": "synthetic-admin"},
        {"action": "disable", "name": "sandbox"},
        {"action": "revoke", "name": "sandbox"},
        {"action": "update", "name": "sandbox", "roles": ["reader"], "reads": ["/"], "writes": []},
        {
            "action": "create",
            "name": "reader",
            "roles": ["reader"],
            "reads": ["/public/"],
            "writes": [],
            "key": "create-reader",
        },
        {
            "action": "create",
            "name": "reader",
            "roles": ["reader"],
            "reads": ["/public/"],
            "writes": [],
            "key": "create-reader",
        },
        {"action": "authenticate", "token": "{{LAST}}"},
        {"action": "policy", "name": "reader"},
        {"action": "rename", "name": "reader", "new_name": "renamed"},
        {"action": "authenticate", "token": "{{LAST}}"},
        {"action": "disable", "name": "renamed"},
        {"action": "authenticate", "token": "{{LAST}}"},
        {"action": "delete", "name": "renamed"},
        {"action": "revoke", "name": "renamed"},
        {"action": "rotate", "name": "renamed", "key": "rotate-one"},
        {"action": "authenticate", "token": "{{LAST}}"},
        {"action": "enable", "name": "renamed"},
        {"action": "authenticate", "token": "{{LAST}}"},
        {"action": "rotate", "name": "renamed", "key": "rotate-one"},
        {"action": "revoke", "name": "renamed"},
        {"action": "delete", "name": "renamed"},
        {"action": "policy", "name": "renamed"},
        {
            "action": "create",
            "name": "second",
            "roles": ["admin", "reader"],
            "reads": ["/"],
            "writes": ["/"],
        },
        {"action": "disable", "name": "sandbox"},
        {"action": "revoke", "name": "sandbox"},
        {"action": "update", "name": "sandbox", "roles": ["reader"], "reads": ["/"], "writes": []},
        {"action": "rename", "name": "second", "new_name": "renamed"},
        {
            "action": "create",
            "name": "second",
            "roles": ["admin"],
            "reads": ["/"],
            "writes": [],
            "key": "burned",
        },
        {
            "action": "create",
            "name": "different",
            "roles": ["reader"],
            "reads": ["/"],
            "writes": [],
            "key": "burned",
        },
        {"action": "rotate", "name": "missing", "key": "missing"},
        {
            "action": "create",
            "name": "empty-key",
            "roles": ["reader"],
            "reads": ["/"],
            "writes": [],
            "key": "  ",
        },
    ]
    with tempfile.TemporaryDirectory(prefix="memento-access-oracle-") as directory:
        connection = control.connect_control_db(Path(directory) / "control.sqlite")
        control.migrate_control_db(connection)
        with (
            patch.object(module.secrets, "token_bytes", random.bytes),
            patch.object(module.secrets, "token_urlsafe", random.token),
            patch.object(module, "_now", lambda: "2026-09-18T00:00:00Z"),
        ):
            store = module.AccessStore(connection, "synthetic-master")
            authorization = config.AuthorizationConfig(
                principals={
                    "piclaw-workspace": config.NamespacePolicy(
                        roles=("reader",),
                        token_env="TEST",
                        read_prefixes=("/",),
                        write_prefixes=("/",),
                    )
                }
            )
            store.bootstrap(authorization, {"piclaw-workspace": "synthetic-admin"})
            store.bootstrap(authorization, {"piclaw-workspace": "ignored-replacement"})
            initial_wrap = connection.execute(
                "SELECT value FROM access_meta WHERE key='verifier_key'"
            ).fetchone()[0]
            cases = []
            last = ""
            for command in commands:
                args = {k: v for k, v in command.items() if k != "action"}
                for key in ["roles", "reads", "writes"]:
                    if key in args:
                        args[key] = tuple(args[key])
                action = command["action"]
                item: dict[str, Any] = {"input": command}
                try:
                    if action == "authenticate":
                        principal = store.authenticate(args["token"].replace("{{LAST}}", last))
                        item["result"] = principal.model_dump(mode="json") if principal else None
                    elif action == "policy":
                        policy = store.policy(args["name"])
                        item["result"] = policy.model_dump(mode="json") if policy else None
                    elif action == "create":
                        principal, last = store.create(
                            actor="admin",
                            name=args["name"],
                            roles=args["roles"],
                            read_prefixes=args["reads"],
                            write_prefixes=args["writes"],
                            idempotency_key=args.get("key"),
                        )
                        item["result"] = asdict(principal)
                        item["token"] = last
                    elif action == "rotate":
                        last = store.rotate(
                            actor="admin", name=args["name"], idempotency_key=args.get("key")
                        )
                        item["token"] = last
                    elif action == "update":
                        item["result"] = asdict(
                            store.update(
                                actor="admin",
                                name=args["name"],
                                roles=args["roles"],
                                read_prefixes=args["reads"],
                                write_prefixes=args["writes"],
                            )
                        )
                    elif action == "rename":
                        item["result"] = asdict(store.rename(actor="admin", **args))
                    elif action in {"disable", "enable"}:
                        item["result"] = asdict(
                            store.set_enabled(
                                actor="admin", name=args["name"], enabled=action == "enable"
                            )
                        )
                    elif action == "revoke":
                        item["result"] = asdict(store.revoke(actor="admin", **args))
                    else:
                        item["result"] = asdict(store.delete(actor="admin", **args))
                except module.AccessError as exc:
                    item["error"] = str(exc)
                cases.append(item)
            store.rotate_master_key("synthetic-master", "synthetic-new-master")
            final_wrap = connection.execute(
                "SELECT value FROM access_meta WHERE key='verifier_key'"
            ).fetchone()[0]
            module.AccessStore(connection, "synthetic-new-master")
            rows = {
                table: [
                    dict(row) for row in connection.execute(f"SELECT * FROM {table} ORDER BY 1")
                ]
                for table in [
                    "access_principals",
                    "access_credentials",
                    "access_audit",
                    "access_idempotency",
                ]
            }
            result = {
                "initial_wrap": initial_wrap,
                "final_wrap": final_wrap,
                "cases": cases,
                "principals": [asdict(item) for item in store.list()],
                "audit": store.audit(100),
                "rows": rows,
            }
        connection.close()
    return result
