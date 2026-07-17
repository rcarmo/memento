from __future__ import annotations

from collections.abc import Mapping
from typing import Any, cast

from memento.config import Principal
from memento.service import MemoryService, ServiceContext

try:  # pragma: no cover - optional runtime dependency
    from aioumcp import AsyncMCPServer  # type: ignore[import-not-found]
    from umcp_shared import MCPPrincipal, get_request_context  # type: ignore[import-not-found]
except ImportError:  # pragma: no cover - optional runtime dependency
    AsyncMCPServer = object
    MCPPrincipal = object

    def get_request_context() -> Any:
        raise RuntimeError("uMCP is not installed")


class MementoMCPServer(AsyncMCPServer):  # type: ignore[misc]
    def __init__(self, service: MemoryService, *, bearer_tokens: Mapping[str, Principal]) -> None:
        super().__init__()
        self._service = service
        self._bearer_tokens = dict(bearer_tokens)

    def get_instructions(self) -> str:
        return "Deterministic shared memory service backed by Git Markdown."

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
        principal = self._resolve_request_principal(request.principal)
        return ServiceContext(principal=principal, mcp_session_id=request.session_id)

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
    ) -> dict[str, Any]:
        return self._service.memory_search(
            self._context(), query=query, concept_type=concept_type, limit=limit, cursor=cursor
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

    async def resource_status(self) -> dict[str, Any]:
        payload = self._service.memory_status(self._context()).model_dump(mode="json")
        return {
            "mimeType": "application/json",
            "text": __import__("json").dumps(payload, sort_keys=True),
        }

    async def resource_help(self) -> dict[str, Any]:
        payload = self._service.memory_help(self._context()).model_dump(mode="json")
        return {
            "mimeType": "application/json",
            "text": __import__("json").dumps(payload, sort_keys=True),
        }

    async def _notify_for_envelope(self, envelope: Mapping[str, Any]) -> None:
        if envelope.get("status") != "success":
            return
        data = envelope.get("data")
        if not isinstance(data, Mapping):
            return
        changed_paths = data.get("changed_paths")
        if changed_paths:
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
