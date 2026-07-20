from __future__ import annotations

import json

from memento.graph_debug.export import export_graph_json, export_graph_svg
from memento.graph_debug.models import GraphEdge, GraphNode, GraphPosition, GraphRevisions


def _node(concept_id: str, title: str) -> GraphNode:
    return GraphNode(
        id=concept_id,
        path=f"/projects/{concept_id}.md",
        title=title,
        type="project",
        status="active",
        namespace="/projects/",
        updated_at="2026-07-20T00:00:00Z",
        combined_bytes=123,
        coarse_position=GraphPosition(x=1 if concept_id == "a" else -1, y=0, z=0),
    )


def test_json_and_svg_exports_are_bounded_and_safe() -> None:
    nodes = (_node("a", '<script>alert("x")</script>'), _node("b", "B"))
    edges = (
        GraphEdge(
            id="edge",
            source="a",
            target="b",
            raw_target="/projects/b.md",
            resolution="resolved",
            first_seen_revision="rev",
            last_checked_revision="rev",
        ),
    )
    revisions = GraphRevisions(repository="rev", index="rev", stale=False)
    encoded = export_graph_json(nodes, edges, revisions=revisions, settings={"size": "combined"})
    payload = json.loads(encoded)
    assert len(payload["nodes"]) == 2
    assert "embedding_blob" not in encoded.decode()
    assert "authorization" not in encoded.decode().casefold()
    svg = export_graph_svg(nodes, edges).decode()
    assert svg.startswith("<svg")
    assert "<script>" not in svg
    assert "&lt;script&gt;" in svg
    assert 'class="edge"' in svg
