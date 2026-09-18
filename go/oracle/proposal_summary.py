"""Capture bounded summaries and concept-body/asset matching without previews."""

from __future__ import annotations

import hashlib
import importlib
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    proposals: Any = importlib.import_module("memento.control.proposals")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    cases = []
    for scenario in [
        "empty",
        "rename",
        "missing-fields",
        "nonobject",
        "null-changes",
        "first-body",
        "fallback-body",
        "missing-body",
        "bad-body",
        "no-match",
        "truncated",
        "missing-base",
    ]:
        changes: list[Any] = []
        assets: list[Any] = []
        if scenario == "rename":
            changes = [{"kind": "rename", "path": "/a.md", "new_path": "/b.md"}]
        elif scenario == "missing-fields":
            changes = [{}]
        elif scenario == "nonobject":
            changes = [None, {"kind": "patch", "path": "/a.md"}]
        elif scenario not in {"empty", "null-changes"}:
            changes = [
                {"kind": "patch", "path": "/a.md", "body": "first \r\n"},
                {"kind": "create", "path": "/a.md", "body": "second"},
            ]
            if scenario != "first-body":
                changes = [{"kind": "patch", "path": "/a.md", "body": None}]
            digest = hashlib.sha256(
                b"first" if scenario == "first-body" else b"current"
            ).hexdigest()
            manifest = {
                "file_count": 7,
                "total_uncompressed_bytes": 32,
                "entries": [
                    None,
                    {"path": "z.md", "sha256": digest},
                    {"path": "a.md", "sha256": digest},
                    {"path": "a.md", "sha256": digest},
                    {"path": 1, "sha256": digest},
                    {"path": "other", "sha256": "mismatch"},
                    {},
                ],
            }
            if scenario == "no-match":
                manifest = {}
            assets = [
                SimpleNamespace(
                    asset_id="asset",
                    concept_path="/a.md",
                    asset_kind="docs",
                    version="1.0.0",
                    sha256="zip-sha",
                    manifest=manifest,
                )
            ]
        if scenario == "truncated":
            changes = [{"kind": "patch", "path": f"/{i}.md"} for i in range(201)]
            assets = [
                SimpleNamespace(**{**vars(assets[0]), "asset_id": f"asset-{i}"}) for i in range(201)
            ]
        record = SimpleNamespace(
            proposal_id="proposal",
            author_principal="author",
            intent="😀" * (2001 if scenario == "truncated" else 2000),
            status=proposals.ProposalStatus.APPROVED,
            base_revision="base",
            reviewed_by="reviewer",
            applied_operation_id=None,
            applied_revision=None,
            created_at="created",
            updated_at="updated",
            expires_at=None,
            patch={"changes": None if scenario == "null-changes" else changes},
        )
        runtime = object.__new__(service.MemoryService)
        runtime._deps = SimpleNamespace(
            control_connection=None, repo_paths=SimpleNamespace(current_dir="unused")
        )

        def read(*args: Any, scenario: str = scenario) -> Any:
            if scenario == "missing-body":
                raise FileNotFoundError("missing")
            if scenario == "bad-body":
                raise frontmatter.FrontmatterError("invalid")
            return SimpleNamespace(
                document=SimpleNamespace(
                    body="current \n", frontmatter=SimpleNamespace(id="12345678")
                )
            )

        def diff(*args: Any, scenario: str = scenario, **kwargs: Any) -> tuple[str, ...]:
            if scenario == "missing-base":
                raise service.GitError("missing base")
            return ("/a.md",)

        with (
            patch.object(service, "get_main_revision", lambda _: "main"),
            patch.object(service, "diff_main_paths", diff),
            patch.object(service, "read_bundle_entry", read),
            patch.object(
                service, "list_proposal_assets", lambda *args, assets=assets, **kwargs: assets
            ),
        ):
            for include in [False, True]:
                item: dict[str, Any] = {
                    "scenario": scenario,
                    "include_conflicts": include,
                    "record": {**vars(record), "status": record.status.value},
                    "assets": [vars(asset) for asset in assets],
                }
                try:
                    item["expected"] = runtime._proposal_summary_payload(
                        record, include_conflicts=include
                    )
                except Exception as exc:
                    item["error_type"] = type(exc).__name__
                cases.append(item)
    return cases
