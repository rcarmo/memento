from __future__ import annotations

import html
import json
from collections.abc import Iterable

from memento.graph_debug.models import GraphEdge, GraphNode, GraphRevisions


def export_graph_json(
    nodes: Iterable[GraphNode],
    edges: Iterable[GraphEdge],
    *,
    revisions: GraphRevisions,
    settings: dict[str, object] | None = None,
) -> bytes:
    payload = {
        "schema_version": 1,
        "revisions": revisions.model_dump(mode="json"),
        "nodes": [node.model_dump(mode="json", exclude={"embedding": {"error"}}) for node in nodes],
        "edges": [edge.model_dump(mode="json") for edge in edges],
        "settings": settings or {},
    }
    encoded = json.dumps(payload, sort_keys=True, separators=(",", ":"))
    forbidden = ("embedding_blob", "bearer", "authorization", "token_env")
    if any(item in encoded.casefold() for item in forbidden):
        raise ValueError("graph export contains forbidden fields")
    return encoded.encode("utf-8")


def export_graph_svg(
    nodes: Iterable[GraphNode],
    edges: Iterable[GraphEdge],
    *,
    width: int = 1600,
    height: int = 1000,
) -> bytes:
    bounded_nodes = tuple(nodes)
    node_by_id = {node.id: node for node in bounded_nodes}
    scale = min(width, height) * 0.09
    center_x, center_y = width / 2, height / 2

    def point(node: GraphNode) -> tuple[float, float]:
        return center_x + node.coarse_position.x * scale, center_y - node.coarse_position.y * scale

    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">',
        "<style>text{font:12px system-ui,sans-serif;fill:#1a2a40}.edge{fill:none;stroke:#7090b0;stroke-opacity:.55}.node{fill:#2b6cb0;stroke:#1a2a40}</style>",
        '<rect width="100%" height="100%" fill="#e8eff6"/>',
    ]
    for edge in edges:
        source = node_by_id.get(edge.source)
        target = node_by_id.get(edge.target or "")
        if source is None or target is None:
            continue
        x1, y1 = point(source)
        x2, y2 = point(target)
        cx, cy = (x1 + x2) / 2, (y1 + y2) / 2 - min(120, abs(x2 - x1) * 0.18 + 20)
        parts.append(
            f'<path class="edge" d="M{x1:.2f},{y1:.2f} Q{cx:.2f},{cy:.2f} {x2:.2f},{y2:.2f}"/>'
        )
    for node in bounded_nodes:
        x, y = point(node)
        radius = max(4.0, min(24.0, 4.0 + len(str(max(1, node.combined_bytes))) * 1.4))
        label = html.escape(node.title, quote=True)
        parts.append(f'<circle class="node" cx="{x:.2f}" cy="{y:.2f}" r="{radius:.2f}"/>')
        parts.append(f'<text x="{x + radius + 4:.2f}" y="{y + 4:.2f}">{label}</text>')
    parts.append("</svg>")
    return "".join(parts).encode("utf-8")
