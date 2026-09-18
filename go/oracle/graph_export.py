"""Graph JSON/SVG export byte fixtures."""

from __future__ import annotations

import base64
from typing import Any

from memento.graph_debug.export import (  # type: ignore[import-untyped]
    export_graph_json,
    export_graph_svg,
)
from memento.graph_debug.models import (  # type: ignore[import-untyped]
    GraphEdge,
    GraphNode,
    GraphPosition,
    GraphRevisions,
)


def fixtures() -> dict[str, Any]:
    nodes = [
        GraphNode(
            id="a",
            path="/a",
            title="A & <B>",
            type="concept",
            status="active",
            namespace="/",
            updated_at="now",
            combined_bytes=123,
            coarse_position=GraphPosition(x=1, y=-0.5, z=0),
        ),
        GraphNode(
            id="b",
            path="/b",
            title="Beta",
            type="concept",
            status="active",
            namespace="/",
            updated_at="now",
            combined_bytes=0,
            coarse_position=GraphPosition(x=-1, y=0.5, z=0),
        ),
    ]
    edges = [
        GraphEdge(
            id="e",
            source="a",
            target="b",
            raw_target="/b",
            resolution="resolved",
            first_seen_revision="r",
            last_checked_revision="r",
        ),
        GraphEdge(
            id="missing",
            source="a",
            target="missing",
            raw_target="/missing",
            resolution="broken",
            first_seen_revision="r",
            last_checked_revision="r",
        ),
    ]
    revisions = GraphRevisions(repository="r", index="r", stale=False)
    return {
        "json": base64.b64encode(
            export_graph_json(nodes, edges, revisions=revisions, settings={"theme": "dark"})
        ).decode(),
        "svg": base64.b64encode(export_graph_svg(nodes, edges, width=400, height=200)).decode(),
    }
