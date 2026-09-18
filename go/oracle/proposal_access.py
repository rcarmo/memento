"""Capture proposal input validation, path policy and record visibility."""

from __future__ import annotations

import importlib
from dataclasses import asdict
from types import SimpleNamespace
from typing import Any


def fixtures() -> dict[str, Any]:
    service: Any = importlib.import_module("memento.service")
    authz: Any = importlib.import_module("memento.authz")
    runtime = object.__new__(service.MemoryService)
    bases: list[dict[str, Any]] = [
        {
            "kind": "create",
            "path": "/public/a.md",
            "concept_type": "anything",
            "title": "",
            "body": "body\r\n",
        },
        {"kind": "patch", "path": "/public/a.md"},
        {"kind": "rename", "path": "/public/a.md", "new_path": "/public/b.md"},
        {"kind": "trash", "path": "/public/a.md"},
        {
            "kind": "attach_asset_pack",
            "path": "/public/a.md",
            "asset_kind": "anything",
            "version": "unvalidated",
            "asset_id": "id",
            "zip_sha256": "unchecked",
            "manifest": {"nested": [1, None, True]},
        },
    ]
    inputs: list[list[Any]] = [
        [],
        [{}],
        [{"kind": "unknown"}],
        [{"kind": None}],
        [{"kind": 1}],
        [None],
    ]
    value: Any
    status: Any
    for base in bases:
        inputs.append([base])
        inputs.append([{**base, "extra": "rejected"}])
        for field in base:
            inputs.append([{key: value for key, value in base.items() if key != field}])
            for value in [None, 1, True, [], {}]:
                inputs.append([{**base, field: value}])
    for base in bases[:2]:
        for field in ["tags", "aliases"]:
            for value in [None, [], ["", "a", "a", "é"], [1], "x", {}]:
                inputs.append([{**base, field: value}])
        for field in ["description"] + (["title", "body"] if base["kind"] == "patch" else []):
            for value in [None, "", "text", 1, []]:
                inputs.append([{**base, field: value}])
    for status in [None, "active", "deprecated", "tombstone", "archived", 1, {}]:
        inputs.append([{**bases[1], "status": status}])
    normalization = []
    for changes in inputs:
        item: dict[str, Any] = {"changes": changes}
        try:
            item["expected"] = [
                change.model_dump(mode="json") for change in runtime._normalize_changes(changes)
            ]
        except Exception as exc:
            item["error_type"] = type(exc).__name__
            if isinstance(exc, service.ServiceError):
                item["error"] = str(exc)
        normalization.append(item)

    policies = [
        authz.EffectivePolicy(
            "author", ("proposer",), ("/public/",), ("/public/",), ("/private/",)
        ),
        authz.EffectivePolicy("author", ("reader",), ("/public/",), (), ("/private/",)),
        authz.EffectivePolicy("other", ("proposer",), ("/",), ("/",), ("/private/",)),
        authz.EffectivePolicy("other", ("curator",), ("/",), ("/",), ("/private/",)),
        authz.EffectivePolicy("other", ("admin",), ("/",), ("/",), ("/private/",)),
        authz.EffectivePolicy("other", ("curator", "admin"), ("/",), ("/",), ("/private/",)),
        authz.EffectivePolicy(
            "other", ("curator",), ("/public/", "/private/"), ("/public/",), ("/private/",)
        ),
    ]
    batches: list[list[dict[str, Any]]] = [[], *[[base] for base in bases], bases[:2]]
    for path in [
        "/private/a.md",
        "/trash/public/a.md",
        "/a.MD",
        "relative.md",
        "/public/../a.md",
        "/public/a.txt",
    ]:
        batches.extend([[{**bases[1], "path": path}], [{**bases[2], "new_path": path}]])
    batches.extend(
        [
            [bases[3], bases[1]],
            [bases[3], bases[3]],
            [{"kind": "trash", "path": f"/public/{i}.md"} for i in range(20)],
            [{"kind": "trash", "path": f"/public/{i}.md"} for i in range(21)],
        ]
    )
    authorization = []
    visibility = []
    for policy in policies:
        for changes in batches:
            for action in ["read", "write", "review"]:
                item = {"policy": asdict(policy), "changes": changes, "action": action}
                try:
                    runtime._validate_change_auth(
                        policy, runtime._normalize_changes(changes), action=action
                    )
                    item["allowed"] = True
                except (service.ServiceError, authz.AuthorizationError) as exc:
                    item["error"] = str(exc)
                authorization.append(item)
        for changes in batches + [[{"kind": "unknown"}], [{"kind": "patch"}]]:
            for require_write in [False, True]:
                proposal = SimpleNamespace(
                    author_principal="author", proposal_id="proposal", patch={"changes": changes}
                )
                item = {
                    "policy": asdict(policy),
                    "changes": changes,
                    "require_write": require_write,
                }
                try:
                    item["allowed"] = runtime._can_access_proposal(
                        policy, proposal, require_write=require_write
                    )
                    try:
                        runtime._require_proposal_access(
                            policy, proposal, require_write=require_write
                        )
                    except service.ForbiddenError as exc:
                        item["error"] = str(exc)
                except Exception as exc:
                    item["error_type"] = type(exc).__name__
                visibility.append(item)
    return {
        "normalization": normalization,
        "authorization": authorization,
        "visibility": visibility,
    }
