from __future__ import annotations

import json
from collections.abc import Mapping
from dataclasses import asdict
from inspect import Parameter, signature
from pathlib import Path
from typing import Any, Literal, cast, get_args, get_origin

from pydantic import BaseModel, ConfigDict

from memento.access import AccessStore
from memento.activity import ActivityClock
from memento.admin import AdminHTTPHandler
from memento.config import Principal
from memento.executor import (
    AnswerArgs,
    AssetGetArgs,
    AssetPruneArgs,
    AssetStageBeginArgs,
    AssetStageStatusArgs,
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
    ProposeAssetChange,
    ProposeCreateChange,
    ProposeFreeformArgs,
    ProposePatchChange,
    ProposeRenameChange,
    ProposeUpdateArgs,
    ReadArgs,
    RenameArgs,
    SearchArgs,
    execute_plan_schema,
)
from memento.graph_debug import GraphDebugHTTPHandler
from memento.graph_debug.refresh import GraphEmbeddingRefreshCoordinator
from memento.graph_debug.snapshot import GraphSnapshotService
from memento.mcp_registry import (
    OPERATION_SPEC_BY_OP,
    OPERATION_SPECS,
    WORKFLOW_TEMPLATES,
    OperationSpec,
    tool_names_for_surface,
)
from memento.service import MemoryService, ServiceContext
from memento.staging_http import AssetStagingHTTPHandler

try:  # pragma: no cover - optional runtime dependency
    from aioumcp import AsyncMCPServer
    from umcp_shared import (
        MCPHTTPResponse,
        MCPPrincipal,
        get_request_context,
    )
except ImportError:  # pragma: no cover - optional runtime dependency
    AsyncMCPServer = object
    MCPPrincipal = object

    def get_request_context() -> Any:
        raise RuntimeError("uMCP is not installed")


class _ProposeArgsSchema(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    intent: str
    base_revision: str
    changes: list[
        ProposeCreateChange | ProposePatchChange | ProposeRenameChange | ProposeAssetChange
    ]
    rationale: str | None = None


OperationName = Literal[
    "help",
    "status",
    "search",
    "read",
    "list",
    "graph",
    "audit",
    "answer",
    "route",
    "propose",
    "propose_freeform",
    "propose_update",
    "proposal_get",
    "proposal_list",
    "proposal_review",
    "proposal_apply",
    "asset_stage_begin",
    "asset_stage_status",
    "asset_get",
    "asset_prune",
    "create",
    "patch",
    "rename",
    "execute",
]
WorkflowGoal = Literal["inspect", "propose", "curate", "asset_pack"]


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
    "memory_asset_stage_begin": AssetStageBeginArgs,
    "memory_asset_stage_status": AssetStageStatusArgs,
    "memory_asset_get": AssetGetArgs,
    "memory_asset_prune": AssetPruneArgs,
    "memory_create": CreateArgs,
    "memory_patch": PatchArgs,
    "memory_rename": RenameArgs,
}


def execute_tool_schema() -> dict[str, Any]:
    plan_schema = execute_plan_schema()
    properties = dict(plan_schema.get("properties", {}))
    properties["plan"] = plan_schema
    schema: dict[str, Any] = {
        "type": "object",
        "properties": properties,
        "oneOf": [
            {
                "required": ["plan"],
                "not": {
                    "anyOf": [
                        {"required": ["operations"]},
                        {"required": ["stop_on_error"]},
                        {"required": ["returns"]},
                    ]
                },
            },
            {"required": ["operations"], "not": {"required": ["plan"]}},
        ],
        "additionalProperties": False,
    }
    if "$defs" in plan_schema:
        schema["$defs"] = plan_schema["$defs"]
    return schema


def normalize_execute_tool_arguments(
    *,
    plan: dict[str, Any] | None = None,
    operations: list[dict[str, Any]] | None = None,
    stop_on_error: bool = True,
    returns: list[dict[str, Any]] | None = None,
) -> dict[str, Any]:
    if plan is not None:
        if operations is not None or returns is not None or stop_on_error is not True:
            raise ValueError(
                "memory_execute accepts either plan or top-level plan fields, not both"
            )
        return plan
    if operations is None:
        raise ValueError("memory_execute requires plan or operations")
    return {
        "operations": operations,
        "stop_on_error": stop_on_error,
        "returns": returns or [],
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
        graph_snapshot_service: GraphSnapshotService | None = None,
        graph_refresh_coordinator: GraphEmbeddingRefreshCoordinator | None = None,
        access_store: AccessStore | None = None,
        activity: ActivityClock | None = None,
    ) -> None:
        self._umcp_log_file = log_file
        super().__init__()
        self._service = service
        self._bearer_tokens = dict(bearer_tokens)
        self._access_store = access_store
        self._activity = activity or ActivityClock()
        self._principals_by_name: dict[str, Principal] = {}
        for principal in self._bearer_tokens.values():
            if principal.name in self._principals_by_name:
                raise ValueError(
                    f"duplicate principal name configured for bearer tokens: {principal.name}"
                )
            self._principals_by_name[principal.name] = principal
        self._admin_http = AdminHTTPHandler(access_store)
        self._staging_http = (
            AssetStagingHTTPHandler(service._deps.staged_asset_store, self._authenticate_headers)
            if service._deps.staged_asset_store is not None
            else None
        )
        self._graph_debug_http = GraphDebugHTTPHandler(
            service._deps.config.observability.graph_explorer,
            snapshot_service=graph_snapshot_service,
            refresh_coordinator=graph_refresh_coordinator,
            authorization=service._deps.config.authorization,
            access_store=access_store,
        )

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
        if self._access_store is not None:
            message += (
                " Managed administrators receive direct access_* tools, not memory_execute "
                "operations. Keep the administrator bearer token out of ordinary agent runtimes, "
                "memory, chat and tool input or output. Use a separate admin profile for principal "
                "lifecycle; capture credentials returned once by create or rotate in a secret store."
            )
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
        if self._access_store is not None:
            try:
                principal = self._context().principal
            except RuntimeError:
                principal = None
            if principal is not None and "admin" in principal.roles:
                tools.extend(self._access_tools())
        return {"tools": tools}

    @staticmethod
    def _access_tools() -> list[dict[str, Any]]:
        object_schema = {"type": "object", "additionalProperties": False}
        entries: list[tuple[str, str, dict[str, Any]]] = [
            ("access_principal_list", "List managed principals.", {}),
            (
                "access_audit_list",
                "List recent access changes.",
                {"limit": {"type": "integer", "minimum": 1, "maximum": 100}},
            ),
            (
                "access_principal_create",
                "Create a least-privilege principal; return its credential once for immediate secret-store capture.",
                {
                    "name": {"type": "string"},
                    "roles": {"type": "array", "items": {"type": "string"}},
                    "read_prefixes": {"type": "array", "items": {"type": "string"}},
                    "write_prefixes": {"type": "array", "items": {"type": "string"}},
                    "idempotency_key": {"type": "string"},
                },
            ),
            (
                "access_principal_update",
                "Replace principal roles and namespaces; keep administration separate from routine curation.",
                {
                    "name": {"type": "string"},
                    "roles": {"type": "array", "items": {"type": "string"}},
                    "read_prefixes": {"type": "array", "items": {"type": "string"}},
                    "write_prefixes": {"type": "array", "items": {"type": "string"}},
                },
            ),
            (
                "access_principal_rename",
                "Rename a principal.",
                {"name": {"type": "string"}, "new_name": {"type": "string"}},
            ),
            ("access_principal_disable", "Disable a principal.", {"name": {"type": "string"}}),
            ("access_principal_enable", "Enable a principal.", {"name": {"type": "string"}}),
            (
                "access_credential_rotate",
                "Rotate a principal credential and return it once for immediate secret-store capture.",
                {"name": {"type": "string"}, "idempotency_key": {"type": "string"}},
            ),
            (
                "access_principal_revoke",
                "Revoke a principal credential.",
                {"name": {"type": "string"}},
            ),
            (
                "access_principal_delete",
                "Tombstone a disabled, revoked principal.",
                {"name": {"type": "string"}},
            ),
        ]
        tools = []
        for name, description, properties in entries:
            schema: dict[str, Any] = {**object_schema, "properties": properties}
            if name not in {"access_principal_list", "access_audit_list"}:
                schema["required"] = list(properties.keys())
            tools.append(
                {
                    "name": name,
                    "description": description,
                    "inputSchema": schema,
                    "annotations": {"roles": ["admin"], "operation": name.removeprefix("access_")},
                }
            )
        return tools

    def _tool_input_schema(self, method: Any, tool_name: str) -> dict[str, Any]:
        if tool_name == "memory_execute":
            return execute_tool_schema()
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

    def _authenticate_headers(self, headers: Mapping[str, str]) -> Principal | None:
        authorization = headers.get("authorization", "")
        if not authorization.startswith("Bearer "):
            return None
        token = authorization.removeprefix("Bearer ")
        return (
            self._access_store.authenticate(token)
            if self._access_store is not None
            else self._bearer_tokens.get(token)
        )

    def handle_http_request(
        self,
        *,
        method: str,
        path: str,
        headers: Mapping[str, str],
        body: bytes,
        peer: str | None,
    ) -> MCPHTTPResponse | None:
        self._activity.touch()
        if self._staging_http is not None:
            staging_response = self._staging_http.handle(
                method=method, path=path, headers=headers, body=body
            )
            if staging_response is not None:
                return staging_response
        admin_response = self._admin_http.handle(
            method=method, path=path, headers=headers, body=body
        )
        if admin_response is not None:
            return admin_response
        return self._graph_debug_http.handle(
            method=method,
            path=path,
            headers=headers,
            body=body,
            peer=peer,
        )

    def authenticate_request(
        self, *, method: str, path: str, headers: Mapping[str, str], peer: str | None
    ) -> MCPPrincipal | None:
        self._activity.touch()
        principal = self._authenticate_headers(headers)
        if principal is None:
            return None
        return MCPPrincipal(name=principal.name, roles=principal.roles, metadata=principal.metadata)

    def authorize_request(
        self, principal: MCPPrincipal | None, *, rpc_method: str | None, tool_name: str | None
    ) -> bool:
        if principal is None:
            return False
        if tool_name is not None and tool_name.startswith("access_"):
            return "admin" in principal.roles
        return True

    def _context(self) -> ServiceContext:
        self._activity.touch()
        request = get_request_context()
        principal = self._resolve_request_principal(getattr(request, "principal", None))
        return ServiceContext(
            principal=principal, mcp_session_id=getattr(request, "session_id", None)
        )

    def _resolve_request_principal(self, name: str | None) -> Principal:
        if name is None:
            raise RuntimeError("missing authenticated principal")
        principal = self._principals_by_name.get(name)
        if principal is None and self._access_store is not None:
            policy = self._access_store.policy(name)
            if policy is not None:
                principal = Principal(name=name, roles=policy.roles)
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
        query_syntax: str = "plain",
    ) -> dict[str, Any]:
        return self._service.memory_search(
            self._context(),
            query=query,
            concept_type=concept_type,
            limit=limit,
            cursor=cursor,
            search_mode=search_mode,
            query_syntax=query_syntax,
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

    async def tool_memory_asset_stage_begin(
        self, asset_kind: str, version: str, idempotency_key: str
    ) -> dict[str, Any]:
        context = self._context()
        if "proposer" not in context.principal.roles:
            return self._service._failure(ValueError("proposer role is required")).model_dump(
                mode="json"
            )
        store = self._service._deps.staged_asset_store
        if store is None:
            return self._service._failure(ValueError("asset staging is unavailable")).model_dump(
                mode="json"
            )
        try:
            ticket, raw_token = store.begin_upload(
                principal=context.principal.name,
                idempotency_key=idempotency_key,
                asset_kind=asset_kind,
                version=version,
            )
        except ValueError as exc:
            return self._service._failure(exc).model_dump(mode="json")
        return self._service._success(
            {
                "state": ticket.state,
                "asset_kind": ticket.asset_kind,
                "version": ticket.version,
                "idempotency_key": ticket.idempotency_key,
                "expires_at": ticket.expires_at,
                "upload_path": "/assets/staging/upload",
                "upload_method": "POST",
                "upload_content_type": "application/zip",
                "upload_ticket_header": "X-Memento-Upload-Ticket",
                "upload_ticket": raw_token,
                "workflow": "memory://workflow/asset_pack",
                "proposal_contract": "memory://catalog/propose",
            },
            next_tools=(
                "memory_asset_stage_status",
                "memory://workflow/asset_pack",
                "memory://catalog/propose",
                "memory_execute",
            ),
        ).model_dump(mode="json")

    async def tool_memory_asset_stage_status(self, idempotency_key: str) -> dict[str, Any]:
        context = self._context()
        if "proposer" not in context.principal.roles:
            return self._service._failure(ValueError("proposer role is required")).model_dump(
                mode="json"
            )
        store = self._service._deps.staged_asset_store
        if store is None:
            return self._service._failure(ValueError("asset staging is unavailable")).model_dump(
                mode="json"
            )
        try:
            ticket = store.ticket_status(
                principal=context.principal.name, idempotency_key=idempotency_key
            )
            payload: dict[str, Any] = {
                "state": ticket.state,
                "asset_kind": ticket.asset_kind,
                "version": ticket.version,
                "idempotency_key": ticket.idempotency_key,
                "expires_at": ticket.expires_at,
                "staged_asset_id": ticket.staged_asset_id,
                "consumed_at": ticket.consumed_at,
            }
            if ticket.staged_asset_id is not None:
                payload["staged_asset"] = store.get(
                    principal=context.principal.name,
                    staged_asset_id=ticket.staged_asset_id,
                ).public_payload()
        except ValueError as exc:
            return self._service._failure(exc).model_dump(mode="json")
        return self._service._success(
            payload,
            next_tools=(
                "memory_asset_stage_status",
                "memory://workflow/asset_pack",
                "memory://catalog/propose",
                "memory_execute",
            ),
        ).model_dump(mode="json")

    async def tool_memory_asset_get(
        self, id_or_path: str, asset_kind: str, version: str | None = None
    ) -> dict[str, Any]:
        return self._service.memory_asset_get(
            self._context(),
            id_or_path=id_or_path,
            asset_kind=asset_kind,
            version=version,
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

    async def tool_memory_execute(
        self,
        plan: dict[str, Any] | None = None,
        operations: list[dict[str, Any]] | None = None,
        stop_on_error: bool = True,
        returns: list[dict[str, Any]] | None = None,
    ) -> dict[str, Any]:
        try:
            normalized = normalize_execute_tool_arguments(
                plan=plan,
                operations=operations,
                stop_on_error=stop_on_error,
                returns=returns,
            )
        except ValueError as exc:
            return self._service._failure(exc).model_dump(mode="json")
        envelope = self._service.memory_execute(self._context(), plan=normalized)
        await self._notify_for_envelope(envelope.model_dump(mode="json"))
        return envelope.model_dump(mode="json")

    def _require_access_admin(self) -> Principal:
        principal = self._context().principal
        if self._access_store is None or "admin" not in principal.roles:
            raise RuntimeError("admin access is required")
        return principal

    def _access(self) -> AccessStore:
        if self._access_store is None:
            raise RuntimeError("access management is not configured")
        return self._access_store

    async def tool_access_principal_list(self) -> dict[str, Any]:
        self._require_access_admin()
        return {"principals": [asdict(item) for item in self._access().list()]}

    async def tool_access_audit_list(self, limit: int = 50) -> dict[str, Any]:
        self._require_access_admin()
        return {"events": list(self._access().audit(limit))}

    async def tool_access_principal_create(
        self,
        name: str,
        roles: list[str],
        read_prefixes: list[str],
        write_prefixes: list[str],
        idempotency_key: str,
    ) -> dict[str, Any]:
        actor = self._require_access_admin()
        principal, token = self._access().create(
            actor=actor.name,
            name=name,
            roles=tuple(roles),
            read_prefixes=tuple(read_prefixes),
            write_prefixes=tuple(write_prefixes),
            idempotency_key=idempotency_key,
        )
        return {"principal": asdict(principal), "credential": token}

    async def tool_access_principal_update(
        self, name: str, roles: list[str], read_prefixes: list[str], write_prefixes: list[str]
    ) -> dict[str, Any]:
        actor = self._require_access_admin()
        item = self._access().update(
            actor=actor.name,
            name=name,
            roles=tuple(roles),
            read_prefixes=tuple(read_prefixes),
            write_prefixes=tuple(write_prefixes),
        )
        return {"principal": asdict(item)}

    async def tool_access_principal_rename(self, name: str, new_name: str) -> dict[str, Any]:
        actor = self._require_access_admin()
        item = self._access().rename(actor=actor.name, name=name, new_name=new_name)
        return {"principal": asdict(item)}

    async def tool_access_principal_disable(self, name: str) -> dict[str, Any]:
        actor = self._require_access_admin()
        item = self._access().set_enabled(actor=actor.name, name=name, enabled=False)
        return {"principal": asdict(item)}

    async def tool_access_principal_enable(self, name: str) -> dict[str, Any]:
        actor = self._require_access_admin()
        item = self._access().set_enabled(actor=actor.name, name=name, enabled=True)
        return {"principal": asdict(item)}

    async def tool_access_credential_rotate(
        self, name: str, idempotency_key: str
    ) -> dict[str, Any]:
        actor = self._require_access_admin()
        return {
            "name": name,
            "credential": self._access().rotate(
                actor=actor.name, name=name, idempotency_key=idempotency_key
            ),
        }

    async def tool_access_principal_revoke(self, name: str) -> dict[str, Any]:
        actor = self._require_access_admin()
        item = self._access().revoke(actor=actor.name, name=name)
        return {"principal": asdict(item)}

    async def tool_access_principal_delete(self, name: str) -> dict[str, Any]:
        actor = self._require_access_admin()
        item = self._access().delete(actor=actor.name, name=name)
        return {"principal": asdict(item)}

    async def prompt_publish_asset_pack(
        self,
        target_path: str = "/skills/example.md",
        asset_kind: str = "skill",
        version: str = "1.0.0",
    ) -> str:
        """Build an MCP-native asset publication plan. Categories: assets, proposals"""
        return "\n".join(
            (
                "Publish and verify a versioned asset pack with one authenticated curator profile.",
                f"Target concept: {target_path}",
                f"Asset kind: {asset_kind}",
                f"Version: {version}",
                "1. Read memory://workflow/asset_pack and memory://catalog/propose.",
                "2. For a skill pack, use canonical UTF-8/LF text with no trailing whitespace or final newline; make the concept body and ZIP-root SKILL.md byte-identical, then base64-encode the ZIP.",
                "3. Submit a propose operation (directly or through memory_execute) using the current repository revision and this change:",
                json.dumps(
                    {
                        "kind": "attach_asset_pack",
                        "path": target_path,
                        "asset_kind": asset_kind,
                        "version": version,
                        "zip_base64": "<base64 ZIP bytes>",
                    },
                    sort_keys=True,
                ),
                "4. Read the proposal and verify its generated manifest, SHA-256, target path, asset kind, and version.",
                "5. With the same authenticated curator profile, approve and apply using a fresh expected revision and durable idempotency key.",
                "6. Retrieve the accepted version with memory_asset_get and verify the returned manifest, SHA-256, and decoded ZIP bytes.",
                "Use memory_asset_stage_begin, raw HTTP upload, memory_asset_stage_status, and staged_asset_id only when the MCP request would exceed the configured ceiling or the client deliberately uses raw binary HTTP upload.",
            )
        )

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

    async def resource_template_catalog(self, operation: OperationName) -> dict[str, Any]:
        """Read one operation contract. Values include propose, asset_stage_begin, and asset_stage_status."""
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

    async def resource_template_workflow(self, goal: WorkflowGoal) -> dict[str, Any]:
        """Read one workflow. Valid goals: inspect, propose, curate, asset_pack."""
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
        payload = {
            "goal": goal,
            "uri": f"memory://workflow/{goal}",
            "description": meta["description"],
            "operations": direct,
            "execute_only_operations": execute_only,
        }
        for key in ("profile", "steps", "staging_fallback"):
            if key in meta:
                payload[key] = meta[key]
        return payload

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
