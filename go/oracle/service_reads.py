"""Models-off read/list/search/graph service contracts with real index data."""

from __future__ import annotations

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
    index_module: Any = importlib.import_module("memento.derived.index")
    schema: Any = importlib.import_module("memento.repository.schema")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    authz: Any = importlib.import_module("memento.authz")
    files = {}
    for id, path, description, body in [
        ("a", "/public/a.md", "description", "[b](/public/b.md) alpha"),
        ("b", "/public/b.md", None, "beta alpha"),
        ("p", "/private/p.md", None, "secret alpha"),
        ("t", "/trash/public/t.md", None, "archived alpha"),
    ]:
        text = frontmatter.serialize_concept(
            schema.ConceptDocument(
                frontmatter=schema.ConceptFrontmatter(
                    id=id,
                    type="concept",
                    title=id,
                    status="active",
                    description=description,
                    tags=("z", "a", "a"),
                    aliases=("alias",),
                    source_refs=("synthetic",),
                    supersedes=(),
                    created_at=datetime(2026, 1, 1, tzinfo=UTC),
                    updated_at=datetime(2026, 9, 18, tzinfo=UTC),
                    updated_by="actor",
                ),
                body=body,
            )
        )
        # Preserve microseconds in parsed model_dump, unlike source serializer.
        files[path] = text.replace("2026-09-18T00:00:00Z", "2026-09-18T00:00:00.123456Z")
    policies = [
        authz.EffectivePolicy("actor", ("proposer",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("actor", ("reader",), ("/public/",), (), ("/private/",)),
        authz.EffectivePolicy("admin", ("admin",), ("/",), (), ("/private/",)),
    ]
    commands: list[tuple[str, dict[str, Any]]] = [
        ("read", {"id_or_path": "/public/a.md"}),
        ("read", {"id_or_path": "a"}),
        ("read", {"id_or_path": "/private/p.md"}),
        ("read", {"id_or_path": "p"}),
        ("read", {"id_or_path": "/missing.md"}),
        ("read", {"id_or_path": "missing"}),
        ("read", {"id_or_path": "/trash/public/t.md"}),
        ("read", {"id_or_path": "/public/../a.md"}),
        ("list", {}),
        ("list", {"path_prefix": "/public/a"}),
        ("list", {"path_prefix": "/trash/"}),
        ("list", {"path_prefix": "/trash"}),
        ("list", {"path_prefix": "relative"}),
        ("search", {"query": "what is alpha?"}),
        ("search", {"query": "alpha", "limit": 1, "cursor": "offset:1"}),
        ("search", {"query": "alpha", "search_mode": "semantic"}),
        ("search", {"query": "alpha", "search_mode": "hybrid"}),
        ("search", {"query": "alpha", "search_mode": "invalid"}),
        ("search", {"query": "alpha", "search_mode": ""}),
        ("search", {"query": '"unterminated', "query_syntax": "fts5"}),
        ("graph", {"id_or_path": "a", "depth": 2}),
        ("graph", {"id_or_path": "/public/a.md"}),
        ("graph", {"id_or_path": "p"}),
        ("graph", {"id_or_path": "/private/p.md"}),
        ("graph", {"id_or_path": "missing"}),
        ("graph", {"id_or_path": "/missing.md"}),
    ]
    cases = []
    with tempfile.TemporaryDirectory(prefix="memento-read-service-") as directory:
        root = Path(directory) / "bundle"
        root.mkdir()
        for path, text in files.items():
            target = root / path.removeprefix("/")
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(text)
        index = index_module.DerivedIndex(Path(directory) / "index.sqlite")
        index.rebuild(root, repo_revision="indexed")
        index.set_repo_revision("main")
        runtime = object.__new__(service.MemoryService)
        runtime._deps = SimpleNamespace(
            repo_paths=SimpleNamespace(current_dir=root),
            derived_index=index,
            config=SimpleNamespace(
                intelligent_tiers=SimpleNamespace(
                    semantic_search=SimpleNamespace(default_search_mode="lexical")
                )
            ),
        )
        for policy in policies:
            runtime._policy = lambda _, policy=policy: policy
            for method, args in commands:
                with patch.object(service, "get_main_revision", lambda _: "main"):
                    result = getattr(runtime, "memory_" + method)(None, **args)
                cases.append(
                    {
                        "method": method,
                        "arguments": args,
                        "policy": asdict(policy),
                        "expected": result.model_dump(mode="json"),
                    }
                )
        malformed = []
        for path in ["/private/bad.md", "/public/bad.md", "/trash/public/bad.md"]:
            file = root / path.removeprefix("/")
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_text("not a concept")
            runtime._policy = lambda _: policies[0]
            for method, args in [
                ("read", {"id_or_path": "a"}),
                ("read", {"id_or_path": "/public/a.md"}),
                ("list", {"path_prefix": "/public/a"}),
            ]:
                with patch.object(service, "get_main_revision", lambda _: "main"):
                    result = getattr(runtime, "memory_" + method)(None, **args)
                malformed.append(
                    {
                        "path": path,
                        "method": method,
                        "arguments": args,
                        "expected": result.model_dump(mode="json"),
                    }
                )
            file.unlink()
    return {"files": files, "cases": cases, "malformed": malformed}
