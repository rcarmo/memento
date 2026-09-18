"""Dynamic resource discovery/read/subscription parity for sync and async bases."""

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
        self.logger = logging.getLogger("oracle-resources")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    def text() -> str:
        return "hello ☃"

    def blob() -> bytes:
        return bytes([0, 1, 255])

    def mixed() -> list[Any]:
        return [
            {"text": "entry"},
            {"uri": "custom:", "mimeType": "custom/type", "text": "override"},
            ["nested", None, True, 7],
        ]

    def fail() -> Any:
        raise RuntimeError("private detail")

    def cancelled() -> Any:
        raise shared.MCPRequestCancelled("cancelled")

    def template(path: str) -> str:
        return f"path={path}"

    servers = []
    for base in [sync.MCPServer, asynchronous.AsyncMCPServer]:
        cls = type("ResourceOracle", (base,), {"_setup_logging": setup})
        s = cls()
        s.register_resource(
            "test:text",
            text,
            name="text",
            title="Title",
            description="Description",
            mime_type="text/markdown",
            size=7,
            annotations={"priority": 1},
        )
        s.register_resource("test:blob", blob)
        s.register_resource("test:mixed", mixed, mime_type="text/custom")
        s.register_resource("test:failure", fail)
        s.register_resource("test:cancelled", cancelled)
        s.register_resource_template(
            "test:///{path}",
            template,
            name="template",
            title="Template",
            description="path",
            mime_type="text/plain",
            annotations={"priority": 0.5},
        )
        servers.append(s)
    calls = [
        ("resources/list", {}),
        ("resources/list", {"pageSize": 2}),
        ("resources/templates/list", {}),
        ("resources/templates/list", {"pageSize": 1}),
        ("resources/read", {}),
        ("resources/read", {"uri": "missing"}),
        *(
            ("resources/read", {"uri": uri})
            for uri in [
                "test:text",
                "test:blob",
                "test:mixed",
                "test:failure",
                "test:cancelled",
                "test:///raw%20path",
                "test:///two/segments",
            ]
        ),
        ("resources/subscribe", {}),
        ("resources/unsubscribe", {}),
        ("resources/subscribe", {"uri": "missing", "_session_id": "s1"}),
        ("resources/unsubscribe", {"uri": "missing", "_session_id": "s1"}),
        ("resources/subscribe", {"uri": "missing"}),
        ("resources/unsubscribe", {"uri": "missing"}),
    ]
    cases = []
    for transport in ["stdio", "streamable-http"]:
        for method, params in calls:
            request = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
            context = shared.MCPRequestContext(transport=transport, principal="reader")
            outputs = [
                servers[0].process_request(request, context=context),
                asyncio.run(servers[1].process_request_async(request, context=context)),
            ]
            if outputs[0] != outputs[1]:
                raise AssertionError(f"resource sync/async mismatch {method} {params}")
            cases.append({"request": request, "transport": transport, "expected": outputs[0]})
    return {"cases": cases}
