from __future__ import annotations

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


def expand_router_action(action: RouterAction) -> ExpandedRouterAction:
    if isinstance(action, SearchThenReadAction):
        search_args: dict[str, Any] = {"query": action.query, "limit": 1}
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
                "query": action.query,
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
                            "query": action.query,
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
        return DirectToolExpansion(
            tool="memory_read",
            args={"id_or_path": action.id_or_path},
            projection=ProjectionSpec(ref=READ_FIELD_PROJECTIONS[action.field]),
        )
    if isinstance(action, UnknownAction):
        return None
    raise TypeError(f"unsupported router action: {type(action)!r}")


__all__ = [
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
]
