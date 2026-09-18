"""Actual structural executor models, normalization and preflight ordering."""

from __future__ import annotations

import importlib
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def capture(call: Any, module: Any, label: str) -> dict[str, Any]:
    try:
        value = call()
        return {
            "expected": value.model_dump(mode="json") if hasattr(value, "model_dump") else value
        }
    except Exception as exc:
        return {
            "error": module._validation_message(exc, label=label)
            if isinstance(exc, module.ValidationError)
            else str(exc),
            "type": type(exc).__name__,
        }


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.executor")
    server: Any = importlib.import_module("memento.server")
    registry: Any = importlib.import_module("memento.mcp_registry")
    contracts = []
    for name, model in module._OPERATION_MODELS.items():
        fields = model.model_fields["args"].annotation.model_fields
        contracts.append(
            {
                "name": name,
                "commit_capable": registry.OPERATION_SPEC_BY_OP[name].commit_capable,
                "fields": list(fields),
                "required": [name for name, field in fields.items() if field.is_required()],
            }
        )
    values: list[Any] = [
        None,
        1,
        [],
        {},
        {"operations": []},
        {"operations": None},
        {"operations": {}},
        {"operations": [None]},
        {"operations": [{}]},
        {"operations": [{"op": "bad"}]},
        {"operations": [{"op": 1}]},
        {"operations": [{"op": "read"}]},
        {"operations": [{"op": "read", "args": None}]},
        {"operations": [{"op": "read", "args": []}]},
        {"operations": [{"op": "read", "args": {"id_or_path": 7}, "save_as": "bad name"}]},
        {"operations": [{"op": "read", "save_as": 7, "extra": 1}]},
        {"operations": [], "extra": 1},
        {"operations": [], "stop_on_error": None},
        {"operations": [], "returns": None},
        {"operations": [], "returns": [None]},
        {"operations": [], "returns": [{}]},
        {"operations": [], "returns": [{"ref": 7, "name": 8, "fields": [1, 2], "extra": 1}]},
        {"operations": [], "returns": [{"ref": "literal", "fields": None}]},
        {"operations": [], "returns": [{"ref": "$bad..ref", "limit": 0}]},
        {"operations": [], "returns": [{"ref": "literal", "name": "", "limit": 10**40}]},
    ]
    values.append(
        {"operations": [], "returns": [{"ref": "$x", "fields": ["path", "tags", "path"]}]}
    )
    for name in module._OPERATION_MODELS:
        values.append({"operations": [{"op": name, "save_as": None}]})
    for value in [
        True,
        False,
        0,
        1,
        2,
        1.0,
        1.5,
        "yes",
        "off",
        "TRUE",
        " false ",
        "bad",
        [],
        {},
        10**400,
        -1,
        "t",
        "N",
        "on",
        "no",
        "1",
        "0",
    ]:
        values.append({"operations": [], "stop_on_error": value})
    for value in [
        True,
        False,
        None,
        1,
        1.0,
        1.5,
        "1",
        "+2",
        " 2 ",
        "1_0",
        "1.0",
        "1.00",
        "1e2",
        "١",
        "bad",
        [],
        {},
        -1,
        "9" * 4301,
        "0" * 4301,
        "0" * 4300,
        "1.000_0",
        "  +001_000.00  ",
        "１",
        "\u00a01\u00a0",
        "\x1c1\x1c",
        "1.",
        "-0.0",
        1e100,
        2.0**63,
        -(2.0**63),
        1e18,
        "0" * 4301 + "1",
    ]:
        values.append({"operations": [], "returns": [{"ref": "$x", "limit": value}]})
    plans = [
        {"value": v, **capture(lambda v=v: module._InputPlan.model_validate(v), module, "plan")}
        for v in values
    ]
    normalization = []
    for args in [
        {},
        {"operations": []},
        {"plan": {}},
        {"plan": {"operations": []}},
        {"plan": {}, "operations": []},
        {"plan": {}, "returns": []},
        {"plan": {}, "stop_on_error": False},
        {"plan": {}, "stop_on_error": 1},
        {"plan": {}, "stop_on_error": None},
        {"operations": [], "returns": None},
        {"operations": [], "returns": False},
        {"operations": [], "returns": "text"},
        {"operations": None},
        {"plan": False},
        {"operations": [], "returns": 0},
        {"operations": [], "returns": 1},
        {"operations": [], "returns": {}},
        {"operations": [], "returns": {"x": 1}},
        {"operations": [], "returns": []},
        {"operations": [], "returns": [1]},
        {"operations": [], "returns": ""},
    ]:
        normalization.append(
            {
                "arguments": args,
                **capture(
                    lambda args=args: server.normalize_execute_tool_arguments(**args),
                    module,
                    "plan",
                ),
            }
        )
    limits = []
    for value in [
        None,
        {},
        {"max_operations": 0},
        {"max_operations": 33},
        {"max_intermediates": 0},
        {"max_intermediates": 65},
        {"max_records": 501},
        {"max_output_bytes": 511},
        {"max_output_bytes": 10**50},
        {"max_time_seconds": 0},
        {"max_time_seconds": 31},
        {"max_time_seconds": "bad"},
        {"max_time_seconds": None},
        {"max_time_seconds": True},
        {"max_time_seconds": False},
        {"max_time_seconds": "0x1p2"},
        {"max_time_seconds": "1_0"},
        {"max_time_seconds": "1_0.0"},
        {"max_time_seconds": "1e999999999"},
        {"max_time_seconds": ".5"},
        {"max_time_seconds": " -infinity "},
        {"max_time_seconds": "\u00a01\u00a0"},
        {"max_time_seconds": []},
        {"max_time_seconds": 1.5},
        {"max_output_bytes": "512.0"},
        {"max_time_seconds": "nan"},
        {"max_time_seconds": "inf"},
        {"max_records": "10", "max_time_seconds": "2.5"},
        {"max_records": 1.5},
        {"extra": 1},
        {"max_records": False},
        {"max_operations": None, "max_records": 0, "max_output_bytes": 0, "extra": 1},
    ]:
        limits.append(
            {
                "value": value,
                **capture(
                    lambda value=value: module.ExecuteLimits.model_validate(value), module, "limits"
                ),
            }
        )
    preflights = []
    for raw, max_ops in [
        ({"operations": []}, 12),
        ({"operations": [{"op": "list"}] * 13}, 12),
        ({"operations": [{"op": "create"}, {"op": "purge"}]}, 12),
        ({"operations": [{"op": "read", "args": {"id_or_path": "$x"}}]}, 12),
        ({"operations": [{"op": "read", "args": {"id_or_path": "$x", "extra": 1}}]}, 12),
        ({"operations": [{"op": "read", "args": {"unused": "$x"}}]}, 12),
        ({"operations": [{"op": "asset_get", "args": {"id_or_path": "$x"}}]}, 12),
        ({"operations": [{"op": "read", "args": {"id_or_path": "$x..bad"}}]}, 12),
        (
            {
                "operations": [
                    {
                        "op": "search",
                        "args": {"query": "$x", "limit": "not validated yet"},
                        "save_as": "invalid name",
                    }
                ]
            },
            12,
        ),
        ({"operations": [{"op": "help"}, {"op": "status"}, {"op": "list"}]}, 12),
    ]:
        calls: list[str] = []
        validate = module._validated_operation

        def record(
            item: Any,
            *,
            args: Any,
            strict: bool = False,
            calls: Any = calls,
            validate: Any = validate,
        ) -> Any:
            calls.append(item.op)
            return validate(item, args=args, strict=strict)

        class PreflightComplete(Exception):
            pass

        def stop_at_clock() -> float:
            raise PreflightComplete

        runtime = module.MemoryExecutor(
            SimpleNamespace(), module.ExecuteLimits(max_operations=max_ops)
        )
        # Actual run() preflights before its first clock read; do not duplicate
        # the reference algorithm to manufacture expected outcomes.
        with (
            patch.object(module, "_validated_operation", record),
            patch.object(module, "monotonic", stop_at_clock),
        ):
            try:
                response = runtime.run(None, plan=raw)
                result: dict[str, Any] = {"error": response.message, "type": "ValueError"}
            except PreflightComplete:
                result = {"expected": None}
        preflights.append({"plan": raw, "max_operations": max_ops, "calls": calls, **result})
    return {
        "contracts": contracts,
        "plans": plans,
        "normalization": normalization,
        "limits": limits,
        "preflights": preflights,
    }
