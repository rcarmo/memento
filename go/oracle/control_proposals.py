"""Proposal/asset record and list-query parity; service ACL/review logic is separate."""

from __future__ import annotations

import base64
import importlib
import tempfile
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from typing import Any
from unittest.mock import patch


class Clock(datetime):
    @classmethod
    def now(cls, tz: Any = None) -> Clock:
        return cls(2026, 9, 18, tzinfo=UTC)


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.control.proposals")
    db: Any = importlib.import_module("memento.control.db")
    with tempfile.TemporaryDirectory(prefix="memento-proposal-oracle-") as directory:
        connection = db.connect_control_db(Path(directory) / "control.sqlite")
        db.migrate_control_db(connection)
        created = []
        with patch.object(module, "datetime", Clock):
            for i, status in enumerate(module.ProposalStatus):
                pid = f"proposal-{i}"
                asset = module.ProposalAssetInput(
                    asset_id="asset-one",
                    concept_path="/public/a.md",
                    asset_kind="skill",
                    version="1.0",
                    media_type="application/zip",
                    sha256="synthetic",
                    blob_bytes=b"synthetic\x00\xff",
                    manifest_json='{ "name": "日本" }',
                )
                record = module.create_proposal(
                    connection,
                    proposal_id=pid,
                    author_principal="alice" if i % 2 == 0 else "bob",
                    client_instance_id=None,
                    base_revision="base",
                    intent="synthetic",
                    rationale="because",
                    patch={"changes": [], "z": "日本", "n": 1.0},
                    expires_in_days=-1 if i % 3 == 0 else 30,
                    assets=(asset,),
                )
                record = module.update_proposal_status(
                    connection, pid, status=status, reviewed_by="reviewer", review_comment="comment"
                )
                created.append(asdict(record))
            # Omitted values clear prior review fields; FK-protected apply links
            # are tested separately with an actual operation row in Go.
            cleared = module.update_proposal_status(
                connection, "proposal-0", status=module.ProposalStatus.SUBMITTED
            )
            queries = []
            for status in [None, *module.ProposalStatus]:
                for unresolved in [False, True]:
                    for effective in [False, True]:
                        kwargs = {"status": status, "unresolved": unresolved}
                        if effective:
                            kwargs.update(current_revision="changed", now="2026-10-19T00:00:00Z")
                        queries.append(
                            {
                                "status": status,
                                "unresolved": unresolved,
                                "effective": effective,
                                "expected": [
                                    asdict(r) for r in module.list_proposals(connection, **kwargs)
                                ],
                            }
                        )
            extra_queries: list[dict[str, Any]] = [
                {"author_principal": "alice"},
                {"cursor": "proposal-3", "limit": 2},
                {"cursor": "missing"},
                {"limit": 0},
                {"status": module.ProposalStatus.EXPIRED, "now": "2026-10-19T00:00:00Z"},
            ]
            for kwargs in extra_queries:
                item: dict[str, Any] = {"input": kwargs}
                try:
                    item["expected"] = [
                        asdict(r) for r in module.list_proposals(connection, **kwargs)
                    ]
                except ValueError as exc:
                    item["error"] = str(exc)
                queries.append(item)
            assets = []
            for record in module.list_proposal_assets(connection):
                item = asdict(record)
                item["blob_bytes"] = base64.b64encode(item["blob_bytes"]).decode()
                assets.append(item)
        connection.close()
    return {"created": created, "cleared": asdict(cleared), "queries": queries, "assets": assets}
