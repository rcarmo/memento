"""Asset metadata paging, budgets, validation and no archive verification."""

from __future__ import annotations

import base64
import importlib
import json
import tempfile
from dataclasses import asdict
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch

from asset_get import fixtures as asset_fixtures


def fixtures() -> dict[str, Any]:
    service: Any = importlib.import_module("memento.service")
    authz: Any = importlib.import_module("memento.authz")
    files = asset_fixtures()["files"]
    files["/trash/public/t.md"] = files["/public/empty.md"]
    policies = [
        authz.EffectivePolicy("reader", ("reader",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("proposer", ("proposer",), ("/",), (), ()),
        authz.EffectivePolicy("reader", ("reader",), ("/else/",), (), ()),
    ]
    cases = []
    with tempfile.TemporaryDirectory(prefix="memento-asset-metadata-") as directory:
        root = Path(directory)
        for path, encoded in files.items():
            target = root / path[1:]
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(base64.b64decode(encoded))
        runtime = object.__new__(service.MemoryService)
        runtime._deps = SimpleNamespace(repo_paths=SimpleNamespace(current_dir=root))

        def run(args: dict[str, Any], policy: Any, mutation: dict[str, Any] | None = None) -> None:
            runtime._policy = lambda _: policy
            with (
                patch.object(service, "get_main_revision", lambda _: "main"),
                patch.object(
                    service,
                    "get_path_commit_timestamp",
                    lambda *a, **kw: "2026-01-02T03:04:05+01:00",
                ),
            ):
                result = runtime.memory_asset_metadata(None, **args).model_dump(mode="json")
            cases.append(
                {
                    "arguments": args,
                    "policy": asdict(policy),
                    "mutation": mutation,
                    "expected": result,
                }
            )

        commands: list[dict[str, Any]] = [
            {},
            {"id_or_path": "12345678"},
            {"id_or_path": "/public/a.md", "include_files": True, "file_limit": 1},
            {"path_prefix": "/public/", "limit": 1},
            {"path_prefix": "/public/", "cursor": "/public/a.md"},
            {"path_prefix": "/trash/"},
            {"path_prefix": ""},
            {"asset_kind": "missing"},
            {"asset_kind": "docs", "version": "9.0.0"},
            {"asset_kind": "docs", "version": "1.9.0", "include_files": True},
            {"version_limit": 1},
            {"include_files": 1},
            {"id_or_path": "/private/p.md"},
            {"id_or_path": "missing"},
            {"id_or_path": "/missing.md"},
            {"id_or_path": "/public/a.md", "path_prefix": "/"},
            {"id_or_path": "/public/a.md", "cursor": "/public/a.md"},
            {"version": "1.0.0"},
            {"limit": 0},
            {"limit": 21},
            {"version_limit": 0},
            {"version_limit": 6},
            {"file_limit": 0},
            {"file_limit": 101},
            {"asset_kind": "BAD"},
            {"asset_kind": "docs", "version": "bad"},
            {"path_prefix": "bad"},
            {"cursor": "/else/a.md", "path_prefix": "/public/"},
            {"cursor": "bad"},
            {"path_prefix": "/private/"},
        ]
        for command in commands:
            run(command, policies[0])
        for policy in policies[1:]:
            run({}, policy)
        path = "/.assets/12345678/docs/1.10.0.json"
        original = base64.b64decode(files[path])
        metadata = json.loads(original)
        mutations = [
            ("schema_version", 2),
            ("schema_version", True),
            ("schema_version", 1.0),
            ("kind", "bad"),
            ("concept_id", "bad"),
            ("asset_kind", "bad"),
            ("version", "bad"),
            ("concept_path", "relative"),
            ("zip_sha256", "BAD"),
            ("accepted_by", ""),
            ("source_proposal_id", None),
            ("created_at", "2026-09-18T12:00:00.123456+02:00"),
            ("created_at", "bad"),
            ("created_at", "2026-09-18"),
            ("manifest", {**metadata["manifest"], "sha256": "0" * 64}),
            ("manifest", {**metadata["manifest"], "file_count": 999}),
        ]
        for key, value in mutations:
            content = json.dumps({**metadata, key: value}).encode()
            (root / path[1:]).write_bytes(content)
            for version in [None, "9.0.0"]:
                args: dict[str, Any] = {
                    "id_or_path": "/public/a.md",
                    "asset_kind": "docs",
                    "version": version,
                    "include_files": True,
                }
                run(args, policies[0], {"path": path, "base64": base64.b64encode(content).decode()})
        (root / path[1:]).write_bytes(original)
        zip_path = "/.assets/12345678/docs/1.10.0.zip"
        for blob in [b"", b"not a zip"]:
            (root / zip_path[1:]).write_bytes(blob)
            run(
                {"id_or_path": "/public/a.md", "include_files": True},
                policies[0],
                {"path": zip_path, "base64": base64.b64encode(blob).decode()},
            )
        (root / zip_path[1:]).write_bytes(base64.b64decode(files[zip_path]))
        # Metadata-only skill fixtures intentionally use arbitrary archive bytes;
        # this endpoint trusts manifest shape/digest equality, not ZIP contents.
        for match in [True, False]:
            import hashlib

            skill = {
                **metadata,
                "asset_kind": "skill",
                "version": "1.0.0",
                "manifest": {
                    **metadata["manifest"],
                    "entries": [
                        {
                            **metadata["manifest"]["entries"][0],
                            "path": "SKILL.md",
                            "sha256": hashlib.sha256(
                                b"synthetic" if match else b"different"
                            ).hexdigest(),
                        }
                    ],
                },
            }
            additions = {
                "/.assets/12345678/skill/1.0.0.json": base64.b64encode(
                    json.dumps(skill).encode()
                ).decode(),
                "/.assets/12345678/skill/1.0.0.zip": base64.b64encode(b"synthetic").decode(),
            }
            for name, encoded in additions.items():
                target = root / name[1:]
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(base64.b64decode(encoded))
            run(
                {"id_or_path": "/public/a.md", "asset_kind": "skill", "include_files": True},
                policies[0],
                {"files": additions},
            )
            for name in additions:
                (root / name[1:]).unlink()
    budgets = []
    for scenario in [
        "kinds50",
        "kinds51",
        "empty-kinds51",
        "versions50",
        "versions55",
        "files500",
        "files501",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-metadata-budget-") as directory:
            root = Path(directory)
            concept = base64.b64decode(files["/public/a.md"])
            (root / "a.md").write_bytes(concept)
            meta = json.loads(base64.b64decode(files["/.assets/12345678/docs/1.10.0.json"]))
            kind_count = (
                51
                if "51" in scenario
                else 50
                if scenario == "kinds50"
                else 11
                if scenario == "versions55"
                else 10
                if scenario == "versions50"
                else 6
                if scenario == "files501"
                else 5
            )
            version_count = 5 if scenario.startswith("versions") else 1
            for i in range(kind_count):
                kind = f"k{i:02}"
                target = root / ".assets" / "12345678" / kind
                target.mkdir(parents=True)
                if scenario == "empty-kinds51":
                    continue
                for v in range(version_count):
                    version = f"1.0.{v}"
                    item = {
                        **meta,
                        "asset_kind": kind,
                        "version": version,
                        "created_at": "2026-01-01T00:00:00Z",
                    }
                    if scenario.startswith("files"):
                        count = 1 if i == 5 else 100
                        item["manifest"] = {
                            **meta["manifest"],
                            "entries": [
                                {**meta["manifest"]["entries"][0], "path": f"f{n:03}.txt"}
                                for n in range(count)
                            ],
                        }
                    (target / f"{version}.json").write_text(json.dumps(item))
                    (target / f"{version}.zip").write_bytes(b"metadata only")
            budget_files = {
                "/" + str(p.relative_to(root)): base64.b64encode(p.read_bytes()).decode()
                for p in root.rglob("*")
                if p.is_file()
            }
            empty_dirs = [
                "/" + str(p.relative_to(root))
                for p in root.rglob("*")
                if p.is_dir() and not any(p.iterdir())
            ]
            runtime._deps.repo_paths.current_dir = root
            runtime._policy = lambda _: policies[0]
            args = {
                "id_or_path": "/a.md",
                "include_files": scenario.startswith("files"),
                "file_limit": 100,
            }
            with patch.object(service, "get_main_revision", lambda _: "main"):
                response = runtime.memory_asset_metadata(None, **args).model_dump(mode="json")
            budgets.append(
                {
                    "scenario": scenario,
                    "files": budget_files,
                    "directories": empty_dirs,
                    "arguments": args,
                    "policy": asdict(policies[0]),
                    "expected": response,
                }
            )
    return {"files": files, "cases": cases, "budgets": budgets}
