"""Bounded manifest comparisons and Python timestamp/asset-summary helpers."""

from __future__ import annotations

import base64
import hashlib
import importlib
import tempfile
from dataclasses import asdict
from pathlib import Path
from types import SimpleNamespace
from typing import Any
from unittest.mock import patch

from asset_get import fixtures as asset_fixtures


def timestamp_fixtures() -> list[dict[str, Any]]:
    service: Any = importlib.import_module("memento.service")
    values: list[Any] = [
        None,
        0,
        True,
        "",
        "bad",
        "2026-01-01",
        "2026-01-01T12:00:00",
        "2026-01-01Z",
        "2026-01-01T00:00:00Z",
        "2026-01-01T01:30:00+01:30",
        "2026-01-01 00:00:00.123456789Z",
        "20260101T123456+0130",
        "2026-01-01x12:34Z",
        "2026-01-01🍀12:34:56,123Z",
        "2026-01-01T12Z",
        "2026-01-01T12.25Z",
        "2026-01-01T12:34.25Z",
        "2026-01-01T00:00:00+00:00:30.500000",
        "2026-01-01T00:00:00+01:90",
        "2026-01-01T00:00:00-02",
        "2026-01-01T24:00Z",
        "2026-01-01T00:60Z",
        "2026-01-01T00:00:60Z",
        "2026-02-30T00:00Z",
        "0000-01-01T00:00Z",
        "2026-01-01T00:00+24:00",
        "2026-01-01T00:00+bad",
        "2026-01-01T1234:56Z",
        "2026-0101T00:00Z",
        "202601-01T00:00Z",
        "2026-01-01T00:00:00z",
        "2026-01-01T00:00:00+000000.25",
        "2026-01-01T00:00:00+01:02:03.1234567",
    ]
    for date in [
        "2026-W01-1",
        "2026W011",
        "2026-W01",
        "2026W01",
        "2026-W53-7",
        "2025-W53-1",
        "2026-W00-1",
        "2026-W01-0",
        "2026-W011",
        "2026W01-1",
    ]:
        values.append(date + "T00:00:00Z")
    for clock in [
        "12",
        "1234",
        "12:34",
        "123456",
        "12:34:56",
        "12.25",
        "12:34.25",
        "12:34:56.1234567",
        "",
        "12:3456",
        "1234:56",
    ]:
        values.append("2026-01-01T" + clock + "Z")
    values += [
        "2026-01-01\n12:34:56Z",
        "2026-01-01T",
        "9999-12-31T23:59:59-01",
        "0001-01-01T00:00+01",
        "2026-01-01T12:00+00:99",
        "2026-01-01T12:00+00:00:99",
        "2026-01-01T12:00+00:00:00.5",
    ]
    cases = []
    for value in values:
        try:
            stamp = service.MemoryService._manifest_timestamp(value)
            result = {"expected": service.MemoryService._format_timestamp(stamp)}
        except Exception as exc:
            result = {"error": str(exc)}
        cases.append({"value": value, **result})
    return cases


def fixtures() -> dict[str, Any]:
    service: Any = importlib.import_module("memento.service")
    authz: Any = importlib.import_module("memento.authz")
    files = asset_fixtures()["files"]
    base: dict[str, Any] = {
        "name": "a",
        "local_path": "local/a.md",
        "memento_path": "/public/a.md",
        "local_updated_at": "2026-01-01T00:00:00Z",
        "local_body_sha256": hashlib.sha256(b"synthetic").hexdigest(),
        "local_bytes": 9,
    }
    policies = [
        authz.EffectivePolicy("reader", ("reader",), ("/",), (), ("/private/",)),
        authz.EffectivePolicy("proposer", ("proposer",), ("/",), (), ()),
        authz.EffectivePolicy("reader", ("reader",), ("/else/",), (), ()),
    ]
    commands: list[dict[str, Any]] = [
        {},
        {"include_asset_metadata": True},
        {"items": [{**base, "local_bytes": 999}]},
        {"items": [{**base, "local_body_sha256": base["local_body_sha256"].upper()}]},
    ]
    for stamp in ["2027-01-01T00:00:00Z", "2025-01-01T00:00:00Z", "2026-01-01T01:00:00+01:00"]:
        commands.append(
            {"items": [{**base, "local_body_sha256": "0" * 64, "local_updated_at": stamp}]}
        )
    commands += [
        {"items": [{**base, "memento_path": "/public/local.md"}]},
        {"items": [{**base, "local_bytes": 10**50}]},
        {"path_prefix": "bad"},
        {"items": []},
        {"items": [base] * 51},
        {"items": [base, base]},
        {"items": [base, {**base, "name": "b"}]},
        {"match": {"unknown": 1, "aaa": 2}},
        {"match": {"path_template": 1}},
        {"match": {"path_template": "x" * 1025}},
        {"match": {"path_template": "/{name}/{name}"}},
        {"match": {"path_template": "/{other}"}},
        {"match": {"aliases": None}},
        {"match": {"aliases": {"a": 1}}},
        {"match": {"aliases": {"": "a"}}},
        {"match": {"aliases": {str(i): "a" for i in range(51)}}},
    ]
    for field, value in [
        ("unknown", True),
        ("name", ""),
        ("name", "a" * 129),
        ("local_path", None),
        ("local_path", "a" * 513),
        ("local_body_sha256", "bad"),
        ("local_body_sha256", None),
        ("local_bytes", True),
        ("local_bytes", -1),
        ("local_bytes", 1.0),
        ("local_updated_at", None),
        ("local_updated_at", "bad"),
        ("local_updated_at", "2026-01-01"),
        ("memento_path", 1),
        ("memento_path", "/private/p.md"),
        ("memento_path", "/else/a.md"),
        ("memento_path", "/public/../a.md"),
        ("memento_path", "/public//a.md"),
        ("memento_path", "/public/index.md"),
    ]:
        commands.append({"items": [{**base, field: value}]})
    no_target = {k: v for k, v in base.items() if k != "memento_path"}
    commands += [
        {"items": [no_target]},
        {
            "items": [no_target],
            "match": {"path_template": "/public/{name}.md", "aliases": {"a": "empty"}},
        },
        {"items": [no_target], "match": {"path_template": "/public/{name}.md"}},
        {
            "items": [{**base, "name": "b", "memento_path": "/public/local.md"}, base],
            "include_asset_metadata": True,
        },
    ]
    cases = []
    with tempfile.TemporaryDirectory(prefix="memento-manifest-") as directory:
        root = Path(directory)
        for path, encoded in files.items():
            target = root / path[1:]
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(base64.b64decode(encoded))
        runtime = object.__new__(service.MemoryService)
        runtime._deps = SimpleNamespace(repo_paths=SimpleNamespace(current_dir=root))

        def run(args: dict[str, Any], policy: Any, extra: int = 0) -> None:
            runtime._policy = lambda _: policy
            with patch.object(service, "get_main_revision", lambda _: "main"):
                result = runtime.memory_compare_manifest(None, **args).model_dump(mode="json")
            cases.append(
                {"arguments": args, "policy": asdict(policy), "extra": extra, "expected": result}
            )

        for command in commands:
            run({"path_prefix": "/public/", "items": [base], **command}, policies[0])
        for policy in policies[1:]:
            run({"path_prefix": "/public/", "items": []}, policy)
        for extra in [48, 49, 50]:
            for i in range(extra):
                (root / f"public/z{i:03}.md").write_bytes(
                    base64.b64decode(files["/public/empty.md"])
                )
            run({"path_prefix": "/public/", "items": [base]}, policies[0], extra)
            if extra == 48:
                run(
                    {
                        "path_prefix": "/public/",
                        "items": [{**base, "memento_path": "/public/new.md"}],
                    },
                    policies[0],
                    extra,
                )
            for i in range(extra):
                (root / f"public/z{i:03}.md").unlink()
    summaries = []
    for value in [
        None,
        {},
        [],
        [{}],
        [1],
        [{"kind": "docs", "latest_version": "1.0.0", "latest_sha256": "x"}] * 21,
        [{"kind": "docs", "latest_version": 1, "latest_sha256": "x"}],
    ]:
        summaries.append(
            {"value": value, "expected": service.MemoryService._manifest_asset_summary(value)}
        )
    return {"files": files, "cases": cases, "summaries": summaries}
