from __future__ import annotations

import json
from collections.abc import Mapping
from inspect import Parameter, signature
from pathlib import Path
from typing import Any, cast, get_args, get_origin

from pydantic import BaseModel, ConfigDict

from memento.config import Principal
from memento.executor import (
    AnswerArgs,
    AssetGetArgs,
    AssetPruneArgs,
    AuditArgs,
    CreateArgs,
    EmptyArgs,
    GraphArgs,
    ListArgs,
    PatchArgs,
    ProposalApplyArgs,
    ProposalGetArgs,
    ProposalListArgs,
    ProposalReviewArgs,
    ProposeFreeformArgs,
    ProposeUpdateArgs,
    ReadArgs,
    RenameArgs,
    SearchArgs,
    SkillGetArgs,
    SkillProposeArgs,
    SkillPruneArgs,
    execute_plan_schema,
)
from memento.mcp_registry import (
    OPERATION_SPEC_BY_OP,
    OPERATION_SPECS,
    WORKFLOW_TEMPLATES,
    OperationSpec,
    tool_names_for_surface,
)
from memento.service import (
    CreateChange,
    MemoryService,
    PatchChange,
    RenameChange,
    ServiceContext,
)

try:  # pragma: no cover - optional runtime dependency
    from aioumcp import AsyncMCPServer  # type: ignore[import-not-found]
    from umcp_shared import MCPPrincipal, get_request_context  # type: ignore[import-not-found]
except ImportError:  # pragma: no cover - optional runtime dependency
    AsyncMCPServer = object
    MCPPrincipal = object

    def get_request_context() -> Any:
        raise RuntimeError("uMCP is not installed")


class _ProposeArgsSchema(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    intent: str
    base_revision: str
    changes: list[CreateChange | PatchChange | RenameChange]
    rationale: str | None = None


class RouteArgs(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    request: str
    execute: bool = True


_TOOL_ARG_MODELS: dict[str, type[BaseModel]] = {
    "memory_help": EmptyArgs,
    "memory_status": EmptyArgs,
    "memory_search": SearchArgs,
    "memory_read": ReadArgs,
    "memory_list": ListArgs,
    "memory_graph": GraphArgs,
    "memory_audit": AuditArgs,
    "memory_answer": AnswerArgs,
    "memory_route": RouteArgs,
    "memory_propose": _ProposeArgsSchema,
    "memory_propose_freeform": ProposeFreeformArgs,
    "memory_propose_update": ProposeUpdateArgs,
    "memory_proposal_get": ProposalGetArgs,
    "memory_proposal_list": ProposalListArgs,
    "memory_proposal_review": ProposalReviewArgs,
    "memory_proposal_apply": ProposalApplyArgs,
    "memory_asset_get": AssetGetArgs,
    "memory_skill_get": SkillGetArgs,
    "memory_skill_propose": SkillProposeArgs,
    "memory_asset_prune": AssetPruneArgs,
    "memory_skill_prune": SkillPruneArgs,
    "memory_create": CreateArgs,
    "memory_patch": PatchArgs,
    "memory_rename": RenameArgs,
}


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


EXECUTE_CAPABLE_OPERATIONS = frozenset(
    {
        "help",
        "status",
        "search",
        "read",
        "list",
        "graph",
        "audit",
        "answer",
        "propose",
        "propose_freeform",
        "propose_update",
        "proposal_get",
        "proposal_list",
        "proposal_review",
        "proposal_apply",
        "create",
        "patch",
        "rename",
    }
)


class MementoMCPServer(AsyncMCPServer):  # type: ignore[misc]
    def __init__(
        self,
        service: MemoryService,
        *,
        bearer_tokens: Mapping[str, Principal],
        log_file: Path | None = None,
    ) -> None:
        self._umcp_log_file = log_file
        super().__init__()
        self._service = service
        self._bearer_tokens = dict(bearer_tokens)
        self._principals_by_name: dict[str, Principal] = {}
        for principal in self._bearer_tokens.values():
            if principal.name in self._principals_by_name:
                raise ValueError(
                    f"duplicate principal name configured for bearer tokens: {principal.name}"
                )
            self._principals_by_name[principal.name] = principal

    def _setup_logging(self) -> None:
        if self._umcp_log_file is not None:
            self.log_file = self._umcp_log_file
        super()._setup_logging()

    def get_instructions(self) -> str:
        visible_specs = self._visible_operation_specs()
        visible_tools = ", ".join(spec.tool_name for spec in visible_specs)
        message = (
            "Deterministic shared memory service backed by Git Markdown. "
            f"Configured tool surface: {self._service._deps.config.mcp.tool_surface}. "
            f"Direct tools: {visible_tools}. See memory://catalog and memory://workflow/inspect."
        )
        if self._execute_tool_available():
            message += " memory_execute can compose additional execute-only operations listed in the catalog."
        return message

    def discover_tools(self) -> dict[str, Any]:
        tools: list[dict[str, Any]] = []
        for spec in self._visible_operation_specs():
            method = getattr(self, f"tool_{spec.tool_name}")
            tools.append(
                {
                    "name": spec.tool_name,
                    "description": spec.description,
                    "inputSchema": self._tool_input_schema(method, spec.tool_name),
                    "annotations": {"roles": list(spec.roles), "operation": spec.op_name},
                }
            )
        return {"tools": tools}

    def _tool_input_schema(self, method: Any, tool_name: str) -> dict[str, Any]:
        if tool_name == "memory_execute":
            return execute_plan_schema()
        model = _TOOL_ARG_MODELS.get(tool_name)
        if model is not None:
            generated = model.model_json_schema()
            return {str(key): value for key, value in generated.items()}
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
        principal = self._principals_by_name.get(name)
        if principal is None:
            raise RuntimeError(f"unknown request principal: {name}")
        return principal

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

    async def tool_memory_route(self, request: str, execute: bool = True) -> dict[str, Any]:
        return self._service.memory_route(
            self._context(), request=request, execute=execute
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

    async def tool_memory_asset_get(
        self, id_or_path: str, asset_kind: str, version: str | None = None
    ) -> dict[str, Any]:
        return self._service.memory_asset_get(
            self._context(),
            id_or_path=id_or_path,
            asset_kind=asset_kind,
            version=version,
        ).model_dump(mode="json")

    async def tool_memory_skill_get(
        self, skill_name: str, version: str | None = None
    ) -> dict[str, Any]:
        return self._service.memory_asset_get(
            self._context(),
            id_or_path=skill_name,
            asset_kind="skill",
            version=version,
        ).model_dump(mode="json")

    async def tool_memory_skill_propose(
        self,
        skill_name: str,
        version: str,
        skill_md: str,
        zip_base64: str,
        rationale: str | None = None,
    ) -> dict[str, Any]:
        return self._service.memory_skill_propose(
            self._context(),
            skill_name=skill_name,
            version=version,
            skill_md=skill_md,
            zip_base64=zip_base64,
            rationale=rationale,
        ).model_dump(mode="json")

    async def tool_memory_asset_prune(
        self,
        id_or_path: str,
        asset_kind: str,
        *,
        keep: int = 5,
        expected_revision: str,
        idempotency_key: str,
    ) -> dict[str, Any]:
        envelope = self._service.memory_asset_prune(
            self._context(),
            id_or_path=id_or_path,
            asset_kind=asset_kind,
            keep=keep,
            expected_revision=expected_revision,
            idempotency_key=idempotency_key,
        )
        await self._notify_for_envelope(envelope.model_dump(mode="json"))
        return envelope.model_dump(mode="json")

    async def tool_memory_skill_prune(
        self,
        skill_name: str,
        *,
        keep: int = 5,
        expected_revision: str,
        idempotency_key: str,
    ) -> dict[str, Any]:
        envelope = self._service.memory_asset_prune(
            self._context(),
            id_or_path=skill_name,
            asset_kind="skill",
            keep=keep,
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
        payload: dict[str, Any] = {
            "tool_surface": self._service._deps.config.mcp.tool_surface,
            "operations": [
                self._catalog_operation(spec.op_name, direct_tool_available=True)
                for spec in self._visible_operation_specs()
            ],
            "workflows": {goal: self._workflow_payload(goal) for goal in WORKFLOW_TEMPLATES},
        }
        execute_only = self._execute_only_specs()
        if execute_only:
            payload["execute_only_operations"] = [
                self._catalog_operation(spec.op_name, direct_tool_available=False)
                for spec in execute_only
            ]
        return {"mimeType": "application/json", "text": json.dumps(payload, sort_keys=True)}

    async def resource_template_catalog(self, operation: str) -> dict[str, Any]:
        spec = OPERATION_SPEC_BY_OP.get(operation)
        if spec is None:
            raise RuntimeError(f"unknown operation: {operation}")
        return {
            "mimeType": "application/json",
            "text": json.dumps(
                self._catalog_operation(
                    operation,
                    direct_tool_available=spec in self._visible_operation_specs(),
                ),
                sort_keys=True,
            ),
        }

    async def resource_template_workflow(self, goal: str) -> dict[str, Any]:
        payload = self._workflow_payload(goal)
        return {"mimeType": "application/json", "text": json.dumps(payload, sort_keys=True)}

    def _catalog_operation(self, operation: str, *, direct_tool_available: bool) -> dict[str, Any]:
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
            "direct_tool_available": direct_tool_available,
            "available_via_execute": (
                not direct_tool_available
                and self._execute_tool_available()
                and spec.op_name in EXECUTE_CAPABLE_OPERATIONS
            ),
            "examples": list(spec.examples),
            "input_schema": self._tool_input_schema(method, spec.tool_name),
        }

    def _visible_operation_specs(self) -> tuple[OperationSpec, ...]:
        answer_enabled = (
            self._service._deps.config.mcp.compact_answer_enabled
            and self._service._deps.config.intelligent_tiers.deep_answers.enabled
        )
        names = set(
            tool_names_for_surface(
                self._service._deps.config.mcp.tool_surface,
                answer_enabled=answer_enabled,
                route_enabled=self._service._route_tool_enabled(),
            )
        )
        return tuple(spec for spec in OPERATION_SPECS if spec.tool_name in names)

    def _execute_tool_available(self) -> bool:
        return any(spec.op_name == "execute" for spec in self._visible_operation_specs())

    def _execute_only_specs(self) -> tuple[OperationSpec, ...]:
        if not self._execute_tool_available():
            return ()
        visible = {spec.op_name for spec in self._visible_operation_specs()}
        return tuple(
            spec
            for spec in OPERATION_SPECS
            if spec.op_name not in visible
            and spec.op_name != "execute"
            and spec.op_name in EXECUTE_CAPABLE_OPERATIONS
        )

    def _workflow_payload(self, goal: str) -> dict[str, Any]:
        meta = WORKFLOW_TEMPLATES.get(goal)
        if meta is None:
            raise RuntimeError(f"unknown workflow: {goal}")
        visible = {spec.op_name for spec in self._visible_operation_specs()}
        direct = [
            self._catalog_operation(name, direct_tool_available=True)
            for name in meta["operations"]
            if name in visible
        ]
        execute_only = []
        if self._execute_tool_available():
            execute_only = [
                self._catalog_operation(name, direct_tool_available=False)
                for name in meta["operations"]
                if name not in visible
            ]
        return {
            "goal": goal,
            "uri": f"memory://workflow/{goal}",
            "description": meta["description"],
            "operations": direct,
            "execute_only_operations": execute_only,
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
