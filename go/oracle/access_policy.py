"""Capture authorization paths and managed-policy validation without real secrets."""

from __future__ import annotations

import importlib
from dataclasses import asdict
from typing import Any


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.authz")
    config: Any = importlib.import_module("memento.config")
    access: Any = importlib.import_module("memento.access")
    protected = ("/private/", "/private/nested/")
    cases = []
    for roles in [("reader",), ("reader", "admin")]:
        for reads in [
            ("/",),
            ("/public/",),
            ("/", "/private/"),
            ("/", "/private/nested/"),
            ("/private/part/",),
        ]:
            policy = module.EffectivePolicy("agent", roles, reads, ("/public/",), protected)
            for action in ["read", "write", "other"]:
                for path in [
                    "/",
                    "/public",
                    "/public/",
                    "/public/a.md",
                    "/publicity/a",
                    "/private",
                    "/private/a",
                    "/private/nested/a",
                    "/private/part/a",
                    "/trash/public/a",
                    "/trash/private/a",
                    "/trash/trash/public/a",
                    "/trash/",
                    "/trash",
                    "relative",
                    "//public/a",
                    "/public//",
                    "/public/../private/a",
                    "/public/./a",
                    "/public/a\x00",
                    "/public/a\\b",
                ]:
                    item: dict[str, Any] = {
                        "policy": asdict(policy),
                        "path": path,
                        "action": action,
                    }
                    try:
                        item["expected"] = asdict(
                            module.authorize_path(policy, path, action=action)
                        )
                    except module.AuthorizationError as exc:
                        item["error"] = str(exc)
                    cases.append(item)
    policy_cases = []
    for name, roles, reads, writes in [
        ("agent", ("reader", "reader"), ("/", "/"), ("/public/",)),
        ("UPPER", ("reader",), ("/",), ()),
        ("", ("reader",), ("/",), ()),
        ("agent", (), ("/",), ()),
        ("agent", ("unknown",), ("/",), ()),
        ("agent", ("admin",), (), ()),
        ("agent", ("reader",), ("bad",), ()),
        ("agent", ("reader",), ("/one/",), ("/other/",)),
        ("agent", ("reader",), ("/one/",), ("/one/nested/",)),
        ("agent", ("reader",), ("/../",), ("/../",)),
    ]:
        item = {"name": name, "roles": roles, "reads": reads, "writes": writes}
        try:
            item["expected"] = access._validate_policy(name, roles, reads, writes)
        except access.AccessError as exc:
            item["error"] = str(exc)
        policy_cases.append(item)
    resolved = []
    authorization = config.AuthorizationConfig(
        principals={
            "agent": config.NamespacePolicy(
                roles=("reader", "proposer"),
                token_env="TOKEN",
                read_prefixes=("/",),
                write_prefixes=(),
            )
        },
        protected_read_prefixes=protected,
    )
    for name, roles in [
        ("missing", ("reader",)),
        ("agent", ("reader",)),
        ("agent", ("admin",)),
        ("agent", ("reader", "proposer", "admin")),
    ]:
        item = {"name": name, "roles": roles}
        try:
            item["expected"] = asdict(
                module.resolve_policy(authorization, config.Principal(name=name, roles=roles))
            )
        except module.AuthorizationError as exc:
            item["error"] = str(exc)
        resolved.append(item)
    return {
        "cases": cases,
        "managed": policy_cases,
        "config": authorization.model_dump(mode="json"),
        "resolved": resolved,
    }
