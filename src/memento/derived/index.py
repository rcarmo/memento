from __future__ import annotations

import hashlib
import json
import re
import shutil
import sqlite3
import time
from collections.abc import Callable, Sequence
from contextlib import suppress
from dataclasses import dataclass
from enum import StrEnum
from pathlib import Path
from tempfile import TemporaryDirectory
from threading import Lock

from memento.authz import EffectivePolicy
from memento.config import SemanticSearchConfig
from memento.repository.bundle import scan_bundle
from memento.repository.frontmatter import parse_concept_file
from memento.repository.links import extract_structural_links
from memento.repository.paths import is_reserved_bundle_path
from memento.repository.schema import ConceptDocument
from memento.semantic import (
    EmbeddingClient,
    EmbeddingModelInfo,
    SemanticSearchError,
    ValidatedEmbedding,
    cosine_similarity,
    embedding_content_hash,
    embedding_text,
    pack_f32le,
    unpack_f32le,
    validate_embedding,
)

SCHEMA_VERSION = "2"
DEFAULT_SEARCH_LIMIT = 20
MAX_SEARCH_LIMIT = 100
MAX_GRAPH_DEPTH = 2
POLL_INTERVAL_SECONDS = 0.02
RRF_K = 60

MIGRATIONS = (
    """
    CREATE TABLE IF NOT EXISTS concepts (
        id TEXT PRIMARY KEY,
        path TEXT NOT NULL UNIQUE,
        type TEXT NOT NULL,
        title TEXT NOT NULL,
        description TEXT,
        status TEXT NOT NULL,
        tags_json TEXT NOT NULL,
        aliases_json TEXT NOT NULL,
        body TEXT NOT NULL,
        content_hash TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        repo_revision TEXT NOT NULL
    )
    """,
    """
    CREATE VIRTUAL TABLE IF NOT EXISTS concept_fts USING fts5(
        concept_id UNINDEXED,
        title,
        description,
        aliases,
        tags,
        body,
        path,
        tokenize='unicode61'
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS links (
        source_id TEXT NOT NULL,
        target_id TEXT,
        raw_target TEXT NOT NULL,
        target_path TEXT,
        anchor TEXT,
        link_kind TEXT NOT NULL,
        resolution_state TEXT NOT NULL,
        first_seen_revision TEXT NOT NULL,
        last_checked_revision TEXT NOT NULL
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS graph_metrics (
        concept_id TEXT PRIMARY KEY,
        inbound_degree INTEGER NOT NULL,
        outbound_degree INTEGER NOT NULL,
        broken_link_count INTEGER NOT NULL,
        orphan_flag INTEGER NOT NULL
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS index_state (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL,
        updated_at TEXT NOT NULL
    )
    """,
    "CREATE INDEX IF NOT EXISTS idx_concepts_path ON concepts(path)",
    "CREATE INDEX IF NOT EXISTS idx_concepts_type ON concepts(type)",
    "CREATE INDEX IF NOT EXISTS idx_concepts_status ON concepts(status)",
    "CREATE INDEX IF NOT EXISTS idx_links_source ON links(source_id)",
    "CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_id)",
    """
    CREATE TABLE IF NOT EXISTS concept_embeddings (
        concept_id TEXT PRIMARY KEY,
        path TEXT NOT NULL,
        embedding_text_hash TEXT NOT NULL,
        model_id TEXT NOT NULL,
        dimensions INTEGER NOT NULL,
        embedding_revision TEXT NOT NULL,
        status TEXT NOT NULL,
        model_revision TEXT NOT NULL,
        embedding_blob BLOB,
        embedding_norm REAL,
        updated_at TEXT NOT NULL,
        error_message TEXT
    )
    """,
    "CREATE INDEX IF NOT EXISTS idx_concept_embeddings_path ON concept_embeddings(path)",
    "CREATE INDEX IF NOT EXISTS idx_concept_embeddings_status ON concept_embeddings(status)",
)


class DerivedIndexCorruptionError(RuntimeError):
    """Raised when the derived database cannot be used safely."""


class DerivedSearchError(ValueError):
    """Raised when a user-supplied search query is invalid."""


class SearchFreshness(StrEnum):
    EVENTUAL = "eventual"
    STRICT = "strict"


class SearchMode(StrEnum):
    LEXICAL = "lexical"
    SEMANTIC = "semantic"
    HYBRID = "hybrid"


@dataclass(frozen=True, slots=True)
class DerivedIndexState:
    repo_revision: str
    index_revision: str
    schema_version: str
    status: str
    quarantine_path: str | None


@dataclass(frozen=True, slots=True)
class SearchResult:
    concept_id: str
    path: str
    title: str
    concept_type: str
    status: str
    tags: tuple[str, ...]
    score: float
    snippet: str


@dataclass(frozen=True, slots=True)
class SearchPage:
    results: tuple[SearchResult, ...]
    next_cursor: str | None
    repo_revision: str
    index_revision: str
    warnings: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class GraphEdge:
    concept_id: str
    path: str
    title: str
    depth: int
    direction: str
    broken_link_count: int
    orphan_flag: bool


@dataclass(frozen=True, slots=True)
class GraphNeighborhood:
    center_id: str
    outbound: tuple[GraphEdge, ...]
    inbound: tuple[GraphEdge, ...]
    broken_targets: tuple[str, ...]
    repo_revision: str
    index_revision: str


@dataclass(frozen=True, slots=True)
class GraphMetrics:
    concept_id: str
    inbound_degree: int
    outbound_degree: int
    broken_link_count: int
    orphan_flag: bool


@dataclass(frozen=True, slots=True)
class ParityReport:
    matches: bool
    expected_revision: str
    current_revision: str
    details: str


@dataclass(frozen=True, slots=True)
class DerivedStatusSnapshot:
    state: DerivedIndexState
    visible_concepts: int


@dataclass(frozen=True, slots=True)
class SemanticStatus:
    enabled: bool
    ready: bool
    model_id: str | None
    dimensions: int | None
    embedding_revision: str | None
    sqlite_vector_enabled: bool
    warnings: tuple[str, ...] = ()


class DerivedIndex:
    def __init__(
        self,
        db_path: Path,
        *,
        semantic_config: SemanticSearchConfig | None = None,
        embedding_client: EmbeddingClient | None = None,
        defer_embeddings: bool = False,
    ) -> None:
        self._db_path = db_path
        self._semantic_config = semantic_config or SemanticSearchConfig()
        self._embedding_client = embedding_client
        self._defer_embeddings = defer_embeddings
        self._sqlite_vector_enabled = False
        self._sqlite_vector_warning: str | None = None
        self._initialized_identity: tuple[int, int] | None = None
        self._initialization_lock = Lock()

    @property
    def db_path(self) -> Path:
        return self._db_path

    def rebuild(self, bundle_root: Path, *, repo_revision: str) -> None:
        self._with_quarantine(
            lambda connection: self._rebuild(connection, bundle_root, repo_revision)
        )

    def update_paths(
        self,
        bundle_root: Path,
        *,
        repo_revision: str,
        changed_paths: tuple[str, ...],
    ) -> None:
        def run(connection: sqlite3.Connection) -> None:
            self._ensure_ready(connection)
            changed_documents: list[tuple[str, object]] = []
            with connection:
                self._set_state(connection, "repo_revision", repo_revision)
                for bundle_path in sorted(dict.fromkeys(changed_paths)):
                    self._delete_path(connection, bundle_path)
                    absolute = bundle_root / bundle_path.removeprefix("/")
                    if (
                        bundle_path.endswith(".md")
                        and absolute.exists()
                        and absolute.is_file()
                        and not is_reserved_bundle_path(bundle_path)
                    ):
                        document = parse_concept_file(absolute)
                        changed_documents.append((bundle_path, document))
                        self._upsert_entry(connection, bundle_path, document, repo_revision)
                connection.execute("UPDATE concepts SET repo_revision = ?", (repo_revision,))
                self._recompute_links(connection, repo_revision)
                self._recompute_metrics(connection)
                if not self._defer_embeddings:
                    self._update_embeddings(
                        connection,
                        repo_revision=repo_revision,
                        changed_documents=tuple(changed_documents),
                        full_rebuild=False,
                    )
                self._set_state(connection, "index_revision", repo_revision)
                self._set_state(connection, "status", "ready")

        self._with_quarantine(run)

    def refresh_embeddings(self, bundle_root: Path, *, repo_revision: str) -> None:
        bundle = scan_bundle(bundle_root)
        self._refresh_embedding_documents(
            bundle_root,
            repo_revision=repo_revision,
            changed_documents=tuple(
                (entry.bundle_path, entry.document) for entry in bundle.entries
            ),
            full_rebuild=False,
        )

    def refresh_embedding_paths(
        self,
        bundle_root: Path,
        *,
        repo_revision: str,
        paths: tuple[str, ...],
    ) -> None:
        unique_paths = tuple(sorted(dict.fromkeys(paths)))
        changed_documents: list[tuple[str, object]] = []
        for bundle_path in unique_paths:
            absolute = bundle_root / bundle_path.removeprefix("/")
            if (
                not bundle_path.startswith("/")
                or not absolute.is_file()
                or not bundle_path.endswith(".md")
            ):
                raise ValueError(f"embedding refresh path is unavailable: {bundle_path}")
            changed_documents.append((bundle_path, parse_concept_file(absolute)))
        self._refresh_embedding_documents(
            bundle_root,
            repo_revision=repo_revision,
            changed_documents=tuple(changed_documents),
            full_rebuild=False,
        )

    def _refresh_embedding_documents(
        self,
        bundle_root: Path,
        *,
        repo_revision: str,
        changed_documents: tuple[tuple[str, object], ...],
        full_rebuild: bool,
    ) -> None:
        del bundle_root

        def run(connection: sqlite3.Connection) -> None:
            self._ensure_ready(connection)
            with connection:
                self._update_embeddings(
                    connection,
                    repo_revision=repo_revision,
                    changed_documents=changed_documents,
                    full_rebuild=full_rebuild,
                )

        self._with_quarantine(run)

    def search(
        self,
        *,
        policy: EffectivePolicy,
        query: str,
        concept_type: str | None = None,
        tags: tuple[str, ...] = (),
        status: str | None = None,
        path_prefix: str | None = None,
        limit: int = DEFAULT_SEARCH_LIMIT,
        cursor: str | None = None,
        freshness: SearchFreshness = SearchFreshness.EVENTUAL,
        timeout_seconds: float = 1.0,
        search_mode: SearchMode = SearchMode.LEXICAL,
        query_syntax: str = "fts5",
    ) -> SearchPage:
        if freshness is SearchFreshness.STRICT:
            self.wait_for_freshness(timeout_seconds=timeout_seconds)
        bounded_limit = max(1, min(limit, MAX_SEARCH_LIMIT))
        offset = _decode_cursor(cursor)
        try:
            with self._connect() as connection:
                self._ensure_ready(connection)
                state = DerivedIndexState(
                    repo_revision=self._get_state(connection, "repo_revision"),
                    index_revision=self._get_state(connection, "index_revision"),
                    schema_version=self._get_state(connection, "schema_version"),
                    status=self._get_state(connection, "status"),
                    quarantine_path=self._get_state_optional(connection, "quarantine_path"),
                )
                lexical_query = _lexical_query(query, query_syntax=query_syntax)
                lexical_rows = self._search_lexical_rows(
                    connection,
                    policy=policy,
                    query=lexical_query,
                    concept_type=concept_type,
                    tags=tags,
                    status=status,
                    path_prefix=path_prefix,
                    limit=max(bounded_limit + offset + 1, self._semantic_config.max_candidates),
                    offset=0,
                )
                warnings: list[str] = []
                if search_mode is SearchMode.LEXICAL:
                    rows = lexical_rows[offset : offset + bounded_limit + 1]
                else:
                    try:
                        rows = self._search_semantic_rows(
                            connection,
                            policy=policy,
                            query=query,
                            concept_type=concept_type,
                            tags=tags,
                            status=status,
                            path_prefix=path_prefix,
                            limit=bounded_limit,
                            offset=offset,
                            search_mode=search_mode,
                            lexical_rows=lexical_rows,
                        )
                    except SemanticSearchError as exc:
                        warnings.append(f"semantic_search_unavailable: {exc}")
                        rows = lexical_rows[offset : offset + bounded_limit + 1]
        except sqlite3.OperationalError as exc:
            if self._is_search_query_error(exc):
                raise DerivedSearchError("invalid FTS query") from exc
            self._handle_corruption(exc)
        except sqlite3.DatabaseError as exc:
            self._handle_corruption(exc)
        page_rows = rows[:bounded_limit]
        next_cursor = _encode_cursor(offset + bounded_limit) if len(rows) > bounded_limit else None
        return SearchPage(
            results=tuple(self._row_to_search_result(row) for row in page_rows),
            next_cursor=next_cursor,
            repo_revision=state.repo_revision,
            index_revision=state.index_revision,
            warnings=tuple(warnings),
        )

    def graph(
        self,
        *,
        policy: EffectivePolicy,
        concept_id: str,
        depth: int = 1,
        freshness: SearchFreshness = SearchFreshness.EVENTUAL,
        timeout_seconds: float = 1.0,
    ) -> GraphNeighborhood:
        bounded_depth = max(0, min(depth, MAX_GRAPH_DEPTH))
        if freshness is SearchFreshness.STRICT:
            self.wait_for_freshness(timeout_seconds=timeout_seconds)
        try:
            with self._connect() as connection:
                self._ensure_ready(connection)
                state = DerivedIndexState(
                    repo_revision=self._get_state(connection, "repo_revision"),
                    index_revision=self._get_state(connection, "index_revision"),
                    schema_version=self._get_state(connection, "schema_version"),
                    status=self._get_state(connection, "status"),
                    quarantine_path=self._get_state_optional(connection, "quarantine_path"),
                )
                self._authorize_concept_id(connection, policy, concept_id)
                outbound = self._collect_neighbors(
                    connection, policy, concept_id, "outbound", bounded_depth
                )
                inbound = self._collect_neighbors(
                    connection, policy, concept_id, "inbound", bounded_depth
                )
                broken = tuple(
                    row[0]
                    for row in connection.execute(
                        "SELECT DISTINCT raw_target "
                        "FROM links "
                        "WHERE source_id = ? AND target_id IS NULL "
                        "ORDER BY raw_target",
                        (concept_id,),
                    ).fetchall()
                )
        except sqlite3.DatabaseError as exc:
            self._handle_corruption(exc)
        return GraphNeighborhood(
            center_id=concept_id,
            outbound=outbound,
            inbound=inbound,
            broken_targets=broken,
            repo_revision=state.repo_revision,
            index_revision=state.index_revision,
        )

    def metrics(self, concept_id: str) -> GraphMetrics:
        try:
            with self._connect() as connection:
                row = connection.execute(
                    """
                    SELECT concept_id, inbound_degree, outbound_degree,
                           broken_link_count, orphan_flag
                    FROM graph_metrics
                    WHERE concept_id = ?
                    """,
                    (concept_id,),
                ).fetchone()
        except sqlite3.DatabaseError as exc:
            self._handle_corruption(exc)
        if row is None:
            raise KeyError(concept_id)
        return GraphMetrics(
            concept_id=row["concept_id"],
            inbound_degree=int(row["inbound_degree"]),
            outbound_degree=int(row["outbound_degree"]),
            broken_link_count=int(row["broken_link_count"]),
            orphan_flag=bool(row["orphan_flag"]),
        )

    def status_snapshot(self, policy: EffectivePolicy) -> DerivedStatusSnapshot:
        try:
            with self._connect() as connection:
                state = DerivedIndexState(
                    repo_revision=self._get_state(connection, "repo_revision"),
                    index_revision=self._get_state(connection, "index_revision"),
                    schema_version=self._get_state(connection, "schema_version"),
                    status=self._get_state(connection, "status"),
                    quarantine_path=self._get_state_optional(connection, "quarantine_path"),
                )
                prefixes = _authorized_prefix_conditions(policy.read_prefixes)
                row = connection.execute(
                    f"SELECT COUNT(*) AS total FROM concepts AS c WHERE {prefixes.sql}",
                    prefixes.parameters,
                ).fetchone()
                return DerivedStatusSnapshot(state=state, visible_concepts=int(row["total"] or 0))
        except sqlite3.DatabaseError as exc:
            self._handle_corruption(exc)
        raise AssertionError("unreachable")

    def get_state(self) -> DerivedIndexState:
        try:
            with self._connect() as connection:
                self._migrate(connection)
                return DerivedIndexState(
                    repo_revision=self._get_state(connection, "repo_revision"),
                    index_revision=self._get_state(connection, "index_revision"),
                    schema_version=self._get_state(connection, "schema_version"),
                    status=self._get_state(connection, "status"),
                    quarantine_path=self._get_state_optional(connection, "quarantine_path"),
                )
        except sqlite3.DatabaseError as exc:
            self._handle_corruption(exc)
        raise AssertionError("unreachable")

    def set_repo_revision(self, repo_revision: str) -> None:
        with self._connect() as connection:
            self._migrate(connection)
            with connection:
                self._set_state(connection, "repo_revision", repo_revision)

    def semantic_status(self) -> SemanticStatus:
        if not self._semantic_config.enabled:
            return SemanticStatus(
                enabled=False,
                ready=False,
                model_id=None,
                dimensions=None,
                embedding_revision=None,
                sqlite_vector_enabled=False,
            )
        with self._connect() as connection:
            self._migrate(connection)
            row = connection.execute(
                "SELECT COUNT(*) AS total, "
                "SUM(CASE WHEN status = 'ready' THEN 1 ELSE 0 END) AS ready_count "
                "FROM concept_embeddings"
            ).fetchone()
            concept_count = int(
                connection.execute("SELECT COUNT(*) FROM concepts").fetchone()[0] or 0
            )
            repo_revision = self._get_state(connection, "repo_revision")
            revision_raw = self._get_state_optional(connection, "semantic_embedding_revision")
        ready_count = int(row["ready_count"] or 0)
        missing_count = max(0, concept_count - ready_count)
        revision = revision_raw or None
        warnings = [item for item in (self._sqlite_vector_warning,) if item is not None]
        if self._embedding_client is None:
            warnings.append("semantic_embedding_client_unavailable")
        if missing_count:
            warnings.append(
                f"semantic_embeddings_degraded: {missing_count} of {concept_count} embeddings not ready"
            )
        return SemanticStatus(
            enabled=True,
            ready=(
                self._embedding_client is not None
                and concept_count == ready_count
                and revision == repo_revision
            ),
            model_id=self._semantic_config.model_id,
            dimensions=self._semantic_config.dimensions,
            embedding_revision=revision,
            sqlite_vector_enabled=self._sqlite_vector_enabled,
            warnings=tuple(warnings),
        )

    def wait_for_freshness(self, *, timeout_seconds: float) -> DerivedIndexState:
        deadline = time.monotonic() + timeout_seconds
        while True:
            state = self.get_state()
            if state.index_revision == state.repo_revision:
                return state
            if time.monotonic() >= deadline:
                raise TimeoutError(
                    "derived index is stale: "
                    f"repo_revision={state.repo_revision} "
                    f"index_revision={state.index_revision}"
                )
            time.sleep(POLL_INTERVAL_SECONDS)

    def parity_check(self, bundle_root: Path, *, repo_revision: str) -> ParityReport:
        with TemporaryDirectory() as temp_dir:
            clean_index = DerivedIndex(Path(temp_dir) / "derived.sqlite")
            clean_index.rebuild(bundle_root, repo_revision=repo_revision)
            clean_dump = clean_index._normalized_dump()
        current_dump = self._normalized_dump()
        return ParityReport(
            matches=clean_dump == current_dump,
            expected_revision=repo_revision,
            current_revision=self.get_state().index_revision,
            details="match" if clean_dump == current_dump else "normalized derived state differs",
        )

    def _normalized_dump(self) -> dict[str, object]:
        with self._connect() as connection:
            self._ensure_ready(connection)
            concepts = [
                dict(row)
                for row in connection.execute(
                    "SELECT id, path, type, title, description, status, "
                    "tags_json, aliases_json, body, content_hash, updated_at, "
                    "repo_revision FROM concepts ORDER BY path"
                ).fetchall()
            ]
            links = [
                dict(row)
                for row in connection.execute(
                    "SELECT source_id, target_id, raw_target, target_path, "
                    "anchor, link_kind, resolution_state "
                    "FROM links ORDER BY source_id, raw_target, anchor"
                ).fetchall()
            ]
            metrics = [
                dict(row)
                for row in connection.execute(
                    "SELECT concept_id, inbound_degree, outbound_degree, "
                    "broken_link_count, orphan_flag "
                    "FROM graph_metrics ORDER BY concept_id"
                ).fetchall()
            ]
            embeddings = [
                dict(row)
                for row in connection.execute(
                    "SELECT concept_id, path, embedding_text_hash, model_id, dimensions, "
                    "embedding_revision, status, model_revision, embedding_norm, error_message "
                    "FROM concept_embeddings ORDER BY path"
                ).fetchall()
            ]
        return {"concepts": concepts, "links": links, "metrics": metrics, "embeddings": embeddings}

    def _rebuild(
        self, connection: sqlite3.Connection, bundle_root: Path, repo_revision: str
    ) -> None:
        bundle = scan_bundle(bundle_root)
        self._migrate(connection)
        with connection:
            for table in (
                "concepts",
                "concept_fts",
                "links",
                "graph_metrics",
                "concept_embeddings",
            ):
                connection.execute(f"DELETE FROM {table}")
            self._set_state(connection, "status", "rebuilding")
            self._set_state(connection, "repo_revision", repo_revision)
            changed_documents: list[tuple[str, object]] = []
            for entry in bundle.entries:
                changed_documents.append((entry.bundle_path, entry.document))
                self._upsert_entry(connection, entry.bundle_path, entry.document, repo_revision)
            self._recompute_links(connection, repo_revision)
            self._recompute_metrics(connection)
            if not self._defer_embeddings:
                self._update_embeddings(
                    connection,
                    repo_revision=repo_revision,
                    changed_documents=tuple(changed_documents),
                    full_rebuild=True,
                )
            self._set_state(connection, "index_revision", repo_revision)
            self._set_state(connection, "status", "ready")

    def _upsert_entry(
        self,
        connection: sqlite3.Connection,
        bundle_path: str,
        document: object,
        repo_revision: str,
    ) -> None:
        from memento.repository.schema import ConceptDocument

        if not isinstance(document, ConceptDocument):
            raise TypeError("document must be a ConceptDocument")
        frontmatter = document.frontmatter
        tags_json = json.dumps(frontmatter.tags)
        aliases_json = json.dumps(frontmatter.aliases)
        with connection:
            connection.execute(
                """
                INSERT INTO concepts(
                    id, path, type, title, description, status,
                    tags_json, aliases_json, body, content_hash, updated_at, repo_revision
                )
                VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(id) DO UPDATE SET
                    path=excluded.path,
                    type=excluded.type,
                    title=excluded.title,
                    description=excluded.description,
                    status=excluded.status,
                    tags_json=excluded.tags_json,
                    aliases_json=excluded.aliases_json,
                    body=excluded.body,
                    content_hash=excluded.content_hash,
                    updated_at=excluded.updated_at,
                    repo_revision=excluded.repo_revision
                """,
                (
                    frontmatter.id,
                    bundle_path,
                    frontmatter.type,
                    frontmatter.title,
                    frontmatter.description,
                    frontmatter.status,
                    tags_json,
                    aliases_json,
                    document.body,
                    hashlib.sha256(document.body.encode("utf-8")).hexdigest(),
                    frontmatter.updated_at.isoformat().replace("+00:00", "Z"),
                    repo_revision,
                ),
            )
            connection.execute("DELETE FROM concept_fts WHERE concept_id = ?", (frontmatter.id,))
            connection.execute(
                "INSERT INTO concept_fts("
                "concept_id, title, description, aliases, tags, body, path"
                ") VALUES(?, ?, ?, ?, ?, ?, ?)",
                (
                    frontmatter.id,
                    frontmatter.title,
                    frontmatter.description or "",
                    " ".join(frontmatter.aliases),
                    " ".join(frontmatter.tags),
                    document.body,
                    bundle_path,
                ),
            )

    def _delete_path(self, connection: sqlite3.Connection, bundle_path: str) -> None:
        row = connection.execute(
            "SELECT id FROM concepts WHERE path = ?", (bundle_path,)
        ).fetchone()
        if row is None:
            return
        concept_id = row["id"]
        connection.execute("DELETE FROM concept_fts WHERE concept_id = ?", (concept_id,))
        connection.execute(
            "DELETE FROM links WHERE source_id = ? OR target_id = ?", (concept_id, concept_id)
        )
        connection.execute("DELETE FROM graph_metrics WHERE concept_id = ?", (concept_id,))
        connection.execute("DELETE FROM concept_embeddings WHERE concept_id = ?", (concept_id,))
        connection.execute("DELETE FROM concepts WHERE id = ?", (concept_id,))

    def _recompute_links(self, connection: sqlite3.Connection, repo_revision: str) -> None:
        connection.execute("DELETE FROM links")
        concepts = connection.execute(
            "SELECT id, path, body FROM concepts ORDER BY path"
        ).fetchall()
        path_to_id = {row["path"]: row["id"] for row in concepts}
        for row in concepts:
            for link in extract_structural_links(row["body"]):
                target_path, anchor = _split_target(link.href)
                target_id = path_to_id.get(target_path) if target_path.startswith("/") else None
                resolution_state = "resolved" if target_id is not None else "broken"
                connection.execute(
                    """
                    INSERT INTO links(
                        source_id, target_id, raw_target, target_path, anchor,
                        link_kind, resolution_state, first_seen_revision, last_checked_revision
                    )
                    VALUES(?, ?, ?, ?, ?, 'markdown', ?, ?, ?)
                    """,
                    (
                        row["id"],
                        target_id,
                        link.href,
                        target_path if target_path.startswith("/") else None,
                        anchor,
                        resolution_state,
                        repo_revision,
                        repo_revision,
                    ),
                )

    def _recompute_metrics(self, connection: sqlite3.Connection) -> None:
        connection.execute("DELETE FROM graph_metrics")
        concept_ids = [
            row[0] for row in connection.execute("SELECT id FROM concepts ORDER BY id").fetchall()
        ]
        for concept_id in concept_ids:
            outbound = connection.execute(
                "SELECT COUNT(*) FROM links WHERE source_id = ? AND target_path IS NOT NULL",
                (concept_id,),
            ).fetchone()[0]
            inbound = connection.execute(
                "SELECT COUNT(*) FROM links WHERE target_id = ?",
                (concept_id,),
            ).fetchone()[0]
            broken = connection.execute(
                "SELECT COUNT(*) FROM links WHERE source_id = ? AND target_id IS NULL",
                (concept_id,),
            ).fetchone()[0]
            orphan = 1 if inbound == 0 and outbound == 0 else 0
            connection.execute(
                "INSERT INTO graph_metrics("
                "concept_id, inbound_degree, outbound_degree, broken_link_count, orphan_flag"
                ") VALUES(?, ?, ?, ?, ?)",
                (concept_id, inbound, outbound, broken, orphan),
            )

    def _update_embeddings(
        self,
        connection: sqlite3.Connection,
        *,
        repo_revision: str,
        changed_documents: tuple[tuple[str, object], ...],
        full_rebuild: bool,
    ) -> None:
        if not self._semantic_config.enabled:
            self._set_state(connection, "semantic_embedding_revision", "disabled")
            return
        if self._embedding_client is None:
            self._set_state(connection, "semantic_embedding_revision", "unavailable")
            return
        model_info = self._embedding_client.model_info()
        self._validate_model_info(model_info)
        stale_rows = connection.execute(
            "SELECT concept_id FROM concept_embeddings WHERE model_id != ? OR dimensions != ? OR model_revision != ?",
            (self._semantic_config.model_id, self._semantic_config.dimensions, model_info.revision),
        ).fetchall()
        for row in stale_rows:
            connection.execute(
                "UPDATE concept_embeddings SET status='stale', embedding_revision=?, model_id=?, dimensions=?, model_revision=? WHERE concept_id=?",
                (
                    repo_revision,
                    self._semantic_config.model_id,
                    self._semantic_config.dimensions,
                    model_info.revision,
                    row["concept_id"],
                ),
            )
        pending: list[tuple[str, ConceptDocument, str, str]] = []
        for bundle_path, document in changed_documents:
            if not isinstance(document, ConceptDocument):
                raise TypeError("document must be a ConceptDocument")
            text = embedding_text(
                title=document.frontmatter.title,
                description=document.frontmatter.description,
                body=document.body,
            )[: self._semantic_config.max_input_chars]
            text_hash = embedding_content_hash(text)
            existing = connection.execute(
                "SELECT embedding_text_hash, model_id, dimensions, model_revision, status FROM concept_embeddings WHERE concept_id = ?",
                (document.frontmatter.id,),
            ).fetchone()
            if (
                existing is not None
                and not full_rebuild
                and existing["embedding_text_hash"] == text_hash
                and existing["model_id"] == self._semantic_config.model_id
                and int(existing["dimensions"]) == self._semantic_config.dimensions
                and existing["model_revision"] == model_info.revision
                and existing["status"] == "ready"
            ):
                connection.execute(
                    "UPDATE concept_embeddings SET path=?, embedding_revision=? WHERE concept_id=?",
                    (bundle_path, repo_revision, document.frontmatter.id),
                )
                continue
            pending.append((bundle_path, document, text, text_hash))
        if pending:
            for start in range(0, len(pending), self._semantic_config.max_batch_size):
                batch = pending[start : start + self._semantic_config.max_batch_size]
                vectors = self._embed_pending_batch(batch)
                for (bundle_path, document, _text, text_hash), vector in zip(
                    batch, vectors, strict=True
                ):
                    if isinstance(vector, Exception):
                        self._write_degraded_embedding(
                            connection,
                            bundle_path=bundle_path,
                            document=document,
                            text_hash=text_hash,
                            repo_revision=repo_revision,
                            model_revision=model_info.revision,
                            error_message=str(vector),
                        )
                        continue
                    try:
                        validated = validate_embedding(
                            vector, dimensions=self._semantic_config.dimensions
                        )
                    except SemanticSearchError as exc:
                        self._write_degraded_embedding(
                            connection,
                            bundle_path=bundle_path,
                            document=document,
                            text_hash=text_hash,
                            repo_revision=repo_revision,
                            model_revision=model_info.revision,
                            error_message=str(exc),
                        )
                        continue
                    self._write_ready_embedding(
                        connection,
                        bundle_path=bundle_path,
                        document=document,
                        text_hash=text_hash,
                        repo_revision=repo_revision,
                        model_revision=model_info.revision,
                        validated=validated,
                    )
        concept_count = int(connection.execute("SELECT COUNT(*) FROM concepts").fetchone()[0] or 0)
        ready_count = int(
            connection.execute(
                "SELECT COUNT(*) FROM concept_embeddings WHERE status='ready'"
            ).fetchone()[0]
            or 0
        )
        embedding_revision = repo_revision if concept_count == ready_count else "partial"
        self._set_state(connection, "semantic_embedding_revision", embedding_revision)

    def _embed_pending_batch(
        self, pending: Sequence[tuple[str, ConceptDocument, str, str]]
    ) -> tuple[tuple[float, ...] | Exception, ...]:
        assert self._embedding_client is not None
        texts = [item[2] for item in pending]
        try:
            vectors = self._embedding_client.embed_batch(texts)
            if len(vectors) != len(pending):
                raise SemanticSearchError("embedding client returned mismatched batch length")
            return vectors
        except Exception as exc:
            if len(pending) == 1:
                return (SemanticSearchError(str(exc)),)
        results: list[tuple[float, ...] | Exception] = []
        for _bundle_path, _document, text, _text_hash in pending:
            try:
                results.append(self._embedding_client.embed(text))
            except Exception as exc:
                results.append(SemanticSearchError(str(exc)))
        return tuple(results)

    def _write_ready_embedding(
        self,
        connection: sqlite3.Connection,
        *,
        bundle_path: str,
        document: ConceptDocument,
        text_hash: str,
        repo_revision: str,
        model_revision: str,
        validated: ValidatedEmbedding,
    ) -> None:
        connection.execute(
            """
            INSERT INTO concept_embeddings(
                concept_id, path, embedding_text_hash, model_id, dimensions,
                embedding_revision, status, model_revision, embedding_blob,
                embedding_norm, updated_at, error_message
            )
            VALUES(?, ?, ?, ?, ?, ?, 'ready', ?, ?, ?, datetime('now'), NULL)
            ON CONFLICT(concept_id) DO UPDATE SET
                path=excluded.path,
                embedding_text_hash=excluded.embedding_text_hash,
                model_id=excluded.model_id,
                dimensions=excluded.dimensions,
                embedding_revision=excluded.embedding_revision,
                status=excluded.status,
                model_revision=excluded.model_revision,
                embedding_blob=excluded.embedding_blob,
                embedding_norm=excluded.embedding_norm,
                updated_at=datetime('now'),
                error_message=NULL
            """,
            (
                document.frontmatter.id,
                bundle_path,
                text_hash,
                self._semantic_config.model_id,
                self._semantic_config.dimensions,
                repo_revision,
                model_revision,
                pack_f32le(validated.values),
                validated.norm,
            ),
        )

    def _write_degraded_embedding(
        self,
        connection: sqlite3.Connection,
        *,
        bundle_path: str,
        document: ConceptDocument,
        text_hash: str,
        repo_revision: str,
        model_revision: str,
        error_message: str,
    ) -> None:
        connection.execute(
            """
            INSERT INTO concept_embeddings(
                concept_id, path, embedding_text_hash, model_id, dimensions,
                embedding_revision, status, model_revision, embedding_blob,
                embedding_norm, updated_at, error_message
            )
            VALUES(?, ?, ?, ?, ?, ?, 'degraded', ?, NULL, NULL, datetime('now'), ?)
            ON CONFLICT(concept_id) DO UPDATE SET
                path=excluded.path,
                embedding_text_hash=excluded.embedding_text_hash,
                model_id=excluded.model_id,
                dimensions=excluded.dimensions,
                embedding_revision=excluded.embedding_revision,
                status=excluded.status,
                model_revision=excluded.model_revision,
                embedding_blob=NULL,
                embedding_norm=NULL,
                updated_at=datetime('now'),
                error_message=excluded.error_message
            """,
            (
                document.frontmatter.id,
                bundle_path,
                text_hash,
                self._semantic_config.model_id,
                self._semantic_config.dimensions,
                repo_revision,
                model_revision,
                error_message[:1000],
            ),
        )

    def _validate_model_info(self, model_info: EmbeddingModelInfo) -> None:
        if model_info.dimensions != self._semantic_config.dimensions:
            raise SemanticSearchError(
                f"embedding dimensions mismatch: model={model_info.dimensions} config={self._semantic_config.dimensions}"
            )
        if len(model_info.model_id.strip()) == 0:
            raise SemanticSearchError("embedding model id must not be empty")

    def _search_lexical_rows(
        self,
        connection: sqlite3.Connection,
        *,
        policy: EffectivePolicy,
        query: str,
        concept_type: str | None,
        tags: tuple[str, ...],
        status: str | None,
        path_prefix: str | None,
        limit: int,
        offset: int,
    ) -> list[sqlite3.Row]:
        conditions = ["f.rowid IN (SELECT rowid FROM concept_fts WHERE concept_fts MATCH ?)"]
        parameters: list[object] = [query]
        prefix_conditions = _authorized_prefix_conditions(policy.read_prefixes)
        conditions.append(prefix_conditions.sql)
        parameters.extend(prefix_conditions.parameters)
        if concept_type is not None:
            conditions.append("c.type = ?")
            parameters.append(concept_type)
        if status is not None:
            conditions.append("c.status = ?")
            parameters.append(status)
        if path_prefix is not None:
            conditions.append("c.path LIKE ? ESCAPE '\\'")
            parameters.append(f"{_escape_like(path_prefix)}%")
        for tag in tags:
            conditions.append("EXISTS (SELECT 1 FROM json_each(c.tags_json) WHERE value = ?)")
            parameters.append(tag)
        where_clause = " AND ".join(f"({condition})" for condition in conditions)
        return list(
            connection.execute(
                f"""
                SELECT
                    c.id,
                    c.path,
                    c.title,
                    c.type,
                    c.status,
                    c.tags_json,
                    snippet(concept_fts, 5, '', '', ' … ', 16) AS snippet,
                    bm25(concept_fts, 10.0, 5.0, 5.0, 4.0, 1.0, 5.0) AS score
                FROM concept_fts AS f
                JOIN concepts AS c ON c.id = f.concept_id
                WHERE {where_clause}
                ORDER BY score, c.id
                LIMIT ? OFFSET ?
                """,
                (*parameters, limit, offset),
            ).fetchall()
        )

    def _search_semantic_rows(
        self,
        connection: sqlite3.Connection,
        *,
        policy: EffectivePolicy,
        query: str,
        concept_type: str | None,
        tags: tuple[str, ...],
        status: str | None,
        path_prefix: str | None,
        limit: int,
        offset: int,
        search_mode: SearchMode,
        lexical_rows: Sequence[sqlite3.Row],
    ) -> list[sqlite3.Row]:
        if self._embedding_client is None:
            raise SemanticSearchError("semantic search embedding client is unavailable")
        query_vector = validate_embedding(
            self._embedding_client.embed(query),
            dimensions=self._semantic_config.dimensions,
        ).values
        lexical_rank = {row["id"]: rank for rank, row in enumerate(lexical_rows, start=1)}
        candidate_rows = self._semantic_candidate_rows(
            connection,
            policy=policy,
            concept_type=concept_type,
            tags=tags,
            status=status,
            path_prefix=path_prefix,
        )
        semantic_scored = [
            (self._embedding_cosine(connection, query_vector, row["embedding_blob"]), row)
            for row in candidate_rows
        ]
        semantic_ordered = sorted(
            semantic_scored,
            key=lambda item: (-item[0], item[1]["path"], item[1]["id"]),
        )
        semantic_rank = {
            row["id"]: rank for rank, (_score, row) in enumerate(semantic_ordered, start=1)
        }
        scored: list[tuple[float, float, int, sqlite3.Row]] = []
        for cosine_score, row in semantic_ordered:
            if search_mode is SearchMode.SEMANTIC:
                final_score = cosine_score
            else:
                lexical_component = 1.0 / (
                    RRF_K + lexical_rank.get(row["id"], len(lexical_rows) + 1)
                )
                semantic_component = 1.0 / (RRF_K + semantic_rank[row["id"]])
                final_score = lexical_component + semantic_component
            scored.append((final_score, cosine_score, lexical_rank.get(row["id"], 10**9), row))
        ordered = [
            item[3]
            for item in sorted(
                scored,
                key=lambda item: (-item[0], -item[1], item[2], item[3]["path"], item[3]["id"]),
            )
        ]
        return ordered[offset : offset + limit + 1]

    def _semantic_candidate_rows(
        self,
        connection: sqlite3.Connection,
        *,
        policy: EffectivePolicy,
        concept_type: str | None,
        tags: tuple[str, ...],
        status: str | None,
        path_prefix: str | None,
    ) -> list[sqlite3.Row]:
        conditions = ["e.status = 'ready'"]
        parameters: list[object] = []
        prefix_conditions = _authorized_prefix_conditions(policy.read_prefixes)
        conditions.append(prefix_conditions.sql.replace("c.", "e."))
        parameters.extend(prefix_conditions.parameters)
        if concept_type is not None:
            conditions.append("c.type = ?")
            parameters.append(concept_type)
        if status is not None:
            conditions.append("c.status = ?")
            parameters.append(status)
        if path_prefix is not None:
            conditions.append("e.path LIKE ? ESCAPE '\\'")
            parameters.append(f"{_escape_like(path_prefix)}%")
        for tag in tags:
            conditions.append("EXISTS (SELECT 1 FROM json_each(c.tags_json) WHERE value = ?)")
            parameters.append(tag)
        where_clause = " AND ".join(f"({condition})" for condition in conditions)
        return list(
            connection.execute(
                f"""
                SELECT c.id, c.path, c.title, c.type, c.status, c.tags_json, c.title AS snippet,
                       e.embedding_blob, e.embedding_norm
                FROM concept_embeddings AS e
                JOIN concepts AS c ON c.id = e.concept_id
                WHERE {where_clause}
                ORDER BY e.path, c.id
                LIMIT ?
                """,
                (*parameters, self._semantic_config.max_candidates),
            ).fetchall()
        )

    def _embedding_cosine(
        self, connection: sqlite3.Connection, query_vector: Sequence[float], blob: bytes
    ) -> float:
        if self._sqlite_vector_enabled:
            try:
                row = connection.execute(
                    "SELECT vector_cosine(?, ?) AS cosine",
                    (pack_f32le(query_vector), blob),
                ).fetchone()
                if row is not None and row["cosine"] is not None:
                    return float(row["cosine"])
            except sqlite3.DatabaseError:
                pass
        return cosine_similarity(query_vector, unpack_f32le(blob))

    @staticmethod
    def _row_to_search_result(row: sqlite3.Row) -> SearchResult:
        try:
            raw_score = row["score"]
        except IndexError:
            raw_score = None
        score = float(-raw_score) if raw_score is not None else 0.0
        return SearchResult(
            concept_id=row["id"],
            path=row["path"],
            title=row["title"],
            concept_type=row["type"],
            status=row["status"],
            tags=tuple(json.loads(row["tags_json"])),
            score=score,
            snippet=_bounded_snippet(row["snippet"] or row["title"]),
        )

    def _collect_neighbors(
        self,
        connection: sqlite3.Connection,
        policy: EffectivePolicy,
        concept_id: str,
        direction: str,
        depth: int,
    ) -> tuple[GraphEdge, ...]:
        if depth == 0:
            return ()
        seen: set[str] = {concept_id}
        frontier = [(concept_id, 0)]
        edges: list[GraphEdge] = []
        while frontier:
            current_id, current_depth = frontier.pop(0)
            if current_depth >= depth:
                continue
            query, parameters = self._neighbor_query(direction, policy.read_prefixes)
            rows = connection.execute(query, (current_id, *parameters)).fetchall()
            for row in rows:
                target_id = row["id"]
                if target_id in seen:
                    continue
                seen.add(target_id)
                next_depth = current_depth + 1
                frontier.append((target_id, next_depth))
                edges.append(
                    GraphEdge(
                        concept_id=target_id,
                        path=row["path"],
                        title=row["title"],
                        depth=next_depth,
                        direction=direction,
                        broken_link_count=int(row["broken_link_count"]),
                        orphan_flag=bool(row["orphan_flag"]),
                    )
                )
        return tuple(sorted(edges, key=lambda item: (item.depth, item.path, item.concept_id)))

    def _neighbor_query(
        self, direction: str, prefixes: tuple[str, ...]
    ) -> tuple[str, tuple[str, ...]]:
        prefix_conditions = _authorized_prefix_conditions(prefixes, alias="c")
        if direction == "outbound":
            return (
                f"""
                SELECT c.id, c.path, c.title, m.broken_link_count, m.orphan_flag
                FROM links AS l
                JOIN concepts AS c ON c.id = l.target_id
                JOIN graph_metrics AS m ON m.concept_id = c.id
                WHERE l.source_id = ? AND {prefix_conditions.sql}
                """,
                prefix_conditions.parameters,
            )
        return (
            f"""
            SELECT c.id, c.path, c.title, m.broken_link_count, m.orphan_flag
            FROM links AS l
            JOIN concepts AS c ON c.id = l.source_id
            JOIN graph_metrics AS m ON m.concept_id = c.id
            WHERE l.target_id = ? AND {prefix_conditions.sql}
            """,
            prefix_conditions.parameters,
        )

    def _authorize_concept_id(
        self, connection: sqlite3.Connection, policy: EffectivePolicy, concept_id: str
    ) -> None:
        row = connection.execute("SELECT path FROM concepts WHERE id = ?", (concept_id,)).fetchone()
        if row is None:
            raise KeyError(concept_id)
        if not _path_allowed(row["path"], policy.read_prefixes):
            raise KeyError(concept_id)

    def _connect(self) -> sqlite3.Connection:
        self._db_path.parent.mkdir(parents=True, exist_ok=True)
        connection = sqlite3.connect(self._db_path)
        connection.row_factory = sqlite3.Row
        connection.execute("PRAGMA foreign_keys=ON")
        self._ensure_initialized(connection)
        self._configure_sqlite_vector(connection)
        return connection

    def _ensure_initialized(self, connection: sqlite3.Connection) -> None:
        identity = self._database_identity()
        if self._initialized_identity == identity:
            return
        with self._initialization_lock:
            identity = self._database_identity()
            if self._initialized_identity == identity:
                return
            connection.execute("PRAGMA application_id=1296646996")
            connection.execute("PRAGMA journal_mode=WAL")
            self._migrate(connection, force=True)
            self._initialized_identity = self._database_identity()

    def _database_identity(self) -> tuple[int, int] | None:
        try:
            stat = self._db_path.stat()
        except FileNotFoundError:
            return None
        return stat.st_dev, stat.st_ino

    def _configure_sqlite_vector(self, connection: sqlite3.Connection) -> None:
        extension_path = self._semantic_config.sqlite_extension_path
        if not extension_path:
            self._sqlite_vector_enabled = False
            return
        try:
            connection.enable_load_extension(True)
            connection.load_extension(extension_path)
            self._sqlite_vector_enabled = True
            self._sqlite_vector_warning = None
        except (AttributeError, sqlite3.DatabaseError) as exc:
            self._sqlite_vector_enabled = False
            self._sqlite_vector_warning = f"sqlite_vector_extension_unavailable: {exc}"
        finally:
            with suppress(AttributeError):
                connection.enable_load_extension(False)

    def _migrate(self, connection: sqlite3.Connection, *, force: bool = False) -> None:
        if not force and self._initialized_identity == self._database_identity():
            return
        with connection:
            for statement in MIGRATIONS:
                connection.execute(statement)
            if self._get_state_optional(connection, "schema_version") is None:
                self._set_state(connection, "schema_version", SCHEMA_VERSION)
            elif self._get_state(connection, "schema_version") != SCHEMA_VERSION:
                raise DerivedIndexCorruptionError("unsupported derived schema version")
            for key, value in (
                ("repo_revision", ""),
                ("index_revision", ""),
                ("status", "ready"),
                ("semantic_embedding_revision", ""),
            ):
                if self._get_state_optional(connection, key) is None:
                    self._set_state(connection, key, value)

    def _ensure_ready(self, connection: sqlite3.Connection) -> None:
        self._migrate(connection)
        if self._get_state(connection, "status") == "quarantined":
            raise DerivedIndexCorruptionError("derived index is quarantined")

    def _with_quarantine(self, action: Callable[[sqlite3.Connection], None]) -> None:
        self._validate_db_file()
        try:
            with self._connect() as connection:
                self._migrate(connection)
                action(connection)
        except (sqlite3.DatabaseError, DerivedIndexCorruptionError) as exc:
            quarantine_path = self._quarantine_db()
            with self._connect() as connection:
                self._migrate(connection)
                with connection:
                    self._set_state(connection, "status", "quarantined")
                    self._set_state(connection, "quarantine_path", str(quarantine_path))
            raise DerivedIndexCorruptionError(str(exc)) from exc

    def _validate_db_file(self) -> None:
        if not self._db_path.exists() or self._db_path.stat().st_size == 0:
            return
        with self._db_path.open("rb") as handle:
            header = handle.read(16)
        if header != b"SQLite format 3\x00":
            raise sqlite3.DatabaseError("invalid sqlite header")

    def _handle_corruption(self, exc: sqlite3.DatabaseError) -> None:
        quarantine_path = self._quarantine_db()
        with self._connect() as connection:
            self._migrate(connection)
            with connection:
                self._set_state(connection, "status", "quarantined")
                self._set_state(connection, "quarantine_path", str(quarantine_path))
        raise DerivedIndexCorruptionError(str(exc)) from exc

    def _is_search_query_error(self, exc: sqlite3.OperationalError) -> bool:
        message = str(exc).casefold()
        return (
            "unterminated string" in message
            or "malformed match expression" in message
            or "fts5: syntax error" in message
            or "no such column" in message
        )

    def _quarantine_db(self) -> Path:
        self._initialized_identity = None
        if not self._db_path.exists():
            return self._db_path.with_suffix(".missing")
        quarantine_path = self._db_path.with_name(
            f"{self._db_path.stem}.quarantine-{int(time.time() * 1000)}{self._db_path.suffix}"
        )
        shutil.move(self._db_path, quarantine_path)
        wal_path = self._db_path.with_name(self._db_path.name + "-wal")
        shm_path = self._db_path.with_name(self._db_path.name + "-shm")
        for extra in (wal_path, shm_path):
            if extra.exists():
                extra.unlink()
        return quarantine_path

    def _get_state(self, connection: sqlite3.Connection, key: str) -> str:
        value = self._get_state_optional(connection, key)
        if value is None:
            raise KeyError(key)
        return value

    def _get_state_optional(self, connection: sqlite3.Connection, key: str) -> str | None:
        row = connection.execute("SELECT value FROM index_state WHERE key = ?", (key,)).fetchone()
        if row is None:
            return None
        return str(row["value"])

    def _set_state(self, connection: sqlite3.Connection, key: str, value: str) -> None:
        connection.execute(
            """
            INSERT INTO index_state(key, value, updated_at) VALUES(?, ?, datetime('now'))
            ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=datetime('now')
            """,
            (key, value),
        )


def _lexical_query(query: str, *, query_syntax: str) -> str:
    if query_syntax == "fts5":
        if not query.strip():
            raise DerivedSearchError("search query must not be empty")
        return query
    if query_syntax != "plain":
        raise DerivedSearchError(f"unsupported query_syntax: {query_syntax}")
    terms = re.findall(r"\w+", query, flags=re.UNICODE)
    if not terms:
        raise DerivedSearchError("plain search query must contain at least one word")
    return " ".join(f'"{term.replace(chr(34), chr(34) * 2)}"' for term in terms)


def _decode_cursor(cursor: str | None) -> int:
    if cursor is None:
        return 0
    if not cursor.startswith("offset:"):
        raise ValueError("unsupported cursor")
    return max(0, int(cursor.split(":", 1)[1]))


def _encode_cursor(offset: int) -> str:
    return f"offset:{offset}"


@dataclass(frozen=True, slots=True)
class _PrefixConditions:
    sql: str
    parameters: tuple[str, ...]


def _authorized_prefix_conditions(
    prefixes: tuple[str, ...], *, alias: str = "c"
) -> _PrefixConditions:
    clauses: list[str] = []
    parameters: list[str] = []
    for prefix in prefixes:
        clauses.append(
            f"{alias}.path = substr(?, 1, length(?) - 1) OR {alias}.path LIKE ? ESCAPE '\\'"
        )
        parameters.extend((prefix, prefix, f"{_escape_like(prefix)}%"))
    return _PrefixConditions(sql="(" + " OR ".join(clauses) + ")", parameters=tuple(parameters))


def _path_allowed(path: str, prefixes: tuple[str, ...]) -> bool:
    return any(path == prefix[:-1] or path.startswith(prefix) for prefix in prefixes)


def _escape_like(value: str) -> str:
    return value.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")


def _split_target(href: str) -> tuple[str, str | None]:
    if "#" not in href:
        return href, None
    path, anchor = href.split("#", 1)
    return path, anchor or None


def _bounded_snippet(snippet: str) -> str:
    compact = " ".join(snippet.split())
    return compact[:240]
