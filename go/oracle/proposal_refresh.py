"""Exercise source queue-conflict/refresh methods with synthetic Git callbacks."""

from __future__ import annotations

import importlib
import tempfile
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch


def fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    git: Any = importlib.import_module("memento.repository.git")
    changes = [
        {"kind": "patch", "path": "/a.md"},
        {"kind": "rename", "path": "/b.md", "new_path": "/c.md"},
        {"kind": "trash", "path": "/d.md"},
        {"kind": "attach_asset_pack", "path": "/asset.md"},
        {"kind": "patch", "path": "/asset.md"},
    ]
    cases = []

    def missing_read(*args: Any) -> Any:
        raise FileNotFoundError("synthetic")

    def asset_read(*args: Any) -> Any:
        return SimpleNamespace(document=SimpleNamespace(frontmatter=SimpleNamespace(id="12345678")))

    def missing_base(*args: Any, **kwargs: Any) -> Any:
        raise git.GitError("synthetic historical base unavailable")

    for status in proposals.ProposalStatus:
        for scenario in [
            "same",
            "clean",
            "conflict",
            "asset",
            "missing-base",
            "expired",
            "equal-expiry",
        ]:
            with tempfile.TemporaryDirectory(prefix="memento-refresh-oracle-") as directory:
                connection = db.connect_control_db(Path(directory) / "control.sqlite")
                db.migrate_control_db(connection)
                record = proposals.create_proposal(
                    connection,
                    proposal_id="proposal",
                    author_principal="proposer",
                    client_instance_id="client",
                    base_revision="main" if scenario == "same" else "base",
                    intent="synthetic",
                    rationale="retain",
                    patch={"changes": changes},
                )
                expires = (
                    "2026-09-17T00:00:00Z"
                    if scenario == "expired"
                    else "2026-09-18T00:00:00Z"
                    if scenario == "equal-expiry"
                    else "2099-01-01T00:00:00Z"
                )
                connection.execute(
                    "UPDATE proposals SET status=?,created_at='created',updated_at='updated',expires_at=?,reviewed_by='reviewer',review_comment='keep'",
                    (status, expires),
                )
                connection.commit()
                record = proposals.get_proposal(connection, "proposal")
                runtime = object.__new__(service.MemoryService)
                runtime._deps = SimpleNamespace(
                    control_connection=connection,
                    repo_paths=SimpleNamespace(current_dir=Path(directory)),
                )
                runtime._now = lambda: datetime(2026, 9, 18, tzinfo=UTC)
                changed = (
                    ["/a.md", "/c.md", "/trash/d.md"]
                    if scenario == "conflict"
                    else ["/.assets/12345678/docs/1.0.0.json"]
                    if scenario == "asset"
                    else []
                )

                def diff(*args: Any, changed: list[str] = changed, **kwargs: Any) -> Any:
                    return tuple(changed)

                with (
                    patch.object(service, "get_main_revision", lambda _: "main"),
                    patch.object(
                        service,
                        "diff_main_paths",
                        missing_base if scenario == "missing-base" else diff,
                    ),
                    patch.object(
                        service,
                        "read_bundle_entry",
                        asset_read if scenario == "asset" else missing_read,
                    ),
                ):
                    conflicts = runtime._proposal_conflicts(record)
                    refreshed = runtime._refresh_proposal_status(record)
                    # A second refresh must not create duplicate history.
                    runtime._refresh_proposal_status(refreshed)
                events = [
                    dict(row)
                    for row in connection.execute("SELECT * FROM proposal_events ORDER BY event_id")
                ]
                cases.append(
                    {
                        "status": status,
                        "scenario": scenario,
                        "changed": changed,
                        "before": asdict(record),
                        "conflicts": conflicts,
                        "after": asdict(refreshed),
                        "events": events,
                    }
                )
                connection.close()
    return cases
