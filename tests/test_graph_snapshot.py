from __future__ import annotations

import json
import sqlite3
from datetime import UTC, datetime
from pathlib import Path

import pytest

from memento.config import GraphExplorerConfig
from memento.control.db import connect_control_db, migrate_control_db
from memento.graph_debug.models import GraphOverview
from memento.graph_debug.snapshot import GraphSnapshotError, GraphSnapshotService
from memento.repository.frontmatter import serialize_concept
from memento.repository.schema import ConceptDocument, ConceptFrontmatter, ConceptStatus


def _write_concept(root: Path, path: str, *, concept_id: str, title: str, body: str) -> None:
    target = root / path.removeprefix("/")
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(
        serialize_concept(
            ConceptDocument(
                frontmatter=ConceptFrontmatter(
                    schema_version=1,
                    id=concept_id,
                    type="project",
                    title=title,
                    status=ConceptStatus.ACTIVE,
                    description=f"{title} description",
                    aliases=(),
                    tags=("graph",),
                    source_refs=(),
                    supersedes=(),
                    created_at=datetime(2026, 7, 20, tzinfo=UTC),
                    updated_at=datetime(2026, 7, 20, tzinfo=UTC),
                    updated_by="tester",
                ),
                body=body,
            )
        ),
        encoding="utf-8",
    )


def _snapshot(
    tmp_path: Path, *, direct_limit: int = 2, preview_chars: int = 12
) -> GraphSnapshotService:
    root = tmp_path / "current"
    _write_concept(
        root, "/projects/a.md", concept_id="a-id", title="A", body="Alpha body with more text.\n"
    )
    _write_concept(root, "/projects/b.md", concept_id="b-id", title="B", body="Beta body.\n")
    asset = root / ".assets" / "a-id" / "document" / "1.0.0.json"
    asset.parent.mkdir(parents=True)
    asset.write_text(
        json.dumps(
            {
                "asset_kind": "document",
                "version": "1.0.0",
                "source_proposal_id": "proposal-1",
            }
        ),
        encoding="utf-8",
    )
    asset.with_suffix(".zip").write_bytes(b"asset-payload")

    derived_path = tmp_path / "derived.sqlite"
    derived = sqlite3.connect(derived_path)
    derived.executescript(
        """
        CREATE TABLE concepts(id TEXT PRIMARY KEY,path TEXT,type TEXT,title TEXT,status TEXT,tags_json TEXT,updated_at TEXT,repo_revision TEXT,body TEXT,content_hash TEXT);
        CREATE VIRTUAL TABLE concept_fts USING fts5(concept_id UNINDEXED,title,description,aliases,tags,body,path,tokenize='unicode61');
        CREATE TABLE links(source_id TEXT,target_id TEXT,raw_target TEXT,target_path TEXT,anchor TEXT,link_kind TEXT,resolution_state TEXT,first_seen_revision TEXT,last_checked_revision TEXT);
        CREATE TABLE graph_metrics(concept_id TEXT PRIMARY KEY,inbound_degree INTEGER,outbound_degree INTEGER,broken_link_count INTEGER,orphan_flag INTEGER);
        CREATE TABLE concept_embeddings(concept_id TEXT PRIMARY KEY,status TEXT,model_id TEXT,dimensions INTEGER,embedding_revision TEXT,model_revision TEXT,updated_at TEXT,error_message TEXT,embedding_blob BLOB);
        CREATE TABLE index_state(key TEXT PRIMARY KEY,value TEXT);
        """
    )
    derived.executemany(
        "INSERT INTO concepts VALUES(?,?,?,?,?,?,?,?,?,?)",
        (
            (
                "a-id",
                "/projects/a.md",
                "project",
                "A",
                "active",
                '["graph"]',
                "2026-07-20T00:00:00Z",
                "rev-1",
                "Alpha",
                "hash-a",
            ),
            (
                "b-id",
                "/projects/b.md",
                "project",
                "B",
                "active",
                '["graph"]',
                "2026-07-20T00:00:00Z",
                "rev-1",
                "Beta",
                "hash-b",
            ),
        ),
    )
    derived.executemany(
        "INSERT INTO concept_fts(concept_id,title,description,aliases,tags,body,path) VALUES(?,?,?,?,?,?,?)",
        (
            (
                "a-id",
                "A",
                "Alpha description",
                "",
                "graph shared",
                "Alpha body with more text",
                "/projects/a.md",
            ),
            ("b-id", "B", "Beta description", "", "graph", "Beta body", "/projects/b.md"),
        ),
    )
    derived.execute(
        "INSERT INTO links VALUES(?,?,?,?,?,?,?,?,?)",
        (
            "a-id",
            "b-id",
            "/projects/b.md",
            "/projects/b.md",
            None,
            "markdown",
            "resolved",
            "rev-1",
            "rev-1",
        ),
    )
    derived.executemany(
        "INSERT INTO graph_metrics VALUES(?,?,?,?,?)",
        (("a-id", 0, 1, 0, 0), ("b-id", 1, 0, 0, 0)),
    )
    derived.execute(
        "INSERT INTO concept_embeddings VALUES(?,?,?,?,?,?,?,?,?)",
        (
            "a-id",
            "ready",
            "gte",
            384,
            "rev-1",
            "model-1",
            "2026-07-20T00:00:00Z",
            None,
            b"SECRET-VECTOR",
        ),
    )
    derived.executemany(
        "INSERT INTO index_state VALUES(?,?)",
        (
            ("repo_revision", "rev-1"),
            ("index_revision", "rev-1"),
            ("semantic_embedding_revision", "rev-1"),
        ),
    )
    derived.commit()
    derived.close()

    control_path = tmp_path / "control.sqlite"
    control = connect_control_db(control_path)
    migrate_control_db(control)
    control.execute(
        "INSERT INTO proposals(proposal_id,author_principal,base_revision,intent,patch_json,patch_hash,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)",
        (
            "proposal-1",
            "alice",
            "rev-1",
            "Update A",
            '{"changes":[{"path":"/projects/a.md"}]}',
            "hash",
            "submitted",
            "2026-07-20T00:00:00Z",
            "2026-07-20T00:00:00Z",
        ),
    )
    control.commit()
    control.close()
    return GraphSnapshotService(
        GraphExplorerConfig(
            enabled=True,
            direct_node_limit=direct_limit,
            preview_chars=preview_chars,
        ),
        repository_root=root,
        derived_db_path=derived_path,
        control_db_path=control_path,
    )


def test_overview_is_bounded_deterministic_and_omits_vectors(tmp_path: Path) -> None:
    service = _snapshot(tmp_path, direct_limit=1)
    first = service.overview()
    second = service.overview()
    assert first == second
    assert isinstance(first, GraphOverview)
    assert first.truncated is True
    assert first.mode == "aggregated"
    assert first.nodes == ()
    assert sum(cluster.member_count for cluster in first.clusters) == 2
    assert first.metrics.memory_count == 2
    assert first.metrics.asset_bytes > 0
    cluster = first.clusters[0]
    expansion = service.expand_cluster(cluster.id)
    assert expansion.parent_position == cluster.coarse_position
    assert [node.id for node in expansion.nodes] == ["a-id", "b-id"]
    assert expansion.next_cursor is None
    detail = service.detail("a-id")
    assert detail.node.proposal_count == 1
    assert detail.node.pending_proposal_count == 1
    assert detail.node.asset_bytes > 0
    encoded = first.model_dump_json()
    assert "SECRET-VECTOR" not in encoded
    assert "embedding_blob" not in encoded
    assert first.edges == ()


def test_graph_search_uses_fts_and_returns_bounded_metadata(tmp_path: Path) -> None:
    service = _snapshot(tmp_path)
    payload = service.search("alpha shared")
    assert payload["schema_version"] == 1
    assert payload["results"] == [
        {
            "id": "a-id",
            "path": "/projects/a.md",
            "title": "A",
            "type": "project",
            "tags": ("graph",),
            "snippet": "Alpha body with more text",
        }
    ]
    with pytest.raises(GraphSnapshotError, match="contain words"):
        service.search("---")


def test_detail_and_neighbourhood_are_bounded_and_revision_aware(tmp_path: Path) -> None:
    service = _snapshot(tmp_path)
    detail = service.detail("a-id")
    assert detail.node.updated_by == "tester"
    assert detail.preview == "Alpha body w"
    assert detail.preview_truncated is True
    assert len(detail.outbound) == 1
    assert detail.outbound[0].canonical is True
    assert detail.outbound[0].kind == "explicit"
    assert len(detail.assets) == 1
    assert detail.assets[0].payload_bytes == len(b"asset-payload")
    assert [item.proposal_id for item in detail.proposals] == ["proposal-1"]
    assert detail.revisions.repository == detail.revisions.index == "rev-1"
    assert detail.revisions.stale is False

    neighbourhood = service.neighbourhood("a-id")
    assert [node.id for node in neighbourhood.nodes] == ["a-id", "b-id"]
    assert len(neighbourhood.edges) == 1
    with pytest.raises(GraphSnapshotError, match="depth"):
        service.neighbourhood("a-id", depth=2)
    with pytest.raises(GraphSnapshotError, match="unknown"):
        service.detail("missing")
    with pytest.raises(GraphSnapshotError, match="unknown cluster"):
        service.expand_cluster("missing")


def test_snapshot_reports_stale_revisions(tmp_path: Path) -> None:
    service = _snapshot(tmp_path)
    connection = sqlite3.connect(service._derived_db_path)
    connection.execute("UPDATE index_state SET value='rev-0' WHERE key='index_revision'")
    connection.commit()
    connection.close()
    assert service.overview().revisions.stale is True
