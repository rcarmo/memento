"""Synthetic aggregate graph-layout fixtures."""

from __future__ import annotations

from dataclasses import asdict
from typing import Any

from memento.graph_debug.layout import aggregate_layout  # type: ignore[import-untyped]
from memento.graph_debug.models import (  # type: ignore[import-untyped]
    GraphEdge,
    GraphNode,
    GraphPosition,
)


def node(i: int, namespace: str = "/projects/", orphan: bool = False) -> GraphNode:
    return GraphNode(
        id=f"node-{i:05d}",
        path=f"{namespace}node-{i:05d}.md",
        title=f"Node {i}",
        type="project",
        status="active",
        namespace=namespace,
        updated_at="2026-07-20T00:00:00Z",
        markdown_bytes=100 + i,
        combined_bytes=100 + i,
        explicit_in_degree=1,
        explicit_out_degree=1,
        orphan=orphan,
        coarse_position=GraphPosition(x=0, y=0, z=0),
    )


def edge(a: int, b: int, kind: str = "explicit", similarity: float | None = None) -> GraphEdge:
    return GraphEdge(
        id=f"edge-{a}-{b}",
        source=f"node-{a:05d}",
        target=f"node-{b:05d}",
        raw_target=f"/projects/node-{b:05d}.md",
        kind=kind,
        resolution="resolved",
        first_seen_revision="rev",
        last_checked_revision="rev",
        similarity=similarity,
    )


def dump(layout: Any) -> dict[str, Any]:
    value = asdict(layout)
    for cluster in value["clusters"]:
        cluster["coarse_position"] = cluster["coarse_position"].model_dump(mode="json")
    return value


def fixtures() -> dict[str, Any]:
    connected = [node(0), node(1), node(2, "/systems/"), node(3, "/systems/")]
    overflow = [node(i, f"/namespace-{i}/") for i in range(5)] + [node(5, "/trash/")]
    return {
        "connected": dump(
            aggregate_layout(
                connected,
                [edge(0, 1), edge(1, 2), edge(2, 3)],
                repository_revision="rev",
                cluster_limit=10,
            )
        ),
        "sparse": dump(
            aggregate_layout(
                [node(i, "/skills/") for i in range(10)],
                [],
                repository_revision="rev",
                cluster_limit=20,
            )
        ),
        "overflow": dump(
            aggregate_layout(overflow, [], repository_revision="revision", cluster_limit=3)
        ),
        "semantic": dump(
            aggregate_layout(
                [node(1, "/a/"), node(2, "/b/")],
                [edge(1, 2, "semantic_similarity", 0.91)],
                repository_revision="revision",
                cluster_limit=10,
            )
        ),
    }
