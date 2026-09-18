"""Capture the executor's non-discriminated propose union."""

from __future__ import annotations

import importlib
import json
from typing import Any


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.executor")
    operation = module._PlannedOperation(op="propose", args={})
    variants: list[dict[str, Any]] = [
        {
            "kind": "create",
            "path": "/a",
            "concept_type": "note",
            "title": "t",
            "body": "b",
        },
        {"kind": "patch", "path": "/a"},
        {"kind": "rename", "path": "/a", "new_path": "/b"},
        {
            "kind": "attach_asset_pack",
            "path": "/a",
            "asset_kind": "docs",
            "version": "1",
            "zip_base64": "eA==",
        },
        {"kind": "trash", "path": "/a"},
    ]
    base: dict[str, Any] = {
        "intent": "i",
        "base_revision": "r",
        "changes": variants,
    }
    arguments: list[dict[str, Any]] = [
        {},
        base,
        {**base, "changes": []},
        {**base, "changes": [{}]},
        {**base, "changes": [None]},
        {**base, "unknown": 1},
    ]
    invalid: list[Any] = [None, True, 0, "", [], {}, [None]]
    for key in ["intent", "base_revision", "changes", "rationale"]:
        for value in invalid:
            arguments.append({**base, key: value})
    for variant in variants:
        for key in variant:
            for value in invalid:
                arguments.append({**base, "changes": [{**variant, key: value}]})
        arguments.append({**base, "changes": [{**variant, "unknown": 1}]})
    asset = variants[3]
    arguments.extend(
        [
            {**base, "changes": [{**asset, "zip_base64": None}]},
            {**base, "changes": [{**asset, "staged_asset_id": "s"}]},
            {
                **base,
                "changes": [{**asset, "staged_asset_id": "s", "zip_base64": "z"}],
            },
        ]
    )
    cases: list[dict[str, Any]] = []
    for strict in [False, True]:
        for args in arguments:
            try:
                validated = module._validated_operation(operation, args=args, strict=strict)
                result = {"expected": json.loads(json.dumps(validated))}
            except module.ValidationError as exc:
                result = {
                    "issues": [
                        {
                            "path": ".".join(map(str, error["loc"])),
                            "message": error["msg"],
                        }
                        for error in exc.errors()
                    ]
                }
            cases.append({"arguments": args, "strict": strict, **result})
    return {"cases": cases}
