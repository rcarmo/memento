"""Synthetic bundle scanning, audit and generated Markdown from the baseline."""

from __future__ import annotations

import importlib
import json
import tempfile
from dataclasses import asdict
from pathlib import Path
from typing import Any


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.repository.bundle")
    schema: Any = importlib.import_module("memento.repository.schema")
    base = {
        "id": "one",
        "type": "concept",
        "title": "Alpha",
        "status": "active",
        "created_at": "2026-09-18T00:00:00Z",
        "updated_at": "2026-09-18T00:00:00Z",
        "updated_by": "test-agent",
    }
    datasets = [
        [],
        [("/a.md", base, "")],
        [
            (
                "/a.md",
                base,
                "[good](/nested/b.md) [bad](/missing.md) [reserved](/index.md) [external](https://example.com) [network](//host/a)",
            ),
            (
                "/nested/b.md",
                {
                    **base,
                    "id": "two",
                    "title": "Link [trap](x)",
                    "updated_at": "2026-09-18T00:00:00.123456Z",
                    "updated_by": "agent_*`demo`",
                },
                "[bad](/missing.md#anchor) ![ignored](/image.md)",
            ),
            ("/nested/deep/c.md", {**base, "id": "one", "title": "Straße"}, ""),
            ("/Z.md", {**base, "id": "four", "title": "éclair"}, ""),
            ("/alpha.md", {**base, "id": "five", "title": "alpha"}, ""),
            (
                "/lower.md",
                {**base, "id": "six", "title": "strasse"},
                "[asset](/.assets/blob) [hidden](/hidden.md)",
            ),
        ],
    ]
    generated = []
    for dataset in datasets:
        entries = [
            module.BundleEntry(
                bundle_path=path,
                document=schema.ConceptDocument(
                    frontmatter=schema.ConceptFrontmatter.model_validate(meta), body=body
                ),
            )
            for path, meta, body in dataset
        ]
        bundle = module.RepositoryBundle(root=Path("/synthetic"), entries=tuple(entries))
        generated.append(
            {
                "entries": [
                    {"path": path, "metadata": meta, "body": body} for path, meta, body in dataset
                ],
                "indexes": module.generate_directory_indexes(bundle),
                "log": module.generate_root_log(bundle),
            }
        )
    scans = []
    files = {
        path: "---\n"
        + "\n".join(f"{k}: {json.dumps(v)}" for k, v in meta.items())
        + "\n---\n"
        + body
        + "\n"
        for path, meta, body in datasets[-1]
    }
    for tree in ["normal", "symlink-file", "symlink-dir", "directory-md", "invalid-concept"]:
        for filtered in [False, True]:
            with tempfile.TemporaryDirectory(prefix="memento-bundle-oracle-") as directory:
                root = Path(directory)
                for path, content in files.items():
                    target = root / path[1:]
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_text(content)
                (root / "index.md").write_text("reserved")
                (root / "log.md").write_text("reserved")
                (root / ".assets").mkdir()
                (root / ".assets" / "bad.md").write_text("reserved")
                if tree == "symlink-file":
                    (root / "linked.md").symlink_to(root / "a.md")
                if tree == "symlink-dir":
                    (root / "linked").symlink_to(root / "nested", target_is_directory=True)
                if tree == "directory-md":
                    (root / "folder.md").mkdir()
                if tree == "invalid-concept":
                    (root / "bad.md").write_text("invalid")
                include = (
                    (lambda path: path not in {"/linked.md", "/bad.md", "/folder.md", "/hidden.md"})
                    if filtered
                    else None
                )
                include_dir = (lambda path: path != "/nested/deep/") if filtered else None
                case: dict[str, Any] = {"tree": tree, "filtered": filtered}
                try:
                    case["paths"] = module.list_bundle_paths(
                        root, include_path=include, include_directory=include_dir
                    )
                    case["scan"] = [
                        entry.bundle_path
                        for entry in module.scan_bundle(
                            root, include_path=include, include_directory=include_dir
                        ).entries
                    ]
                except Exception as exc:
                    case["error"] = str(exc)
                try:
                    audit = module.audit_repository(root, include_path=include)
                    case["audit"] = asdict(audit)
                    case["ok"] = audit.ok
                except Exception as exc:
                    case["audit_error"] = str(exc)
                scans.append(case)
    return {"generated": generated, "files": files, "scans": scans}
