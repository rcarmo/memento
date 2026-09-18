"""Tool result/schema subset fixtures emitted from pinned sync/async uMCP."""

from __future__ import annotations

import importlib
import json
from typing import Any


def fixtures() -> dict[str, Any]:
    sync = importlib.import_module("umcp")
    asynchronous = importlib.import_module("aioumcp")
    values: list[Any] = [
        None,
        True,
        False,
        0,
        1,
        -1,
        1.0,
        1.5,
        1e-5,
        1e-4,
        1e15,
        1e16,
        -0.0,
        "hello ☃ <>&",
        "a\u2028b\u2029c",
        [],
        [1, True, None, "x"],
        {"z": 1, "a": 2},
        {"nested": [{"b": 1, "a": "界"}]},
    ]
    schemas: list[Any] = [
        None,
        {},
        {"type": "null"},
        {"type": "string"},
        {"type": "integer"},
        {"type": "number"},
        {"type": "boolean"},
        {"type": "array"},
        {"type": "array", "items": {"type": "integer"}},
        {"type": "object"},
        {
            "type": "object",
            "properties": {"z": {"type": "integer"}},
            "required": ["z"],
            "additionalProperties": False,
        },
        {"type": "object", "additionalProperties": {"type": "integer"}},
        {"required": ["x"]},
        {"type": ["integer", "null"]},
        {"anyOf": [{"type": "string"}, {"type": "number"}]},
        {"oneOf": [{"type": "number"}, {"type": "integer"}]},
        {"enum": [1, True, "one", None]},
        {"enum": [{"z": 1, "a": 2}, [1, True, None, "x"]]},
        {"type": "ignored"},
        {"type": "string", "minLength": 99},
    ]

    def method() -> Any:
        return None

    cases = []
    for value in values:
        for schema in schemas:
            outcomes = []
            for cls in [sync.MCPServer, asynchronous.AsyncMCPServer]:
                server = object.__new__(cls)
                try:
                    result = server._format_tool_result(method, value, output_schema=schema)
                    outcome = {"result": result, "error": None}
                except ValueError as exc:
                    outcome = {"result": None, "error": str(exc)}
                outcomes.append(outcome)
            if outcomes[0] != outcomes[1]:
                raise AssertionError("tool result sync/async mismatch")
            cases.append(
                {
                    "value_json": json.dumps(value, ensure_ascii=False),
                    "schema_json": json.dumps(schema, ensure_ascii=False),
                    "expected": outcomes[0],
                }
            )
    return {"cases": cases}
