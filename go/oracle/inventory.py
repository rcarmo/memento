"""Inventory prefix intersections, projections and parse-after-pagination semantics."""

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
    files["/public/sub/b.md"] = files["/public/empty.md"]
    files["/public/a.md"] = base64.b64encode(
        base64.b64decode(files["/public/a.md"]).replace(b"synthetic", "synthetic café\n".encode())
    ).decode()
    policies = [
        authz.EffectivePolicy("actor", ("reader",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("actor", ("reader",), ("/public/sub/",), (), ("/private/",)),
        authz.EffectivePolicy("actor", ("reader",), ("/private/",), (), ("/private/",)),
        authz.EffectivePolicy("actor", ("proposer",), ("/",), (), ("/private/",)),
    ]
    commands: list[dict[str, Any]] = [
        {},
        {"path_prefix": "/public/"},
        {"path_prefix": "/public/sub/"},
        {"path_prefix": "/private/"},
        {"path_prefix": "/trash/"},
        {"path_prefix": "/trash/public/"},
        {"path_prefix": "/missing/"},
        {"path_prefix": "/public/", "limit": 1},
        {"path_prefix": "/public/", "limit": 1, "cursor": "/public/a.md"},
        {"path_prefix": "/public/", "cursor": "/public/zz.md"},
        {"fields": ["title", "path", "title"]},
        {"fields": ["body_bytes"]},
        {"fields": ["body_sha256"]},
        {"fields": ["assets"]},
        {"fields": []},
        {"fields": "path"},
        {"fields": ""},
        {"fields": ["z", "a", "z"]},
        {"limit": 0},
        {"limit": 101},
        {"limit": True},
        {"cursor": "/private/p.md", "path_prefix": "/public/"},
        {"cursor": "/public/"},
        {"cursor": "/public/../a.md"},
        {"path_prefix": "public/"},
        {"path_prefix": "/public"},
        {"path_prefix": "//"},
        {"path_prefix": "/a//b/"},
        {"path_prefix": "/a/../"},
        {"path_prefix": "/a\\b/"},
        {"path_prefix": "/a\u0000b/"},
    ]
    cases: list[dict[str, Any]] = []
    with tempfile.TemporaryDirectory(prefix="memento-inventory-") as directory:
        root = Path(directory)
        for path, encoded in files.items():
            target = root / path[1:]
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(base64.b64decode(encoded))
        memory = object.__new__(service.MemoryService)
        memory._deps = SimpleNamespace(repo_paths=SimpleNamespace(current_dir=root))

        def run(args: dict[str, Any], policy: Any, mutation: dict[str, Any] | None = None) -> None:
            memory._policy = lambda _: policy
            with patch.object(service, "get_main_revision", lambda _: "main"):
                result = memory.memory_inventory(None, **args).model_dump(mode="json")
            cases.append(
                {
                    "arguments": args,
                    "policy": asdict(policy),
                    "mutation": mutation,
                    "expected": result,
                }
            )

        for policy in policies:
            for args in commands:
                run(args, policy)
        for path in [
            "/private/bad.md",
            "/public/zbad.md",
            "/public/sub/bad.md",
            "/trash/public/bad.md",
        ]:
            target = root / path[1:]
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text("not a concept")
            mutation = {"path": path, "base64": base64.b64encode(b"not a concept").decode()}
            malformed_commands: list[dict[str, Any]] = [
                {"limit": 1},
                {"path_prefix": "/public/", "limit": 1},
                {"path_prefix": "/public/", "cursor": "/public/empty.md"},
                {"path_prefix": "/trash/"},
            ]
            for args in malformed_commands:
                run(args, policies[0], mutation)
            target.unlink()
        metadata_path = "/.assets/12345678/docs/1.10.0.json"
        original = base64.b64decode(files[metadata_path])
        metadata = json.loads(original)
        for digest in [None, 17, "unverified"]:
            encoded = base64.b64encode(
                json.dumps({**metadata, "zip_sha256": digest}).encode()
            ).decode()
            (root / metadata_path[1:]).write_bytes(base64.b64decode(encoded))
            for fields in [["path"], ["assets"]]:
                run(
                    {"path_prefix": "/public/", "limit": 1, "fields": fields},
                    policies[0],
                    {"path": metadata_path, "base64": encoded},
                )
        (root / metadata_path[1:]).write_bytes(original)
    return {"files": files, "cases": cases}
