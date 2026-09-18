"""Completion/config/logging scenarios from sync and async uMCP."""

from __future__ import annotations

import asyncio
import importlib
import json
import logging
from typing import Any, Literal


def fixtures(shared: Any) -> dict[str, Any]:
    sync = importlib.import_module("umcp")
    asynchronous = importlib.import_module("aioumcp")

    def setup(self: Any) -> None:
        self.logger = logging.getLogger("oracle-completion")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    def choose(value: Literal["alpha", "beta", "alphabet"]) -> str:
        return value

    def resource(path: str) -> str:
        return path

    def provider(**kwargs: Any) -> Any:
        prefix = kwargs["prefix"]
        if prefix == "invalid":
            return {"values": "not list"}
        if prefix == "value-error":
            raise ValueError("provider bad argument")
        if prefix == "failure":
            raise RuntimeError("private")
        return {
            "values": ["alpha", "alpine", "alpha", "beta", 1, True],
            "total": 10,
            "hasMore": True,
        }

    servers = []
    for base in [sync.MCPServer, asynchronous.AsyncMCPServer]:
        cls = type("CompletionOracle", (base,), {"_setup_logging": setup})
        s = cls()
        s.register_prompt("choose", choose)
        s.register_resource_template("test:///{path}", resource, name="template")
        s.register_completion_provider("ref/prompt", "choose", "value", provider)
        servers.append(s)
    ref = {"type": "ref/prompt", "name": "choose"}
    argument = {"name": "value", "value": "al"}
    base = {"ref": ref, "argument": argument}
    parameters = [
        {},
        {"ref": ref},
        {**base, "context": "x"},
        {**base, "maxValues": 0},
        {**base, "maxValues": True},
        {**base, "maxValues": "2"},
        {**base, "argument": {"name": ""}},
        {**base, "context": {"arguments": "x"}},
        {**base, "ref": {"type": "bad"}},
        {**base, "ref": {"type": "ref/prompt"}},
        {**base, "ref": {"type": "ref/prompt", "name": "missing"}},
        {**base, "ref": {"type": "ref/resource"}},
        {**base, "ref": {"type": "ref/resource", "uri": "missing"}},
        {**base, "argument": {"name": "missing"}},
        base,
        {**base, "maxValues": 1},
        {**base, "maxValues": 10**40},
        {**base, "argument": {"name": "value", "value": None}},
        {**base, "argument": {"name": "value", "value": True}},
        {**base, "argument": {"name": "value", "value": "invalid"}},
        {**base, "argument": {"name": "value", "value": "value-error"}},
        {**base, "argument": {"name": "value", "value": "failure"}},
        {
            "ref": {"type": "ref/resource", "name": "template"},
            "argument": {"name": "path", "value": ""},
        },
        {
            "ref": {"type": "ref/resource", "uriTemplate": "test:///{path}"},
            "argument": {"name": "path"},
        },
    ]
    calls = [
        ("initialize", {}),
        ("initialize", {"protocolVersion": "2024-11-05"}),
        ("initialize", {"protocolVersion": "unsupported"}),
        *(("completion/complete", p) for p in parameters),
        ("logging/setLevel", {"level": "invalid"}),
        ("logging/setLevel", {"level": "debug"}),
    ]
    cases = []
    for method, params in calls:
        request = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
        outputs = [
            servers[0].process_request(request),
            asyncio.run(servers[1].process_request_async(request)),
        ]
        if outputs[0] != outputs[1]:
            raise AssertionError(f"completion mismatch {request}")
        cases.append({"request": request, "expected": outputs[0]})
    data = {
        "token": "private",
        "nested": [
            {"Password": "secret", "text": "Bearer abc.def then sk-ABC and api_key=123 token:xyz"}
        ],
        "count": 1,
    }
    return {"cases": cases, "log_input": data, "log_sanitized": servers[0]._sanitize_log_data(data)}
