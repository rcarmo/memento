"""Configured catalog/help/workflow contracts, resource metadata and prompt text."""

from __future__ import annotations

import asyncio
import importlib
import inspect
import json
from types import SimpleNamespace
from typing import Any, get_args
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.server")
    service: Any = importlib.import_module("memento.service")
    registry: Any = importlib.import_module("memento.mcp_registry")
    executor: Any = importlib.import_module("memento.executor")
    umcp: Any = importlib.import_module("aioumcp")
    server = object.__new__(module.MementoMCPServer)
    server._umcp_log_file = None
    umcp.AsyncMCPServer.__init__(server)
    server._principals_by_name = {}
    server._access_store = None
    memory = object.__new__(service.MemoryService)
    memory._policy = lambda _: "trusted"
    config: Any = None
    memory._route_tool_enabled = lambda: config.route_enabled
    server._service = memory
    server._context = lambda: "trusted"
    limits = executor.ExecuteLimits().model_dump(mode="json")
    contracts = {
        spec.op_name: server._catalog_operation(spec.op_name, direct_tool_available=True)
        for spec in registry.OPERATION_SPECS
    }
    cases = []
    for surface in ("compact", "standard", "read_only", "curator", "admin"):
        for answer in (False, True):
            for route in (False, True):
                config = SimpleNamespace(
                    route_enabled=route,
                    mcp=SimpleNamespace(
                        tool_surface=surface,
                        compact_answer_enabled=answer,
                        execute=executor.ExecuteLimits(),
                    ),
                    intelligent_tiers=SimpleNamespace(deep_answers=SimpleNamespace(enabled=answer)),
                )
                memory._deps = SimpleNamespace(config=config, repo_paths=None)
                with patch.object(service, "get_main_revision", lambda _: "main"):
                    help_payload = memory.memory_help(None).model_dump(mode="json")
                catalog = json.loads(asyncio.run(server.resource_catalog())["text"])
                cases.append(
                    {
                        "surface": surface,
                        "answer": answer,
                        "route": route,
                        "catalog": catalog,
                        "help": help_payload,
                        "tools": server.discover_tools()["tools"],
                    }
                )
    # Fix config above for individual hidden-operation contracts.
    config.mcp.tool_surface = "read_only"
    hidden = json.loads(asyncio.run(server.resource_template_catalog("propose"))["text"])
    protocol = []
    memory.memory_status = lambda _: SimpleNamespace(model_dump=lambda **_: {"test_only": "status"})
    for method, params in [
        ("resources/list", {}),
        ("resources/templates/list", {}),
        ("prompts/list", {}),
        ("resources/read", {"uri": "memory://catalog"}),
        ("resources/read", {"uri": "memory://help"}),
        ("resources/read", {"uri": "memory://status"}),
        ("resources/read", {"uri": "memory://catalog/propose"}),
        ("resources/read", {"uri": "memory://catalog/invalid"}),
        ("resources/read", {"uri": "memory://workflow/asset_pack"}),
        ("resources/read", {"uri": "memory://workflow/trash"}),
        ("resources/read", {"uri": "memory://workflow/bad"}),
        ("prompts/get", {"name": "publish_asset_pack"}),
        (
            "completion/complete",
            {
                "ref": {"type": "ref/resource", "uri": "memory://catalog/{operation}"},
                "argument": {"name": "operation", "value": "asset"},
            },
        ),
        (
            "completion/complete",
            {
                "ref": {"type": "ref/resource", "uri": "memory://workflow/{goal}"},
                "argument": {"name": "goal", "value": ""},
            },
        ),
    ]:
        request = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
        with patch.object(service, "get_main_revision", lambda _: "main"):
            response = asyncio.run(
                server.process_request_async(
                    request,
                    context=importlib.import_module("umcp_shared").MCPRequestContext(
                        transport="streamable-http", principal="actor"
                    ),
                )
            )
        protocol.append({"request": request, "expected": response})

    # Hidden tools are still callable through getattr; discovery is not an ACL.
    async def hidden_call(method: str, context: Any, **arguments: Any) -> Any:
        return SimpleNamespace(model_dump=lambda **_: {"test_only": True})

    server._memory_call = hidden_call
    request = json.dumps(
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tools/call",
            "params": {
                "name": "memory_propose",
                "arguments": {"intent": "test", "base_revision": "main", "changes": []},
            },
        }
    )
    response = asyncio.run(
        server.process_request_async(
            request,
            context=importlib.import_module("umcp_shared").MCPRequestContext(
                transport="streamable-http", principal="actor"
            ),
        )
    )
    protocol.append({"request": request, "expected": response})
    prompt_cases = []
    for args in [{}, {"target_path": "/skills/café.md", "asset_kind": "docs", "version": "2.0.0"}]:
        prompt_cases.append(
            {"arguments": args, "expected": asyncio.run(server.prompt_publish_asset_pack(**args))}
        )
    return {
        "protocol": protocol,
        "operation_values": get_args(module.OperationName),
        "workflow_values": get_args(module.WorkflowGoal),
        "contracts": contracts,
        "workflows": registry.WORKFLOW_TEMPLATES,
        "execute_capable": sorted(module.EXECUTE_CAPABLE_OPERATIONS),
        "limits": limits,
        "cases": cases,
        "hidden": hidden,
        "resources": server.discover_resources()["resources"],
        "templates": server.discover_resource_templates()["resourceTemplates"],
        "prompts": server.discover_prompts()["prompts"],
        "prompt_cases": prompt_cases,
        "prompt_description": inspect.getdoc(server.prompt_publish_asset_pack),
    }
