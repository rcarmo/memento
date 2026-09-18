"""Capture the actual executor compare_manifest argument model."""

from __future__ import annotations

import importlib
import json
from pathlib import Path
from typing import Any


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.executor")
    operation = module._PlannedOperation(op="compare_manifest", args={})
    root = Path(__file__).resolve().parents[1]
    timestamps = json.loads((root / "testdata/parity/manifest-timestamps.json").read_text())
    base: dict[str, Any] = {
        "name": "n",
        "local_path": "local/n.md",
        "memento_path": "/public/n.md",
        "local_updated_at": "2026-01-02T03:04:05Z",
        "local_body_sha256": "a" * 64,
        "local_bytes": 2,
    }
    arguments: list[dict[str, Any]] = [
        {},
        {"items": []},
        {"items": [{}]},
        {"items": [None]},
        {"items": [base]},
        {"items": [base] * 51},
        {"items": [{**base, "memento_path": None}]},
        {
            "items": [{**base, "memento_path": None}],
            "match": {"path_template": "/{name}"},
        },
        {
            "items": [base],
            "match": {"aliases": {str(index): str(index) for index in range(51)}},
        },
    ]
    invalid: list[Any] = [None, True, 0, "", "x", [], {}, [None]]
    for key in ["path_prefix", "items", "match", "include_asset_metadata"]:
        for value in invalid:
            arguments.append({"items": [base], key: value})
    for key in base:
        for value in invalid + [-1]:
            arguments.append({"items": [{**base, key: value}]})
    for case in timestamps:
        arguments.append({"items": [{**base, "local_updated_at": case["value"]}]})
    cases: list[dict[str, Any]] = []
    for strict in [False, True]:
        for args in arguments:
            try:
                validated = module._validated_operation(operation, args=args, strict=strict)
                result = {"expected": json.loads(json.dumps(validated, default=str))}
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
