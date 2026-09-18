"""Capture archival reports using disposable SQLite/filesystem state."""

from __future__ import annotations

import importlib
import json
import sqlite3
import tempfile
from dataclasses import asdict
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    authz: Any = importlib.import_module("memento.authz")
    cases = []
    for scenario in [
        "empty",
        "ordinary",
        "mixed",
        "duplicate",
        "stale-repo",
        "stale-index",
        "index-not-ready",
        "no-read",
        "no-write",
        "trash-path",
        "extension",
        "destination",
        "missing",
        "valid",
        "references-100",
        "references-101",
        "assets-100",
        "assets-101",
        "repo-advanced",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-archival-") as directory:
            root = Path(directory)
            index = root / "index.sqlite"
            connection = sqlite3.connect(index)
            schema = "CREATE TABLE index_state(key TEXT,value TEXT); CREATE TABLE concepts(id TEXT,path TEXT); CREATE TABLE links(source_id TEXT,target_path TEXT,raw_target TEXT,resolution_state TEXT);"
            connection.executescript(schema)
            states = [
                ("index_revision", "old" if scenario == "stale-index" else "main"),
                ("status", "building" if scenario == "index-not-ready" else "ready"),
            ]
            connection.executemany("INSERT INTO index_state VALUES(?,?)", states)
            refs = [
                ("/public/source.md", "target", "resolved"),
                ("/private/source.md", "hidden", "resolved"),
                ("/trash/public/old.md", "archived", "resolved"),
                ("/public/source.md", "target", "resolved"),
            ]
            if scenario.startswith("references-"):
                refs = [
                    (f"/private/{i:03d}.md", "hidden", "resolved")
                    for i in range(int(scenario.split("-")[1]))
                ]
            for i, (path, target, resolution) in enumerate(refs):
                connection.execute("INSERT INTO concepts VALUES(?,?)", (str(i), path))
                connection.execute(
                    "INSERT INTO links VALUES(?,?,?,?)",
                    (str(i), "/public/a.md", target, resolution),
                )
            connection.commit()
            connection.close()
            changes: list[dict[str, Any]] = [{"kind": "trash", "path": "/public/a.md"}]
            if scenario == "empty":
                changes = []
            elif scenario == "ordinary":
                changes = [{"kind": "patch", "path": "/public/a.md"}]
            elif scenario == "mixed":
                changes.append({"kind": "patch", "path": "/public/b.md"})
            elif scenario == "duplicate":
                changes *= 2
            elif scenario == "trash-path":
                changes[0]["path"] = "/trash/public/a.md"
            elif scenario == "extension":
                changes[0]["path"] = "/public/a.txt"
            if scenario == "destination":
                destination = root / "trash/public/a.md"
                destination.parent.mkdir(parents=True)
                destination.write_text("collision")
            kinds = ["docs", "skill"]
            versions = {"docs": ["1.0.0", "2.0.0"], "skill": ["1.0.0"]}
            if scenario.startswith("assets-"):
                kinds = ["docs"]
                versions = {"docs": [f"1.0.{i}" for i in range(int(scenario.split("-")[1]))]}
            policy = authz.EffectivePolicy(
                "author",
                ("curator",),
                () if scenario == "no-read" else ("/public/",),
                () if scenario == "no-write" else ("/public/",),
                ("/private/",),
            )
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                repo_paths=SimpleNamespace(current_dir=root),
                derived_index=SimpleNamespace(db_path=index),
            )
            revisions = ["main", "next"] if scenario == "repo-advanced" else ["main", "main"]
            call = iter(revisions)

            def read(*args: Any, scenario: str = scenario) -> Any:
                if scenario == "missing":
                    raise FileNotFoundError("synthetic")
                return SimpleNamespace(
                    document=SimpleNamespace(frontmatter=SimpleNamespace(id="12345678"))
                )

            item: dict[str, Any] = {
                "scenario": scenario,
                "schema": schema,
                "states": states,
                "references": refs,
                "changes": changes,
                "policy": asdict(policy),
                "revision": "old" if scenario == "stale-repo" else "main",
                "revisions": revisions,
                "kinds": kinds,
                "versions": versions,
            }
            with (
                patch.object(service, "get_main_revision", lambda _, call=call: next(call)),
                patch.object(service, "read_bundle_entry", read),
                patch.object(service, "list_asset_kinds", lambda *args, kinds=kinds: kinds),
                patch.object(
                    service,
                    "list_asset_versions",
                    lambda root, id, kind, versions=versions: versions[kind],
                ),
                patch.object(
                    service,
                    "load_asset_metadata",
                    lambda root, id, kind, version: {"zip_sha256": f"{kind}:{version}"},
                ),
            ):
                try:
                    item["expected"] = runtime._archival_impact(
                        policy, runtime._normalize_changes(changes), item["revision"]
                    )
                except Exception as exc:
                    item["error_type"] = type(exc).__name__
                    item["error"] = str(exc)
            cases.append(item)
    return cases


def visible_fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    authz: Any = importlib.import_module("memento.authz")
    proposals: Any = importlib.import_module("memento.control.proposals")
    runtime = object.__new__(service.MemoryService)
    runtime._deps = SimpleNamespace(repo_paths=None)
    runtime._archival_impact = lambda policy, changes, revision: [
        {"recomputed_for": policy.principal}
    ]
    policy = authz.EffectivePolicy("reviewer", ("curator",), ("/public/",), ("/public/",), ())
    reports = [
        {
            "path": "/public/a.md",
            "inbound_references": [{"path": "/public/source.md"}, {"path": "/private/source.md"}],
            "retained": "yes",
        },
        {"path": "/private/a.md", "inbound_references": []},
    ]
    cases = []
    for status in proposals.ProposalStatus:
        for revision in ["main", "old"]:
            record = SimpleNamespace(
                status=status,
                base_revision=revision,
                patch={
                    "changes": [{"kind": "trash", "path": "/public/a.md"}],
                    "archival_impact": reports,
                },
            )
            with patch.object(service, "get_main_revision", lambda _: "main"):
                expected = runtime._visible_archival_impact(record, policy)
            cases.append(
                {
                    "status": status.value,
                    "base_revision": revision,
                    "patch": json.loads(json.dumps(record.patch)),
                    "policy": asdict(policy),
                    "expected": expected,
                }
            )
    return cases
