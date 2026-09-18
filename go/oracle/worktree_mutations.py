"""Capture disposable worktree concept mutations, including partial failures."""

from __future__ import annotations

import importlib
import tempfile
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    schema: Any = importlib.import_module("memento.repository.schema")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    authz: Any = importlib.import_module("memento.authz")
    clock = datetime(2026, 9, 18, 5, 0, 0, 123456, tzinfo=UTC)

    def concept(id: str, body: str, title: str = "Title") -> str:
        return str(
            frontmatter.serialize_concept(
                schema.ConceptDocument(
                    frontmatter=schema.ConceptFrontmatter(
                        id=id,
                        type="concept",
                        title=title,
                        status="active",
                        description="original",
                        tags=("original",),
                        created_at=clock,
                        updated_at=clock,
                        updated_by="original",
                    ),
                    body=body,
                )
            )
        )

    cases = []
    for scenario in [
        "create",
        "create-exists",
        "create-invalid",
        "create-size",
        "patch",
        "patch-null",
        "patch-copy",
        "patch-status",
        "patch-missing",
        "patch-invalid",
        "patch-size",
        "patch-default",
        "patch-default-override",
        "rename",
        "rename-same",
        "rename-missing",
        "rename-collision",
        "rename-unwritable-ref",
        "rename-invalid-ref",
        "rename-self-link",
        "rename-size",
        "rename-rewrite-size",
        "trash",
        "trash-collision",
        "trash-denied",
        "trash-invalid",
        "trash-path",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-worktree-") as directory:
            root = Path(directory)
            initial = {
                "/public/a.md": concept("12345678", "Original [self](/public/a.md).\n"),
                "/public/ref.md": concept(
                    "87654321", "[target](/public/a.md#section) and `[/public/a.md]`"
                ),
                "/private/ref.md": concept("11111111", "no link"),
            }
            limit = 65536
            kind = scenario.split("-")[0]
            if kind == "create":
                change: dict[str, Any] = {
                    "kind": "create",
                    "path": "/new/deep/a.md",
                    "concept_type": "concept",
                    "title": "New",
                    "body": "\nHello  \r\n",
                    "tags": ["z", "", "a", "z"],
                    "aliases": ["b", "a"],
                }
                if scenario == "create-exists":
                    change["path"] = "/public/a.md"
                if scenario == "create-invalid":
                    change["concept_type"] = "not-controlled"
                if scenario == "create-size":
                    limit = 10
            elif kind == "patch":
                change = {
                    "kind": "patch",
                    "path": "/public/a.md",
                    "title": "Patched",
                    "description": "",
                    "body": "Patched  \r\n",
                    "tags": ["b", "", "a", "b"],
                    "aliases": [],
                }
                if scenario == "patch-null":
                    change = {
                        "kind": "patch",
                        "path": "/public/a.md",
                        "title": None,
                        "description": None,
                        "body": None,
                        "tags": None,
                        "aliases": None,
                    }
                if scenario == "patch-copy":
                    change["title"] = ""
                if scenario in {"patch-status", "patch-default-override"}:
                    change["status"] = "deprecated"
                if scenario == "patch-missing":
                    change["path"] = "/public/missing.md"
                if scenario == "patch-invalid":
                    initial["/public/a.md"] = "not a concept"
                if scenario == "patch-size":
                    limit = 10
                if scenario.startswith("patch-default"):
                    initial["/public/a.md"] = initial["/public/a.md"].replace(
                        "status: active\n", ""
                    )
            elif kind == "rename":
                change = {
                    "kind": "rename",
                    "path": "/public/a.md",
                    "new_path": "/new/deep/renamed.md",
                }
                if scenario == "rename-same":
                    change["new_path"] = "/public/a.md"
                if scenario == "rename-missing":
                    change["path"] = "/public/missing.md"
                if scenario == "rename-collision":
                    change["new_path"] = "/public/ref.md"
                if scenario == "rename-unwritable-ref":
                    initial["/private/ref.md"] = concept("11111111", "[target](/public/a.md)")
                if scenario == "rename-invalid-ref":
                    initial["/private/ref.md"] = "not a concept"
                if scenario == "rename-size":
                    limit = 10
                if scenario == "rename-rewrite-size":
                    limit = 600
                    initial["/public/ref.md"] = concept("87654321", "[target](/public/a.md) " * 50)
            else:
                change = {"kind": "trash", "path": "/public/a.md"}
                if scenario == "trash-collision":
                    initial["/trash/public/a.md"] = "collision"
                if scenario == "trash-invalid":
                    initial["/public/a.md"] = "not a concept"
                if scenario == "trash-path":
                    change["path"] = "/trash/public/a.md"
            for path, text in initial.items():
                file = root / path.removeprefix("/")
                file.parent.mkdir(parents=True, exist_ok=True)
                file.write_text(text)
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                config=SimpleNamespace(limits=SimpleNamespace(max_concept_bytes=limit))
            )
            runtime._now = lambda: clock
            policy = authz.EffectivePolicy(
                "actor",
                ("curator",),
                ("/",),
                () if scenario == "trash-denied" else ("/public/", "/new/"),
                (),
            )
            item: dict[str, Any] = {
                "scenario": scenario,
                "kind": kind,
                "initial": initial,
                "change": change,
                "limit": limit,
                "policy": {
                    "principal": policy.principal,
                    "roles": policy.roles,
                    "read_prefixes": policy.read_prefixes,
                    "write_prefixes": policy.write_prefixes,
                    "protected_read_prefixes": policy.protected_read_prefixes,
                },
            }
            with patch.object(service, "uuid4", lambda: "00000000-0000-4000-8000-000000000000"):
                try:
                    normalized = runtime._normalize_changes([change])
                    item["changed"] = runtime._apply_changes(
                        root, normalized, actor="actor", policy=policy
                    )
                except Exception as exc:
                    item["error_type"] = type(exc).__name__
                    item["error"] = str(exc)
            item["files"] = {
                "/" + path.relative_to(root).as_posix(): path.read_text()
                for path in sorted(root.rglob("*"))
                if path.is_file()
            }
            item["directories"] = [
                "/" + path.relative_to(root).as_posix()
                for path in sorted(root.rglob("*"))
                if path.is_dir()
            ]
            cases.append(item)
    return cases
