"""Synthetic GraphSnapshotService revision/path fixtures."""

from __future__ import annotations

import sqlite3
import tempfile
from pathlib import Path
from typing import Any

from memento.authz import EffectivePolicy  # type: ignore[import-untyped]
from memento.config import GraphExplorerConfig  # type: ignore[import-untyped]
from memento.graph_debug.snapshot import (  # type: ignore[import-untyped]
    GraphSnapshotService,
    _scoped_nodes,
)


def fixtures() -> dict[str, Any]:
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        derived = root / "derived.sqlite"
        control = root / "control.sqlite"
        with sqlite3.connect(derived) as db:
            db.executescript(
                "CREATE TABLE index_state(key TEXT PRIMARY KEY,value TEXT,updated_at TEXT);"
                "CREATE TABLE concepts(id TEXT PRIMARY KEY,path TEXT,type TEXT,title TEXT,status TEXT,tags_json TEXT,updated_at TEXT,repo_revision TEXT,body TEXT,content_hash TEXT);"
                "CREATE TABLE graph_metrics(concept_id TEXT PRIMARY KEY,inbound_degree INTEGER,outbound_degree INTEGER,broken_link_count INTEGER,orphan_flag INTEGER);"
                "CREATE TABLE concept_embeddings(concept_id TEXT PRIMARY KEY,status TEXT,model_id TEXT,dimensions INTEGER,embedding_revision TEXT,model_revision TEXT,updated_at TEXT,error_message TEXT);"
                "CREATE TABLE links(source_id TEXT,target_id TEXT,raw_target TEXT,target_path TEXT,anchor TEXT,link_kind TEXT,resolution_state TEXT,first_seen_revision TEXT,last_checked_revision TEXT);"
                "INSERT INTO index_state VALUES('repo_revision','main','now'),('index_revision','old','now'),('semantic_embedding_revision','embed','now');"
                "INSERT INTO concepts VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','/a.md','concept','Alpha','active','[\"one\"]','2026-01-02T00:00:00Z','main','Body','ha'),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','/b.md','concept','Beta','active','[]','2026-01-03T00:00:00Z','main','Body','hb');"
                "INSERT INTO graph_metrics VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d',9,9,9,0);"
                "INSERT INTO concept_embeddings VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','ready','model',3,'main','v1','2026-01-04T00:00:00Z',NULL);"
                "INSERT INTO links VALUES('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d','6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e','/public/b.md','/public/b.md',NULL,'internal','resolved','r1','r2'),('5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d',NULL,'/private/missing.md','/private/missing.md','x','internal','broken','r1','r2'),('6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e',NULL,'/public/missing.md','/public/missing.md',NULL,'internal','broken','r1','r2');"
            )
        with sqlite3.connect(control) as db:
            db.executescript(
                "CREATE TABLE proposals(proposal_id TEXT,status TEXT,patch_json TEXT,author_principal TEXT);"
                "INSERT INTO proposals VALUES('one','draft','{\"path\":\"/a.md\"}','reader'),('two','applied','{\"changes\":[{\"concept_path\":\"/a.md\"},{\"new_path\":\"/b.md\"}]}','reader');"
            )
        (root / "a.md").write_text(
            "---\nschema_version: 1\nid: '5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d'\ntype: concept\ntitle: Alpha\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-02T00:00:00Z\nupdated_by: alice\n---\nBody\n",
            encoding="utf-8",
        )
        (root / "b.md").write_text(
            "---\nschema_version: 1\nid: '6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e'\ntype: concept\ntitle: Beta\nstatus: active\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-03T00:00:00Z\nupdated_by: bob\n---\nBody\n",
            encoding="utf-8",
        )
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
                for item in service._edges(
                    db,
                    ids={
                        "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d",
                        "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e",
                    },
                    limit=10,
                    policy=policy,
                )
            ]
        with sqlite3.connect(derived) as db:
            db.row_factory = sqlite3.Row
            with sqlite3.connect(control) as control_db:
                control_db.row_factory = sqlite3.Row
                proposal_counts = service._proposal_counts(control_db, policy)
            raw_nodes = service._nodes(db, proposal_counts=proposal_counts, limit=10)
            nodes = [item.model_dump(mode="json") for item in raw_nodes]
            scoped_nodes = [
                item.model_dump(mode="json")
                for item in _scoped_nodes(
                    raw_nodes,
                    service._edges(
                        db, ids={item.id for item in raw_nodes}, limit=10, policy=policy
                    ),
                )
            ]

        return {
            "revisions": revisions,
            "edges": edges,
            "nodes": nodes,
            "scoped_nodes": scoped_nodes,
            "paths_cases": [
                {"ids": [], "paths": list(service.paths_for_ids(()))},
                {
                    "ids": [
                        "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e",
                        "missing",
                        "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d",
                        "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e",
                    ],
                    "paths": list(
                        service.paths_for_ids(
                            (
                                "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e",
                                "missing",
                                "5c8fd31c-35f4-4fb2-a9b7-dd2e5935443d",
                                "6d9fe42d-46a5-4fc3-b8c8-ee3f6046554e",
                            )
                        )
                    ),
                },
            ],
        }
