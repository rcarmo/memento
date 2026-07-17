from __future__ import annotations

import json
from collections.abc import Mapping
from inspect import Parameter, signature
from typing import Any, cast, get_args, get_origin

from memento.config import Principal
from memento.executor import execute_plan_schema
from memento.mcp_registry import (
    OPERATION_SPEC_BY_OP,
    OPERATION_SPECS,
    WORKFLOW_TEMPLATES,
    tool_names_for_surface,
)
from memento.service import MemoryService, ServiceContext

try:  # pragma: no cover - optional runtime dependency
    from aioumcp import AsyncMCPServer  # type: ignore[import-not-found]
    from umcp_shared import MCPPrincipal, get_request_context  # type: ignore[import-not-found]
except ImportError:  # pragma: no cover - optional runtime dependency
    AsyncMCPServer = object
    MCPPrincipal = object

    def get_request_context() -> Any:
        raise RuntimeError("uMCP is not installed")


def _annotation_schema(annotation: Any) -> dict[str, Any]:
    if annotation is Parameter.empty or annotation is Any:
        return {}
    origin = get_origin(annotation)
    args = get_args(annotation)
    if origin is list or origin is tuple:
        return {"type": "array", "items": _annotation_schema(args[0]) if args else {}}
    if origin is not None and type(None) in args:
        concrete = [item for item in args if item is not type(None)]
        return _annotation_schema(concrete[0]) if len(concrete) == 1 else {}
    return {
        str: {"type": "string"},
        int: {"type": "integer"},
        float: {"type": "number"},
        bool: {"type": "boolean"},
        dict: {"type": "object"},
    }.get(annotation, {})


class MementoMCPServer(AsyncMCPServer):  # type: ignore[misc]
    def __init__(self, service: MemoryService, *, bearer_tokens: Mapping[str, Principal]) -> None:
        super().__init__()
        self._service = service
        self._bearer_tokens = dict(bearer_tokens)

    def get_instructions(self) -> str:
        compact = "memory_help, memory_status, memory_search, memory_read, memory_execute"
        return (
            "Deterministic shared memory service backed by Git Markdown. "
            f"Configured tool surface: {self._service._deps.config.mcp.tool_surface}. "
            f"Compact workflow: {compact}. See memory://catalog and memory://workflow/inspect."
        )

    def discover_tools(self) -> dict[str, Any]:
        answer_enabled = (
            self._service._deps.config.mcp.compact_answer_enabled
            and self._service._deps.config.intelligent_tiers.deep_answers.enabled
        )
        names = set(
            tool_names_for_surface(
                self._service._deps.config.mcp.tool_surface,
                answer_enabled=answer_enabled,
            )
        )
        tools: list[dict[str, Any]] = []
        for spec in OPERATION_SPECS:
            if spec.tool_name not in names:
                continue
            method = getattr(self, f"tool_{spec.tool_name}")
            tool_def = {
                "name": spec.tool_name,
                "description": spec.description,
                "inputSchema": execute_plan_schema()
                if spec.tool_name == "memory_execute"
                else self._tool_input_schema(method),
                "annotations": {"roles": list(spec.roles), "operation": spec.op_name},
            }
            tools.append(tool_def)
        return {"tools": tools}

    def _tool_input_schema(self, method: Any) -> dict[str, Any]:
        extractor = getattr(self, "_extract_parameters_from_signature", None)
        if extractor is not None:
            extracted = extractor(signature(method), method)
            if isinstance(extracted, dict) and extracted:
                return {str(key): value for key, value in extracted.items()}
        properties: dict[str, Any] = {}
        required: list[str] = []
        for name, parameter in signature(method).parameters.items():
            properties[name] = _annotation_schema(parameter.annotation)
            if parameter.default is Parameter.empty:
                required.append(name)
        fallback: dict[str, Any] = {
            "type": "object",
            "properties": properties,
            "additionalProperties": False,
        }
        if required:
            fallback["required"] = required
        return fallback

    def authenticate_request(
        self, *, method: str, path: str, headers: Mapping[str, str], peer: str | None
    ) -> MCPPrincipal | None:
        authorization = headers.get("authorization", "")
        if not authorization.startswith("Bearer "):
            return None
        token = authorization.removeprefix("Bearer ")
        principal = self._bearer_tokens.get(token)
        if principal is None:
            return None
        return MCPPrincipal(name=principal.name, roles=principal.roles, metadata=principal.metadata)

    def authorize_request(
        self, principal: MCPPrincipal | None, *, rpc_method: str | None, tool_name: str | None
    ) -> bool:
        return principal is not None

    def _context(self) -> ServiceContext:
        request = get_request_context()
        principal = self._resolve_request_principal(getattr(request, "principal", None))
        return ServiceContext(
            principal=principal, mcp_session_id=getattr(request, "session_id", None)
        )

    def _resolve_request_principal(self, name: str | None) -> Principal:
        if name is None:
            raise RuntimeError("missing authenticated principal")
        for principal in self._bearer_tokens.values():
            if principal.name == name:
                return principal
        raise RuntimeError(f"unknown request principal: {name}")

    async def tool_memory_help(self) -> dict[str, Any]:
        return self._service.memory_help(self._context()).model_dump(mode="json")

    async def tool_memory_status(self) -> dict[str, Any]:
        return self._service.memory_status(self._context()).model_dump(mode="json")

    async def tool_memory_search(
        self,
        query: str,
        concept_type: str | None = None,
        limit: int = 20,
        cursor: str | None = None,
        search_mode: str | None = None,
    ) -> dict[str, Any]:
        return self._service.memory_search(
            self._context(),
            query=query,
            concept_type=concept_type,
            limit=limit,
            cursor=cursor,
            search_mode=search_mode,
        ).model_dump(mode="json")

    async def tool_memory_read(self, id_or_path: str) -> dict[str, Any]:
        return self._service.memory_read(self._context(), id_or_path=id_or_path).model_dump(
            mode="json"
        )

    async def tool_memory_list(self, path_prefix: str = "/") -> dict[str, Any]:
        return self._service.memory_list(self._context(), path_prefix=path_prefix).model_dump(
            mode="json"
        )

    async def tool_memory_graph(self, id_or_path: str, depth: int = 1) -> dict[str, Any]:
        return self._service.memory_graph(
            self._context(), id_or_path=id_or_path, depth=depth
        ).model_dump(mode="json")

    async def tool_memory_audit(self, path: str | None = None) -> dict[str, Any]:
        return self._service.memory_audit(self._context(), path=path).model_dump(mode="json")

    async def tool_memory_answer(
        self, question: str, answer_mode: str = "summary"
    ) -> dict[str, Any]:
        return self._service.memory_answer(
            self._context(), question=question, answer_mode=answer_mode
        ).model_dump(mode="json")

    async def tool_memory_propose(
        self,
        intent: str,
        base_revision: str,
        changes: list[dict[str, Any]],
        rationale: str | None = None,
    ) -> dict[str, Any]:
        return self._service.memory_propose(
            self._context(),
            intent=intent,
            base_revision=base_revision,
            changes=changes,
            rationale=rationale,
        ).model_dump(mode="json")

    async def tool_memory_propose_freeform(
        self, content: str, suggested_path: str | None = None, intent: str | None = None
    ) -> dict[str, Any]:
        return self._service.memory_propose_freeform(
            self._context(), content=content, suggested_path=suggested_path, intent=intent
        ).model_dump(mode="json")

    async def tool_memory_propose_update(
        self, instruction: str, target_hint: str | None = None
    ) -> dict[str, Any]:
        return self._service.memory_propose_update(
            self._context(), instruction=instruction, target_hint=target_hint
        ).model_dump(mode="json")

    async def tool_memory_proposal_get(self, proposal_id: str) -> dict[str, Any]:
        return self._service.memory_proposal_get(
            self._context(), proposal_id=proposal_id
        ).model_dump(mode="json")

    async def tool_memory_proposal_list(self, status: str | None = None) -> dict[str, Any]:
        return self._service.memory_proposal_list(self._context(), status=status).model_dump(
            mode="json"
        )

    async def tool_memory_proposal_review(
        self, proposal_id: str, decision: str, comment: str | None = None
    ) -> dict[str, Any]:
        return self._service.memory_proposal_review(
            self._context(), proposal_id=proposal_id, decision=decision, comment=comment
        ).model_dump(mode="json")

    async def tool_memory_proposal_apply(
        self, proposal_id: str, expected_revision: str, idempotency_key: str
    ) -> dict[str, Any]:
        envelope = self._service.memory_proposal_apply(
            self._context(),
            proposal_id=proposal_id,
            expected_revision=expected_revision,
            idempotency_key=idempotency_key,
        )
        await self._notify_for_envelope(envelope.model_dump(mode="json"))
        return envelope.model_dump(mode="json")

    async def tool_memory_create(
        self,
        path: str,
        concept_type: str,
        title: str,
        body: str,
        expected_revision: str,
        idempotency_key: str,
        description: str | None = None,
        tags: tuple[str, ...] = (),
        aliases: tuple[str, ...] = (),
    ) -> dict[str, Any]:
        envelope = self._service.memory_create(
            self._context(),
            path=path,
            concept_type=concept_type,
            title=title,
            body=body,
            expected_revision=expected_revision,
            idempotency_key=idempotency_key,
            description=description,
            tags=tags,
            aliases=aliases,
        )
        await self._notify_for_envelope(envelope.model_dump(mode="json"))
        return envelope.model_dump(mode="json")

    async def tool_memory_patch(
        self,
        path: str,
        expected_revision: str,
        idempotency_key: str,
        title: str | None = None,
        description: str | None = None,
        body: str | None = None,
        status: str | None = None,
        tags: tuple[str, ...] | None = None,
        aliases: tuple[str, ...] | None = None,
    ) -> dict[str, Any]:
        envelope = self._service.memory_patch(
            self._context(),
            path=path,
            expected_revision=expected_revision,
            idempotency_key=idempotency_key,
            title=title,
            description=description,
            body=body,
            status=status,
            tags=tags,
            aliases=aliases,
        )
        await self._notify_for_envelope(envelope.model_dump(mode="json"))
        return envelope.model_dump(mode="json")

    async def tool_memory_rename(
        self, path: str, new_path: str, expected_revision: str, idempotency_key: str
    ) -> dict[str, Any]:
        envelope = self._service.memory_rename(
            self._context(),
            path=path,
            new_path=new_path,
            expected_revision=expected_revision,
            idempotency_key=idempotency_key,
        )
        await self._notify_for_envelope(envelope.model_dump(mode="json"))
        return envelope.model_dump(mode="json")

    async def tool_memory_execute(self, plan: dict[str, Any]) -> dict[str, Any]:
        envelope = self._service.memory_execute(self._context(), plan=plan)
        await self._notify_for_envelope(envelope.model_dump(mode="json"))
        return envelope.model_dump(mode="json")

    async def resource_status(self) -> dict[str, Any]:
        payload = self._service.memory_status(self._context()).model_dump(mode="json")
        return {"mimeType": "application/json", "text": json.dumps(payload, sort_keys=True)}

    async def resource_help(self) -> dict[str, Any]:
        payload = self._service.memory_help(self._context()).model_dump(mode="json")
        return {"mimeType": "application/json", "text": json.dumps(payload, sort_keys=True)}

    async def resource_catalog(self) -> dict[str, Any]:
        payload = {
            "tool_surface": self._service._deps.config.mcp.tool_surface,
            "operations": [self._catalog_operation(spec.op_name) for spec in OPERATION_SPECS],
            "workflows": {
                goal: {
                    "uri": f"memory://workflow/{goal}",
                    "description": meta["description"],
                    "operations": meta["operations"],
                }
                for goal, meta in WORKFLOW_TEMPLATES.items()
            },
        }
        return {"mimeType": "application/json", "text": json.dumps(payload, sort_keys=True)}

    async def resource_template_catalog(self, operation: str) -> dict[str, Any]:
        return {
            "mimeType": "application/json",
            "text": json.dumps(self._catalog_operation(operation), sort_keys=True),
        }

    async def resource_template_workflow(self, goal: str) -> dict[str, Any]:
        meta = WORKFLOW_TEMPLATES.get(goal)
        if meta is None:
            raise RuntimeError(f"unknown workflow: {goal}")
        payload = {
            "goal": goal,
            "description": meta["description"],
            "operations": [self._catalog_operation(name) for name in meta["operations"]],
        }
        return {"mimeType": "application/json", "text": json.dumps(payload, sort_keys=True)}

    def _catalog_operation(self, operation: str) -> dict[str, Any]:
        spec = OPERATION_SPEC_BY_OP.get(operation)
        if spec is None:
            raise RuntimeError(f"unknown operation: {operation}")
        method = getattr(self, f"tool_{spec.tool_name}")
        return {
            "operation": spec.op_name,
            "tool": spec.tool_name,
            "description": spec.description,
            "roles": list(spec.roles),
            "commit_capable": spec.commit_capable,
            "examples": list(spec.examples),
            "input_schema": execute_plan_schema()
            if spec.tool_name == "memory_execute"
            else self._extract_parameters_from_signature(signature(method), method)
            or {"type": "object", "properties": {}, "additionalProperties": False},
        }

    async def _notify_for_envelope(self, envelope: Mapping[str, Any]) -> None:
        if envelope.get("status") != "success":
            return
        data = envelope.get("data")
        if not isinstance(data, Mapping):
            return
        changed_paths = data.get("changed_paths")
        revisions = data.get("revisions")
        if changed_paths or any(
            item.get("operation_id") for item in revisions or [] if isinstance(item, Mapping)
        ):
            await self.notify_resource_list_changed()
        await self.notify_resource_updated("memory://status")


cast(Any, MementoMCPServer.resource_status)._mcp_resource = {
    "uri": "memory://status",
    "title": "Service status",
    "mime_type": "application/json",
}
cast(Any, MementoMCPServer.resource_help)._mcp_resource = {
    "uri": "memory://help",
    "title": "Service help",
    "mime_type": "application/json",
}
cast(Any, MementoMCPServer.resource_catalog)._mcp_resource = {
    "uri": "memory://catalog",
    "title": "Operation catalog",
    "mime_type": "application/json",
}
cast(Any, MementoMCPServer.resource_template_catalog)._mcp_resource_template = {
    "uri_template": "memory://catalog/{operation}",
    "title": "Operation catalog entry",
    "mime_type": "application/json",
}
cast(Any, MementoMCPServer.resource_template_workflow)._mcp_resource_template = {
    "uri_template": "memory://workflow/{goal}",
    "title": "Workflow template",
    "mime_type": "application/json",
}
