"""Accepted-asset service reads with real metadata/ZIP files and source locks."""

from __future__ import annotations

import base64
import importlib
import json
import tempfile
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch

from asset_pack import archive


def fixtures() -> dict[str, Any]:
    service: Any = importlib.import_module("memento.service")
    schema: Any = importlib.import_module("memento.repository.schema")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    packs: Any = importlib.import_module("memento.skill_packs")
    accepted: Any = importlib.import_module("memento.repository.asset_packs")
    authz: Any = importlib.import_module("memento.authz")
    blob = archive(
        [
            ("file.txt", "synthetic café\n".encode(), 0o100644),
            ("binary.bin", bytes([0, 255, 1]), 0o100644),
        ]
    )
    manifest = packs.validate_asset_pack(
        asset_kind="docs", version="1.0.0", zip_bytes=blob
    ).manifest
    digest = manifest.sha256
    policies = [
        authz.EffectivePolicy("actor", ("reader",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("actor", ("proposer",), ("/",), (), ("/private/",)),
    ]
    cases = []
    with tempfile.TemporaryDirectory(prefix="memento-asset-get-") as directory:
        root = Path(directory)
        for id, path in [
            ("12345678", "/public/a.md"),
            ("abcdef12", "/private/p.md"),
            ("eeeeeeee", "/public/empty.md"),
        ]:
            target = root / path[1:]
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(
                frontmatter.serialize_concept(
                    schema.ConceptDocument(
                        frontmatter=schema.ConceptFrontmatter(
                            id=id,
                            type="concept",
                            title=id,
                            status="active",
                            created_at=datetime(2026, 1, 1, tzinfo=UTC),
                            updated_at=datetime(2026, 1, 1, tzinfo=UTC),
                            updated_by="actor",
                        ),
                        body="synthetic",
                    )
                )
            )
        for version in ["1.9.0", "1.10.0"]:
            accepted.write_asset_version(
                root,
                concept_id="12345678",
                concept_path="/public/a.md",
                asset_kind="docs",
                version=version,
                zip_bytes=blob,
                manifest=manifest,
                accepted_by="actor",
                source_proposal_id="p",
            )
        files = {
            "/" + str(p.relative_to(root)): base64.b64encode(p.read_bytes()).decode()
            for p in root.rglob("*")
            if p.is_file()
        }
        memory = object.__new__(service.MemoryService)
        memory._deps = SimpleNamespace(
            repo_paths=SimpleNamespace(current_dir=root, bare_dir=root / "bare")
        )
        commands: list[dict[str, Any]] = [
            {},
            {"id_or_path": "12345678"},
            {"view": "manifest"},
            {"view": "file", "file_path": "file.txt"},
            {"view": "file", "file_path": "binary.bin"},
            {"offset": True},
            {"limit": False},
            {"offset": -1, "limit": True},
            {"view": "invalid", "offset": True},
            {"limit": 7},
            {"offset": 5, "limit": 9, "version": "1.9.0", "expected_sha256": digest},
            {"offset": len(blob), "version": "1.9.0", "expected_sha256": digest},
            {
                "view": "file",
                "file_path": "file.txt",
                "offset": 10,
                "limit": 1,
                "version": "1.9.0",
                "expected_sha256": digest,
            },
            {"id_or_path": "/private/p.md"},
            {"id_or_path": "abcdef12"},
            {"id_or_path": "/missing.md"},
            {"id_or_path": "/public/empty.md"},
            {"version": "3.0.0"},
            {"version": "bad"},
            {"asset_kind": "INVALID"},
            {"view": "invalid"},
            {"offset": -1},
            {"limit": 0},
            {"limit": 262145},
            {"view": "file"},
            {"file_path": "file.txt"},
            {"view": "manifest", "limit": 5},
            {"offset": 1},
            {"offset": 1, "version": "1.9.0"},
            {"offset": 1, "expected_sha256": digest},
            {"expected_sha256": "0" * 64},
            {"view": "file", "file_path": "missing"},
            {"view": "file", "file_path": "../file.txt"},
            {"offset": len(blob) + 1, "version": "1.9.0", "expected_sha256": digest},
        ]

        def run(args: dict[str, Any], policy: Any, mutation: dict[str, Any] | None = None) -> None:
            memory._policy = lambda _: policy
            with patch.object(service, "get_main_revision", lambda _: "main"):
                response = memory.memory_asset_get(None, **args).model_dump(mode="json")
            cases.append(
                {
                    "arguments": args,
                    "policy": asdict(policy),
                    "mutation": mutation,
                    "expected": response,
                }
            )

        for policy in policies:
            for command in commands:
                run({"id_or_path": "/public/a.md", "asset_kind": "docs", **command}, policy)
        meta_path = "/.assets/12345678/docs/1.10.0.json"
        zip_path = "/.assets/12345678/docs/1.10.0.zip"
        original = (root / meta_path[1:]).read_bytes()
        metadata = json.loads(original)
        for field, value in [
            ("concept_id", "abcdef12"),
            ("asset_kind", "other"),
            ("version", "1.0.0"),
            ("zip_sha256", "invalid"),
            ("zip_sha256", "0" * 64),
            ("manifest", {**metadata["manifest"], "file_count": 3}),
        ]:
            replacement = json.dumps({**metadata, field: value}).encode()
            (root / meta_path[1:]).write_bytes(replacement)
            run(
                {"id_or_path": "/public/a.md", "asset_kind": "docs"},
                policies[0],
                {"path": meta_path, "base64": base64.b64encode(replacement).decode()},
            )
        (root / meta_path[1:]).write_bytes(original)
        for bad in [b"", b"corrupt archive"]:
            (root / zip_path[1:]).write_bytes(bad)
            for view in ["archive", "manifest", "file"]:
                args = {"id_or_path": "/public/a.md", "asset_kind": "docs", "view": view}
                if view == "file":
                    args["file_path"] = "file.txt"
                run(args, policies[0], {"path": zip_path, "base64": base64.b64encode(bad).decode()})
    return {"files": files, "cases": cases}
