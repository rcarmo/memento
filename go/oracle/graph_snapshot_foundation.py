"""Synthetic GraphSnapshotService revision/path fixtures."""

from __future__ import annotations

import sqlite3
import tempfile
from pathlib import Path
from typing import Any

from memento.authz import EffectivePolicy  # type: ignore[import-untyped]
from memento.config import GraphExplorerConfig  # type: ignore[import-untyped]
from memento.graph_debug.snapshot import GraphSnapshotService  # type: ignore[import-untyped]


def fixtures() -> dict[str, Any]:
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        derived = root / "derived.sqlite"
        control = root / "control.sqlite"
        with sqlite3.connect(derived) as db:
            db.executescript(
                "CREATE TABLE index_state(key TEXT PRIMARY KEY,value TEXT,updated_at TEXT);"
                "CREATE TABLE concepts(id TEXT PRIMARY KEY,path TEXT);"
                "CREATE TABLE links(source_id TEXT,target_id TEXT,raw_target TEXT,target_path TEXT,anchor TEXT,link_kind TEXT,resolution_state TEXT,first_seen_revision TEXT,last_checked_revision TEXT);"
                "INSERT INTO index_state VALUES('repo_revision','main','now'),('index_revision','old','now'),('semantic_embedding_revision','embed','now');"
                "INSERT INTO concepts VALUES('a','/a.md'),('b','/b.md');"
                "INSERT INTO links VALUES('a','b','/public/b.md','/public/b.md',NULL,'internal','resolved','r1','r2'),('a',NULL,'/private/missing.md','/private/missing.md','x','internal','broken','r1','r2'),('b',NULL,'/public/missing.md','/public/missing.md',NULL,'internal','broken','r1','r2');"
            )
        control.touch()
        service = GraphSnapshotService(
            GraphExplorerConfig(),
            repository_root=root,
            derived_db_path=derived,
            control_db_path=control,
        )
        with sqlite3.connect(derived) as db:
            db.row_factory = sqlite3.Row
            revisions = service._revisions(db).model_dump(mode="json")
        policy = EffectivePolicy(
            principal="reader",
            roles=("reader",),
            read_prefixes=("/",),
            write_prefixes=(),
            protected_read_prefixes=("/private/",),
        )
        with sqlite3.connect(derived) as db:
            db.row_factory = sqlite3.Row
            edges = [
                item.model_dump(mode="json")
                for item in service._edges(db, ids={"a", "b"}, limit=10, policy=policy)
            ]
        return {
            "revisions": revisions,
            "edges": edges,
            "paths_cases": [
                {"ids": [], "paths": list(service.paths_for_ids(()))},
                {
                    "ids": ["b", "missing", "a", "b"],
                    "paths": list(service.paths_for_ids(("b", "missing", "a", "b"))),
                },
            ],
        }
