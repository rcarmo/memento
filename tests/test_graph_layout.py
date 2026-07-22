from __future__ import annotations

from memento.graph_debug.layout import aggregate_layout
from memento.graph_debug.models import GraphEdge, GraphNode, GraphPosition


def node(index: int, namespace: str = "/projects/") -> GraphNode:
    return GraphNode(
        id=f"node-{index:05d}",
        path=f"{namespace}node-{index:05d}.md",
        title=f"Node {index}",
        type="project",
        status="active",
        namespace=namespace,
        updated_at="2026-07-20T00:00:00Z",
        markdown_bytes=100 + index,
        combined_bytes=100 + index,
        explicit_in_degree=1,
        explicit_out_degree=1,
        coarse_position=GraphPosition(x=0, y=0, z=0),
    )


def edge(source: int, target: int) -> GraphEdge:
    return GraphEdge(
        id=f"edge-{source}-{target}",
        source=f"node-{source:05d}",
        target=f"node-{target:05d}",
        raw_target=f"/projects/node-{target:05d}.md",
        resolution="resolved",
        first_seen_revision="rev",
        last_checked_revision="rev",
    )


def test_aggregate_layout_is_deterministic_and_input_order_independent() -> None:
    nodes = [node(0), node(1), node(2, "/systems/"), node(3, "/systems/")]
    edges = [edge(0, 1), edge(1, 2), edge(2, 3)]
    first = aggregate_layout(nodes, edges, repository_revision="rev", cluster_limit=10)
    second = aggregate_layout(
        reversed(nodes), reversed(edges), repository_revision="rev", cluster_limit=10
    )
    assert first == second
    assert len(first.clusters) == 2
    assert len(first.edges) == 1
    assert sum(cluster.member_count for cluster in first.clusters) == 4
    assert all(
        coordinate == coordinate
        for cluster in first.clusters
        for coordinate in (
            cluster.coarse_position.x,
            cluster.coarse_position.y,
            cluster.coarse_position.z,
        )
    )


def test_aggregate_layout_groups_sparse_namespace_isolates() -> None:
    nodes = [node(index, "/skills/") for index in range(10)]
    layout = aggregate_layout(nodes, (), repository_revision="rev", cluster_limit=20)
    assert len(layout.clusters) == 1
    assert layout.clusters[0].namespace == "/skills/"
    assert layout.clusters[0].member_count == 10


def test_aggregate_layout_handles_isolates_and_cluster_bound() -> None:
    nodes = [node(index, f"/namespace-{index}/") for index in range(20)]
    layout = aggregate_layout(nodes, (), repository_revision="rev", cluster_limit=5)
    assert len(layout.clusters) == 5
    assert sum(cluster.member_count for cluster in layout.clusters) == 20
    assert any(cluster.id == "cluster:overflow" for cluster in layout.clusters)


def test_scale_fixtures_are_bounded() -> None:
    for size in (500, 2_000, 10_000):
        nodes = [node(index, f"/namespace-{index % 20}/") for index in range(size)]
        edges = [edge(index, index + 1) for index in range(size - 1)]
        layout = aggregate_layout(
            nodes, edges, repository_revision=f"rev-{size}", cluster_limit=500
        )
        assert len(layout.clusters) <= 500
        assert sum(cluster.member_count for cluster in layout.clusters) == size
        assert len(layout.edges) <= len(edges)
