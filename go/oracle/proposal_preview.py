"""Source previews, difflib boundaries and complete retained proposal payloads."""

from __future__ import annotations

import difflib
import importlib
import tempfile
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> dict[str, Any]:
    service: Any = importlib.import_module("memento.service")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    schema: Any = importlib.import_module("memento.repository.schema")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    stamp = datetime(2026, 1, 1, tzinfo=UTC)
    initial = str(
        frontmatter.serialize_concept(
            schema.ConceptDocument(
                frontmatter=schema.ConceptFrontmatter(
                    id="12345678",
                    type="concept",
                    title="Title",
                    status="active",
                    created_at=stamp,
                    updated_at=stamp,
                    updated_by="original",
                ),
                body="one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten",
            )
        )
    )
    create = {
        "kind": "create",
        "path": "/a.md",
        "concept_type": "concept",
        "title": "Title",
        "body": "body",
    }
    inputs: list[tuple[str, list[dict[str, Any]], int]] = [
        ("empty", [], 65536),
        ("create", [create], 65536),
        ("create-invalid", [{**create, "title": ""}], 65536),
        ("create-limit", [create], 1),
        (
            "patch",
            [
                {
                    "kind": "patch",
                    "path": "/a.md",
                    "body": "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nTEN",
                }
            ],
            65536,
        ),
        ("unchanged", [{"kind": "patch", "path": "/a.md"}], 65536),
        (
            "patch-copy",
            [{"kind": "patch", "path": "/a.md", "title": "", "tags": ["b", "", "a", "b"]}],
            65536,
        ),
        ("patch-limit", [{"kind": "patch", "path": "/a.md"}], 1),
        ("missing", [{"kind": "patch", "path": "/missing.md"}], 65536),
        ("default", [{"kind": "patch", "path": "/a.md"}], 65536),
        (
            "descriptions",
            [
                {"kind": "trash", "path": "/a.md"},
                {"kind": "rename", "path": "/a.md", "new_path": "/b.md"},
                {
                    "kind": "attach_asset_pack",
                    "path": "/a.md",
                    "asset_kind": "docs",
                    "version": "1.0.0",
                    "asset_id": "asset",
                    "zip_sha256": "digest",
                    "manifest": {},
                },
            ],
            65536,
        ),
    ]
    previews = []
    payloads = []
    with tempfile.TemporaryDirectory(prefix="memento-preview-") as directory:
        root = Path(directory)
        runtime = object.__new__(service.MemoryService)
        for name, changes, limit in inputs:
            text = initial.replace("status: active\n", "") if name == "default" else initial
            (root / "a.md").write_text(text)
            runtime._deps = SimpleNamespace(
                repo_paths=SimpleNamespace(current_dir=root),
                config=SimpleNamespace(limits=SimpleNamespace(max_concept_bytes=limit)),
            )
            item: dict[str, Any] = {
                "name": name,
                "initial": text,
                "changes": changes,
                "limit": limit,
            }
            try:
                item["expected"] = runtime._preview_changes(runtime._normalize_changes(changes))
            except Exception as exc:
                item["error_type"] = type(exc).__name__
            previews.append(item)
        connection = db.connect_control_db(root / "control.sqlite")
        db.migrate_control_db(connection)
        runtime._deps = SimpleNamespace(
            repo_paths=SimpleNamespace(current_dir=root), control_connection=connection
        )
        for count in [0, 1, 51]:
            id = f"p{count}"
            record = proposals.create_proposal(
                connection,
                proposal_id=id,
                author_principal="author",
                client_instance_id=None,
                base_revision="base",
                intent="intent",
                rationale="why",
                patch={
                    "changes": [{"kind": "rename", "path": "/a.md", "new_path": "/b.md"}],
                    **(
                        {
                            "consulted_concepts": None,
                            "contradictions": [{"path": "/a.md"}],
                            "target_hint": "hint",
                        }
                        if count
                        else {}
                    ),
                },
            )
            connection.execute(
                "UPDATE proposals SET created_at='created',updated_at='updated',expires_at=NULL WHERE proposal_id=?",
                (id,),
            )
            for i in range(count):
                connection.execute(
                    "INSERT INTO proposal_events(proposal_id,actor,action,from_status,to_status,base_revision,repo_revision,details_json,created_at) VALUES(?,?,?,?,?,?,?,?,?)",
                    (
                        id,
                        "reviewer",
                        "review",
                        "submitted",
                        "approved",
                        "base",
                        "main",
                        '{"synthetic": true}',
                        str(i),
                    ),
                )
            connection.commit()
            record = proposals.get_proposal(connection, id)
            with (
                patch.object(service, "get_main_revision", lambda _: "main"),
                patch.object(service, "diff_main_paths", lambda *args, **kwargs: ("/b.md",)),
            ):
                payloads.append(
                    {
                        "record": asdict(record),
                        "events": [
                            dict(row)
                            for row in connection.execute(
                                "SELECT * FROM proposal_events WHERE proposal_id=? ORDER BY event_id",
                                (id,),
                            )
                        ],
                        "expected": runtime._proposal_payload(record, "preview text"),
                    }
                )
        connection.close()
    diffs = []
    for old, new in [
        ("", "x\n"),
        ("x\n", ""),
        ("x", "y"),
        ("a\r\nb\vnext\u2028last\x85", "a\r\nb\fnext\u2029last\x1c"),
        ("same\n" * 300 + "old\n", "same\n" * 300 + "new\n"),
        ("a\nb\nc\nd\ne\nf\ng\nh\ni\n", "A\nb\nc\nd\ne\nf\ng\nh\nI\n"),
    ]:
        diffs.append(
            {
                "before": old,
                "after": new,
                "expected": "".join(
                    difflib.unified_diff(
                        old.splitlines(keepends=True),
                        new.splitlines(keepends=True),
                        fromfile="a/a.md",
                        tofile="b/a.md",
                    )
                ),
            }
        )
    return {"previews": previews, "payloads": payloads, "diffs": diffs}
