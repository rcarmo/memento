"""Models-off content kernel snapshots; quarantine/search wrappers are separate."""

from __future__ import annotations

import hashlib
import importlib
import tempfile
from pathlib import Path
from typing import Any


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.derived.index")
    tables = [
        "concepts",
        "concept_fts",
        "links",
        "graph_metrics",
        "index_state",
        "concept_embeddings",
    ]

    def concept(id: str, title: str, body: str, description: str | None = None) -> str:
        return (
            f"---\nid: '{id}'\ntype: concept\ntitle: {title}\nstatus: active\n"
            + (f"description: {description}\n" if description else "")
            + "tags: [z, a, z]\naliases: [One, Deux]\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-09-18T00:00:00.123456Z\nupdated_by: actor\n---\n"
            + body
            + "\n"
        )

    first = {
        "/a.md": concept(
            "a",
            "First",
            "[b](/z.md#anchor) [b again](/z.md) [missing](/missing.md) [relative](foo) [anchor](#local) [web](https://example.org) [mail](mailto:test@example.org) [network](//example.org) [self](/a.md)",
        ),
        "/z.md": concept("z", "Second", "A body", "Description"),
        "/private/p.md": concept("p", "Private", "[trash](/trash/t.md)"),
        "/trash/t.md": concept("t", "Archived", "[first](/a.md)"),
        "/index.md": "generated",
        "/.assets/ignored.md": "asset",
    }

    def snapshot(connection: Any) -> dict[str, Any]:
        data = {}
        for table in tables:
            rows = [
                dict(row) for row in connection.execute(f"SELECT * FROM {table} ORDER BY rowid")
            ]
            if table == "index_state":
                # SQLite wall time is independently checked for format; no state
                # values, graph edges, hashes or concept timestamps are removed.
                for row in rows:
                    row.pop("updated_at")
            if table == "concept_embeddings":
                for row in rows:
                    if row["embedding_blob"] is not None:
                        row["embedding_blob"] = row["embedding_blob"].hex()
            data[table] = rows
        return data

    sequences = []
    for defer in [False, True]:
        with tempfile.TemporaryDirectory(prefix="memento-derived-") as directory:
            root = Path(directory) / "bundle"
            root.mkdir()
            store = module.DerivedIndex(Path(directory) / "index.sqlite", defer_embeddings=defer)
            connection = store._connect()
            steps = []
            commands: list[dict[str, Any]] = [
                {"action": "rebuild", "revision": "r1", "files": first},
                {"action": "seed-embeddings"},
                {"action": "rebuild", "revision": "r2", "files": {}},
                {
                    "action": "update",
                    "revision": "r3",
                    "paths": ["/0.md", "/z.md", "/z.md"],
                    "files": {"/0.md": first["/z.md"], "/z.md": None},
                },
                {
                    "action": "update",
                    "revision": "r4",
                    "paths": [
                        "/.assets/ignored.md",
                        "/index.md",
                        "/private/p.md",
                        "/missing.md",
                        "/text.txt",
                    ],
                    "files": {"/private/p.md": None},
                },
                {
                    "action": "update",
                    "revision": "r5",
                    "paths": ["/a.md"],
                    "files": {"/a.md": concept("a", "Updated", "[renamed](/0.md)")},
                },
                {
                    "action": "update",
                    "revision": "r6",
                    "paths": ["/a.md", "/bad.md"],
                    "files": {
                        "/a.md": concept("a", "Partial", "committed before parse failure"),
                        "/bad.md": "invalid",
                    },
                },
                {"action": "rebuild", "revision": "r7", "files": {}},
                {"action": "rebuild", "revision": "r8", "files": {"/bad.md": None}},
                {
                    "action": "duplicate-rebuild",
                    "revision": "r9",
                    "files": {"/duplicate.md": first["/a.md"]},
                },
            ]
            for command in commands:
                for name, text in command.get("files", {}).items():
                    target = root / name.removeprefix("/")
                    if text is None:
                        target.unlink(missing_ok=True)
                    else:
                        target.parent.mkdir(parents=True, exist_ok=True)
                        target.write_text(text)
                item = dict(command)
                try:
                    if command["action"] == "seed-embeddings":
                        digest = hashlib.sha256(b"Second\n\nDescription\n\nA body").hexdigest()
                        for id, path, status, text_hash in [
                            ("z", "/z.md", "ready", digest),
                            ("a", "/a.md", "ready", "different"),
                            ("gone", "/gone.md", "ready", "different"),
                        ]:
                            connection.execute(
                                "INSERT INTO concept_embeddings VALUES(?,?,?,?,?,?,?,?,?,?,?,?)",
                                (
                                    id,
                                    path,
                                    text_hash,
                                    "model",
                                    1,
                                    "r1",
                                    status,
                                    "model-rev",
                                    b"\x00\x00\x80?",
                                    1.0,
                                    "stamp",
                                    None,
                                ),
                            )
                        connection.commit()
                    elif command["action"] in {"rebuild", "duplicate-rebuild"}:
                        store._rebuild(connection, root, command["revision"])
                    else:
                        # Capture the update kernel without automatic quarantine.
                        # Context-manager commit boundaries remain real SQLite.
                        store._with_quarantine = lambda fn, connection=connection: fn(connection)
                        store.update_paths(
                            root,
                            repo_revision=command["revision"],
                            changed_paths=tuple(command["paths"]),
                        )
                except Exception as exc:
                    item["error_type"] = type(exc).__name__
                item["snapshot"] = snapshot(connection)
                steps.append(item)
            sequences.append({"defer": defer, "steps": steps})
            connection.close()
    migrations = []
    for scenario in ["empty", "version", "reclassify", "quarantined"]:
        with tempfile.TemporaryDirectory(prefix="memento-derived-schema-") as directory:
            store = module.DerivedIndex(Path(directory) / "index.sqlite")
            connection = store._connect()
            statements: list[str] = []
            if scenario == "version":
                statements = ["UPDATE index_state SET value='99' WHERE key='schema_version'"]
            elif scenario == "quarantined":
                statements = ["UPDATE index_state SET value='quarantined' WHERE key='status'"]
            elif scenario == "reclassify":
                statements = [
                    "DELETE FROM index_state WHERE key='external_links_classified'",
                    "INSERT INTO links VALUES('source','target','https://example.org','/wrong',NULL,'markdown','broken','old','old')",
                    "INSERT INTO links VALUES('source',NULL,'/missing','/missing',NULL,'markdown','broken','old','old')",
                ]
            for statement in statements:
                connection.execute(statement)
            connection.commit()
            item = {"scenario": scenario, "statements": statements}
            try:
                store._migrate(connection, force=True)
            except Exception as exc:
                item["error_type"] = type(exc).__name__
            item["snapshot"] = snapshot(connection)
            item["schema"] = [
                dict(row)
                for row in connection.execute(
                    "SELECT name,sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY name"
                )
            ]
            migrations.append(item)
            connection.close()
    return {"sequences": sequences, "migrations": migrations}
