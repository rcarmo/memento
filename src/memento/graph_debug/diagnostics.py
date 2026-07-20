from __future__ import annotations

import hashlib
import statistics
from collections import Counter, defaultdict
from collections.abc import Iterable, Mapping

from memento.graph_debug.models import GraphDiagnostic, GraphEdge, GraphNode, GraphRevisions


def diagnose_graph(
    nodes: Iterable[GraphNode],
    edges: Iterable[GraphEdge],
    *,
    revisions: GraphRevisions,
    content_hashes: Mapping[str, str] | None = None,
) -> tuple[GraphDiagnostic, ...]:
    ordered_nodes = tuple(sorted(nodes, key=lambda item: item.id))
    ordered_edges = tuple(sorted(edges, key=lambda item: item.id))
    diagnostics: list[GraphDiagnostic] = []
    diagnostics.extend(_structural(ordered_nodes, ordered_edges))
    diagnostics.extend(_lifecycle(ordered_nodes, revisions))
    diagnostics.extend(_size_outliers(ordered_nodes))
    diagnostics.extend(_tag_drift(ordered_nodes))
    diagnostics.extend(_namespace_outliers(ordered_nodes, ordered_edges))
    if content_hashes:
        diagnostics.extend(_exact_duplicates(content_hashes))
    return tuple(sorted(diagnostics, key=lambda item: (item.severity, item.rule, item.id)))


def apply_diagnostic_ids(
    nodes: Iterable[GraphNode], diagnostics: Iterable[GraphDiagnostic]
) -> tuple[GraphNode, ...]:
    by_concept: dict[str, list[str]] = defaultdict(list)
    for diagnostic in diagnostics:
        for concept_id in diagnostic.concept_ids:
            by_concept[concept_id].append(diagnostic.id)
    return tuple(
        node.model_copy(update={"anomaly_ids": tuple(sorted(by_concept.get(node.id, ())))})
        for node in nodes
    )


def _structural(
    nodes: tuple[GraphNode, ...], edges: tuple[GraphEdge, ...]
) -> list[GraphDiagnostic]:
    diagnostics = []
    cluster_namespaces = {node.namespace for node in nodes}
    namespace_links: set[tuple[str, str]] = set()
    node_by_id = {node.id: node for node in nodes}
    for edge in edges:
        if edge.target and edge.source in node_by_id and edge.target in node_by_id:
            source = node_by_id[edge.source].namespace
            target = node_by_id[edge.target].namespace
            if source != target:
                namespace_links.add((source, target))
                namespace_links.add((target, source))
    degrees = [node.explicit_in_degree + node.explicit_out_degree for node in nodes]
    degree_threshold = max(20, _percentile(degrees, 0.95)) if degrees else 20
    for node in nodes:
        degree = node.explicit_in_degree + node.explicit_out_degree
        if node.orphan or degree == 0:
            diagnostics.append(
                _diagnostic(
                    "orphan",
                    "warning",
                    (node.id,),
                    "No explicit inbound or outbound links.",
                    {"explicit_degree": degree},
                    {"maximum_orphan_degree": 0},
                )
            )
        if node.broken_link_count:
            diagnostics.append(
                _diagnostic(
                    "broken_links",
                    "error",
                    (node.id,),
                    f"{node.broken_link_count} explicit link target(s) do not resolve.",
                    {"broken_link_count": node.broken_link_count},
                    {"maximum": 0},
                )
            )
        if degree > degree_threshold:
            diagnostics.append(
                _diagnostic(
                    "high_degree",
                    "info",
                    (node.id,),
                    "Explicit degree exceeds the graph high-degree threshold.",
                    {"explicit_degree": degree},
                    {"threshold": degree_threshold},
                )
            )
    for namespace in sorted(cluster_namespaces):
        members = tuple(node.id for node in nodes if node.namespace == namespace)
        if len(cluster_namespaces) > 1 and not any(namespace in pair for pair in namespace_links):
            diagnostics.append(
                _diagnostic(
                    "isolated_cluster",
                    "info",
                    members,
                    f"Namespace {namespace} has no explicit links to another namespace.",
                    {"namespace": namespace, "member_count": len(members)},
                    {"minimum_external_edges": 1},
                )
            )
    return diagnostics


def _lifecycle(nodes: tuple[GraphNode, ...], revisions: GraphRevisions) -> list[GraphDiagnostic]:
    diagnostics = []
    if revisions.stale:
        diagnostics.append(
            _diagnostic(
                "index_stale",
                "error",
                tuple(node.id for node in nodes),
                "Derived index revision does not match the repository revision.",
                {"repository_revision": revisions.repository, "index_revision": revisions.index},
            )
        )
    for node in nodes:
        embedding = node.embedding
        if embedding.status == "error":
            diagnostics.append(
                _diagnostic(
                    "embedding_failed",
                    "warning",
                    (node.id,),
                    "Derived embedding generation failed.",
                    {"status": embedding.status, "error": embedding.error},
                    derived=True,
                )
            )
        elif embedding.status != "ready":
            diagnostics.append(
                _diagnostic(
                    "embedding_missing",
                    "info",
                    (node.id,),
                    "No current ready embedding is available; explicit relationships are unaffected.",
                    {"status": embedding.status},
                    derived=True,
                )
            )
        elif embedding.embedding_revision != revisions.repository:
            diagnostics.append(
                _diagnostic(
                    "embedding_stale",
                    "warning",
                    (node.id,),
                    "Embedding revision differs from the repository revision; explicit relationships are unaffected.",
                    {
                        "embedding_revision": embedding.embedding_revision,
                        "repository_revision": revisions.repository,
                    },
                    derived=True,
                )
            )
        if node.pending_proposal_count:
            diagnostics.append(
                _diagnostic(
                    "pending_proposals",
                    "info",
                    (node.id,),
                    "Memory has pending proposal or review state.",
                    {"pending_proposal_count": node.pending_proposal_count},
                    {"maximum": 0},
                )
            )
    return diagnostics


def _size_outliers(nodes: tuple[GraphNode, ...]) -> list[GraphDiagnostic]:
    diagnostics = []
    by_namespace: dict[str, list[GraphNode]] = defaultdict(list)
    for node in nodes:
        by_namespace[node.namespace].append(node)
    for namespace, members in sorted(by_namespace.items()):
        if len(members) < 4:
            continue
        values = [node.combined_bytes for node in members]
        median = statistics.median(values)
        mad = statistics.median(abs(value - median) for value in values)
        threshold = median + max(1024.0, 6.0 * mad)
        for node in members:
            if node.combined_bytes > threshold:
                diagnostics.append(
                    _diagnostic(
                        "size_outlier",
                        "warning",
                        (node.id,),
                        f"Combined size is an outlier in namespace {namespace}.",
                        {"combined_bytes": node.combined_bytes, "namespace_median": median},
                        {"threshold_bytes": threshold, "mad": mad},
                    )
                )
    return diagnostics


def _tag_drift(nodes: tuple[GraphNode, ...]) -> list[GraphDiagnostic]:
    diagnostics = []
    by_namespace: dict[str, list[GraphNode]] = defaultdict(list)
    for node in nodes:
        by_namespace[node.namespace].append(node)
    for namespace, members in sorted(by_namespace.items()):
        if len(members) < 4:
            continue
        counts = Counter(tag for node in members for tag in node.tags)
        common = {tag for tag, count in counts.items() if count / len(members) >= 0.75}
        if not common:
            continue
        for node in members:
            missing = sorted(common - set(node.tags))
            if missing:
                diagnostics.append(
                    _diagnostic(
                        "tag_drift",
                        "info",
                        (node.id,),
                        f"Memory lacks common namespace tag(s): {', '.join(missing)}.",
                        {"namespace": namespace, "missing_tags": ",".join(missing)},
                        {"common_tag_fraction": 0.75},
                    )
                )
    return diagnostics


def _namespace_outliers(
    nodes: tuple[GraphNode, ...], edges: tuple[GraphEdge, ...]
) -> list[GraphDiagnostic]:
    diagnostics = []
    node_by_id = {node.id: node for node in nodes}
    neighbours: dict[str, list[str]] = defaultdict(list)
    for edge in edges:
        if edge.target in node_by_id and edge.source in node_by_id:
            neighbours[edge.source].append(edge.target)
            neighbours[edge.target].append(edge.source)
    for node in nodes:
        linked = neighbours.get(node.id, [])
        if len(linked) < 3:
            continue
        other = sum(node_by_id[item].namespace != node.namespace for item in linked)
        ratio = other / len(linked)
        if ratio >= 0.8:
            diagnostics.append(
                _diagnostic(
                    "namespace_outlier",
                    "info",
                    (node.id,),
                    "Most explicit neighbours are outside the memory namespace.",
                    {"external_neighbour_fraction": ratio, "explicit_neighbours": len(linked)},
                    {"threshold": 0.8},
                )
            )
    return diagnostics


def _exact_duplicates(content_hashes: Mapping[str, str]) -> list[GraphDiagnostic]:
    groups: dict[str, list[str]] = defaultdict(list)
    for concept_id, content_hash in content_hashes.items():
        groups[content_hash].append(concept_id)
    return [
        _diagnostic(
            "exact_duplicate",
            "warning",
            tuple(sorted(concept_ids)),
            "Memories have identical canonical content hashes.",
            {"content_hash": content_hash, "count": len(concept_ids)},
        )
        for content_hash, concept_ids in sorted(groups.items())
        if len(concept_ids) > 1
    ]


def _diagnostic(
    rule: str,
    severity: str,
    concept_ids: tuple[str, ...],
    message: str,
    measured: dict[str, str | int | float | bool | None],
    threshold: dict[str, str | int | float | bool | None] | None = None,
    *,
    derived: bool = False,
) -> GraphDiagnostic:
    identity = hashlib.sha256((rule + "\0" + "\0".join(sorted(concept_ids))).encode()).hexdigest()[
        :16
    ]
    return GraphDiagnostic(
        id=f"diagnostic:{rule}:{identity}",
        rule=rule,
        severity=severity,  # type: ignore[arg-type]
        concept_ids=tuple(sorted(concept_ids)),
        message=message,
        measured=measured,
        threshold=threshold or {},
        derived=derived,
    )


def _percentile(values: list[int], fraction: float) -> int:
    if not values:
        return 0
    ordered = sorted(values)
    return ordered[round((len(ordered) - 1) * fraction)]
