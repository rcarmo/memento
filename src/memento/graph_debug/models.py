from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class GraphModel(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)


class GraphRevisions(GraphModel):
    repository: str
    index: str
    embedding: str | None = None
    stale: bool


class GraphPosition(GraphModel):
    x: float
    y: float
    z: float


class GraphEmbeddingState(GraphModel):
    status: str = "missing"
    model_id: str | None = None
    dimensions: int | None = None
    embedding_revision: str | None = None
    model_revision: str | None = None
    updated_at: str | None = None
    error: str | None = None


class GraphNode(GraphModel):
    id: str
    path: str
    title: str
    type: str
    status: str
    tags: tuple[str, ...] = ()
    namespace: str
    updated_at: str
    updated_by: str | None = None
    markdown_bytes: int = 0
    asset_bytes: int = 0
    combined_bytes: int = 0
    explicit_in_degree: int = 0
    explicit_out_degree: int = 0
    broken_link_count: int = 0
    orphan: bool = False
    proposal_count: int = 0
    pending_proposal_count: int = 0
    embedding: GraphEmbeddingState = Field(default_factory=GraphEmbeddingState)
    coarse_position: GraphPosition


class GraphEdge(GraphModel):
    id: str
    source: str
    target: str | None = None
    raw_target: str
    kind: Literal["explicit"] = "explicit"
    canonical: Literal[True] = True
    resolution: str
    anchor: str | None = None
    first_seen_revision: str
    last_checked_revision: str


class GraphMetrics(GraphModel):
    memory_count: int
    markdown_bytes: int
    asset_bytes: int
    explicit_edges: int
    broken_edges: int
    orphan_count: int


class GraphOverview(GraphModel):
    schema_version: Literal[1] = 1
    mode: Literal["direct"] = "direct"
    revisions: GraphRevisions
    metrics: GraphMetrics
    nodes: tuple[GraphNode, ...]
    edges: tuple[GraphEdge, ...]
    truncated: bool = False


class GraphProposalSummary(GraphModel):
    proposal_id: str
    author: str
    status: str
    intent: str
    base_revision: str
    applied_revision: str | None = None
    created_at: str
    updated_at: str


class GraphAssetSummary(GraphModel):
    asset_kind: str
    version: str
    metadata_bytes: int
    payload_bytes: int
    source_proposal_id: str | None = None


class GraphMemoryDetail(GraphModel):
    schema_version: Literal[1] = 1
    revisions: GraphRevisions
    node: GraphNode
    preview: str
    preview_truncated: bool
    outbound: tuple[GraphEdge, ...]
    inbound: tuple[GraphEdge, ...]
    assets: tuple[GraphAssetSummary, ...]
    proposals: tuple[GraphProposalSummary, ...]


class GraphNeighbourhood(GraphModel):
    schema_version: Literal[1] = 1
    revisions: GraphRevisions
    center_id: str
    nodes: tuple[GraphNode, ...]
    edges: tuple[GraphEdge, ...]
    depth: int = 1
