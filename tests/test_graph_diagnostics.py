from __future__ import annotations

from memento.graph_debug.diagnostics import apply_diagnostic_ids, diagnose_graph
from memento.graph_debug.models import (
    GraphEdge,
    GraphEmbeddingState,
    GraphNode,
    GraphPosition,
    GraphRevisions,
)


def node(
    concept_id: str,
    *,
    namespace: str = "/projects/",
    size: int = 100,
    tags: tuple[str, ...] = ("common",),
    orphan: bool = False,
    broken: int = 0,
    embedding: GraphEmbeddingState | None = None,
    pending: int = 0,
) -> GraphNode:
    return GraphNode(
        id=concept_id,
        path=f"{namespace}{concept_id}.md",
        title=concept_id,
        type="project",
        status="active",
        tags=tags,
        namespace=namespace,
        updated_at="2026-07-20T00:00:00Z",
        combined_bytes=size,
        markdown_bytes=size,
        explicit_in_degree=0 if orphan else 1,
        explicit_out_degree=0 if orphan else 1,
        broken_link_count=broken,
        orphan=orphan,
        pending_proposal_count=pending,
        embedding=embedding or GraphEmbeddingState(status="ready", embedding_revision="rev"),
        coarse_position=GraphPosition(x=0, y=0, z=0),
    )


def edge(source: str, target: str) -> GraphEdge:
    return GraphEdge(
        id=f"{source}-{target}",
        source=source,
        target=target,
        raw_target=target,
        resolution="resolved",
        first_seen_revision="rev",
        last_checked_revision="rev",
    )


def test_diagnostics_are_explainable_and_keep_semantics_derived() -> None:
    nodes = (
        node("a", orphan=True, broken=2, pending=1),
        node("b", embedding=GraphEmbeddingState(status="missing")),
        node("c", embedding=GraphEmbeddingState(status="error", error="worker failed")),
        node("d", size=10_000, tags=()),
    )
    diagnostics = diagnose_graph(
        nodes,
        (),
        revisions=GraphRevisions(repository="rev", index="old", embedding="old", stale=True),
        content_hashes={"a": "same", "b": "same", "c": "c", "d": "d"},
    )
    rules = {item.rule for item in diagnostics}
    assert {
        "orphan",
        "broken_links",
        "index_stale",
        "embedding_missing",
        "embedding_failed",
        "pending_proposals",
        "exact_duplicate",
        "size_outlier",
        "tag_drift",
    } <= rules
    semantic = [item for item in diagnostics if item.rule.startswith("embedding_")]
    assert semantic and all(item.derived for item in semantic)
    assert all(item.measured for item in diagnostics)
    with_ids = apply_diagnostic_ids(nodes, diagnostics)
    assert with_ids[0].anomaly_ids


def test_namespace_outlier_uses_only_explicit_edges() -> None:
    nodes = (
        node("a", namespace="/projects/"),
        node("b", namespace="/systems/"),
        node("c", namespace="/systems/"),
        node("d", namespace="/systems/"),
    )
    diagnostics = diagnose_graph(
        nodes,
        (edge("a", "b"), edge("a", "c"), edge("a", "d")),
        revisions=GraphRevisions(repository="rev", index="rev", stale=False),
    )
    outlier = next(item for item in diagnostics if item.rule == "namespace_outlier")
    assert outlier.concept_ids == ("a",)
    assert outlier.derived is False
