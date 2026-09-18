"""Synthetic fixtures for extended graph diagnostics."""

from __future__ import annotations

from typing import Any

from memento.graph_debug.diagnostics import diagnose_graph  # type: ignore[import-untyped]
from memento.graph_debug.models import (  # type: ignore[import-untyped]
    GraphEdge,
    GraphNode,
    GraphPosition,
    GraphRevisions,
)


def node(
    i: int, namespace: str = "/same/", size: int = 100, tags: tuple[str, ...] = ("common",)
) -> GraphNode:
    return GraphNode(
        id=f"n{i}",
        path=f"{namespace}n{i}.md",
        title=f"N{i}",
        type="concept",
        status="active",
        namespace=namespace,
        tags=tags,
        updated_at="now",
        combined_bytes=size,
        markdown_bytes=size,
        coarse_position=GraphPosition(x=0, y=0, z=0),
    )


def edge(a: int, b: int) -> GraphEdge:
    return GraphEdge(
        id=f"e{a}-{b}",
        source=f"n{a}",
        target=f"n{b}",
        raw_target=f"/n{b}",
        resolution="resolved",
        first_seen_revision="r",
        last_checked_revision="r",
    )


def fixtures() -> dict[str, Any]:
    nodes = [
        node(0, size=5000, tags=()),
        node(1),
        node(2),
        node(3),
        node(4, "/other/"),
        node(5, "/third/"),
        node(6, "/fourth/"),
    ]
    edges = [edge(0, 4), edge(0, 5), edge(0, 6)]
    diagnostics = diagnose_graph(
        nodes,
        edges,
        revisions=GraphRevisions(repository="r", index="r", stale=False),
        content_hashes={"n1": "same", "n2": "same", "n3": "other"},
    )
    return {
        "nodes": [item.model_dump(mode="json") for item in nodes],
        "edges": [item.model_dump(mode="json") for item in edges],
        "diagnostics": [item.model_dump(mode="json") for item in diagnostics],
        "content_hashes": {"n1": "same", "n2": "same", "n3": "other"},
    }
