"""Reference discovery cursors, including historical permissive decoding."""

from __future__ import annotations

import base64
import importlib
import json
from typing import Any


def fixtures(shared: Any) -> list[dict[str, Any]]:
    sync = importlib.import_module("umcp")
    asynchronous = importlib.import_module("aioumcp")
    items = [{"name": "alpha"}, {"name": "beta"}, {"name": "gamma"}]
    common = {
        "items": items,
        "label": "tools",
        "result_key": "tools",
        "principal": "reader",
        "default_size": 0,
    }
    out: list[dict[str, Any]] = []

    def add(params: dict[str, Any], **overrides: Any) -> dict[str, Any]:
        case = {**common, **overrides, "params": params}
        results = []
        for cls in [sync.MCPServer, asynchronous.AsyncMCPServer]:
            server = object.__new__(cls)
            server.default_list_page_size = case["default_size"]
            token = shared.set_request_context(
                shared.MCPRequestContext(principal=case["principal"])
            )
            try:
                try:
                    result = server._page_list(
                        label=case["label"],
                        items=case["items"],
                        result_key=case["result_key"],
                        identity_keys=("name",),
                        params=params,
                    )
                    expected = {"result": result, "error": None}
                except ValueError as exc:
                    expected = {"result": None, "error": {"code": -32602, "message": str(exc)}}
                results.append(expected)
            finally:
                shared.reset_request_context(token)
        if results[0] != results[1]:
            raise AssertionError("pagination sync/async mismatch")
        out.append({"input": case, "expected": results[0]})
        return results[0]

    param_cases: list[dict[str, Any]] = [
        {},
        {"pageSize": None},
        {"pageSize": 1},
        {"pageSize": 3},
        {"pageSize": 10**40},
        {"pageSize": 0},
        {"pageSize": -1},
        {"pageSize": True},
        {"pageSize": 1.0},
        {"pageSize": "1"},
        {"cursor": 1},
        {"cursor": False},
        {"cursor": []},
        {"cursor": ""},
        {"cursor": "界"},
        {"cursor": "%%%"},
    ]
    for params in param_cases:
        add(params)
    first = add({"pageSize": 1})["result"]["nextCursor"]
    add({"cursor": first, "pageSize": 1})
    add({"cursor": first})
    add({"cursor": first}, default_size=1)
    add({"cursor": first}, default_size=-1)
    add({"cursor": first}, default_size=-10)
    add({"cursor": first}, principal="other")
    add({"cursor": first}, items=[{"name": "changed"}])
    add({"pageSize": 1}, items=[])
    add({"pageSize": 1}, label='界"\\\b\f\n\r\t😀\x01', principal="rüi")
    original = json.loads(base64.urlsafe_b64decode(first + "=" * (-len(first) % 4)))
    payload_cases: list[Any] = [
        None,
        [],
        {},
        {**original, "v": True},
        {**original, "v": 1.0},
        {**original, "v": 2},
        {**original, "v": "1"},
        {**original, "l": "other"},
        {**original, "o": True},
        {**original, "o": False},
        {**original, "o": -1},
        {**original, "o": 1.0},
        {**original, "o": 10**40},
        {**original, "o": None},
        {**original, "f": "bad"},
    ]
    for value in payload_cases:
        cursor = base64.urlsafe_b64encode(json.dumps(value).encode()).decode().rstrip("=")
        add({"cursor": cursor, "pageSize": 1})
    for payload in [b"not json", b"\xff", b"{} {}"]:
        add({"cursor": base64.urlsafe_b64encode(payload).decode().rstrip("=")})
    for cursor in [
        first + "====",
        first + "!!!!",
        "!!!!" + first,
        first + "!",
        first + "_",
        first + "-",
        first.replace("-", "+").replace("_", "/"),
    ]:
        add({"cursor": cursor, "pageSize": 1})
    return out
