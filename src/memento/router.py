from __future__ import annotations

import json
import re
from enum import StrEnum
from typing import Annotated, Any, Literal

from pydantic import BaseModel, ConfigDict, Field, TypeAdapter

from memento.executor import ExecutePlan, ExecuteReturnProjection


class SearchMode(StrEnum):
    LEXICAL = "lexical"
    SEMANTIC = "semantic"
    HYBRID = "hybrid"


STATUS_FIELD_PROJECTIONS = {
    "service_version": "service_version",
    "schema_version": "schema_version",
    "repo_revision": "repo_revision",
    "index_revision": "index_revision",
    "index_stale": "index_stale",
    "principal": "principal",
    "visible_concepts": "visible_concepts",
    "proposal_backlog": "proposal_backlog",
    "limits": "limits",
    "roles": "roles",
    "features": "features",
    "readiness": "readiness",
    "semantic_search_ready": "readiness.semantic_search.ready",
    "semantic_search_model_id": "readiness.semantic_search.model_id",
    "semantic_search_dimensions": "readiness.semantic_search.dimensions",
    "semantic_search_embedding_revision": "readiness.semantic_search.embedding_revision",
    "semantic_search_sqlite_vector_enabled": "readiness.semantic_search.sqlite_vector_enabled",
}
STATUS_FIELDS = tuple(STATUS_FIELD_PROJECTIONS)

READ_FIELD_PROJECTIONS = {
    "path": "path",
    "frontmatter": "frontmatter",
    "body": "body",
    "title": "frontmatter.title",
    "type": "frontmatter.type",
    "status": "frontmatter.status",
    "tags": "frontmatter.tags",
    "aliases": "frontmatter.aliases",
}
READ_FIELDS = tuple(READ_FIELD_PROJECTIONS)

type StatusFieldName = Literal[
    "service_version",
    "schema_version",
    "repo_revision",
    "index_revision",
    "index_stale",
    "principal",
    "visible_concepts",
    "proposal_backlog",
    "limits",
    "roles",
    "features",
    "readiness",
    "semantic_search_ready",
    "semantic_search_model_id",
    "semantic_search_dimensions",
    "semantic_search_embedding_revision",
    "semantic_search_sqlite_vector_enabled",
]
type ReadFieldName = Literal[
    "path", "frontmatter", "body", "title", "type", "status", "tags", "aliases"
]
type SearchPathsLimit = Literal[1, 2, 3, 5]
type GraphDepth = Literal[1, 2]


class RouterActionBase(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)


class SearchThenReadAction(RouterActionBase):
    action: Literal["search_then_read"]
    query: str = Field(min_length=1, max_length=200)
    search_mode: SearchMode | None = None


class SearchPathsAction(RouterActionBase):
    action: Literal["search_paths"]
    query: str = Field(min_length=1, max_length=200)
    limit: SearchPathsLimit = 3
    search_mode: SearchMode = SearchMode.LEXICAL


class StatusFieldAction(RouterActionBase):
    action: Literal["status_field"]
    field: StatusFieldName


class SearchThenGraphAction(RouterActionBase):
    action: Literal["search_then_graph"]
    query: str = Field(min_length=1, max_length=200)
    depth: GraphDepth = 1
    search_mode: SearchMode = SearchMode.LEXICAL


class ReadFieldAction(RouterActionBase):
    action: Literal["read_field"]
    id_or_path: str = Field(min_length=1, max_length=512)
    field: ReadFieldName


class UnknownAction(RouterActionBase):
    action: Literal["UNKNOWN"]


type RouterAction = Annotated[
    SearchThenReadAction
    | SearchPathsAction
    | StatusFieldAction
    | SearchThenGraphAction
    | ReadFieldAction
    | UnknownAction,
    Field(discriminator="action"),
]
ROUTER_ACTION_ADAPTER: TypeAdapter[RouterAction] = TypeAdapter(RouterAction)

CANONICAL_TRAINED_SHALLOW_TOOLS: tuple[dict[str, Any], ...] = (
    {
        "name": "search_then_read",
        "description": "Search for the best matching concept, then read it.",
        "parameters": {
            "query": {"type": "string", "required": True},
            "search_mode": {"type": "string", "required": False},
        },
    },
    {
        "name": "search_paths",
        "description": "Search concepts and return matching paths.",
        "parameters": {
            "query": {"type": "string", "required": True},
            "limit": {"type": "number", "required": False},
            "search_mode": {"type": "string", "required": False},
        },
    },
    {
        "name": "status_field",
        "description": "Return one service status field.",
        "parameters": {"field": {"type": "string", "required": True}},
    },
    {
        "name": "search_then_graph",
        "description": "Search for a concept, then inspect its graph neighborhood.",
        "parameters": {
            "query": {"type": "string", "required": True},
            "depth": {"type": "number", "required": False},
            "search_mode": {"type": "string", "required": False},
        },
    },
    {
        "name": "read_field",
        "description": "Read an exact path or concept id and return one field.",
        "parameters": {
            "id_or_path": {"type": "string", "required": True},
            "field": {"type": "string", "required": True},
        },
    },
    {
        "name": "UNKNOWN",
        "description": "Use for unsupported, unsafe, ambiguous, external or insufficiently identified requests.",
        "parameters": {},
    },
)
CANONICAL_TRAINED_SHALLOW_TOOLS_JSON = json.dumps(
    CANONICAL_TRAINED_SHALLOW_TOOLS, separators=(",", ":")
)


_STATUS_FIELD_ALIASES = {
    "indexed": "index_revision",
    "index": "index_revision",
    "repository": "repo_revision",
    "revision": "repo_revision",
    "stale": "index_stale",
    "semantic_ready": "semantic_search_ready",
}
_READ_FIELD_ALIASES = {
    "name": "title",
    "contents": "body",
    "content": "body",
}


def parse_needle_router_output(payload: str) -> RouterAction:
    parsed = json.loads(payload)
    if not isinstance(parsed, list) or len(parsed) != 1:
        raise ValueError("needle router output must be a single-call JSON array")
    call = parsed[0]
    if not isinstance(call, dict):
        raise ValueError("needle router output call must be an object")
    name = call.get("name")
    arguments = call.get("arguments", {})
    if not isinstance(name, str) or not name:
        raise ValueError("needle router output call name must be a non-empty string")
    if not isinstance(arguments, dict):
        raise ValueError("needle router output call arguments must be an object")
    normalized = dict(arguments)
    if name == "status_field" and isinstance(normalized.get("field"), str):
        normalized["field"] = _STATUS_FIELD_ALIASES.get(normalized["field"], normalized["field"])
    if name == "read_field" and isinstance(normalized.get("field"), str):
        normalized["field"] = _READ_FIELD_ALIASES.get(normalized["field"], normalized["field"])
    return ROUTER_ACTION_ADAPTER.validate_python({"action": name, **normalized})


class ProjectionSpec(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    ref: str
    fields: tuple[str, ...] = ()
    limit: int | None = Field(default=None, ge=1)


class DirectToolExpansion(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    kind: Literal["direct"] = "direct"
    tool: Literal["memory_search", "memory_status", "memory_read"]
    args: dict[str, Any]
    projection: ProjectionSpec | None = None


class ExecutePlanExpansion(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    kind: Literal["execute"] = "execute"
    tool: Literal["memory_execute"] = "memory_execute"
    args: dict[str, Any]


type ExpandedRouterAction = DirectToolExpansion | ExecutePlanExpansion | None

_SEARCH_REQUEST_PREFIX = re.compile(
    r"^(?:(?:please|kindly)\s+)?(?:find|search(?:\s+for)?|show|fetch|locate|get)\s+",
    re.IGNORECASE,
)


def _derive_search_query(request: str) -> str:
    derived = _SEARCH_REQUEST_PREFIX.sub("", request, count=1).strip()
    return derived or request


def expand_router_action(action: RouterAction, *, request: str) -> ExpandedRouterAction:
    """Expand a classification without accepting model-invented free-form values."""
    if isinstance(action, SearchThenReadAction):
        search_args: dict[str, Any] = {"query": _derive_search_query(request), "limit": 1}
        if action.search_mode is not None:
            search_args["search_mode"] = action.search_mode.value
        plan = ExecutePlan.model_validate(
            {
                "operations": (
                    {"op": "search", "args": search_args, "save_as": "hits"},
                    {
                        "op": "read",
                        "args": {"id_or_path": "$hits.results.0.path"},
                        "save_as": "doc",
                    },
                ),
                "returns": (ExecuteReturnProjection(name="document", ref="$doc"),),
            }
        )
        return ExecutePlanExpansion(
            tool="memory_execute", args={"plan": plan.model_dump(mode="python")}
        )
    if isinstance(action, SearchPathsAction):
        return DirectToolExpansion(
            tool="memory_search",
            args={
                "query": _derive_search_query(request),
                "limit": action.limit,
                "search_mode": action.search_mode.value,
            },
            projection=ProjectionSpec(ref="results", fields=("path",), limit=action.limit),
        )
    if isinstance(action, StatusFieldAction):
        return DirectToolExpansion(
            tool="memory_status",
            args={},
            projection=ProjectionSpec(ref=STATUS_FIELD_PROJECTIONS[action.field]),
        )
    if isinstance(action, SearchThenGraphAction):
        plan = ExecutePlan.model_validate(
            {
                "operations": (
                    {
                        "op": "search",
                        "args": {
                            "query": _derive_search_query(request),
                            "limit": 1,
                            "search_mode": action.search_mode.value,
                        },
                        "save_as": "hits",
                    },
                    {
                        "op": "graph",
                        "args": {
                            "id_or_path": "$hits.results.0.path",
                            "depth": action.depth,
                        },
                        "save_as": "graph",
                    },
                ),
                "returns": (ExecuteReturnProjection(name="graph", ref="$graph"),),
            }
        )
        return ExecutePlanExpansion(
            tool="memory_execute", args={"plan": plan.model_dump(mode="python")}
        )
    if isinstance(action, ReadFieldAction):
        # The model may identify an exact reference, but it cannot invent one. Require
        # that reference to be present verbatim in the authenticated user's request.
        if action.id_or_path not in request:
            return None
        return DirectToolExpansion(
            tool="memory_read",
            args={"id_or_path": action.id_or_path},
            projection=ProjectionSpec(ref=READ_FIELD_PROJECTIONS[action.field]),
        )
    if isinstance(action, UnknownAction):
        return None
    raise TypeError(f"unsupported router action: {type(action)!r}")


__all__ = [
    "CANONICAL_TRAINED_SHALLOW_TOOLS",
    "CANONICAL_TRAINED_SHALLOW_TOOLS_JSON",
    "ExpandedRouterAction",
    "GraphDepth",
    "ProjectionSpec",
    "READ_FIELD_PROJECTIONS",
    "READ_FIELDS",
    "ROUTER_ACTION_ADAPTER",
    "RouterAction",
    "STATUS_FIELD_PROJECTIONS",
    "STATUS_FIELDS",
    "SearchMode",
    "expand_router_action",
    "parse_needle_router_output",
]
