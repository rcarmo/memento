"""Models-off status uses real derived/control stores, queue refresh and policy."""

from __future__ import annotations

import base64
import importlib
import tempfile
import threading
from dataclasses import asdict
from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch

from asset_get import fixtures as asset_fixtures


def defaults() -> dict[str, Any]:
    config: Any = importlib.import_module("memento.config")
    package: Any = importlib.import_module("memento")
    return {
        "service_version": package.__version__,
        "schema_version": 2,
        "limits": config.LimitsConfig().model_dump(mode="json"),
        "needle_model_path": config.NeedleRouterConfig().model_path,
    }


def fixtures() -> dict[str, Any]:
    service: Any = importlib.import_module("memento.service")
    config: Any = importlib.import_module("memento.config")
    authz: Any = importlib.import_module("memento.authz")
    db: Any = importlib.import_module("memento.control.db")
    proposals: Any = importlib.import_module("memento.control.proposals")
    transactions: Any = importlib.import_module("memento.repository.transactions")
    derived: Any = importlib.import_module("memento.derived.index")
    files = {k: v for k, v in asset_fixtures()["files"].items() if not k.startswith("/.assets/")}
    files["/trash/public/t.md"] = base64.b64encode(
        base64.b64decode(files["/public/empty.md"]).replace(b"eeeeeeee", b"dddddddd")
    ).decode()
    policies = [
        authz.EffectivePolicy("actor", ("reader",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("curator", ("curator",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("admin", ("admin",), ("/private/",), (), ("/private/",)),
        authz.EffectivePolicy("empty", (), ("/else/",), (), ()),
    ]
    cases = []
    for scenario in [
        "ready",
        "stale",
        "rebuilding",
        "empty",
        "quarantined",
        "hidden-malformed",
        "visible-malformed-ready",
        "visible-malformed-fallback",
        "queue-expiry",
    ]:
        for policy in policies:
            with tempfile.TemporaryDirectory(prefix="memento-status-") as directory:
                root = Path(directory) / "current"
                root.mkdir()
                for path, encoded in files.items():
                    target = root / path[1:]
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(base64.b64decode(encoded))
                index = derived.DerivedIndex(Path(directory) / "derived.sqlite")
                index.rebuild(root, repo_revision="main")
                if scenario in {
                    "stale",
                    "rebuilding",
                    "quarantined",
                    "empty",
                    "visible-malformed-fallback",
                }:
                    with index._connect() as connection:
                        index._set_state(
                            connection, "index_revision", "" if scenario == "empty" else "old"
                        )
                        index._set_state(
                            connection,
                            "status",
                            "rebuilding"
                            if scenario in {"rebuilding", "visible-malformed-fallback"}
                            else "quarantined"
                            if scenario == "quarantined"
                            else "ready",
                        )
                mutation = None
                if scenario in {
                    "hidden-malformed",
                    "visible-malformed-ready",
                    "visible-malformed-fallback",
                }:
                    path = "/private/bad.md" if scenario == "hidden-malformed" else "/public/bad.md"
                    (root / path[1:]).write_text("not a concept")
                    mutation = {"path": path, "text": "not a concept"}
                connection = db.connect_control_db(Path(directory) / "control.sqlite")
                db.migrate_control_db(connection)
                for id, author, path, status in [
                    ("a", "actor", "/public/a.md", "submitted"),
                    ("b", "other", "/public/empty.md", "approved"),
                    ("hidden", "actor", "/private/p.md", "draft"),
                    ("rejected", "actor", "/public/a.md", "rejected"),
                    ("expired", "actor", "/public/a.md", "expired"),
                ]:
                    proposals.create_proposal(
                        connection,
                        proposal_id=id,
                        author_principal=author,
                        client_instance_id=None,
                        base_revision="main",
                        intent="synthetic",
                        rationale=None,
                        patch={"changes": [{"kind": "patch", "path": path, "body": "new"}]},
                    )
                    connection.execute(
                        "UPDATE proposals SET status=?,created_at='2026-09-18T00:00:00Z',updated_at='2026-09-18T00:00:00Z',expires_at=? WHERE proposal_id=?",
                        (
                            status,
                            "2000-01-01T00:00:00Z"
                            if scenario == "queue-expiry"
                            else "2099-01-01T00:00:00Z",
                            id,
                        ),
                    )
                connection.commit()
                before = [asdict(p) for p in proposals.list_proposals(connection)]
                state = asdict(index.get_state())
                snapshot = asdict(index.status_snapshot(policy))
                runtime = object.__new__(service.MemoryService)
                runtime._deps = SimpleNamespace(
                    repo_paths=SimpleNamespace(current_dir=root),
                    control_connection=connection,
                    derived_index=index,
                    config=SimpleNamespace(
                        schema_version=2,
                        limits=config.LimitsConfig(),
                        intelligent_tiers=SimpleNamespace(
                            model_proposals=SimpleNamespace(enabled=False),
                            dream=SimpleNamespace(mode="disabled"),
                            needle_router=config.NeedleRouterConfig(),
                        ),
                    ),
                    needle_router=None,
                )
                runtime._policy = lambda _, policy=policy: policy
                runtime._now = lambda: datetime(2026, 9, 18, tzinfo=UTC)
                with (
                    patch.object(service, "get_main_revision", lambda _: "main"),
                    patch.object(transactions, "_transaction_lock", lambda _: threading.RLock()),
                    patch.object(proposals, "utcnow", lambda: datetime(2026, 9, 18, tzinfo=UTC)),
                ):
                    result = runtime.memory_status(None).model_dump(mode="json")
                after = [asdict(p) for p in proposals.list_proposals(connection)]
                events = [
                    dict(r)
                    for r in connection.execute("SELECT * FROM proposal_events ORDER BY event_id")
                ]
                cases.append(
                    {
                        "scenario": scenario,
                        "policy": asdict(policy),
                        "state": state,
                        "snapshot": snapshot,
                        "mutation": mutation,
                        "before": before,
                        "after": after,
                        "events": events,
                        "expected": result,
                    }
                )
                connection.close()
    return {"files": files, "defaults": defaults(), "cases": cases}
