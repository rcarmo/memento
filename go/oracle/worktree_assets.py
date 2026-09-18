"""Capture accepted asset application without strengthening source checks."""

from __future__ import annotations

import base64
import importlib
import tempfile
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    schema: Any = importlib.import_module("memento.repository.schema")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    authz: Any = importlib.import_module("memento.authz")
    stamp = datetime(2026, 9, 18, tzinfo=UTC)
    text = str(
        frontmatter.serialize_concept(
            schema.ConceptDocument(
                frontmatter=schema.ConceptFrontmatter(
                    id="12345678",
                    type="concept",
                    title="Title",
                    status="active",
                    created_at=stamp,
                    updated_at=stamp,
                    updated_by="original",
                ),
                body="original",
            )
        )
    )
    cases = []
    for scenario in [
        "valid",
        "unchecked-bytes",
        "digest",
        "manifest-digest",
        "bad-manifest",
        "missing-asset",
        "missing-concept",
        "no-proposal",
        "duplicate",
        "adapt",
        "adapt-unrelated",
        "adapt-missing",
        "invalid-version",
        "invalid-kind",
    ]:
        with tempfile.TemporaryDirectory(prefix="memento-asset-apply-") as directory:
            root = Path(directory) / "worktree"
            root.mkdir()
            (root / "a.md").write_text(text)
            if scenario == "missing-concept":
                (root / "a.md").unlink()
            connection = db.connect_control_db(Path(directory) / "control.sqlite")
            db.migrate_control_db(connection)
            manifest: dict[str, Any] = {
                "entries": [],
                "sha256": "a" * 64,
                "total_uncompressed_bytes": 0,
                "file_count": 0,
            }
            blob = b"not-a-zip\x00\xff"
            proposals.create_proposal(
                connection,
                proposal_id="proposal",
                author_principal="author",
                client_instance_id=None,
                base_revision="base",
                intent="synthetic",
                rationale=None,
                patch={"changes": []},
                assets=[
                    proposals.ProposalAssetInput(
                        asset_id="asset",
                        concept_path="/different.md",
                        asset_kind="different",
                        version="9.0.0",
                        media_type="application/zip",
                        sha256="a" * 64,
                        blob_bytes=blob,
                        manifest_json="{}",
                    )
                ],
            )
            change = {
                "kind": "attach_asset_pack",
                "path": "/a.md",
                "asset_kind": "docs",
                "version": "1.0.0",
                "asset_id": "asset",
                "zip_sha256": "a" * 64,
                "manifest": manifest,
            }
            if scenario == "digest":
                change["zip_sha256"] = "b" * 64
            elif scenario == "manifest-digest":
                manifest["sha256"] = "b" * 64
            elif scenario == "bad-manifest":
                manifest["file_count"] = -1
            elif scenario == "missing-asset":
                change["asset_id"] = "missing"
            elif scenario == "invalid-version":
                change["version"] = "latest"
            elif scenario == "invalid-kind":
                change["asset_kind"] = "../bad"
            runtime = object.__new__(service.MemoryService)
            runtime._deps = SimpleNamespace(
                control_connection=connection,
                repo_paths=SimpleNamespace(current_dir=root),
                config=SimpleNamespace(limits=SimpleNamespace(max_concept_bytes=65536)),
            )
            runtime._now = lambda: stamp
            policy = authz.EffectivePolicy("actor", ("curator",), ("/",), ("/",), ())
            changes: list[dict[str, Any]] = [change]
            if scenario.startswith("adapt"):
                changes.insert(
                    0,
                    {
                        "kind": "create",
                        "path": "/other.md" if scenario == "adapt-unrelated" else "/a.md",
                        "concept_type": "concept",
                        "title": "Changed",
                        "body": "new",
                        "tags": ["tag"],
                    },
                )
                if scenario == "adapt-missing":
                    (root / "a.md").unlink()
            initial = {"/" + file.name: file.read_text() for file in root.glob("*.md")}
            item: dict[str, Any] = {
                "scenario": scenario,
                "initial": initial,
                "changes": changes,
                "blob_base64": base64.b64encode(blob).decode(),
            }
            try:
                normalized = runtime._normalize_changes(changes)
                adapted = runtime._adapt_existing_asset_concepts(normalized)
                item["adapted"] = [change.model_dump(mode="json") for change in adapted]
                if scenario.startswith("adapt") and scenario != "adapt":
                    item["changed"] = []  # Only classify adaptation, avoiding random new IDs.
                else:
                    item["changed"] = runtime._apply_changes(
                        root,
                        adapted,
                        actor="actor",
                        policy=policy,
                        proposal_id=None if scenario == "no-proposal" else "proposal",
                    )
                    if scenario == "duplicate":
                        runtime._apply_changes(
                            root, adapted, actor="actor", policy=policy, proposal_id="proposal"
                        )
            except Exception as exc:
                item["error_type"] = type(exc).__name__
                item["error"] = str(exc)
            item["files"] = {
                "/" + file.relative_to(root).as_posix(): base64.b64encode(
                    file.read_bytes()
                ).decode()
                for file in sorted(root.rglob("*"))
                if file.is_file()
            }
            cases.append(item)
            connection.close()
    return cases
