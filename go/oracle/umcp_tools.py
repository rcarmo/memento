"""Registration, annotations, coercion and calls from both Python uMCP bases."""

from __future__ import annotations

import asyncio
import importlib
import json
import logging
from typing import Any


def fixtures(shared: Any) -> dict[str, Any]:
    sync = importlib.import_module("umcp")
    asynchronous = importlib.import_module("aioumcp")

    def setup(self: Any) -> None:
        self.logger = logging.getLogger("oracle-tools")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    def echo(text: str) -> str:
        return text

    def add(a: int, b: int = 1) -> int:
        return a + b

    def boolean(flag: bool = False) -> bool:
        return flag

    def union(value: int | str) -> Any:
        return value

    def numeric(value: int | float) -> Any:
        return value

    def ping() -> str:
        return "pong"

    def structured() -> dict[str, int]:
        return {"z": 1, "a": 2}

    def invalid() -> dict[str, int]:
        return {"z": "wrong"}  # type: ignore[dict-item]

    def argument() -> Any:
        raise ValueError("bad argument")

    def failure() -> Any:
        raise RuntimeError("private details")

    def cancelled() -> Any:
        raise shared.MCPRequestCancelled("Request cancelled")

    methods = {
        "echo": echo,
        "add": add,
        "boolean": boolean,
        "union": union,
        "numeric": numeric,
        "ping": ping,
        "structured": structured,
        "invalid": invalid,
        "argument": argument,
        "failure": failure,
        "cancelled": cancelled,
    }
    servers = []
    for base in [sync.MCPServer, asynchronous.AsyncMCPServer]:
        cls = type("ToolOracle", (base,), {"_setup_logging": setup})
        server = cls()
        for name, fn in methods.items():
            server.register_tool(
                name,
                fn,
                input_schema={"type": "object", "properties": {}, "additionalProperties": False},
            )
        servers.append(server)
    params: list[dict[str, Any]] = [
        {},
        {"name": "missing"},
        {"name": "ping"},
        {"name": "echo", "arguments": {"text": "hello ☃"}},
        {"name": "add", "arguments": {"a": "4"}},
        {"name": "add", "arguments": {"a": 5, "b": 2}},
        {"name": "add", "arguments": {"a": "bad"}},
        {"name": "echo", "arguments": {}},
        {"name": "ping", "arguments": {"z": 1, "a": 2}},
        {"name": "boolean", "arguments": {"flag": "YES"}},
        {"name": "boolean", "arguments": {"flag": " true "}},
        {"name": "union", "arguments": {"value": "42"}},
        {"name": "numeric", "arguments": {"value": "42"}},
        {"name": "numeric", "arguments": {"value": "1.5"}},
        {"name": "numeric", "arguments": {"value": "not numeric"}},
        {"name": "structured"},
        {"name": "invalid"},
        {"name": "argument"},
        {"name": "failure"},
        {"name": "cancelled"},
    ]
    cases = []
    for transport in ["stdio", "streamable-http"]:
        for p in params:
            request = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": p})
            context = shared.MCPRequestContext(transport=transport, principal="reader")
            outputs = [
                servers[0].process_request(request, context=context),
                asyncio.run(servers[1].process_request_async(request, context=context)),
            ]
            if outputs[0] != outputs[1]:
                raise AssertionError(f"tool call sync/async mismatch {p!r}")
            cases.append({"request": request, "transport": transport, "expected": outputs[0]})
    lists = [s.discover_tools() for s in servers]
    if lists[0] != lists[1]:
        raise AssertionError("tool list mismatch")
    annotations = [
        {"name": name, "expected": servers[0]._infer_tool_annotations(name, ping)}
        for name in [
            "plain",
            "get_notes",
            "memory_read",
            "restart_service",
            "web_query",
            "azure_delete",
            "fetch_page",
            "delete_read",
            "office_read",
        ]
    ]
    return {"cases": cases, "tools": lists[0], "annotations": annotations}
