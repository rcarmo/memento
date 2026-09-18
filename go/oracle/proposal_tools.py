"""Capture actual proposal wrapper signature/discovery and uMCP dispatch."""

from __future__ import annotations

import asyncio
import importlib
import inspect
import json
import logging
from functools import wraps
from typing import Any

NAMES = [
    "memory_propose",
    "memory_proposal_get",
    "memory_proposal_list",
    "memory_proposal_asset_get",
    "memory_proposal_rebase",
    "memory_proposal_revise",
    "memory_proposal_review",
    "memory_proposal_apply",
    "memory_operation_get",
]


READ_NAMES = ["memory_read", "memory_list", "memory_search", "memory_graph"]


def fixtures(*, names: list[str] | None = None) -> dict[str, Any]:
    names = names if names is not None else NAMES
    module: Any = importlib.import_module("memento.server")
    umcp: Any = importlib.import_module("aioumcp")
    shared: Any = importlib.import_module("umcp_shared")
    registry: Any = importlib.import_module("memento.mcp_registry")

    def setup(self: Any) -> None:
        self.logger = logging.getLogger("proposal-tool-oracle")
        self.logger.addHandler(logging.NullHandler())
        self.logger.propagate = False

    cls = type("ProposalOracle", (module.MementoMCPServer,), {"_setup_logging": setup})
    server: Any = object.__new__(cls)
    umcp.AsyncMCPServer.__init__(server)
    server._context = lambda: "trusted"

    class Envelope:
        def __init__(self, data: dict[str, Any]) -> None:
            self.data = data

        def model_dump(self, **kwargs: Any) -> dict[str, Any]:
            return self.data

    async def call(method: str, context: Any, **arguments: Any) -> Any:
        return Envelope({"method": method, "arguments": arguments, "context": context})

    async def notify(envelope: Any) -> None:
        pass

    server._memory_call = call
    server._notify_for_envelope = notify
    definitions = []
    for name in [spec.tool_name for spec in registry.OPERATION_SPECS if spec.tool_name in names]:
        method = getattr(server, "tool_" + name)
        spec = registry.OPERATION_SPEC_BY_TOOL[name]
        parameters = []
        for key, parameter in inspect.signature(method).parameters.items():
            item = {"name": key, "required": parameter.default is inspect.Parameter.empty}
            if parameter.default is not inspect.Parameter.empty:
                item["default"] = parameter.default
            parameters.append(item)
        definitions.append(
            {
                "name": name,
                "description": spec.description,
                "inputSchema": server._tool_input_schema(method, name),
                "annotations": {"roles": list(spec.roles), "operation": spec.op_name},
                "parameters": parameters,
            }
        )

    # These wrappers bypass _memory_call. Echo only their argument dispatch here
    # using the real signature; staging_tools.py exercises their actual bodies.
    def direct_echo(method: Any, name: str) -> Any:
        @wraps(method)
        async def echo(**arguments: Any) -> dict[str, Any]:
            return {"method": name, "arguments": arguments, "context": "trusted"}

        return echo

    for name in ("memory_asset_stage_begin", "memory_asset_stage_status"):
        if name in names:
            method = getattr(server, "tool_" + name)
            setattr(server, "tool_" + name, direct_echo(method, name))
    requests: list[dict[str, Any]] = [
        {
            "name": "memory_propose",
            "arguments": {"intent": "new", "base_revision": "base", "changes": []},
        },
        {"name": "memory_proposal_get", "arguments": {"proposal_id": "p"}},
        {"name": "memory_proposal_get", "arguments": {"proposal_id": "p", "view": "nonsense"}},
        {"name": "memory_proposal_list"},
        {"name": "memory_proposal_list", "arguments": {"limit": "2"}},
        {"name": "memory_proposal_list", "arguments": {"limit": True}},
        {
            "name": "memory_proposal_asset_get",
            "arguments": {"proposal_id": "p", "asset_id": "a", "offset": "1_0", "limit": "５"},
        },
        {
            "name": "memory_proposal_rebase",
            "arguments": {"proposal_id": "p", "expected_revision": "r", "idempotency_key": "key"},
        },
        {
            "name": "memory_proposal_revise",
            "arguments": {
                "proposal_id": "p",
                "selected_change_indexes": [1, 0, 1],
                "expected_revision": "r",
            },
        },
        {
            "name": "memory_proposal_review",
            "arguments": {"proposal_id": "p", "decision": "approve"},
        },
        {
            "name": "memory_proposal_apply",
            "arguments": {"proposal_id": "p", "expected_revision": "r", "idempotency_key": "key"},
        },
        {"name": "memory_operation_get", "arguments": {"idempotency_key": "key"}},
        {"name": "memory_operation_get", "arguments": {}},
        {"name": "memory_proposal_get", "arguments": {}},
        {
            "name": "memory_propose",
            "arguments": {
                "intent": "new",
                "base_revision": "r",
                "changes": [],
                "principal": "admin",
            },
        },
    ]
    requests += [
        {"name": "memory_read", "arguments": {"id_or_path": "/public/a.md"}},
        {"name": "memory_list"},
        {"name": "memory_search", "arguments": {"query": "alpha"}},
        {
            "name": "memory_search",
            "arguments": {"query": "alpha", "limit": "2", "search_mode": "hybrid"},
        },
        {"name": "memory_graph", "arguments": {"id_or_path": "id", "depth": "２"}},
    ]
    requests += [
        {"name": "memory_asset_stage_begin", "arguments": {}},
        {
            "name": "memory_asset_stage_begin",
            "arguments": {"asset_kind": "docs", "version": "1.0.0", "idempotency_key": "ticket"},
        },
        {
            "name": "memory_asset_stage_begin",
            "arguments": {"asset_kind": "docs", "version": 1, "idempotency_key": "ticket"},
        },
        {
            "name": "memory_asset_stage_begin",
            "arguments": {
                "asset_kind": "docs",
                "version": "1.0.0",
                "idempotency_key": "ticket",
                "principal": "other",
            },
        },
        {"name": "memory_asset_stage_status", "arguments": {}},
        {"name": "memory_asset_stage_status", "arguments": {"idempotency_key": "ticket"}},
        {"name": "memory_asset_stage_status", "arguments": {"idempotency_key": None}},
    ]
    requests += [
        {
            "name": "memory_asset_get",
            "arguments": {"id_or_path": "/public/a.md", "asset_kind": "docs"},
        },
        {
            "name": "memory_asset_get",
            "arguments": {
                "id_or_path": "12345678",
                "asset_kind": "docs",
                "offset": "2",
                "limit": "4",
                "version": "1.0.0",
                "expected_sha256": "abc",
            },
        },
        {
            "name": "memory_asset_get",
            "arguments": {
                "id_or_path": "12345678",
                "asset_kind": "docs",
                "limit": None,
                "view": "file",
                "file_path": "file.txt",
            },
        },
        {"name": "memory_asset_get", "arguments": {}},
    ]
    calls = []
    for params in requests:
        if params["name"] not in names:
            continue
        request = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params})
        response = asyncio.run(
            server.process_request_async(
                request,
                context=shared.MCPRequestContext(transport="streamable-http", principal="actor"),
            )
        )
        calls.append({"request": request, "expected": response})
    return {"definitions": definitions, "calls": calls}
