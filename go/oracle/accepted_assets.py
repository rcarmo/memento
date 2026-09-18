"""Accepted asset metadata/version helpers against disposable synthetic trees."""

from __future__ import annotations

import base64
import importlib
import tempfile
from datetime import datetime
from pathlib import Path
from typing import Any

from asset_pack import archive


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.repository.asset_packs")
    packs: Any = importlib.import_module("memento.skill_packs")
    raw = archive([("readme.md", b"# Synthetic\n", 0o100644)])
    manifest = packs.validate_asset_pack(asset_kind="docs", version="1.0.0", zip_bytes=raw).manifest
    concept = "12345678-abcd"
    cases = []
    with tempfile.TemporaryDirectory(prefix="memento-accepted-oracle-") as directory:
        root = Path(directory)
        for version, created in [
            ("1.9.0", None),
            ("1.10.0", "2026-09-18T01:02:03.123456+01:00"),
            ("2.0.0", "2026-09-18T00:00:00Z"),
        ]:
            paths = module.write_asset_version(
                root,
                concept_id=concept,
                concept_path="/public/café.md",
                asset_kind="docs",
                version=version,
                zip_bytes=raw,
                manifest=manifest,
                accepted_by="synthetic-agent",
                source_proposal_id="proposal",
                created_at=datetime.fromisoformat(created) if created else None,
            )
            cases.append(
                {
                    "version": version,
                    "created": created,
                    "paths": paths,
                    "metadata": (root / paths[0][1:]).read_text(),
                    "zip": base64.b64encode((root / paths[1][1:]).read_bytes()).decode(),
                }
            )
        versions = module.list_asset_versions(root, concept, "docs")
        resolved = []
        for requested_version in [None, "1.9.0", "3.0.0", "bad"]:
            item: dict[str, Any] = {"version": requested_version}
            try:
                item["expected"] = module.resolve_asset_version(
                    root, concept, "docs", requested_version
                )
            except Exception as exc:
                item["error"] = str(exc)
            resolved.append(item)
        kind_list = module.list_asset_kinds(root, concept)
    paths = []
    for cid, kind, version in [
        (concept, "docs", "1.0.0"),
        ("A" * 64, "a1", "0.0.0"),
        ("--------", "skill", "1.0.0"),
        ("bad", "asset", "1.0.0"),
        (concept, "INVALID", "1.0.0"),
        (concept, "asset", "bad"),
        ("/../outside", "asset", "1.0.0"),
    ]:
        item = {"concept": cid, "kind": kind, "version": version}
        try:
            item["expected"] = module.asset_version_paths(cid, kind, version)
        except Exception as exc:
            item["error"] = str(exc)
        paths.append(item)
    retention = []
    for values, keep in [
        ([], 5),
        (["1.9.0", "1.10.0", "2.0.0", "1.9.0"], 2),
        (["1.0.0"], 0),
        (["bad"], 1),
        (["12345678901234567890.0.0", "9.0.0"], 1),
    ]:
        item = {"versions": values, "keep": keep}
        try:
            item["expected"] = module.retention_partition(tuple(values), keep=keep)
        except Exception as exc:
            item["error"] = str(exc)
        retention.append(item)
    return {
        "concept": concept,
        "manifest": manifest.model_dump(mode="json"),
        "zip": base64.b64encode(raw).decode(),
        "writes": cases,
        "versions": versions,
        "kinds": kind_list,
        "resolved": resolved,
        "paths": paths,
        "retention": retention,
    }
