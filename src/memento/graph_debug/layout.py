from __future__ import annotations

import hashlib
import math
from collections import Counter, defaultdict
from collections.abc import Iterable
from dataclasses import dataclass

from memento.graph_debug.models import GraphEdge, GraphNode, GraphPosition

_LAYOUT_VERSION = "v1"


@dataclass(frozen=True, slots=True)
class AggregateNode:
    id: str
    label: str
    namespace: str
    member_count: int
    markdown_bytes: int
    asset_bytes: int
    combined_bytes: int
    explicit_in_degree: int
    explicit_out_degree: int
    broken_link_count: int
    orphan_count: int
    type_counts: tuple[tuple[str, int], ...]
    status_counts: tuple[tuple[str, int], ...]
    coarse_position: GraphPosition


@dataclass(frozen=True, slots=True)
class AggregateEdge:
    id: str
    source: str
    target: str
    explicit_edge_count: int
    canonical: bool = True


@dataclass(frozen=True, slots=True)
class AggregateLayout:
    seed: str
    version: str
    clusters: tuple[AggregateNode, ...]
    edges: tuple[AggregateEdge, ...]
    memberships: tuple[tuple[str, str], ...]


def aggregate_layout(
    nodes: Iterable[GraphNode],
    edges: Iterable[GraphEdge],
    *,
    repository_revision: str,
    cluster_limit: int,
) -> AggregateLayout:
    ordered_nodes = tuple(sorted(nodes, key=lambda item: item.id))
    ordered_edges = tuple(sorted(edges, key=lambda item: item.id))
    by_namespace: dict[str, list[GraphNode]] = defaultdict(list)
    for node in ordered_nodes:
        by_namespace[node.namespace].append(node)

    groups: list[tuple[str, tuple[GraphNode, ...]]] = []
    for namespace, members in sorted(by_namespace.items()):
        components = _explicit_components(tuple(members), ordered_edges)
        for index, component in enumerate(components):
            cluster_id = _cluster_id(namespace, index, component)
            groups.append((cluster_id, component))
    if len(groups) > cluster_limit:
        groups = _merge_small_groups(groups, cluster_limit)

    memberships = {node.id: cluster_id for cluster_id, members in groups for node in members}
    clusters = tuple(
        _aggregate_node(
            cluster_id,
            members,
            repository_revision=repository_revision,
            ordinal=index,
            total=len(groups),
        )
        for index, (cluster_id, members) in enumerate(groups)
    )
    counts: Counter[tuple[str, str]] = Counter()
    for edge in ordered_edges:
        if edge.target is None:
            continue
        source = memberships.get(edge.source)
        target = memberships.get(edge.target)
        if source is None or target is None or source == target:
            continue
        counts[(source, target)] += 1
    aggregate_edges = tuple(
        AggregateEdge(
            id=f"cluster-explicit:{source}:{target}",
            source=source,
            target=target,
            explicit_edge_count=count,
        )
        for (source, target), count in sorted(counts.items())
    )
    return AggregateLayout(
        seed=repository_revision,
        version=_LAYOUT_VERSION,
        clusters=clusters,
        edges=aggregate_edges,
        memberships=tuple(sorted(memberships.items())),
    )


def _explicit_components(
    members: tuple[GraphNode, ...], edges: tuple[GraphEdge, ...]
) -> tuple[tuple[GraphNode, ...], ...]:
    ids = {node.id for node in members}
    neighbours: dict[str, set[str]] = {node_id: set() for node_id in ids}
    for edge in edges:
        if edge.target is not None and edge.source in ids and edge.target in ids:
            neighbours[edge.source].add(edge.target)
            neighbours[edge.target].add(edge.source)
    node_by_id = {node.id: node for node in members}
    components = []
    unseen = set(ids)
    while unseen:
        start = min(unseen)
        stack = [start]
        component = []
        unseen.remove(start)
        while stack:
            current = stack.pop()
            component.append(node_by_id[current])
            for neighbour in sorted(neighbours[current], reverse=True):
                if neighbour in unseen:
                    unseen.remove(neighbour)
                    stack.append(neighbour)
        components.append(tuple(sorted(component, key=lambda item: item.id)))
    return tuple(sorted(components, key=lambda component: component[0].id))


def _merge_small_groups(
    groups: list[tuple[str, tuple[GraphNode, ...]]], cluster_limit: int
) -> list[tuple[str, tuple[GraphNode, ...]]]:
    retained = sorted(groups, key=lambda item: (-len(item[1]), item[0]))[: cluster_limit - 1]
    retained_ids = {item[0] for item in retained}
    overflow = tuple(
        sorted(
            (
                node
                for cluster_id, members in groups
                if cluster_id not in retained_ids
                for node in members
            ),
            key=lambda item: item.id,
        )
    )
    retained.append(("cluster:overflow", overflow))
    return sorted(retained, key=lambda item: item[0])


def _cluster_id(namespace: str, index: int, members: tuple[GraphNode, ...]) -> str:
    digest = hashlib.sha256(
        (namespace + "\0" + "\0".join(node.id for node in members)).encode("utf-8")
    ).hexdigest()[:12]
    return f"cluster:{namespace.strip('/') or 'root'}:{index}:{digest}"


def _aggregate_node(
    cluster_id: str,
    members: tuple[GraphNode, ...],
    *,
    repository_revision: str,
    ordinal: int,
    total: int,
) -> AggregateNode:
    namespace = members[0].namespace if members else "/"
    return AggregateNode(
        id=cluster_id,
        label=namespace if len(members) != 1 else members[0].title,
        namespace=namespace,
        member_count=len(members),
        markdown_bytes=sum(node.markdown_bytes for node in members),
        asset_bytes=sum(node.asset_bytes for node in members),
        combined_bytes=sum(node.combined_bytes for node in members),
        explicit_in_degree=sum(node.explicit_in_degree for node in members),
        explicit_out_degree=sum(node.explicit_out_degree for node in members),
        broken_link_count=sum(node.broken_link_count for node in members),
        orphan_count=sum(node.orphan for node in members),
        type_counts=tuple(sorted(Counter(node.type for node in members).items())),
        status_counts=tuple(sorted(Counter(node.status for node in members).items())),
        coarse_position=_cluster_position(
            cluster_id,
            repository_revision=repository_revision,
            ordinal=ordinal,
            total=total,
        ),
    )


def _cluster_position(
    cluster_id: str, *, repository_revision: str, ordinal: int, total: int
) -> GraphPosition:
    digest = hashlib.sha256(
        f"{_LAYOUT_VERSION}\0{repository_revision}\0{cluster_id}".encode()
    ).digest()
    jitter = int.from_bytes(digest[:4], "big") / 2**32 - 0.5
    angle = math.tau * (ordinal + 0.5 + jitter * 0.2) / max(1, total)
    radius = 3.0 + math.sqrt(max(1, total)) * 0.6
    z = (int.from_bytes(digest[4:8], "big") / 2**32 - 0.5) * 2.0
    return GraphPosition(x=math.cos(angle) * radius, y=math.sin(angle) * radius, z=z)
