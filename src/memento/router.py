from __future__ import annotations

from enum import StrEnum
from typing import Annotated, Any, Literal

from pydantic import BaseModel, ConfigDict, Field, TypeAdapter

from memento.executor import ExecutePlan, ExecuteReturnProjection


class SearchMode(StrEnum):
    LEXICAL = "lexical"
    SEMANTIC = "semantic"
    HYBRID = "hybrid"


STATUS_FIELDS = (
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
)

READ_FIELDS = (
    "path",
    "frontmatter",
    "body",
)

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
]
type ReadFieldName = Literal["path", "frontmatter", "body"]
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
            projection=ProjectionSpec(ref=action.field),
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
            projection=ProjectionSpec(ref=action.field),
        )
    if isinstance(action, UnknownAction):
        return None
    raise TypeError(f"unsupported router action: {type(action)!r}")


__all__ = [
    "ExpandedRouterAction",
    "GraphDepth",
    "ProjectionSpec",
    "READ_FIELDS",
    "ROUTER_ACTION_ADAPTER",
    "RouterAction",
    "STATUS_FIELDS",
    "SearchMode",
    "expand_router_action",
]
