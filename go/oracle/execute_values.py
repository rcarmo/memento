"""Executor value kernel without replacing validation/dispatch by Go expectations."""

from __future__ import annotations

import copy
import importlib
from typing import Any


def capture(call: Any) -> dict[str, Any]:
    try:
        return {"expected": call()}
    except Exception as exc:
        return {"error": str(exc), "type": type(exc).__name__}


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.executor")
    saved = {
        "hits": {
            "results": [
                {"path": "/a.md", "tags": ["x", "y", "z"]},
                {"path": "/b.md", "tags": []},
                {"path": "/c.md", "tags": ["q"]},
            ],
            "0": "zero",
        },
        "nil": None,
        "number": 7,
        "empty": [],
        "rows": [{"x": 1}, {"x": 2}, {"x": 3}],
        "odd\n": {"a\n": "newline"},
    }
    values = [
        None,
        True,
        7,
        "plain",
        "prefix $hits",
        "$hits",
        "$hits.results.0.path",
        ["$hits", "bad $x"],
        {"key": "$nil"},
        {"$not-a-reference-key": 1},
        {"nested": {"bad": "$hits..bad"}},
        {"nested": ["$absent"]},
        ["$hits", "$bad..path"],
        "$",
        "$unknown",
        "$hits.results.00.path",
        "$hits.results.-1",
        "$hits.a\n",
        "$odd\n",
        "$hits." + "a" * 250,
        "$a" + "b" * 32,
    ]
    contains = [{"value": v, **capture(lambda v=v: module._contains_references(v))} for v in values]
    resolutions = [
        {"value": v, **capture(lambda v=v: module._resolve_references(v, saved))} for v in values
    ]
    references = [
        "literal",
        "$hits",
        "$nil",
        "$unknown",
        "$number.x",
        "$hits.absent",
        "$hits.results.0.path",
        "$hits.results.9",
        "$empty.0",
        "$hits.results.path",
        "$hits.results.0001.path",
        "$hits.results." + "9" * 80,
        "$hits.0",
        "$hits..x",
        "$1bad",
        "$odd\n.a\n",
        "$hits.results.0\n.path",
    ]
    refs = [
        {"ref": r, **capture(lambda r=r: module._resolve_reference(r, saved))} for r in references
    ]
    extracts = []
    for value, path in [
        (saved["hits"], "results.0.path"),
        (saved["hits"], "results.4.path"),
        (saved["hits"], "results.nope"),
        (saved["hits"], "results." + "9" * 80),
        ([], "0"),
        ({}, "missing"),
        (1, "a"),
        ({}, ""),
        (saved["hits"], "results.-1"),
        (saved["odd\n"], "a\n"),
    ]:
        extracts.append(
            {
                "value": value,
                "path": path,
                **capture(lambda value=value, path=path: module._extract_field(value, path)),
            }
        )
    bounds = [
        {"value": v, "limit": n, "expected": module._bound_value(v, n)}
        for v in [saved, [], [1, [2, 3, 4], {"items": [5, 6, 7]}], "x", None]
        for n in [0, 1, 2, 50, -1]
    ]
    projections = []
    scenarios: list[dict[str, Any]] = [
        {"returns": []},
        {"returns": [{"ref": "$hits.results", "fields": ["path", "tags"], "name": "picked"}]},
        {"returns": [{"ref": "$hits.results", "fields": ["path"], "limit": 1}]},
        {"returns": [{"ref": "$hits", "fields": ["results.0.path"]}]},
        {"returns": [{"ref": "$hits.results", "limit": 1}]},
        {"returns": [{"ref": "$hits.results"}]},
        {"returns": [{"ref": "$rows", "fields": ["x"]}], "asset": True},
        {"returns": [{"ref": "$rows"}], "asset": True},
        {"returns": [{"ref": "$rows", "fields": ["x"], "limit": 1}], "asset": True},
        {"returns": [{"ref": "$rows", "limit": 2}], "asset": True},
        {
            "returns": [{"ref": "$rows", "fields": ["x"], "name": "shadowed"}],
            "asset": True,
            "shadowed": True,
        },
        {"returns": [{"ref": "$rows", "fields": ["x", "x"]}], "max_records": 1},
        {"returns": [{"ref": "$nil", "name": "null"}]},
        {"returns": [{"ref": "literal"}]},
        {"returns": [{"ref": "$number", "name": ""}]},
        {"returns": [{"ref": "$number", "name": "same"}, {"ref": "$nil", "name": "same"}]},
        {"returns": [{"ref": "$missing"}]},
        {"returns": [{"ref": "$missing"}], "failed": ["missing"]},
        {"returns": [{"ref": "$hits.results", "fields": ["missing"]}]},
        {"returns": [{"ref": "$hits", "fields": ["results.9"]}]},
    ]
    for scenario in scenarios:
        max_records = scenario.get("max_records", 2)
        executor = module.MemoryExecutor(None, module.ExecuteLimits(max_records=max_records))
        operation: dict[str, Any] = {
            "op": "asset_get" if scenario.get("asset") else "list",
            "args": {},
            "save_as": "rows",
        }
        operations = [operation]
        if scenario.get("shadowed"):
            operations.append({"op": "list", "args": {}, "save_as": "rows"})
        plan = module._InputPlan.model_validate(
            {"operations": operations, "returns": scenario["returns"]}
        )
        failed = set(scenario.get("failed", []))
        projections.append(
            {
                "operations": [
                    {"operation": op["op"], "save_as": op["save_as"]} for op in operations
                ],
                "returns": scenario["returns"],
                "failed": sorted(failed),
                "last": {"last": [1, 2, 3]},
                "max_records": max_records,
                **capture(
                    lambda plan=plan, executor=executor, failed=failed: executor._project_returns(
                        plan, saved, {"last": [1, 2, 3]}, failed_save_as=failed
                    )
                ),
            }
        )
    outputs = []
    trace = [
        {
            "index": i,
            "op": "read",
            "data": {"body": "x" * 200, "unicode": "café\n"},
            "operation_id": f"op{i}",
        }
        for i in range(7)
    ]
    revisions = [
        {"index": i, "repo_revision": "r" * 30, "operation_id": f"op{i}"} for i in range(7)
    ]
    payload: dict[str, Any] = {
        "trace": trace,
        "revisions": revisions,
        "returns": {"result": "r" * 700},
        "stopped": False,
        "stop_reason": None,
    }
    for n in [4096, 3000, 2400, 1800, 1400, 1000, 700, 512, 350, 200, 80]:
        for committed in [False, True]:
            executor = module.MemoryExecutor(
                None, module.ExecuteLimits.model_construct(max_output_bytes=n)
            )
            original = copy.deepcopy(payload)
            outputs.append(
                {
                    "payload": payload,
                    "max_bytes": n,
                    "committed": committed,
                    "size": executor._payload_size(payload),
                    "ensure": capture(
                        lambda executor=executor, committed=committed: executor._ensure_output_size(
                            payload, commit_succeeded=committed
                        )
                    ),
                    "fit": capture(
                        lambda executor=executor, committed=committed: executor._fit_output_payload(
                            payload, commit_succeeded=committed
                        )
                    ),
                }
            )
            assert payload == original
    extra_payloads: list[dict[str, Any]] = [
        {"trace": [{"data": [1, 2]}], "revisions": [], "returns": {}, "stop_reason": "x" * 500},
        {"trace": [1, {}, "x"], "revisions": [], "returns": {"a": "x" * 999}},
        {"unicode": "🍀 café", "integer": 10**40},
        {"trace": [], "revisions": None, "returns": {"a": "x" * 800}},
    ]
    for payload in extra_payloads:
        executor = module.MemoryExecutor(None, module.ExecuteLimits(max_output_bytes=512))
        outputs.append(
            {
                "payload": payload,
                "max_bytes": 512,
                "committed": True,
                "size": executor._payload_size(payload),
                "ensure": capture(
                    lambda executor=executor, payload=payload: executor._ensure_output_size(
                        payload, commit_succeeded=True
                    )
                ),
                "fit": capture(
                    lambda executor=executor, payload=payload: executor._fit_output_payload(
                        payload, commit_succeeded=True
                    )
                ),
            }
        )
    return {
        "saved": saved,
        "contains": contains,
        "resolutions": resolutions,
        "references": refs,
        "extracts": extracts,
        "bounds": bounds,
        "projections": projections,
        "outputs": outputs,
    }
