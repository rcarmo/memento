"""Synthetic GraphSnapshotService revision/path fixtures."""

from __future__ import annotations

import sqlite3
import tempfile
from pathlib import Path
from typing import Any

from memento.config import GraphExplorerConfig  # type: ignore[import-untyped]
from memento.graph_debug.snapshot import GraphSnapshotService  # type: ignore[import-untyped]


def fixtures() -> dict[str, Any]:
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        derived = root / "derived.sqlite"
        control = root / "control.sqlite"
        with sqlite3.connect(derived) as db:
            db.executescript(
                "CREATE TABLE index_state(key TEXT PRIMARY KEY,value TEXT,updated_at TEXT);CREATE TABLE concepts(id TEXT PRIMARY KEY,path TEXT);INSERT INTO index_state VALUES('repo_revision','main','now'),('index_revision','old','now'),('semantic_embedding_revision','embed','now');INSERT INTO concepts VALUES('a','/a.md'),('b','/b.md');"
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
        return {
            "revisions": revisions,
            "paths_cases": [
                {"ids": [], "paths": list(service.paths_for_ids(()))},
                {
                    "ids": ["b", "missing", "a", "b"],
                    "paths": list(service.paths_for_ids(("b", "missing", "a", "b"))),
                },
            ],
        }
