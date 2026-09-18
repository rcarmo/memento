"""Pinned FTS5 ranking/filter pages and plain-query transformations."""

from __future__ import annotations

import importlib
import tempfile
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from typing import Any


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.derived.index")
    authz: Any = importlib.import_module("memento.authz")
    schema: Any = importlib.import_module("memento.repository.schema")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    queries = []
    for text in [
        "",
        "!!!",
        "what is the alpha?",
        "the and is",
        "ALPHA alpha ALPHA",
        "é á 中文 ² Ⅵ _",
        "Straße STRASSE",
        "\x1c\t ",
        "a OR b",
        '"unterminated',
        "hello-world/path",
    ]:
        for syntax in ["plain", "fts5", "invalid"]:
            item: dict[str, Any] = {"query": text, "syntax": syntax}
            try:
                item["expected"] = module._lexical_query(text, query_syntax=syntax)
            except Exception as exc:
                item["error"] = str(exc)
            queries.append(item)
    with tempfile.TemporaryDirectory(prefix="memento-search-") as directory:
        root = Path(directory) / "bundle"
        root.mkdir()
        documents = [
            ("a", "/public/a.md", "Alpha", "alpha beta alpha", "concept", "active", ("x", "y")),
            ("b", "/public/b.md", "Beta", "alpha beta", "project", "deprecated", ("y",)),
            (
                "c",
                "/public/nested/c.md",
                "Third",
                "zero " * 30 + "alpha beta " + "tail " * 30,
                "concept",
                "active",
                ("x",),
            ),
            ("p", "/private/p.md", "Secret", "alpha " * 20, "concept", "active", ("x",)),
            ("n", "/private/nested/n.md", "Nested", "alpha beta", "concept", "active", ()),
            ("t", "/trash/public/t.md", "Trashed", "alpha beta", "concept", "active", ()),
            (
                "u",
                "/public/a_b%/u.md",
                "Unicode é",
                "alpha 中文 " + "😀 " * 180,
                "concept",
                "active",
                ("x",),
            ),
        ]
        files = {}
        stamp = datetime(2026, 1, 1, tzinfo=UTC)
        for id, path, title, body, kind, status, tags in documents:
            text = frontmatter.serialize_concept(
                schema.ConceptDocument(
                    frontmatter=schema.ConceptFrontmatter(
                        id=id,
                        type=kind,
                        title=title,
                        status=status,
                        tags=tags,
                        aliases=("alias",),
                        created_at=stamp,
                        updated_at=stamp,
                        updated_by="actor",
                    ),
                    body=body,
                )
            )
            files[path] = text
            target = root / path.removeprefix("/")
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(text)
        index = module.DerivedIndex(Path(directory) / "index.sqlite")
        index.rebuild(root, repo_revision="main")
        policies = [
            authz.EffectivePolicy("reader", ("reader",), ("/",), (), ("/private/",)),
            authz.EffectivePolicy(
                "reader", ("reader",), ("/public/", "/private/nested/"), (), ("/private/",)
            ),
            authz.EffectivePolicy("admin", ("admin",), ("/",), (), ("/private/",)),
            authz.EffectivePolicy("reader", ("reader",), ("/public/a_b%/",), (), ()),
        ]
        options: list[dict[str, Any]] = [
            {"query": "alpha", "query_syntax": "plain", "limit": 1},
            {"query": "alpha", "query_syntax": "plain", "limit": 2, "cursor": "offset:1"},
            {"query": "alpha beta", "query_syntax": "plain", "limit": 200},
            {"query": "alpha", "query_syntax": "fts5", "limit": 0},
            {"query": "title:Alpha OR body:beta", "query_syntax": "fts5", "limit": 20},
            {"query": "alpha", "query_syntax": "plain", "concept_type": "project"},
            {"query": "alpha", "query_syntax": "plain", "status": "deprecated"},
            {"query": "alpha", "query_syntax": "plain", "tags": ("x", "y")},
            {"query": "alpha", "query_syntax": "plain", "path_prefix": "/public/a_b%/"},
            {"query": "alpha", "query_syntax": "plain", "cursor": "offset:999"},
            {"query": "alpha", "query_syntax": "plain", "cursor": "offset:-1"},
            {"query": "unknowncolumn:alpha", "query_syntax": "fts5"},
            {"query": '"unterminated', "query_syntax": "fts5"},
            {"query": "", "query_syntax": "plain"},
        ]
        pages = []
        for policy in policies:
            for opts in options:
                item = {"policy": asdict(policy), "options": opts}
                try:
                    item["expected"] = asdict(index.search(policy=policy, **opts))
                except Exception as exc:
                    item["error"] = str(exc)
                pages.append(item)
    return {"queries": queries, "files": files, "pages": pages}
