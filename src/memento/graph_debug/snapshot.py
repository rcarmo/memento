from __future__ import annotations

import hashlib
import json
import math
import sqlite3
from pathlib import Path
from typing import Any

from memento.config import GraphExplorerConfig
from memento.graph_debug.models import (
    GraphAssetSummary,
    GraphEdge,
    GraphEmbeddingState,
    GraphMemoryDetail,
    GraphMetrics,
    GraphNeighbourhood,
    GraphNode,
    GraphOverview,
    GraphPosition,
    GraphProposalSummary,
    GraphRevisions,
)
from memento.repository.frontmatter import parse_concept_file

_PENDING_PROPOSALS = {"draft", "submitted", "approved", "stale"}


class GraphSnapshotError(ValueError):
    """Raised when a graph-debug snapshot request is invalid or unavailable."""


class GraphSnapshotService:
    def __init__(
        self,
        config: GraphExplorerConfig,
        *,
        repository_root: Path,
        derived_db_path: Path,
        control_db_path: Path,
    ) -> None:
        self._config = config
        self._repository_root = repository_root
        self._derived_db_path = derived_db_path
        self._control_db_path = control_db_path

    def overview(self) -> GraphOverview:
        with self._derived() as derived, self._control() as control:
            revisions = self._revisions(derived)
            proposal_counts = self._proposal_counts(control)
            nodes = self._nodes(
                derived,
                proposal_counts=proposal_counts,
                limit=self._config.direct_node_limit + 1,
            )
            truncated = len(nodes) > self._config.direct_node_limit
            visible = nodes[: self._config.direct_node_limit]
            ids = {node.id for node in visible}
            edges = self._edges(derived, ids=ids, limit=self._config.edge_limit)
            return GraphOverview(
                revisions=revisions,
                metrics=GraphMetrics(
                    memory_count=self._scalar(derived, "SELECT COUNT(*) FROM concepts"),
                    markdown_bytes=sum(node.markdown_bytes for node in nodes),
                    asset_bytes=sum(node.asset_bytes for node in nodes),
                    explicit_edges=self._scalar(derived, "SELECT COUNT(*) FROM links"),
                    broken_edges=self._scalar(
                        derived,
                        "SELECT COUNT(*) FROM links WHERE resolution_state != 'resolved'",
                    ),
                    orphan_count=self._scalar(
                        derived,
                        "SELECT COUNT(*) FROM graph_metrics WHERE orphan_flag != 0",
                    ),
                ),
                nodes=tuple(visible),
                edges=tuple(edges),
                truncated=truncated,
            )

    def detail(self, concept_id: str) -> GraphMemoryDetail:
        with self._derived() as derived, self._control() as control:
            revisions = self._revisions(derived)
            proposal_counts = self._proposal_counts(control)
            nodes = self._nodes(
                derived,
                proposal_counts=proposal_counts,
                concept_ids=(concept_id,),
                limit=1,
            )
            if not nodes:
                raise GraphSnapshotError("unknown memory")
            node = nodes[0]
            path = self._repository_path(node.path)
            document = parse_concept_file(path)
            preview = document.body[: self._config.preview_chars]
            outbound = self._edges(
                derived,
                source_id=concept_id,
                limit=self._config.edge_limit,
            )
            inbound = self._edges(
                derived,
                target_id=concept_id,
                limit=self._config.edge_limit,
            )
            return GraphMemoryDetail(
                revisions=revisions,
                node=node,
                preview=preview,
                preview_truncated=len(document.body) > len(preview),
                outbound=tuple(outbound),
                inbound=tuple(inbound),
                assets=self._assets(node.id),
                proposals=self._proposals(control, node.path),
            )

    def neighbourhood(self, concept_id: str, *, depth: int = 1) -> GraphNeighbourhood:
        if depth != 1:
            raise GraphSnapshotError("MVP neighbourhood depth must be 1")
        with self._derived() as derived, self._control() as control:
            if (
                derived.execute("SELECT 1 FROM concepts WHERE id = ?", (concept_id,)).fetchone()
                is None
            ):
                raise GraphSnapshotError("unknown memory")
            rows = derived.execute(
                """
                SELECT source_id AS id FROM links WHERE target_id = ?
                UNION
                SELECT target_id AS id FROM links WHERE source_id = ? AND target_id IS NOT NULL
                UNION SELECT ? AS id
                ORDER BY id LIMIT ?
                """,
                (concept_id, concept_id, concept_id, self._config.expansion_node_limit),
            ).fetchall()
            ids = tuple(str(row["id"]) for row in rows)
            nodes = self._nodes(
                derived,
                proposal_counts=self._proposal_counts(control),
                concept_ids=ids,
                limit=self._config.expansion_node_limit,
            )
            visible = {node.id for node in nodes}
            return GraphNeighbourhood(
                revisions=self._revisions(derived),
                center_id=concept_id,
                nodes=tuple(nodes),
                edges=tuple(self._edges(derived, ids=visible, limit=self._config.edge_limit)),
            )

    def _nodes(
        self,
        connection: sqlite3.Connection,
        *,
        proposal_counts: dict[str, tuple[int, int]],
        limit: int,
        concept_ids: tuple[str, ...] | None = None,
    ) -> list[GraphNode]:
        conditions = ""
        parameters: list[object] = []
        if concept_ids is not None:
            if not concept_ids:
                return []
            conditions = f"WHERE c.id IN ({','.join('?' for _ in concept_ids)})"
            parameters.extend(concept_ids)
        parameters.append(limit)
        rows = connection.execute(
            f"""
            SELECT c.id, c.path, c.title, c.type, c.status, c.tags_json, c.updated_at,
                   c.repo_revision, c.body,
                   COALESCE(g.inbound_degree, 0) AS inbound_degree,
                   COALESCE(g.outbound_degree, 0) AS outbound_degree,
                   COALESCE(g.broken_link_count, 0) AS broken_link_count,
                   COALESCE(g.orphan_flag, 0) AS orphan_flag,
                   e.status AS embedding_status, e.model_id, e.dimensions,
                   e.embedding_revision, e.model_revision, e.updated_at AS embedding_updated_at,
                   e.error_message
              FROM concepts AS c
              LEFT JOIN graph_metrics AS g ON g.concept_id = c.id
              LEFT JOIN concept_embeddings AS e ON e.concept_id = c.id
              {conditions}
             ORDER BY c.id
             LIMIT ?
            """,
            parameters,
        ).fetchall()
        result: list[GraphNode] = []
        for row in rows:
            path = str(row["path"])
            markdown_bytes = self._file_size(path)
            assets = self._assets(str(row["id"]))
            asset_bytes = sum(item.metadata_bytes + item.payload_bytes for item in assets)
            proposal_count, pending_count = proposal_counts.get(path, (0, 0))
            result.append(
                GraphNode(
                    id=str(row["id"]),
                    path=path,
                    title=str(row["title"]),
                    type=str(row["type"]),
                    status=str(row["status"]),
                    tags=tuple(json.loads(row["tags_json"])),
                    namespace=self._namespace(path),
                    updated_at=str(row["updated_at"]),
                    updated_by=self._updated_by(path),
                    markdown_bytes=markdown_bytes,
                    asset_bytes=asset_bytes,
                    combined_bytes=markdown_bytes + asset_bytes,
                    explicit_in_degree=int(row["inbound_degree"]),
                    explicit_out_degree=int(row["outbound_degree"]),
                    broken_link_count=int(row["broken_link_count"]),
                    orphan=bool(row["orphan_flag"]),
                    proposal_count=proposal_count,
                    pending_proposal_count=pending_count,
                    embedding=GraphEmbeddingState(
                        status=str(row["embedding_status"] or "missing"),
                        model_id=row["model_id"],
                        dimensions=row["dimensions"],
                        embedding_revision=row["embedding_revision"],
                        model_revision=row["model_revision"],
                        updated_at=row["embedding_updated_at"],
                        error=row["error_message"],
                    ),
                    coarse_position=self._position(str(row["id"])),
                )
            )
        return result

    def _edges(
        self,
        connection: sqlite3.Connection,
        *,
        limit: int,
        ids: set[str] | None = None,
        source_id: str | None = None,
        target_id: str | None = None,
    ) -> list[GraphEdge]:
        clauses: list[str] = []
        parameters: list[object] = []
        if source_id is not None:
            clauses.append("source_id = ?")
            parameters.append(source_id)
        if target_id is not None:
            clauses.append("target_id = ?")
            parameters.append(target_id)
        rows = connection.execute(
            "SELECT rowid, * FROM links"
            + (f" WHERE {' AND '.join(clauses)}" if clauses else "")
            + " ORDER BY source_id, raw_target, rowid LIMIT ?",
            (*parameters, limit),
        ).fetchall()
        result = []
        for row in rows:
            source = str(row["source_id"])
            target = str(row["target_id"]) if row["target_id"] is not None else None
            if ids is not None and (source not in ids or target is not None and target not in ids):
                continue
            result.append(
                GraphEdge(
                    id=f"explicit:{row['rowid']}",
                    source=source,
                    target=target,
                    raw_target=str(row["raw_target"]),
                    resolution=str(row["resolution_state"]),
                    anchor=row["anchor"],
                    first_seen_revision=str(row["first_seen_revision"]),
                    last_checked_revision=str(row["last_checked_revision"]),
                )
            )
        return result

    def _proposal_counts(self, connection: sqlite3.Connection) -> dict[str, tuple[int, int]]:
        counts: dict[str, list[int]] = {}
        for row in connection.execute(
            "SELECT status, patch_json FROM proposals ORDER BY proposal_id"
        ):
            for path in self._proposal_paths(str(row["patch_json"])):
                values = counts.setdefault(path, [0, 0])
                values[0] += 1
                if str(row["status"]) in _PENDING_PROPOSALS:
                    values[1] += 1
        return {path: (values[0], values[1]) for path, values in counts.items()}

    def _proposals(
        self, connection: sqlite3.Connection, concept_path: str
    ) -> tuple[GraphProposalSummary, ...]:
        summaries = []
        for row in connection.execute("SELECT * FROM proposals ORDER BY created_at, proposal_id"):
            if concept_path not in self._proposal_paths(str(row["patch_json"])):
                continue
            summaries.append(
                GraphProposalSummary(
                    proposal_id=str(row["proposal_id"]),
                    author=str(row["author_principal"]),
                    status=str(row["status"]),
                    intent=str(row["intent"]),
                    base_revision=str(row["base_revision"]),
                    applied_revision=row["applied_revision"],
                    created_at=str(row["created_at"]),
                    updated_at=str(row["updated_at"]),
                )
            )
        return tuple(summaries[: self._config.overview_cluster_limit])

    def _assets(self, concept_id: str) -> tuple[GraphAssetSummary, ...]:
        directory = self._repository_root / ".assets" / concept_id
        if not directory.is_dir():
            return ()
        result = []
        for metadata_file in sorted(directory.glob("*/*.json")):
            try:
                payload = json.loads(metadata_file.read_text(encoding="utf-8"))
                zip_file = metadata_file.with_suffix(".zip")
                result.append(
                    GraphAssetSummary(
                        asset_kind=str(payload.get("asset_kind") or metadata_file.parent.name),
                        version=str(payload.get("version") or metadata_file.stem),
                        metadata_bytes=metadata_file.stat().st_size,
                        payload_bytes=zip_file.stat().st_size if zip_file.is_file() else 0,
                        source_proposal_id=(
                            str(payload["source_proposal_id"])
                            if payload.get("source_proposal_id")
                            else None
                        ),
                    )
                )
            except (OSError, ValueError, TypeError):
                continue
        return tuple(result[: self._config.overview_cluster_limit])

    def _revisions(self, connection: sqlite3.Connection) -> GraphRevisions:
        state = {
            str(row["key"]): str(row["value"])
            for row in connection.execute(
                "SELECT key, value FROM index_state WHERE key IN "
                "('repo_revision','index_revision','semantic_embedding_revision')"
            )
        }
        repository = state.get("repo_revision", "")
        index = state.get("index_revision", "")
        embedding = state.get("semantic_embedding_revision") or None
        return GraphRevisions(
            repository=repository,
            index=index,
            embedding=embedding,
            stale=repository != index,
        )

    def _repository_path(self, bundle_path: str) -> Path:
        candidate = (self._repository_root / bundle_path.removeprefix("/")).resolve()
        root = self._repository_root.resolve()
        if not candidate.is_relative_to(root) or not candidate.is_file():
            raise GraphSnapshotError("memory file unavailable")
        return candidate

    def _file_size(self, bundle_path: str) -> int:
        try:
            return self._repository_path(bundle_path).stat().st_size
        except (GraphSnapshotError, OSError):
            return 0

    def _updated_by(self, bundle_path: str) -> str | None:
        try:
            return parse_concept_file(self._repository_path(bundle_path)).frontmatter.updated_by
        except (GraphSnapshotError, OSError, ValueError):
            return None

    @staticmethod
    def _namespace(path: str) -> str:
        parts = [part for part in path.split("/") if part]
        return f"/{parts[0]}/" if parts else "/"

    @staticmethod
    def _position(concept_id: str) -> GraphPosition:
        digest = hashlib.sha256(concept_id.encode("utf-8")).digest()
        angle = int.from_bytes(digest[:8], "big") / 2**64 * math.tau
        radius = 0.5 + int.from_bytes(digest[8:12], "big") / 2**32 * 0.5
        z = int.from_bytes(digest[12:16], "big") / 2**32 * 2 - 1
        return GraphPosition(x=math.cos(angle) * radius, y=math.sin(angle) * radius, z=z)

    @staticmethod
    def _proposal_paths(patch_json: str) -> tuple[str, ...]:
        try:
            payload = json.loads(patch_json)
        except json.JSONDecodeError:
            return ()
        found: set[str] = set()

        def visit(value: Any) -> None:
            if isinstance(value, dict):
                for key, item in value.items():
                    if key in {"path", "new_path", "concept_path"} and isinstance(item, str):
                        found.add(item)
                    visit(item)
            elif isinstance(value, list):
                for item in value:
                    visit(item)

        visit(payload)
        return tuple(sorted(found))

    def _derived(self) -> sqlite3.Connection:
        connection = sqlite3.connect(f"file:{self._derived_db_path}?mode=ro", uri=True)
        connection.row_factory = sqlite3.Row
        return connection

    def _control(self) -> sqlite3.Connection:
        connection = sqlite3.connect(f"file:{self._control_db_path}?mode=ro", uri=True)
        connection.row_factory = sqlite3.Row
        return connection

    @staticmethod
    def _scalar(connection: sqlite3.Connection, query: str) -> int:
        row = connection.execute(query).fetchone()
        return int(row[0] or 0)
