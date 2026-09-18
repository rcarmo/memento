"""Writable audit scope, repository issues and graph adapter/cursor contracts."""

from __future__ import annotations

import base64
import importlib
import json
import sqlite3
import tempfile
from dataclasses import asdict
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch

from asset_get import fixtures as asset_fixtures


def fixtures() -> dict[str, Any]:
    service: Any = importlib.import_module("memento.service")
    authz: Any = importlib.import_module("memento.authz")
    models: Any = importlib.import_module("memento.graph_debug.models")
    files = {k: v for k, v in asset_fixtures()["files"].items() if not k.startswith("/.assets/")}
    files["/public/b.md"] = files["/public/a.md"]
    files["/public/a.md"] = base64.b64encode(
        base64.b64decode(files["/public/a.md"]).replace(
            b"synthetic", b"[missing](/public/missing.md) [secret](/private/missing.md)"
        )
    ).decode()
    diagnostics = [
        models.GraphDiagnostic(
            id=id,
            rule=rule,
            severity=severity,
            concept_ids=ids,
            message="synthetic",
            measured={"count": 1},
        )
        for id, rule, severity, ids in [
            ("z", "broken_links", "warning", ("a", "b", "a", "missing")),
            ("a", "pending_proposals", "info", ("b",)),
            ("e", "embedding_health", "error", ()),
            ("x", "orphans", "warning", ("a",)),
            ("unknown", "orphans", "info", ("missing",)),
        ]
    ]
    nodes = [{"id": "a", "path": "/public/a.md"}, {"id": "b", "path": "/public/b.md"}]
    policies = [
        authz.EffectivePolicy("actor", ("proposer",), ("/",), ("/public/",), ("/private/",)),
        authz.EffectivePolicy("actor", ("reader",), ("/",), ("/",), ()),
        authz.EffectivePolicy("actor", ("proposer",), ("/",), (), ()),
        authz.EffectivePolicy("actor", ("proposer",), ("/public/",), ("/",), ()),
        authz.EffectivePolicy(
            "actor", ("proposer",), ("/private/",), ("/private/",), ("/private/",)
        ),
    ]
    cases = []
    with tempfile.TemporaryDirectory(prefix="memento-audit-") as directory:
        root = Path(directory)
        for path, encoded in files.items():
            target = root / path[1:]
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(base64.b64decode(encoded))
        runtime = object.__new__(service.MemoryService)
        runtime._deps = SimpleNamespace(
            repo_paths=SimpleNamespace(current_dir=root), graph_snapshot_service=None
        )
        commands: list[dict[str, Any]] = [
            {},
            {"path": "/public/a.md"},
            {"path": "/private/p.md"},
            {"rule": "embedding_health"},
            {"severity": "warning"},
            {"rule": "bad"},
            {"severity": "bad"},
            {"limit": 1},
            {"limit": 0},
            {"limit": 201},
            {"cursor": "bad"},
        ]
        page_filters = {"path": None, "rule": None, "severity": None}
        page_cursor = service.MemoryService._encode_audit_cursor(
            ("info", "pending_proposals", "a"), repo_revision="main", filters=page_filters
        )
        end_cursor = service.MemoryService._encode_audit_cursor(
            ("warning", "orphans", "x"), repo_revision="main", filters=page_filters
        )
        commands.extend(
            [
                {"cursor": page_cursor, "limit": 1},
                {"cursor": end_cursor},
                {"cursor": page_cursor, "rule": "broken_links"},
            ]
        )
        for mode in [
            "none",
            "ready",
            "stale",
            "mismatch",
            "snapshot-stale",
            "snapshot-unavailable",
            "oserror",
            "sqlite",
        ]:
            for policy in policies:
                for args in commands:
                    calls: list[dict[str, Any]] = []

                    def overview(*, policy: Any, mode: str = mode, calls: Any = calls) -> Any:
                        calls.append(asdict(policy))
                        if mode.startswith("snapshot"):
                            raise service.GraphSnapshotError(
                                "STALE" if mode == "snapshot-stale" else "missing"
                            )
                        if mode == "oserror":
                            raise OSError("synthetic")
                        if mode == "sqlite":
                            raise sqlite3.OperationalError("synthetic")
                        return SimpleNamespace(
                            revisions=SimpleNamespace(
                                stale=mode == "stale", index="old" if mode == "mismatch" else "main"
                            ),
                            nodes=[SimpleNamespace(**n) for n in nodes],
                            diagnostics=diagnostics,
                        )

                    runtime._deps.graph_snapshot_service = (
                        None if mode == "none" else SimpleNamespace(overview=overview)
                    )
                    runtime._policy = lambda _, policy=policy: policy
                    with patch.object(service, "get_main_revision", lambda _: "main"):
                        response = runtime.memory_audit(None, **args).model_dump(mode="json")
                    cases.append(
                        {
                            "mode": mode,
                            "arguments": args,
                            "policy": asdict(policy),
                            "expected": response,
                            "calls": calls,
                        }
                    )
    policy_cases = []
    for reads, writes, protected in [
        (("/public/",), ("/",), ()),
        (("/",), ("/private/",), ("/private/",)),
        (("/private/",), ("/private/",), ("/private/",)),
        (("/public/sub/",), ("/public/",), ()),
        (("/else/",), ("/public/",), ()),
        (("/trash/public/",), ("/public/",), ()),
        (("/",), ("/public/", "/public/sub/"), ()),
        (("/",), ("/unsafe/../",), ()),
    ]:
        policy = authz.EffectivePolicy("actor", ("proposer",), reads, writes, protected)
        policy_cases.append(
            {
                "policy": asdict(policy),
                "expected": asdict(service.MemoryService._writable_audit_policy(policy)),
            }
        )
    filters = {"path": None, "rule": None, "severity": None}
    key = ("info", "pending_proposals", "a")
    token = service.MemoryService._encode_audit_cursor(key, repo_revision="main", filters=filters)
    cursors = []
    values = [None, token, "bad", token + "=", token + "====", token[:4] + "!" * 4 + token[4:]]
    payloads: list[Any] = [
        None,
        [],
        {},
        {"v": 2, "revision": "main", "filters": filters, "after": key},
        {"v": True, "revision": "main", "filters": filters, "after": key},
        {"v": 1.0, "revision": "main", "filters": filters, "after": key},
        {"v": 1, "revision": "old", "filters": filters, "after": key},
        {"v": 1, "revision": "main", "filters": {}, "after": key},
        {"v": 1, "revision": "main", "filters": filters, "after": [1, 2, 3]},
    ]
    for value in payloads:
        values.append(base64.urlsafe_b64encode(json.dumps(value).encode()).decode().rstrip("="))
    # JSON floats compare after binary rounding, unlike arbitrary rationals.
    for raw in [
        '{"v":1.00000000000000001,"revision":"main","filters":{"path":null,"rule":null,"severity":null},"after":["info","pending_proposals","a"]}',
        '{"v":1e99999999,"revision":"main","filters":{"path":null,"rule":null,"severity":null},"after":["info","pending_proposals","a"]}',
        '{"v":1,"revision":"main","filters":{"path":null,"rule":null,"severity":null},"after":[]}',
        '{"v":1e0,"revision":"main","filters":{"path":null,"rule":null,"severity":null},"after":["info","pending_proposals","a"]}',
    ]:
        values.append(base64.urlsafe_b64encode(raw.encode()).decode().rstrip("="))
    for value in values:
        try:
            result = {
                "expected": service.MemoryService._decode_audit_cursor(
                    value, repo_revision="main", filters=filters
                )
            }
        except Exception as exc:
            result = {"error": str(exc)}
        cursors.append({"cursor": value, **result})
    return {
        "files": files,
        "nodes": nodes,
        "diagnostics": [d.model_dump(mode="json") for d in diagnostics],
        "cases": cases,
        "policies": policy_cases,
        "cursor": token,
        "cursors": cursors,
    }
