"""Prompt discovery/call output and metadata quirks from pinned Python bases."""

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
        self.logger = logging.getLogger("oracle-prompts")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    def hello(name: str = "world") -> str:
        """Hello prompt.

        Categories: Work, work; AI / bad category | good_tag
        """
        return f"Hello {name}"

    def count(value: int) -> int:
        return value

    def messages() -> list[dict[str, Any]]:
        return [{"role": "assistant", "content": {"type": "text", "text": "message"}}]

    def body() -> dict[str, Any]:
        """Body prompt.

        Categories: original
        """
        return {"messages": [], "description": "override", "custom": True}

    def scalar() -> list[Any]:
        return [1, None, True]

    def mapping() -> dict[str, Any]:
        return {"z": 1, "a": 2}

    def fail() -> Any:
        raise ValueError("private argument failure")

    def cancelled() -> Any:
        raise shared.MCPRequestCancelled("cancelled")

    methods = {
        "hello": hello,
        "count": count,
        "messages": messages,
        "body": body,
        "scalar": scalar,
        "mapping": mapping,
        "fail": fail,
        "cancelled": cancelled,
    }
    servers = []
    for base in [sync.MCPServer, asynchronous.AsyncMCPServer]:
        cls = type("PromptOracle", (base,), {"_setup_logging": setup})
        s = cls()
        for name, fn in methods.items():
            s.register_prompt(name, fn, input_schema={})
        s.register_prompt(
            "hello",
            hello,
            description="Discovery override",
            input_schema={},
            categories=["explicit"],
        )
        servers.append(s)
    calls = [
        ("prompts/list", {}),
        ("prompts/list", {"pageSize": 2}),
        ("prompts/get", {}),
        ("prompts/get", {"name": "missing"}),
        *(("prompts/get", {"name": name}) for name in methods),
        ("prompts/get", {"name": "hello", "arguments": {"name": "Rui"}}),
        ("prompts/get", {"name": "hello", "arguments": None}),
        ("prompts/get", {"name": "hello", "arguments": {"z": 1, "a": 2}}),
        ("prompts/get", {"name": "count", "arguments": {"value": "12"}}),
    ]
    cases = []
    for transport in ["stdio", "streamable-http"]:
        for method, params in calls:
            request = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
            ctx = shared.MCPRequestContext(transport=transport, principal="reader")
            outputs = [
                servers[0].process_request(request, context=ctx),
                asyncio.run(servers[1].process_request_async(request, context=ctx)),
            ]
            if outputs[0] != outputs[1]:
                raise AssertionError("prompt sync/async mismatch")
            cases.append({"request": request, "transport": transport, "expected": outputs[0]})
    return {"cases": cases}
