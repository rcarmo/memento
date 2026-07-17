from __future__ import annotations

import hashlib
import json
import shutil
import sqlite3
import time
from collections.abc import Callable
from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from tempfile import TemporaryDirectory

from memento.authz import EffectivePolicy
from memento.repository.bundle import scan_bundle
from memento.repository.frontmatter import parse_concept_file
from memento.repository.links import extract_structural_links
from memento.repository.paths import is_reserved_bundle_path

SCHEMA_VERSION = "1"
DEFAULT_SEARCH_LIMIT = 20
MAX_SEARCH_LIMIT = 100
MAX_GRAPH_DEPTH = 2
POLL_INTERVAL_SECONDS = 0.02

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
)


class DerivedIndexCorruptionError(RuntimeError):
    """Raised when the derived database cannot be used safely."""


class DerivedSearchError(ValueError):
    """Raised when a user-supplied search query is invalid."""


class SearchFreshness(str, Enum):
    EVENTUAL = "eventual"
    STRICT = "strict"


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


class DerivedIndex:
    def __init__(self, db_path: Path) -> None:
        self._db_path = db_path

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
            with connection:
                self._set_state(connection, "repo_revision", repo_revision)
                for bundle_path in sorted(dict.fromkeys(changed_paths)):
                    self._delete_path(connection, bundle_path)
                    absolute = bundle_root / bundle_path.removeprefix("/")
                    if (
                        absolute.exists()
                        and absolute.is_file()
                        and not is_reserved_bundle_path(bundle_path)
                    ):
                        self._upsert_entry(
                            connection, bundle_path, parse_concept_file(absolute), repo_revision
                        )
                connection.execute("UPDATE concepts SET repo_revision = ?", (repo_revision,))
                self._recompute_links(connection, repo_revision)
                self._recompute_metrics(connection)
                self._set_state(connection, "index_revision", repo_revision)
                self._set_state(connection, "status", "ready")

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
                conditions = [
                    "f.rowid IN (SELECT rowid FROM concept_fts WHERE concept_fts MATCH ?)"
                ]
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
                    conditions.append(
                        "EXISTS (SELECT 1 FROM json_each(c.tags_json) WHERE value = ?)"
                    )
                    parameters.append(tag)
                where_clause = " AND ".join(f"({condition})" for condition in conditions)
                rows = connection.execute(
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
                    (*parameters, bounded_limit + 1, offset),
                ).fetchall()
        except sqlite3.OperationalError as exc:
            if self._is_search_query_error(exc):
                raise DerivedSearchError("invalid FTS query") from exc
            self._handle_corruption(exc)
        except sqlite3.DatabaseError as exc:
            self._handle_corruption(exc)
        page_rows = rows[:bounded_limit]
        next_cursor = _encode_cursor(offset + bounded_limit) if len(rows) > bounded_limit else None
        return SearchPage(
            results=tuple(
                SearchResult(
                    concept_id=row["id"],
                    path=row["path"],
                    title=row["title"],
                    concept_type=row["type"],
                    status=row["status"],
                    tags=tuple(json.loads(row["tags_json"])),
                    score=float(-row["score"]),
                    snippet=_bounded_snippet(row["snippet"] or row["title"]),
                )
                for row in page_rows
            ),
            next_cursor=next_cursor,
            repo_revision=state.repo_revision,
            index_revision=state.index_revision,
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
        return {"concepts": concepts, "links": links, "metrics": metrics}

    def _rebuild(
        self, connection: sqlite3.Connection, bundle_root: Path, repo_revision: str
    ) -> None:
        bundle = scan_bundle(bundle_root)
        self._migrate(connection)
        with connection:
            for table in ("concepts", "concept_fts", "links", "graph_metrics"):
                connection.execute(f"DELETE FROM {table}")
            self._set_state(connection, "status", "rebuilding")
            self._set_state(connection, "repo_revision", repo_revision)
            for entry in bundle.entries:
                self._upsert_entry(connection, entry.bundle_path, entry.document, repo_revision)
            self._recompute_links(connection, repo_revision)
            self._recompute_metrics(connection)
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
        connection.execute("PRAGMA application_id=1296646996")
        connection.execute("PRAGMA journal_mode=WAL")
        connection.execute("PRAGMA foreign_keys=ON")
        return connection

    def _migrate(self, connection: sqlite3.Connection) -> None:
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
